import { defineStore } from "pinia";
import { ref } from "vue";

import { fetchJSON, sendJSON } from "../api/client.js";
import defaultServiceIcon from "../assets/app-icon.svg";

const serviceRefreshInterval = 15_000;

const serviceDescriptions = {
  core: "Shared platform services and coordination",
  upload: "File upload and archival",
  timeline: "Location and personal timeline",
  relay: "Notifications and cross-client transfers",
};

/** Builds the display fields the service cards render from an API service. */
export function normalizeService(service) {
  const isCore = service.service_type === "core";
  const status = service.registration_status !== "registered"
    ? "offline"
    : service.availability_status ?? "unknown";
  const details = [
    { label: "Version", value: service.version },
    { label: "Internal URL", value: service.internal_url },
    { label: "Public URL", value: service.public_url || "Not configured" },
  ];
  if (service.native_apps_url) {
    details.push({ label: "Native URL", value: service.native_apps_url });
  }
  if (service.database_name) {
    details.push({ label: "Database", value: service.database_name });
  }
  if (service.health_checked_at) {
    details.push({
      label: "Health check",
      value: new Date(service.health_checked_at).toLocaleString(),
    });
  }
  if (service.health_error) {
    details.push({ label: "Health issue", value: service.health_error });
  }

  return {
    ...service,
    description: serviceDescriptions[service.service_type] ?? `${service.name} service`,
    iconUrl: isCore
      ? defaultServiceIcon
      : `/api/v1/services/${encodeURIComponent(service.id)}/icon`,
    iconFailed: false,
    status,
    type: "service",
    details,
  };
}

export function serviceStatusLabel(status) {
  if (status === "online") return "Online";
  if (status === "offline") return "Offline";
  return "Checking";
}

/** Owns the registered service list, its background refresh, and its edits. */
export const useServicesStore = defineStore("services", () => {
  const services = ref([]);
  const loading = ref(true);
  const error = ref("");
  const showDetails = ref(false);

  let refreshTimer = null;

  /**
   * Refreshes in place so a background poll cannot discard per-card UI state
   * such as a failed icon load or reset an open configuration dialog.
   */
  function mergeServices(updatedServices) {
    const existingServices = new Map(services.value.map((service) => [service.id, service]));
    services.value = updatedServices.map((updated) => {
      const normalized = normalizeService(updated);
      const existing = existingServices.get(updated.id);
      if (!existing) {
        return normalized;
      }
      const iconFailed = existing.iconFailed;
      Object.assign(existing, normalized, { iconFailed });
      return existing;
    });
  }

  async function load({ background = false } = {}) {
    if (!background) {
      loading.value = true;
      error.value = "";
    }
    try {
      mergeServices(await fetchJSON("/api/v1/services", "Unable to load services."));
      error.value = "";
    } catch (loadError) {
      if (!background) {
        error.value = loadError instanceof Error ? loadError.message : "Unable to load services.";
      }
    } finally {
      if (!background) {
        loading.value = false;
      }
    }
  }

  function refreshWhenVisible() {
    if (document.visibilityState === "visible") {
      load({ background: true });
    }
  }

  function startPolling() {
    if (refreshTimer !== null) return;
    refreshTimer = window.setInterval(refreshWhenVisible, serviceRefreshInterval);
    document.addEventListener("visibilitychange", refreshWhenVisible);
  }

  function stopPolling() {
    if (refreshTimer !== null) {
      window.clearInterval(refreshTimer);
      refreshTimer = null;
    }
    document.removeEventListener("visibilitychange", refreshWhenVisible);
  }

  function getConfiguration(serviceID) {
    return fetchJSON(
      `/api/v1/services/${encodeURIComponent(serviceID)}`,
      "Unable to load service configuration.",
    );
  }

  function updateConfiguration(serviceID, payload) {
    return sendJSON(
      `/api/v1/services/${encodeURIComponent(serviceID)}`,
      "PUT",
      payload,
      "Unable to save service configuration.",
    );
  }

  /**
   * Every service that can take part in a platform backup, with the backup
   * kinds it publishes. Kaeru Core is always first.
   */
  function loadBackupOptions() {
    return fetchJSON("/api/v1/backup/options", "Unable to load backup options.");
  }

  function unregister(serviceID) {
    return fetchJSON(
      `/api/v1/services/${encodeURIComponent(serviceID)}/unregister`,
      "Unable to unregister the service.",
      { method: "POST" },
    );
  }

  return {
    services,
    loading,
    error,
    showDetails,
    load,
    startPolling,
    stopPolling,
    getConfiguration,
    updateConfiguration,
    loadBackupOptions,
    unregister,
  };
});
