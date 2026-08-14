<script setup>
import { computed, onMounted, onUnmounted, ref } from "vue";
import {
  mdiBackupRestore,
  mdiChevronDown,
  mdiChevronUp,
  mdiClose,
  mdiCogOutline,
  mdiDatabaseArrowUpOutline,
  mdiDownload,
  mdiPlus,
} from "@mdi/js";
import defaultServiceIcon from "../assets/app-icon.svg";
import emailNotificationIcon from "../assets/notification/email.png";
import kaeruRelayNotificationIcon from "../assets/notification/kaeru-relay.png";

const services = ref([]);
const servicesLoading = ref(true);
const servicesError = ref("");

const serviceDescriptions = {
  core: "Shared platform services and coordination",
  upload: "File upload and archival",
  timeline: "Location and personal timeline",
  relay: "Notifications and cross-client transfers",
};

const clients = ref([
  {
    id: "alex-morgan",
    name: "Alex Morgan",
    disabled: false,
    registeredDeviceCount: 2,
    registeredClients: ["Kaeru for Android", "Kaeru Desktop"],
    oidcGroups: ["kaeru-users", "media-admins"],
    access: [
      { service: "Core", level: "Administrator" },
      { service: "Upload Archiver", level: "Editor" },
      { service: "Timeline", level: "Editor" },
      { service: "Relay", level: "User" },
    ],
    lastUsed: "Today at 14:32",
    lastUsedAt: "2026-08-09T14:32:00+02:00",
  },
  {
    id: "jamie-chen",
    name: "Jamie Chen",
    disabled: false,
    registeredDeviceCount: 1,
    registeredClients: ["Kaeru for iOS"],
    oidcGroups: ["kaeru-users", "timeline-viewers"],
    access: [
      { service: "Core", level: "User" },
      { service: "Upload Archiver", level: "User" },
      { service: "Timeline", level: "Viewer" },
      { service: "Relay", level: "User" },
    ],
    lastUsed: "Yesterday at 21:08",
    lastUsedAt: "2026-08-08T21:08:00+02:00",
  },
  {
    id: "sam-rivera",
    name: "Sam Rivera",
    disabled: true,
    registeredDeviceCount: 0,
    registeredClients: [],
    oidcGroups: ["kaeru-guests"],
    access: [
      { service: "Core", level: "Viewer" },
      { service: "Upload Archiver", level: "Viewer" },
      { service: "Timeline", level: "No access" },
      { service: "Relay", level: "No access" },
    ],
    lastUsed: "August 7 at 09:15",
    lastUsedAt: "2026-08-07T09:15:00+02:00",
  },
]);

const notificationServices = ref([
  {
    id: "kaeru-relay",
    name: "Kaeru Relay",
    iconUrl: kaeruRelayNotificationIcon,
    enabled: false,
    url: "",
  },
  {
    id: "email",
    name: "Email",
    iconUrl: emailNotificationIcon,
    enabled: true,
    host: "",
    port: "",
    username: "",
    password: "",
    fromAddress: "",
  },
]);

const backupSummary = {
  lastBackup: "August 9, 2026 at 02:00",
  lastBackupAt: "2026-08-09T02:00:00+02:00",
  nextBackup: "August 10, 2026 at 02:00",
  nextBackupAt: "2026-08-10T02:00:00+02:00",
  size: "128.4 MB",
  path: "/backups/kaeru/",
  file: "2026-08-09-kaeru-platform-backup.tar.gz",
  schedule: "Every day",
  scheduledTime: "02:00",
  automatic: "Enabled",
  retention: "60",
};

const backupScheduleOptions = [
  "Every day",
  "Every 2 days",
  "Every 3 days",
  "Every 4 days",
  "Every 5 days",
  "Every 6 days",
  "Every 7 days",
];

const availableBackups = [
  "2026-08-09-kaeru-platform-backup.tar.gz",
  "2026-08-08-kaeru-platform-backup.tar.gz",
  "2026-08-07-kaeru-platform-backup.tar.gz",
];

const serviceToDelete = ref(null);
const showServiceDetails = ref(false);
const serviceToConfigure = ref(null);
const serviceConfigurationLoading = ref(false);
const serviceConfigurationSaving = ref(false);
const serviceConfigurationError = ref("");
const serviceDeletionSaving = ref(false);
const serviceConfigurationDraft = ref({
  applicationUrl: "",
  nativeClientsUrl: "",
  defaultUserRole: null,
  roleMappings: [],
});
const serviceRoleOptions = computed(() => {
  const roles = serviceToConfigure.value?.roles ?? [];
  return [
    { title: "No Access", value: null },
    ...roles
      .filter((role) => role.active)
      .map((role) => ({ title: role.name, value: role.key })),
  ];
});
const serviceMappingRoleOptions = computed(() => (
  (serviceToConfigure.value?.roles ?? [])
    .filter((role) => role.active)
    .map((role) => ({ title: role.name, value: role.key }))
));
const expandedUserId = ref(null);
const userActionToConfirm = ref(null);
const notificationServiceToEdit = ref(null);
const notificationProviderDraft = ref({
  enabled: false,
  url: "",
  host: "",
  port: null,
  username: "",
  password: "",
  fromAddress: "",
});
const notificationTestOpen = ref(false);
const testEmailAddress = ref("");
const testClientApp = ref(null);
const about = ref(null);
const aboutError = ref("");
const aboutLoading = ref(true);
const backupConfigurationOpen = ref(false);
const downloadBackupOpen = ref(false);
const selectedBackup = ref(backupSummary.file);
const restoreBackupOpen = ref(false);
const restoreBackupFile = ref(null);
const selectedRestoreServiceIds = ref([]);
const backupConfigurationDraft = ref({
  automatic: true,
  schedule: "Every day",
  scheduledTime: backupSummary.scheduledTime,
  retention: Number(backupSummary.retention),
});
const restoreFileSelected = computed(() => (
  Array.isArray(restoreBackupFile.value)
    ? restoreBackupFile.value.length > 0
    : Boolean(restoreBackupFile.value)
));
const registeredClientApps = computed(() => [
  ...new Set(clients.value.flatMap((client) => client.registeredClients)),
]);
let nextRoleMappingId = 1;
let servicesRefreshTimer = null;

