<script setup>
import { nextTick, onUnmounted, ref, watch } from "vue";
import { mdiCamera, mdiCheck } from "@mdi/js";

import defaultUserIcon from "../../assets/default-user.png";
import { useSessionStore } from "../../stores/session.js";
// MOCK-BACKED: the Notifications tab has no Kaeru Core endpoint behind it.
// See src/mocks/README.md.
import {
  notificationDeliveryMethods,
  notificationDigestFrequencies,
  notificationPreferenceDefaults,
  notificationSeverityOptions,
  notificationToggleOptions,
} from "../../mocks/notifications.js";

const props = defineProps({
  modelValue: { type: Boolean, default: false },
});
const emit = defineEmits(["update:modelValue"]);

const session = useSessionStore();

const loading = ref(false);
const saving = ref(false);
const saved = ref(false);
const error = ref("");
const tab = ref("details");
const formScroll = ref(null);
const avatarInput = ref(null);
const avatarUploading = ref(false);
const avatarFailed = ref(false);
const draft = ref({
  username: "",
  name: "",
  email: "",
  time_format: "24h",
  timezone: "automatic",
  theme: "dark",
});
const notificationDraft = ref({ ...notificationPreferenceDefaults });

let savedTimer = null;

const detailFields = ["username", "name", "email"];
const displayFields = ["time_format", "timezone", "theme"];
const tabs = [
  { title: "Profile", value: "details" },
  { title: "Preferences", value: "preferences" },
  { title: "Notifications", value: "notifications" },
];

const timeFormatOptions = [
  { title: "12-hour", value: "12h" },
  { title: "24-hour", value: "24h" },
];
const themeOptions = [
  { title: "Dark", value: "dark" },
  { title: "Light", value: "light" },
];
const fallbackTimezones = [
  "UTC",
  "America/Chicago",
  "America/Denver",
  "America/Los_Angeles",
  "America/New_York",
  "Asia/Tokyo",
  "Australia/Sydney",
  "Europe/Amsterdam",
  "Europe/London",
];
const supportedTimezones = typeof Intl.supportedValuesOf === "function"
  ? Intl.supportedValuesOf("timeZone")
  : fallbackTimezones;
const timezoneOptions = [
  { title: "Automatic", value: "automatic" },
  ...supportedTimezones.map((timezone) => ({ title: timezone, value: timezone })),
];

function clearSaved() {
  saved.value = false;
  if (savedTimer !== null) {
    window.clearTimeout(savedTimer);
    savedTimer = null;
  }
}

watch(draft, () => {
  if (!saving.value) clearSaved();
}, { deep: true });

onUnmounted(() => {
  if (savedTimer !== null) window.clearTimeout(savedTimer);
});

async function resetScroll() {
  await nextTick();
  formScroll.value?.scrollTo({ top: 0 });
}

async function reportError(message) {
  error.value = message;
  await resetScroll();
}

async function loadPreferences() {
  loading.value = true;
  try {
    draft.value = { ...(await session.loadPreferences()) };
  } catch (loadError) {
    await reportError(loadError instanceof Error
      ? loadError.message
      : "User preferences could not be loaded.");
  } finally {
    loading.value = false;
  }
}

watch(() => props.modelValue, async (open) => {
  if (!open) return;
  tab.value = "details";
  notificationDraft.value = { ...notificationPreferenceDefaults };
  clearSaved();
  error.value = "";
  await resetScroll();
  await loadPreferences();
  await resetScroll();
});

function close() {
  if (saving.value) return;
  clearSaved();
  emit("update:modelValue", false);
}

/** Switching tabs discards unsaved edits on the tab being left. */
async function selectTab(nextTab) {
  if (nextTab === tab.value) return;
  clearSaved();
  error.value = "";
  if (tab.value === "notifications") {
    notificationDraft.value = { ...notificationPreferenceDefaults };
  } else if (session.preferencesBaseline) {
    const fieldsToReset = tab.value === "details" ? detailFields : displayFields;
    for (const field of fieldsToReset) {
      draft.value[field] = session.preferencesBaseline[field];
    }
  }
  tab.value = nextTab;
  await resetScroll();
}

async function save() {
  if (!session.preferencesBaseline) return;
  if (tab.value === "notifications") {
    clearSaved();
    await reportError("Notification preferences could not be saved.");
    return;
  }
  saving.value = true;
  clearSaved();
  error.value = "";
  try {
    const fields = tab.value === "details" ? detailFields : displayFields;
    const savedPreferences = await session.savePreferences(draft.value, fields);
    for (const field of fields) {
      draft.value[field] = savedPreferences[field];
    }
    saved.value = true;
    savedTimer = window.setTimeout(() => {
      saved.value = false;
      savedTimer = null;
    }, 2500);
  } catch (saveError) {
    await reportError(saveError instanceof Error
      ? saveError.message
      : "User preferences could not be saved.");
  } finally {
    saving.value = false;
  }
}

