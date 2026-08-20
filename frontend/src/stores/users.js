import { defineStore } from "pinia";
import { computed, ref } from "vue";

import { fetchJSON } from "../api/client.js";
import defaultUserIcon from "../assets/default-user.png";

/** Builds the display fields the user list renders from an API user summary. */
function normalizeUser(user) {
  const devices = user.registered_devices ?? [];
  const lastSeenAt = user.last_seen_at ?? null;
  return {
    id: user.id,
    name: user.display_name || user.username,
    username: user.username,
    email: user.email,
    avatarUrl: user.avatar_url || defaultUserIcon,
    avatarFailed: false,
    disabled: user.disabled,
    deviceCount: devices.length,
    devices: devices.map((device) => device.name),
    oidcGroups: user.oidc_groups ?? [],
    access: (user.access ?? []).map((access) => ({
      service: access.service_name,
      level: access.role_name,
    })),
    lastSeen: lastSeenAt ? new Date(lastSeenAt).toLocaleString() : "Never",
    lastSeenAt,
  };
}

/**
 * Owns the Kaeru user directory. A user is a person; the devices they have
 * signed in on hang off that user rather than existing as their own directory.
 */
export const useUsersStore = defineStore("users", () => {
  const users = ref([]);
  const loading = ref(true);
  const error = ref("");

  /** Every distinct device name across all users, for device pickers. */
  const deviceNames = computed(() => [
    ...new Set(users.value.flatMap((user) => user.devices)),
  ]);

  async function load() {
    loading.value = true;
    error.value = "";
    try {
      users.value = (await fetchJSON("/api/v1/users", "Unable to load users.")).map(normalizeUser);
    } catch (loadError) {
      error.value = loadError instanceof Error ? loadError.message : "Unable to load users.";
    } finally {
      loading.value = false;
    }
  }

  /**
   * NOT PERSISTED. Kaeru Core has no endpoint for disabling, enabling, or
   * force-logging-out a user yet, so this only updates the local list. Replace
   * with a real request once those endpoints exist. See src/mocks/README.md.
   */
  function applyUnsavedUserAction(user, action) {
    if (action === "disable") {
      user.disabled = true;
    } else if (action === "enable") {
      user.disabled = false;
    }
  }

  return { users, loading, error, deviceNames, load, applyUnsavedUserAction };
});