function normalizeService(service) {
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
    details.push({ label: "Native Clients URL", value: service.native_apps_url });
  }
  if (service.database_name) {
    details.push({ label: "Database", value: service.database_name });
  }
  if (service.health_checked_at) {
    details.push({
      label: "Last health check",
      value: new Date(service.health_checked_at).toLocaleString(),
    });
  }
  if (service.health_error) {
    details.push({ label: "Health issue", value: service.health_error });
  }

  return {
    ...service,
    description: serviceDescriptions[service.service_type]
      ?? `${service.name} service`,
    iconUrl: isCore ? defaultServiceIcon : `/api/v1/services/${encodeURIComponent(service.id)}/icon`,
    iconFailed: false,
    status,
    type: "service",
    details,
  };
}

function serviceStatusLabel(status) {
  if (status === "online") {
    return "Online";
  }
  if (status === "offline") {
    return "Offline";
  }
  return "Checking";
}

async function apiErrorMessage(response, fallback) {
  try {
    const body = await response.json();
    return body.error?.message || fallback;
  } catch {
    return fallback;
  }
}

function mergeServices(updatedServices) {
  const existingServices = new Map(
    services.value.map((service) => [service.id, service]),
  );
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

async function loadServices(background = false) {
  if (!background) {
    servicesLoading.value = true;
    servicesError.value = "";
  }
  try {
    const response = await fetch("/api/v1/services", {
      headers: { Accept: "application/json" },
    });
    if (!response.ok) {
      throw new Error(await apiErrorMessage(response, "Unable to load services."));
    }
    mergeServices(await response.json());
    servicesError.value = "";
  } catch (error) {
    if (!background) {
      servicesError.value = error instanceof Error
        ? error.message
        : "Unable to load services.";
    }
  } finally {
    if (!background) {
      servicesLoading.value = false;
    }
  }
}

function refreshVisibleServices() {
  if (document.visibilityState === "visible") {
    loadServices(true);
  }
}

function handleServiceIconError(service) {
  service.iconFailed = true;
}

async function loadAbout() {
  aboutError.value = "";
  aboutLoading.value = true;

  try {
    const response = await fetch("/api/v1/about", {
      headers: { Accept: "application/json" },
    });
    if (!response.ok) {
      throw new Error(await apiErrorMessage(response, "Unable to load application information."));
    }

    about.value = await response.json();
  } catch {
    aboutError.value = "Unable to load application information.";
  } finally {
    aboutLoading.value = false;
  }
}

onMounted(() => {
  loadServices();
  loadAbout();
  servicesRefreshTimer = window.setInterval(refreshVisibleServices, 15_000);
  document.addEventListener("visibilitychange", refreshVisibleServices);
});

onUnmounted(() => {
  if (servicesRefreshTimer !== null) {
    window.clearInterval(servicesRefreshTimer);
  }
  document.removeEventListener("visibilitychange", refreshVisibleServices);
});

function openBackupConfiguration() {
  backupConfigurationDraft.value = {
    automatic: backupSummary.automatic === "Enabled",
    schedule: backupSummary.schedule,
    scheduledTime: backupSummary.scheduledTime,
    retention: backupSummary.retention === ""
      ? null
      : Number(backupSummary.retention),
  };
  backupConfigurationOpen.value = true;
}

function cancelBackupConfiguration() {
  backupConfigurationOpen.value = false;
}

function saveBackupConfiguration() {
  backupSummary.automatic = backupConfigurationDraft.value.automatic
    ? "Enabled"
    : "Disabled";
  backupSummary.schedule = backupConfigurationDraft.value.schedule;
  backupSummary.scheduledTime = backupConfigurationDraft.value.scheduledTime;
  backupSummary.retention = backupConfigurationDraft.value.retention == null
    || backupConfigurationDraft.value.retention === ""
    ? ""
    : String(backupConfigurationDraft.value.retention);
  backupConfigurationOpen.value = false;
}

function openDownloadBackup() {
  selectedBackup.value = backupSummary.file;
  downloadBackupOpen.value = true;
}

function cancelDownloadBackup() {
  downloadBackupOpen.value = false;
}

function downloadSelectedBackup() {
  downloadBackupOpen.value = false;
}

function openRestoreBackup() {
  restoreBackupFile.value = null;
  selectedRestoreServiceIds.value = services.value.map((service) => service.id);
  restoreBackupOpen.value = true;
}

function cancelRestoreBackup() {
  restoreBackupOpen.value = false;
  restoreBackupFile.value = null;
}

function restoreSelectedBackup() {
  restoreBackupOpen.value = false;
  restoreBackupFile.value = null;
}

async function openServiceConfiguration(service) {
  serviceToConfigure.value = service;
  serviceConfigurationLoading.value = true;
  serviceConfigurationError.value = "";
  serviceConfigurationDraft.value = {
    applicationUrl: service.public_url ?? "",
    nativeClientsUrl: service.native_apps_url ?? "",
    defaultUserRole: null,
    roleMappings: [],
  };

  try {
    const response = await fetch(`/api/v1/services/${encodeURIComponent(service.id)}`, {
      headers: { Accept: "application/json" },
    });
    if (!response.ok) {
      throw new Error(await apiErrorMessage(response, "Unable to load service configuration."));
    }
    const configuration = await response.json();
    if (serviceToConfigure.value?.id !== service.id) {
      return;
    }
    Object.assign(service, configuration);
    serviceToConfigure.value = service;
    serviceConfigurationDraft.value = {
      applicationUrl: configuration.public_url ?? "",
      nativeClientsUrl: configuration.native_apps_url ?? "",
      defaultUserRole: configuration.default_role_key ?? null,
      roleMappings: (configuration.role_mappings ?? []).map((mapping) => ({
        id: `role-mapping-${nextRoleMappingId++}`,
        role: mapping.role_key,
        oidcGroups: mapping.oidc_groups.join(", "),
      })),
    };
  } catch (error) {
    serviceConfigurationError.value = error instanceof Error
      ? error.message
      : "Unable to load service configuration.";
  } finally {
    serviceConfigurationLoading.value = false;
  }
}

function cancelServiceConfiguration() {
  serviceToConfigure.value = null;
  serviceConfigurationError.value = "";
}

function addServiceRoleMapping() {
  serviceConfigurationDraft.value.roleMappings.push({
    id: `role-mapping-${nextRoleMappingId++}`,
    role: null,
    oidcGroups: "",
  });
}

function removeServiceRoleMapping(mappingId) {
  serviceConfigurationDraft.value.roleMappings = (
    serviceConfigurationDraft.value.roleMappings.filter(
      (mapping) => mapping.id !== mappingId,
    )
  );
}

async function saveServiceConfiguration() {
  if (!serviceToConfigure.value) {
    return;
  }
  const roleMappings = [];
  for (const mapping of serviceConfigurationDraft.value.roleMappings) {
    const groups = [...new Set(mapping.oidcGroups
      .split(",")
      .map((group) => group.trim())
      .filter(Boolean))];
    if (!mapping.role || groups.length === 0) {
      serviceConfigurationError.value = "Complete or remove each role mapping before saving.";
      return;
    }
    roleMappings.push({ role_key: mapping.role, oidc_groups: groups });
  }

  serviceConfigurationSaving.value = true;
  serviceConfigurationError.value = "";
  try {
    const response = await fetch(
      `/api/v1/services/${encodeURIComponent(serviceToConfigure.value.id)}`,
      {
        method: "PUT",
        headers: {
          Accept: "application/json",
          "Content-Type": "application/json",
        },
        body: JSON.stringify({
          public_url: serviceConfigurationDraft.value.applicationUrl.trim(),
          native_apps_url: serviceConfigurationDraft.value.nativeClientsUrl.trim() || null,
          default_role_key: serviceToConfigure.value.service_type === "core"
            ? null
            : serviceConfigurationDraft.value.defaultUserRole,
          role_mappings: roleMappings,
        }),
      },
    );
    if (!response.ok) {
      throw new Error(await apiErrorMessage(response, "Unable to save service configuration."));
    }
    const updated = await response.json();
    const normalized = normalizeService(updated);
    Object.assign(serviceToConfigure.value, normalized, { roles: updated.roles });
    cancelServiceConfiguration();
  } catch (error) {
    serviceConfigurationError.value = error instanceof Error
      ? error.message
      : "Unable to save service configuration.";
  } finally {
    serviceConfigurationSaving.value = false;
  }
}

function requestConfiguredServiceDeletion() {
  serviceToDelete.value = serviceToConfigure.value;
}

function cancelConfiguredServiceDeletion() {
  serviceToDelete.value = null;
}

async function confirmConfiguredServiceDeletion() {
  if (!serviceToDelete.value) {
    return;
  }
  serviceDeletionSaving.value = true;
  try {
    const response = await fetch(
      `/api/v1/services/${encodeURIComponent(serviceToDelete.value.id)}/unregister`,
      { method: "POST", headers: { Accept: "application/json" } },
    );
    if (!response.ok) {
      throw new Error(await apiErrorMessage(response, "Unable to unregister the service."));
    }
    const unregistered = normalizeService(await response.json());
    Object.assign(serviceToDelete.value, unregistered);
    serviceToDelete.value = null;
    cancelServiceConfiguration();
  } catch (error) {
    serviceConfigurationError.value = error instanceof Error
      ? error.message
      : "Unable to unregister the service.";
    serviceToDelete.value = null;
  } finally {
    serviceDeletionSaving.value = false;
  }
}

function formatRegisteredDevices(count) {
  if (!count) {
    return "No registered client applications";
  }

  return `${count} registered client ${count === 1 ? "application" : "applications"}`;
}

function toggleUserDetails(userId) {
  expandedUserId.value = expandedUserId.value === userId ? null : userId;
}

function requestUserAction(user, action) {
  userActionToConfirm.value = { user, action };
}

function cancelUserAction() {
  userActionToConfirm.value = null;
}

function confirmUserAction() {
  if (!userActionToConfirm.value) {
    return;
  }

  const { user, action } = userActionToConfirm.value;
  if (action === "disable") {
    user.disabled = true;
  } else if (action === "enable") {
    user.disabled = false;
  }

  cancelUserAction();
}

function openNotificationServiceEditor(notificationService) {
  notificationServiceToEdit.value = notificationService;
  notificationProviderDraft.value = {
    enabled: notificationService.enabled,
    url: notificationService.url ?? "",
    host: notificationService.host ?? "",
    port: notificationService.port ?? null,
    username: notificationService.username ?? "",
    password: notificationService.password ?? "",
    fromAddress: notificationService.fromAddress ?? "",
  };
}

function cancelNotificationServiceEditor() {
  notificationServiceToEdit.value = null;
}

function saveNotificationService() {
  const notificationService = notificationServices.value.find(
    (candidate) => candidate.id === notificationServiceToEdit.value?.id,
  );

  if (notificationService) {
    Object.assign(notificationService, notificationProviderDraft.value);
  }

  cancelNotificationServiceEditor();
}

function openNotificationTest() {
  testEmailAddress.value = "";
  testClientApp.value = null;
  notificationTestOpen.value = true;
}

function cancelNotificationTest() {
  notificationTestOpen.value = false;
}

function sendNotificationTest() {
  notificationTestOpen.value = false;
}
</script>

<template>
  <v-container class="page-content">
    <section aria-labelledby="services-title" class="home-section">
      <div class="home-section-header">
        <h1 id="services-title" class="home-section-title">Services</h1>
        <v-btn
          :aria-expanded="showServiceDetails"
          color="primary"
          variant="text"
          @click="showServiceDetails = !showServiceDetails"
        >
          {{ showServiceDetails ? "Less Details" : "More Details" }}
          <v-icon
            :icon="showServiceDetails ? mdiChevronUp : mdiChevronDown"
            end
          />
        </v-btn>
      </div>

      <p v-if="servicesLoading" class="service-state-message">
        Loading services…
      </p>
      <div v-else-if="servicesError" class="service-state-message service-state-message--error">
        <span>{{ servicesError }}</span>
        <v-btn color="primary" size="small" variant="text" @click="loadServices()">
          Try again
        </v-btn>
      </div>
      <p v-else-if="services.length === 0" class="service-state-message">
        No services are registered.
      </p>
      <div v-else class="service-card-grid">
        <v-sheet
          v-for="service in services"
          :key="service.id"
          :class="[
            'service-card',
            { 'service-card--offline': service.status === 'offline' },
          ]"
          border
          role="button"
          rounded="lg"
          tabindex="0"
          @click="openServiceConfiguration(service)"
          @keydown.enter="openServiceConfiguration(service)"
          @keydown.space.prevent="openServiceConfiguration(service)"
        >
          <div
            :class="['service-card-status', `service-card-status--${service.status}`]"
          >
            <span class="service-card-status-dot" aria-hidden="true" />
            {{ serviceStatusLabel(service.status) }}
          </div>

          <div class="service-card-summary">
            <img
              :src="service.iconFailed ? defaultServiceIcon : service.iconUrl"
              alt=""
              class="service-card-icon"
              @error="handleServiceIconError(service)"
            />

            <div>
              <h2 class="service-card-name">{{ service.name }}</h2>
              <p class="service-card-description">{{ service.description }}</p>
            </div>
          </div>

          <v-expand-transition>
            <dl v-show="showServiceDetails" class="service-card-details">
              <div
                v-for="detail in service.details"
                :key="detail.label"
                class="service-card-detail"
              >
                <dt>{{ detail.label }}</dt>
                <dd>{{ detail.value }}</dd>
              </div>
            </dl>
          </v-expand-transition>
        </v-sheet>
      </div>
    </section>

    <v-dialog
      :model-value="serviceToConfigure !== null"
      max-width="680"
      @update:model-value="(open) => !open && cancelServiceConfiguration()"
    >
      <v-card
        v-if="serviceToConfigure"
        class="service-configuration-dialog"
        rounded="lg"
      >
        <form @submit.prevent="saveServiceConfiguration">
          <v-card-title>Configure {{ serviceToConfigure.name }}</v-card-title>
          <v-card-text class="service-configuration-fields">
            <p
              v-if="serviceConfigurationError"
              class="service-configuration-error"
              role="alert"
            >
              {{ serviceConfigurationError }}
            </p>
            <p v-if="serviceConfigurationLoading" class="service-state-message">
              Loading configuration…
            </p>
            <div class="service-configuration-field">
              <label for="service-application-url" class="service-field-label">
                Application URL
              </label>
              <p id="service-application-url-help" class="service-field-help">
                URL for {{ serviceToConfigure.name }} access
              </p>
              <v-text-field
                id="service-application-url"
                v-model="serviceConfigurationDraft.applicationUrl"
                aria-describedby="service-application-url-help"
                :disabled="serviceConfigurationLoading || serviceConfigurationSaving"
                hide-details="auto"
                placeholder="https://service.example.com"
                type="url"
                variant="outlined"
              />
            </div>

            <div class="service-configuration-field">
              <label for="service-native-url" class="service-field-label">
                Native Clients URL
              </label>
              <p id="service-native-url-help" class="service-field-help">
                URL for access from Kaeru native apps, leave empty to use
                regular application URL
              </p>
              <v-text-field
                id="service-native-url"
                v-model="serviceConfigurationDraft.nativeClientsUrl"
                aria-describedby="service-native-url-help"
                :disabled="serviceConfigurationLoading || serviceConfigurationSaving"
                hide-details="auto"
                placeholder="https://native.service.example.com"
                type="url"
                variant="outlined"
              />
            </div>

            <div class="service-configuration-field">
              <label for="service-default-role" class="service-field-label">
                Default user role
              </label>
              <p id="service-default-role-help" class="service-field-help">
                Default access level for users not belonging to a higher access
                group
              </p>
              <v-select
                id="service-default-role"
                v-model="serviceConfigurationDraft.defaultUserRole"
                aria-describedby="service-default-role-help"
                :disabled="serviceConfigurationLoading || serviceConfigurationSaving || serviceToConfigure.service_type === 'core'"
                :items="serviceRoleOptions"
                hide-details="auto"
                placeholder="No Access"
                variant="outlined"
              />
            </div>

            <div
              v-for="(mapping, index) in serviceConfigurationDraft.roleMappings"
              :key="mapping.id"
              class="service-role-mapping"
            >
              <div class="service-role-mapping-heading">
                <div>
                  <p class="service-field-label">Role Mapping {{ index + 1 }}</p>
                  <p class="service-field-help">
                    Choose OIDC groups which grant access to
                    {{ serviceToConfigure.name }} roles
                  </p>
                </div>
                <v-btn
                  :aria-label="`Delete role mapping ${index + 1}`"
                  color="error"
                  density="comfortable"
                  icon
                  variant="text"
                  @click="removeServiceRoleMapping(mapping.id)"
                >
                  <v-icon :icon="mdiClose" />
                </v-btn>
              </div>

              <div class="service-role-mapping-inputs">
                <v-select
                  v-model="mapping.role"
                  :disabled="serviceConfigurationSaving"
                  :items="serviceMappingRoleOptions"
                  hide-details="auto"
                  :placeholder="`${serviceToConfigure.name} role`"
                  variant="outlined"
                />
                <v-text-field
                  v-model="mapping.oidcGroups"
                  :disabled="serviceConfigurationSaving"
                  hide-details="auto"
                  placeholder="OIDC groups (comma separated)"
                  variant="outlined"
                />
              </div>
            </div>

            <v-btn
              class="add-role-mapping-button"
              color="primary"
              :disabled="serviceConfigurationLoading || serviceConfigurationSaving"
              variant="text"
              @click="addServiceRoleMapping"
            >
              <v-icon :icon="mdiPlus" start />
              Add role mapping
            </v-btn>
          </v-card-text>
          <v-card-actions>
            <v-btn
              v-if="serviceToConfigure.status === 'offline' && serviceToConfigure.registration_status === 'registered' && serviceToConfigure.service_type !== 'core'"
              color="error"
              :disabled="serviceConfigurationSaving"
              variant="text"
              @click="requestConfiguredServiceDeletion"
            >
              Delete
            </v-btn>
            <v-spacer />
            <v-btn :disabled="serviceConfigurationSaving" variant="text" @click="cancelServiceConfiguration">
              Cancel
            </v-btn>
            <v-btn
              color="primary"
              :disabled="serviceConfigurationLoading"
              :loading="serviceConfigurationSaving"
              type="submit"
              variant="flat"
            >
              Save
            </v-btn>
          </v-card-actions>
        </form>
      </v-card>
    </v-dialog>

    <v-dialog
      :model-value="serviceToDelete !== null"
      max-width="480"
      @update:model-value="(open) => !open && cancelConfiguredServiceDeletion()"
    >
      <v-card v-if="serviceToDelete" class="delete-service-dialog" rounded="lg">
        <v-card-title>
          Delete {{ serviceToDelete.name }} service?
        </v-card-title>
        <v-card-text>
          If it starts up again it will be automatically re-registered again.
        </v-card-text>
        <v-card-actions>
          <v-spacer />
          <v-btn variant="text" @click="cancelConfiguredServiceDeletion">
            Cancel
          </v-btn>
          <v-btn
            color="error"
            :loading="serviceDeletionSaving"
            variant="flat"
            @click="confirmConfiguredServiceDeletion"
          >
            Confirm
          </v-btn>
        </v-card-actions>
      </v-card>
    </v-dialog>

    <section aria-labelledby="users-title" class="home-section clients-section">
      <div class="home-section-header">
        <h2 id="users-title" class="home-section-title">Users</h2>
      </div>

      <div class="client-list">
        <v-sheet
          v-for="client in clients"
          :key="client.id"
          class="client-list-item"
          border
          role="button"
          rounded="lg"
          tabindex="0"
          @click="toggleUserDetails(client.id)"
          @keydown.enter="toggleUserDetails(client.id)"
          @keydown.space.prevent="toggleUserDetails(client.id)"
        >
          <div class="client-list-summary">
            <div>
              <div class="client-name-row">
                <h3 class="client-name">{{ client.name }}</h3>
                <span v-if="client.disabled" class="client-disabled-status">
                  Disabled
                </span>
              </div>
              <p class="client-last-used">
                {{ formatRegisteredDevices(client.registeredDeviceCount) }}
              </p>
              <p class="client-last-used">
                Last seen
                <time :datetime="client.lastUsedAt">{{ client.lastUsed }}</time>
              </p>
            </div>
            <v-icon
              :icon="expandedUserId === client.id ? mdiChevronUp : mdiChevronDown"
              class="client-expand-icon"
            />
          </div>

          <v-expand-transition>
            <div
              v-show="expandedUserId === client.id"
              class="client-expanded-details"
            >
              <section class="client-detail-section">
                <h4 class="client-detail-title">Registered Clients</h4>
                <p class="client-detail-description">
                  Client applications that {{ client.name }} has logged in on
                </p>
                <ul
                  v-if="client.registeredClients.length > 0"
                  class="client-readonly-list"
                >
                  <li
                    v-for="registeredClient in client.registeredClients"
                    :key="registeredClient"
                  >
                    {{ registeredClient }}
                  </li>
                </ul>
                <p v-else class="client-empty-detail">No registered clients</p>
              </section>

              <section class="client-detail-section">
                <h4 class="client-detail-title">Access</h4>
                <p class="client-detail-description">
                  OIDC groups: {{ client.oidcGroups.join(", ") }}
                </p>
                <dl class="client-access-list">
                  <div
                    v-for="access in client.access"
                    :key="access.service"
                    class="client-access-row"
                  >
                    <dt>{{ access.service }}</dt>
                    <dd>{{ access.level }}</dd>
                  </div>
                </dl>
              </section>

              <div class="client-user-actions">
                <v-btn
                  :to="{ name: 'events', query: { user: client.name } }"
                  class="client-event-log-button"
                  variant="text"
                  @click.stop
                >
                  Event Log
                </v-btn>
                <div class="client-user-primary-actions">
                  <v-btn
                    color="error"
                    variant="flat"
                    @click.stop="requestUserAction(client, 'logout')"
                  >
                    Force logout
                  </v-btn>
                  <v-btn
                    :color="client.disabled ? 'primary' : 'error'"
                    variant="flat"
                    @click.stop="requestUserAction(
                      client,
                      client.disabled ? 'enable' : 'disable',
                    )"
                  >
                    {{ client.disabled ? "Enable user" : "Disable user" }}
                  </v-btn>
                </div>
              </div>
            </div>
          </v-expand-transition>
        </v-sheet>
      </div>
    </section>

    <v-dialog
      :model-value="userActionToConfirm !== null"
      max-width="480"
      @update:model-value="(open) => !open && cancelUserAction()"
    >
      <v-card
        v-if="userActionToConfirm"
        class="delete-service-dialog"
        rounded="lg"
      >
        <v-card-title>
          <template v-if="userActionToConfirm.action === 'logout'">
            Force logout {{ userActionToConfirm.user.name }}?
          </template>
          <template v-else-if="userActionToConfirm.action === 'disable'">
            Disable {{ userActionToConfirm.user.name }}?
          </template>
          <template v-else>
            Enable {{ userActionToConfirm.user.name }}?
          </template>
        </v-card-title>
        <v-card-text>
          <template v-if="userActionToConfirm.action === 'logout'">
            The user will be required to log in again on all clients. This can
            be used to ensure user access is updated.
          </template>
          <template v-else-if="userActionToConfirm.action === 'disable'">
            The user will be logged out on all clients and not allowed to log
            in anymore unless re-enabled.
          </template>
          <template v-else>
            The user will be able to log in on their client apps again.
          </template>
        </v-card-text>
        <v-card-actions>
          <v-spacer />
          <v-btn variant="text" @click="cancelUserAction">Cancel</v-btn>
          <v-btn
            :color="userActionToConfirm.action === 'enable' ? 'primary' : 'error'"
            variant="flat"
            @click="confirmUserAction"
          >
            Confirm
          </v-btn>
        </v-card-actions>
      </v-card>
    </v-dialog>

    <section
      aria-labelledby="notification-services-title"
      class="home-section notification-services-section"
    >
      <div class="home-section-header">
        <h2 id="notification-services-title" class="home-section-title">
          Notification Services
        </h2>
      </div>

      <div class="notification-service-grid">
        <v-sheet
          v-for="notificationService in notificationServices"
          :key="notificationService.id"
          class="notification-service-card"
          border
          role="button"
          rounded="lg"
          tabindex="0"
          @click="openNotificationServiceEditor(notificationService)"
          @keydown.enter="openNotificationServiceEditor(notificationService)"
          @keydown.space.prevent="openNotificationServiceEditor(notificationService)"
        >
          <img
            :src="notificationService.iconUrl"
            alt=""
            class="notification-service-icon"
          />
          <div>
            <h3 class="notification-service-name">
              {{ notificationService.name }}
            </h3>
            <p
              :class="[
                'notification-service-status',
                { 'notification-service-status--disabled': !notificationService.enabled },
              ]"
            >
              {{ notificationService.enabled ? "Enabled" : "Disabled" }}
            </p>
          </div>
        </v-sheet>
      </div>
    </section>

    <v-dialog
      :model-value="notificationServiceToEdit !== null"
      max-width="560"
      @update:model-value="(open) => !open && cancelNotificationServiceEditor()"
    >
      <v-card
        v-if="notificationServiceToEdit"
        class="notification-provider-dialog"
        rounded="lg"
      >
        <v-card-title>
          Configure {{ notificationServiceToEdit.name }} Notifications
        </v-card-title>
        <v-card-text class="notification-provider-fields">
          <v-switch
            v-model="notificationProviderDraft.enabled"
            color="primary"
            hide-details
            label="Enabled"
          />

          <div
            v-if="notificationServiceToEdit.id === 'kaeru-relay'"
            class="service-field"
          >
            <label for="notification-relay-url" class="service-field-label">
              URL
            </label>
            <p id="notification-relay-url-help" class="notification-field-help">
              URL for the Kaeru Relay notification service
            </p>
            <v-text-field
              id="notification-relay-url"
              v-model="notificationProviderDraft.url"
              aria-describedby="notification-relay-url-help"
              :disabled="!notificationProviderDraft.enabled"
              hide-details="auto"
              placeholder="https://relay.example.com"
              type="url"
              variant="outlined"
            />
          </div>

          <template v-else>
            <div class="service-field">
              <label for="notification-email-host" class="service-field-label">
                Host
              </label>
              <p id="notification-email-host-help" class="notification-field-help">
                Host of the email server (e.g. smtp.example.com)
              </p>
              <v-text-field
                id="notification-email-host"
                v-model="notificationProviderDraft.host"
                aria-describedby="notification-email-host-help"
                :disabled="!notificationProviderDraft.enabled"
                hide-details="auto"
                placeholder="smtp.example.com"
                variant="outlined"
              />
            </div>
            <div class="service-field">
              <label for="notification-email-port" class="service-field-label">
                Port
              </label>
              <p id="notification-email-port-help" class="notification-field-help">
                Port of the email server (e.g. 25, 465, or 587)
              </p>
              <v-text-field
                id="notification-email-port"
                v-model="notificationProviderDraft.port"
                aria-describedby="notification-email-port-help"
                :disabled="!notificationProviderDraft.enabled"
                hide-details="auto"
                placeholder="587"
                variant="outlined"
              />
            </div>
            <div class="service-field">
              <label
                for="notification-email-username"
                class="service-field-label"
              >
                Username
              </label>
              <p
                id="notification-email-username-help"
                class="notification-field-help"
              >
                Username to use when authenticating with the email server
              </p>
              <v-text-field
                id="notification-email-username"
                v-model="notificationProviderDraft.username"
                aria-describedby="notification-email-username-help"
                :disabled="!notificationProviderDraft.enabled"
                hide-details="auto"
                placeholder="Username"
                variant="outlined"
              />
            </div>
            <div class="service-field">
              <label
                for="notification-email-password"
                class="service-field-label"
              >
                Password
              </label>
              <p
                id="notification-email-password-help"
                class="notification-field-help"
              >
                Password to use when authenticating with the email server
              </p>
              <v-text-field
                id="notification-email-password"
                v-model="notificationProviderDraft.password"
                aria-describedby="notification-email-password-help"
                :disabled="!notificationProviderDraft.enabled"
                hide-details="auto"
                placeholder="Password"
                type="password"
                variant="outlined"
              />
            </div>
            <div class="service-field">
              <label
                for="notification-email-from-address"
                class="service-field-label"
              >
                From Address
              </label>
              <p
                id="notification-email-from-address-help"
                class="notification-field-help"
              >
                Sender email address (e.g "Kaeru &lt;kaeru@example.com&gt;")
              </p>
              <v-text-field
                id="notification-email-from-address"
                v-model="notificationProviderDraft.fromAddress"
                aria-describedby="notification-email-from-address-help"
                :disabled="!notificationProviderDraft.enabled"
                hide-details="auto"
                placeholder="Kaeru &lt;kaeru@example.com&gt;"
                variant="outlined"
              />
            </div>
          </template>
        </v-card-text>
        <v-card-actions>
          <v-btn variant="text" @click="cancelNotificationServiceEditor">
            Cancel
          </v-btn>
          <div class="notification-provider-primary-actions">
            <v-btn variant="text" @click="openNotificationTest">Test</v-btn>
            <v-btn
              color="primary"
              variant="flat"
              @click="saveNotificationService"
            >
              Save
            </v-btn>
          </div>
        </v-card-actions>
      </v-card>
    </v-dialog>

    <v-dialog
      :model-value="notificationTestOpen"
      max-width="520"
      @update:model-value="(open) => !open && cancelNotificationTest()"
    >
      <v-card
        v-if="notificationServiceToEdit"
        class="notification-test-dialog"
        rounded="lg"
      >
        <form @submit.prevent="sendNotificationTest">
          <v-card-title>
            {{ notificationServiceToEdit.id === "email"
              ? "Send Test Email"
              : "Send Test Notification" }}
          </v-card-title>
          <v-card-text class="notification-test-fields">
            <div
              v-if="notificationServiceToEdit.id === 'email'"
              class="service-field"
            >
              <label for="test-email-address" class="service-field-label">
                Email address
              </label>
              <p id="test-email-address-help" class="notification-field-help">
                Choose an email address to send the test email to
              </p>
              <v-text-field
                id="test-email-address"
                v-model="testEmailAddress"
                aria-describedby="test-email-address-help"
                autofocus
                hide-details="auto"
                placeholder="name@example.com"
                type="email"
                variant="outlined"
              />
            </div>
            <div v-else class="service-field">
              <label for="test-client-app" class="service-field-label">
                Client app
              </label>
              <p id="test-client-app-help" class="notification-field-help">
                Choose a client app to send the test notification to
              </p>
              <v-select
                id="test-client-app"
                v-model="testClientApp"
                aria-describedby="test-client-app-help"
                :items="registeredClientApps"
                hide-details="auto"
                no-data-text="No registered client apps"
                placeholder="Select a client app"
                variant="outlined"
              />
            </div>
          </v-card-text>
          <v-card-actions>
            <v-spacer />
            <v-btn variant="text" @click="cancelNotificationTest">
              Cancel
            </v-btn>
            <v-btn
              :disabled="notificationServiceToEdit.id === 'email'
                ? testEmailAddress.trim() === ''
                : testClientApp === null"
              color="primary"
              type="submit"
              variant="flat"
            >
              Send
            </v-btn>
          </v-card-actions>
        </form>
      </v-card>
    </v-dialog>

    <section
      aria-labelledby="backup-title"
      class="home-section backup-section"
    >
      <div class="home-section-header">
        <h2 id="backup-title" class="home-section-title">
          Backup and Restore
        </h2>
        <v-menu location="bottom end">
          <template #activator="{ props: activatorProps }">
            <v-btn
              v-bind="activatorProps"
              aria-label="Backup and restore options"
              color="primary"
              variant="text"
            >
              Options
              <v-icon :icon="mdiChevronDown" end />
            </v-btn>
          </template>

          <v-list class="backup-options-menu" density="compact">
            <v-list-item
              :prepend-icon="mdiDatabaseArrowUpOutline"
              link
              title="Backup now"
            />
            <v-list-item
              :prepend-icon="mdiDownload"
              link
              title="Download"
              @click="openDownloadBackup"
            />
            <v-list-item
              :prepend-icon="mdiBackupRestore"
              link
              title="Restore"
              @click="openRestoreBackup"
            />
            <v-list-item
              :prepend-icon="mdiCogOutline"
              link
              title="Configure"
              @click="openBackupConfiguration"
            />
          </v-list>
        </v-menu>
      </div>

      <v-sheet class="backup-summary" border rounded="lg">
        <dl class="backup-details">
          <div class="backup-detail">
            <dt>Automatic backups</dt>
            <dd>
              {{ backupSummary.automatic }} ({{ backupSummary.schedule }} at {{ backupSummary.scheduledTime }}, {{ backupSummary.retention === "" ? "Unlimited" : `${backupSummary.retention} days` }} retention)
            </dd>
          </div>
          <div class="backup-detail">
            <dt>Last backup</dt>
            <dd>
              <time :datetime="backupSummary.lastBackupAt">
                {{ backupSummary.lastBackup }}
              </time>
            </dd>
          </div>
          <div class="backup-detail">
            <dt>Backup directory</dt>
            <dd>{{ backupSummary.path }}</dd>
          </div>
          <div class="backup-detail">
            <dt>Last backup file</dt>
            <dd>{{ backupSummary.file }}</dd>
          </div>
          <div class="backup-detail">
            <dt>Next backup</dt>
            <dd>
              <time :datetime="backupSummary.nextBackupAt">
                {{ backupSummary.nextBackup }}
              </time>
            </dd>
          </div>
        </dl>
      </v-sheet>
    </section>

    <v-dialog
      :model-value="backupConfigurationOpen"
      max-width="520"
      @update:model-value="(open) => !open && cancelBackupConfiguration()"
    >
      <v-card class="backup-configuration-dialog" rounded="lg">
        <form @submit.prevent="saveBackupConfiguration">
          <v-card-title>Configure Backups</v-card-title>
          <v-card-text class="backup-configuration-fields">
            <v-checkbox
              v-model="backupConfigurationDraft.automatic"
              color="primary"
              hide-details
              label="Automatic backups"
            />
            <div class="backup-configuration-field">
              <label for="backup-schedule" class="service-field-label">
                Schedule
              </label>
              <p id="backup-schedule-help" class="backup-field-help">
                Choose how often automatic backups run
              </p>
              <v-select
                id="backup-schedule"
                v-model="backupConfigurationDraft.schedule"
                :aria-describedby="'backup-schedule-help'"
                :disabled="!backupConfigurationDraft.automatic"
                :items="backupScheduleOptions"
                hide-details="auto"
                variant="outlined"
              />
            </div>
            <div class="backup-configuration-field">
              <label for="backup-scheduled-time" class="service-field-label">
                Time
              </label>
              <p id="backup-scheduled-time-help" class="backup-field-help">
                Choose the time of day for automatic backups
              </p>
              <v-text-field
                id="backup-scheduled-time"
                v-model="backupConfigurationDraft.scheduledTime"
                :aria-describedby="'backup-scheduled-time-help'"
                :disabled="!backupConfigurationDraft.automatic"
                hide-details="auto"
                step="60"
                type="time"
                variant="outlined"
              />
            </div>
            <div class="backup-configuration-field">
              <label for="backup-retention" class="service-field-label">
                Retention
              </label>
              <p id="backup-retention-help" class="backup-field-help">
                Leave empty for unlimited retention. Manual backups are never
                deleted
              </p>
              <v-text-field
                id="backup-retention"
                v-model.number="backupConfigurationDraft.retention"
                :aria-describedby="'backup-retention-help'"
                :disabled="!backupConfigurationDraft.automatic"
                hide-details="auto"
                min="1"
                placeholder="Unlimited"
                step="1"
                suffix="days"
                type="number"
                variant="outlined"
              />
            </div>
          </v-card-text>
          <v-card-actions>
            <v-spacer />
            <v-btn variant="text" @click="cancelBackupConfiguration">
              Cancel
            </v-btn>
            <v-btn color="primary" type="submit" variant="flat">Save</v-btn>
          </v-card-actions>
        </form>
      </v-card>
    </v-dialog>

    <v-dialog
      :model-value="restoreBackupOpen"
      max-width="560"
      @update:model-value="(open) => !open && cancelRestoreBackup()"
    >
      <v-card class="restore-backup-dialog" rounded="lg">
        <form @submit.prevent="restoreSelectedBackup">
          <v-card-title>Restore Backup</v-card-title>
          <v-card-text class="restore-backup-fields">
            <div class="backup-configuration-field">
              <label for="restore-backup-file" class="service-field-label">
                Backup
              </label>
              <p id="restore-backup-file-help" class="backup-field-help">
                Choose a backup file to restore
              </p>
              <v-file-input
                id="restore-backup-file"
                v-model="restoreBackupFile"
                accept=".gz,.tar"
                aria-describedby="restore-backup-file-help"
                clearable
                hide-details="auto"
                variant="outlined"
              />
            </div>

            <fieldset
              v-if="restoreFileSelected"
              class="restore-service-selection"
            >
              <legend class="service-field-label">Services</legend>
              <p class="backup-field-help">
                Select the services you wish to restore
              </p>
              <div class="restore-service-list">
                <v-checkbox
                  v-for="service in services"
                  :key="service.id"
                  v-model="selectedRestoreServiceIds"
                  :label="service.name"
                  :value="service.id"
                  color="primary"
                  density="compact"
                  hide-details
                />
              </div>
            </fieldset>
          </v-card-text>
          <v-card-actions>
            <v-spacer />
            <v-btn variant="text" @click="cancelRestoreBackup">Cancel</v-btn>
            <v-btn
              :disabled="!restoreFileSelected || selectedRestoreServiceIds.length === 0"
              color="primary"
              type="submit"
              variant="flat"
            >
              Restore
            </v-btn>
          </v-card-actions>
        </form>
      </v-card>
    </v-dialog>

    <v-dialog
      :model-value="downloadBackupOpen"
      max-width="520"
      @update:model-value="(open) => !open && cancelDownloadBackup()"
    >
      <v-card class="download-backup-dialog" rounded="lg">
        <form @submit.prevent="downloadSelectedBackup">
          <v-card-title>Download Backup</v-card-title>
          <v-card-text class="download-backup-fields">
            <div class="backup-configuration-field">
              <label for="download-backup-selection" class="service-field-label">
                Backup
              </label>
              <p id="download-backup-selection-help" class="backup-field-help">
                Choose an available backup
              </p>
              <v-select
                id="download-backup-selection"
                v-model="selectedBackup"
                aria-describedby="download-backup-selection-help"
                :items="availableBackups"
                hide-details="auto"
                variant="outlined"
              />
            </div>
          </v-card-text>
          <v-card-actions>
            <v-spacer />
            <v-btn variant="text" @click="cancelDownloadBackup">Cancel</v-btn>
            <v-btn color="primary" type="submit" variant="flat">
              Download
            </v-btn>
          </v-card-actions>
        </form>
      </v-card>
    </v-dialog>

    <section
      aria-labelledby="about-title"
      class="home-section about-section"
    >
      <div class="home-section-header">
        <h2 id="about-title" class="home-section-title">About</h2>
      </div>

      <v-sheet class="about-summary" border rounded="lg">
        <p v-if="aboutLoading" class="about-message">
          Loading application information…
        </p>
        <p v-else-if="aboutError" class="about-message text-error">
          {{ aboutError }}
        </p>
        <div v-else-if="about" class="about-details">
          <span class="app-icon about-icon" aria-hidden="true" />
          <div>
            <p class="about-name">{{ about.name }} v{{ about.version }}</p>
            <p class="about-description">{{ about.description }}</p>
          </div>
        </div>
      </v-sheet>
    </section>

  </v-container>
</template>