function chooseAvatar() {
  avatarInput.value?.click();
}

async function uploadAvatar(event) {
  const file = event.target.files?.[0];
  if (!file) return;
  error.value = "";
  if (!["image/jpeg", "image/png"].includes(file.type) || file.size > 5 * 1024 * 1024) {
    event.target.value = "";
    await reportError("Avatar must be a JPG or PNG file no larger than 5 MB.");
    return;
  }

  avatarUploading.value = true;
  try {
    await session.uploadAvatar(file);
    avatarFailed.value = false;
  } catch (uploadError) {
    await reportError(uploadError instanceof Error
      ? uploadError.message
      : "Avatar could not be uploaded.");
  } finally {
    avatarUploading.value = false;
    event.target.value = "";
  }
}
</script>

<template>
  <v-dialog
    :model-value="modelValue"
    aria-label="User preferences"
    max-width="620"
    @update:model-value="(open) => !open && close()"
  >
    <v-card v-if="session.user" class="user-preferences-dialog" rounded="lg">
      <form @submit.prevent="save">
        <v-card-text class="user-preferences-fields">
          <div class="user-preferences-profile">
            <button
              aria-label="Upload a new avatar"
              class="user-preferences-avatar-button"
              :disabled="avatarUploading"
              type="button"
              @click="chooseAvatar"
            >
              <img
                :src="session.user.avatar_url && !avatarFailed ? session.user.avatar_url : defaultUserIcon"
                alt=""
                class="user-preferences-avatar"
                referrerpolicy="no-referrer"
                @error="avatarFailed = true"
              >
              <span class="user-preferences-avatar-overlay">
                <v-icon :icon="mdiCamera" size="22" />
                <span>{{ avatarUploading ? "Uploading" : "Change" }}</span>
              </span>
            </button>
            <input
              ref="avatarInput"
              accept=".jpg,.jpeg,.png,image/jpeg,image/png"
              class="user-preferences-avatar-input"
              type="file"
              @change="uploadAvatar"
            >

            <div class="user-preferences-profile-text">
              <p class="user-preferences-real-name">
                {{ session.user.display_name || session.user.name }}
              </p>
              <p class="user-preferences-username">
                @{{ session.user.name }}
              </p>
            </div>
          </div>

          <div class="user-preferences-tabs" role="tablist" aria-label="User settings sections">
            <button
              v-for="preferenceTab in tabs"
              :id="`user-preferences-tab-${preferenceTab.value}`"
              :key="preferenceTab.value"
              :aria-controls="`user-preferences-panel-${preferenceTab.value}`"
              :aria-selected="tab === preferenceTab.value"
              :class="['user-preferences-tab', { 'user-preferences-tab--active': tab === preferenceTab.value }]"
              role="tab"
              type="button"
              @click="selectTab(preferenceTab.value)"
            >
              {{ preferenceTab.title }}
            </button>
          </div>

          <v-select
            aria-label="User settings section"
            class="user-preferences-tab-select"
            density="compact"
            hide-details
            :items="tabs"
            :model-value="tab"
            variant="plain"
            @update:model-value="selectTab"
          />

          <div ref="formScroll" class="user-preferences-form-scroll">
            <p v-if="error" class="user-preferences-error" role="alert">
              {{ error }}
            </p>

            <div
              v-if="tab === 'details'"
              id="user-preferences-panel-details"
              aria-labelledby="user-preferences-tab-details"
              class="user-preferences-tab-panel"
              role="tabpanel"
            >
              <div class="user-preferences-field">
                <label for="preference-username" class="service-field-label">Username</label>
                <p id="preference-username-help" class="service-field-help">
                  Choose the username shown across Kaeru apps
                </p>
                <v-text-field
                  id="preference-username"
                  v-model="draft.username"
                  aria-describedby="preference-username-help"
                  :disabled="loading || saving"
                  hide-details="auto"
                  variant="outlined"
                />
              </div>

              <div class="user-preferences-field">
                <label for="preference-name" class="service-field-label">Display Name</label>
                <p id="preference-name-help" class="service-field-help">
                  Add an optional real or display name
                </p>
                <v-text-field
                  id="preference-name"
                  v-model="draft.name"
                  aria-describedby="preference-name-help"
                  :disabled="loading || saving"
                  hide-details="auto"
                  placeholder="Name"
                  variant="outlined"
                />
              </div>

              <div class="user-preferences-field">
                <label for="preference-email" class="service-field-label">Email</label>
                <p id="preference-email-help" class="service-field-help">
                  Add an optional email address
                </p>
                <v-text-field
                  id="preference-email"
                  v-model="draft.email"
                  aria-describedby="preference-email-help"
                  :disabled="loading || saving"
                  hide-details="auto"
                  placeholder="name@example.com"
                  type="email"
                  variant="outlined"
                />
              </div>
            </div>

            <div
              v-else-if="tab === 'preferences'"
              id="user-preferences-panel-preferences"
              aria-labelledby="user-preferences-tab-preferences"
              class="user-preferences-tab-panel"
              role="tabpanel"
            >
              <div class="user-preferences-field">
                <label for="preference-time-format" class="service-field-label">Time format</label>
                <p id="preference-time-format-help" class="service-field-help">
                  Choose how times are displayed across Kaeru apps
                </p>
                <v-select
                  id="preference-time-format"
                  v-model="draft.time_format"
                  aria-describedby="preference-time-format-help"
                  :disabled="loading || saving"
                  :items="timeFormatOptions"
                  hide-details="auto"
                  variant="outlined"
                />
              </div>

              <div class="user-preferences-field">
                <label for="preference-timezone" class="service-field-label">Timezone</label>
                <p id="preference-timezone-help" class="service-field-help">
                  Use your device timezone automatically or select a specific timezone
                </p>
                <v-autocomplete
                  id="preference-timezone"
                  v-model="draft.timezone"
                  aria-describedby="preference-timezone-help"
                  :disabled="loading || saving"
                  :items="timezoneOptions"
                  hide-details="auto"
                  variant="outlined"
                />
              </div>

              <div class="user-preferences-field">
                <label for="preference-theme" class="service-field-label">Theme</label>
                <p id="preference-theme-help" class="service-field-help">
                  Choose a light or dark appearance
                </p>
                <v-select
                  id="preference-theme"
                  v-model="draft.theme"
                  aria-describedby="preference-theme-help"
                  :disabled="loading || saving"
                  :items="themeOptions"
                  hide-details="auto"
                  variant="outlined"
                />
              </div>
            </div>

            <div
              v-else
              id="user-preferences-panel-notifications"
              aria-labelledby="user-preferences-tab-notifications"
              class="user-preferences-tab-panel"
              role="tabpanel"
            >
              <div class="user-preferences-field">
                <label for="notification-preference-email" class="service-field-label">
                  Notification Email
                </label>
                <v-text-field
                  id="notification-preference-email"
                  v-model="notificationDraft.email"
                  hide-details="auto"
                  placeholder="notifications@example.com"
                  type="email"
                  variant="outlined"
                />
              </div>

              <div class="user-preferences-field">
                <label for="notification-preference-delivery" class="service-field-label">
                  Delivery Method
                </label>
                <v-select
                  id="notification-preference-delivery"
                  v-model="notificationDraft.deliveryMethod"
                  :items="notificationDeliveryMethods"
                  hide-details="auto"
                  variant="outlined"
                />
              </div>

              <div class="user-preferences-field">
                <label for="notification-preference-digest" class="service-field-label">
                  Digest Frequency
                </label>
                <v-select
                  id="notification-preference-digest"
                  v-model="notificationDraft.digestFrequency"
                  :items="notificationDigestFrequencies"
                  hide-details="auto"
                  variant="outlined"
                />
              </div>

              <div class="user-preferences-field">
                <label for="notification-preference-quiet-start" class="service-field-label">
                  Quiet Hours Start
                </label>
                <v-text-field
                  id="notification-preference-quiet-start"
                  v-model="notificationDraft.quietHoursStart"
                  hide-details="auto"
                  type="time"
                  variant="outlined"
                />
              </div>

              <div class="user-preferences-field">
                <label for="notification-preference-quiet-end" class="service-field-label">
                  Quiet Hours End
                </label>
                <v-text-field
                  id="notification-preference-quiet-end"
                  v-model="notificationDraft.quietHoursEnd"
                  hide-details="auto"
                  type="time"
                  variant="outlined"
                />
              </div>

              <div class="user-preferences-field">
                <label for="notification-preference-severity" class="service-field-label">
                  Minimum Severity
                </label>
                <v-select
                  id="notification-preference-severity"
                  v-model="notificationDraft.minimumSeverity"
                  :items="notificationSeverityOptions"
                  hide-details="auto"
                  variant="outlined"
                />
              </div>

              <div class="user-preferences-field">
                <label for="notification-preference-services" class="service-field-label">
                  Service Status Alerts
                </label>
                <v-select
                  id="notification-preference-services"
                  v-model="notificationDraft.serviceStatusAlerts"
                  :items="notificationToggleOptions"
                  hide-details="auto"
                  variant="outlined"
                />
              </div>

              <div class="user-preferences-field">
                <label for="notification-preference-backups" class="service-field-label">
                  Backup Alerts
                </label>
                <v-select
                  id="notification-preference-backups"
                  v-model="notificationDraft.backupAlerts"
                  :items="notificationToggleOptions"
                  hide-details="auto"
                  variant="outlined"
                />
              </div>

              <div class="user-preferences-field">
                <label for="notification-preference-security" class="service-field-label">
                  Security Alerts
                </label>
                <v-select
                  id="notification-preference-security"
                  v-model="notificationDraft.securityAlerts"
                  :items="notificationToggleOptions"
                  hide-details="auto"
                  variant="outlined"
                />
              </div>

              <div class="user-preferences-field">
                <label for="notification-preference-webhook" class="service-field-label">
                  Webhook URL
                </label>
                <v-text-field
                  id="notification-preference-webhook"
                  v-model="notificationDraft.webhookUrl"
                  hide-details="auto"
                  placeholder="https://example.com/notifications"
                  type="url"
                  variant="outlined"
                />
              </div>
            </div>

            <v-card-actions class="user-preferences-actions user-preferences-actions--mobile">
              <v-btn :disabled="saving" variant="text" @click="close">
                Close
              </v-btn>
              <v-spacer />
              <v-btn
                :aria-label="saved ? 'User settings saved' : 'Save user settings'"
                color="primary"
                :disabled="loading"
                :loading="saving"
                type="submit"
                variant="flat"
                width="80"
              >
                <v-icon v-if="saved" :icon="mdiCheck" />
                <span v-else>Save</span>
              </v-btn>
            </v-card-actions>
          </div>
        </v-card-text>
        <v-card-actions class="user-preferences-actions user-preferences-actions--desktop">
          <v-btn :disabled="saving" variant="text" @click="close">
            Close
          </v-btn>
          <v-spacer />
          <v-btn
            :aria-label="saved ? 'User settings saved' : 'Save user settings'"
            color="primary"
            :disabled="loading"
            :loading="saving"
            type="submit"
            variant="flat"
            width="80"
          >
            <v-icon v-if="saved" :icon="mdiCheck" />
            <span v-else>Save</span>
          </v-btn>
        </v-card-actions>
      </form>
    </v-card>
  </v-dialog>
</template>

<style scoped>
.user-preferences-dialog {
  display: flex;
  height: min(780px, calc(100vh - 32px));
  height: min(780px, calc(100dvh - 32px));
  min-height: min(780px, calc(100vh - 32px));
  min-height: min(780px, calc(100dvh - 32px));
  max-height: min(780px, calc(100vh - 32px));
  max-height: min(780px, calc(100dvh - 32px));
  overflow: hidden;
}

.user-preferences-dialog form {
  display: flex;
  flex: 1 1 auto;
  min-height: 0;
  flex-direction: column;
}

.user-preferences-fields {
  display: flex;
  flex: 1 1 auto;
  gap: 22px;
  min-height: 0;
  flex-direction: column;
  overflow: hidden;
  padding: 24px;
}

.user-preferences-field {
  min-width: 0;
}

.user-preferences-tabs {
  display: flex;
  flex: 0 0 auto;
  overflow-x: auto;
  margin-top: -22px;
  border-bottom: 1px solid rgba(var(--v-theme-on-surface), 0.14);
}

.user-preferences-tab-select {
  display: none;
  flex: 0 0 auto;
  margin-top: -22px;
}

.user-preferences-tab {
  position: relative;
  flex: 1 0 auto;
  min-height: 34px;
  padding: 4px 10px;
  border: 0;
  background: transparent;
  color: rgba(var(--v-theme-on-surface), 0.66);
  font: inherit;
  font-size: 0.875rem;
  font-weight: 600;
  transition: color 150ms ease, background-color 150ms ease;
}

.user-preferences-tab:not(.user-preferences-tab--active):hover {
  background: rgba(var(--v-theme-on-surface), 0.05);
  color: rgb(var(--v-theme-on-surface));
}

.user-preferences-tab--active:hover {
  background: rgba(var(--v-theme-on-surface), 0.05);
  color: rgb(var(--v-theme-primary));
}

.user-preferences-tab::after {
  position: absolute;
  right: 10px;
  bottom: 0;
  left: 10px;
  height: 2px;
  border-radius: 2px 2px 0 0;
  background: rgb(var(--v-theme-primary));
  content: "";
  opacity: 0;
}

.user-preferences-tab--active {
  color: rgb(var(--v-theme-primary));
}

.user-preferences-tab--active::after {
  opacity: 1;
}

.user-preferences-tab:focus-visible {
  outline: 2px solid rgb(var(--v-theme-primary));
  outline-offset: -2px;
}

.user-preferences-tab-panel {
  display: grid;
  gap: 22px;
  align-content: start;
}

.user-preferences-tab-panel--empty {
  min-height: 120px;
}

.user-preferences-form-scroll {
  display: grid;
  flex: 1 1 auto;
  gap: 22px;
  align-content: start;
  min-height: 0;
  margin-right: -24px;
  overflow-y: auto;
  padding-right: 24px;
}

.user-preferences-actions--mobile {
  display: none !important;
}

.user-preferences-dialog .service-field-label {
  color: rgb(var(--v-theme-primary));
}

.user-preferences-dialog .service-field-help {
  margin: 0 0 10px;
  color: rgba(var(--v-theme-on-surface), 0.65);
  font-size: 0.875rem;
  line-height: 1.4;
}

.user-preferences-error {
  margin: 0;
  padding: 12px 14px;
  border: 1px solid rgba(var(--v-theme-error), 0.5);
  border-radius: 6px;
  background: rgba(var(--v-theme-error), 0.1);
  color: rgb(var(--v-theme-error));
}

.user-preferences-profile {
  display: flex;
  flex: 0 0 auto;
  gap: 20px;
  align-items: center;
  padding-bottom: 22px;
  border-bottom: 1px solid rgba(var(--v-theme-on-surface), 0.1);
}

.user-preferences-avatar-button {
  display: block;
  position: relative;
  flex: 0 0 auto;
  width: 68px;
  height: 68px;
  padding: 0;
  overflow: hidden;
  border: 0;
  border-radius: 50%;
  background: rgb(var(--v-theme-surface));
  cursor: pointer;
}

.user-preferences-avatar-button:focus-visible {
  outline: 3px solid rgb(var(--v-theme-primary));
  outline-offset: 3px;
}

.user-preferences-avatar-button:disabled {
  cursor: progress;
}

.user-preferences-avatar {
  display: block;
  width: 100%;
  height: 100%;
  object-fit: cover;
}

.user-preferences-avatar-overlay {
  display: flex;
  position: absolute;
  inset: 0;
  align-items: center;
  justify-content: center;
  flex-direction: column;
  gap: 2px;
  background: rgba(0, 0, 0, 0.62);
  color: #FFFFFF;
  font-size: 0.75rem;
  font-weight: 600;
  opacity: 0;
  transition: opacity 150ms ease;
}

.user-preferences-avatar-button:hover .user-preferences-avatar-overlay,
.user-preferences-avatar-button:focus-visible .user-preferences-avatar-overlay,
.user-preferences-avatar-button:disabled .user-preferences-avatar-overlay {
  opacity: 1;
}

.user-preferences-avatar-input {
  display: none;
}

.user-preferences-profile-text {
  min-width: 0;
}

.user-preferences-profile-text p {
  margin: 0;
  line-height: 1.4;
  overflow-wrap: anywhere;
}

.user-preferences-real-name {
  color: rgb(var(--v-theme-on-surface));
  font-size: 1.3rem;
  font-weight: 600;
}

.user-preferences-username {
  color: rgba(var(--v-theme-on-surface), 0.68);
  font-size: 0.875rem;
}

@media (max-width: 480px) {
  .user-preferences-avatar-button {
    width: 72px;
    height: 72px;
  }

  .user-preferences-tab {
    padding-inline: 8px;
  }
}

@media (max-width: 600px) {
  .user-preferences-tabs {
    display: none;
  }

  .user-preferences-tab-select {
    display: block;
  }

  .user-preferences-actions--desktop {
    display: none !important;
  }

  .user-preferences-actions--mobile {
    display: flex !important;
    padding: 8px 0 0 !important;
  }
}

.user-preferences-dialog :deep(.v-card-actions) {
  flex: 0 0 auto;
  padding: 8px 16px 16px;
}
</style>
