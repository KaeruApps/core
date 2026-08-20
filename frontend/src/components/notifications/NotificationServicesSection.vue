<script setup>
import { ref } from "vue";

// MOCK-BACKED: every provider, save, and test in this section is placeholder
// behaviour until Kaeru Core exposes a notification API. See src/mocks/README.md.
import { createNotificationServices } from "../../mocks/notifications.js";
import { useUsersStore } from "../../stores/users.js";

const users = useUsersStore();

const notificationServices = ref(createNotificationServices());
const serviceToEdit = ref(null);
const testOpen = ref(false);
const testEmailAddress = ref("");
const testDevice = ref(null);
const providerDraft = ref({
  enabled: false,
  url: "",
  host: "",
  port: null,
  username: "",
  password: "",
  fromAddress: "",
});

function openEditor(notificationService) {
  serviceToEdit.value = notificationService;
  providerDraft.value = {
    enabled: notificationService.enabled,
    host: notificationService.host ?? "",
    port: notificationService.port ?? null,
    username: notificationService.username ?? "",
    password: notificationService.password ?? "",
    fromAddress: notificationService.fromAddress ?? "",
  };
}

function cancelEditor() {
  serviceToEdit.value = null;
}

/** MOCK: updates the local list only, nothing is sent to Kaeru Core. */
function saveNotificationService() {
  const notificationService = notificationServices.value.find(
    (candidate) => candidate.id === serviceToEdit.value?.id,
  );
  if (notificationService) {
    Object.assign(notificationService, providerDraft.value);
  }
  cancelEditor();
}

function openTest() {
  testEmailAddress.value = "";
  testDevice.value = null;
  testOpen.value = true;
}

function cancelTest() {
  testOpen.value = false;
}

/** MOCK: no test notification is sent. */
function sendTest() {
  testOpen.value = false;
}
</script>

<template>
  <section
    aria-labelledby="notification-services-title"
    class="home-section notification-services-section"
  >
    <div class="home-section-header">
      <div class="home-section-heading">
        <h2 id="notification-services-title" class="home-section-title">
          Notification Services
        </h2>
        <p class="home-section-subtitle">
          Configure the notification services that will be available to your
          Kaeru services and users.
        </p>
      </div>
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
        @click="openEditor(notificationService)"
        @keydown.enter="openEditor(notificationService)"
        @keydown.space.prevent="openEditor(notificationService)"
      >
        <img :src="notificationService.iconUrl" alt="" class="notification-service-icon" />
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
    :model-value="serviceToEdit !== null"
    max-width="560"
    @update:model-value="(open) => !open && cancelEditor()"
  >
    <v-card v-if="serviceToEdit" class="notification-provider-dialog" rounded="lg">
      <v-card-title>
        Configure {{ serviceToEdit.name }} Notifications
      </v-card-title>
      <v-card-text class="notification-provider-fields">
        <v-switch
          v-model="providerDraft.enabled"
          color="primary"
          hide-details
          label="Enabled"
        />

        <template v-if="serviceToEdit.id === 'email'">
          <div class="service-field">
            <label for="notification-email-host" class="service-field-label">Host</label>
            <p id="notification-email-host-help" class="notification-field-help">
              Host of the email server (e.g. smtp.example.com)
            </p>
            <v-text-field
              id="notification-email-host"
              v-model="providerDraft.host"
              aria-describedby="notification-email-host-help"
              :disabled="!providerDraft.enabled"
              hide-details="auto"
              placeholder="smtp.example.com"
              variant="outlined"
            />
          </div>
          <div class="service-field">
            <label for="notification-email-port" class="service-field-label">Port</label>
            <p id="notification-email-port-help" class="notification-field-help">
              Port of the email server (e.g. 25, 465, or 587)
            </p>
            <v-text-field
              id="notification-email-port"
              v-model="providerDraft.port"
              aria-describedby="notification-email-port-help"
              :disabled="!providerDraft.enabled"
              hide-details="auto"
              placeholder="587"
              variant="outlined"
            />
          </div>
          <div class="service-field">
            <label for="notification-email-username" class="service-field-label">
              Username
            </label>
            <p id="notification-email-username-help" class="notification-field-help">
              Username to use when authenticating with the email server
            </p>
            <v-text-field
              id="notification-email-username"
              v-model="providerDraft.username"
              aria-describedby="notification-email-username-help"
              :disabled="!providerDraft.enabled"
              hide-details="auto"
              placeholder="Username"
              variant="outlined"
            />
          </div>
          <div class="service-field">
            <label for="notification-email-password" class="service-field-label">
              Password
            </label>
            <p id="notification-email-password-help" class="notification-field-help">
              Password to use when authenticating with the email server
            </p>
            <v-text-field
              id="notification-email-password"
              v-model="providerDraft.password"
              aria-describedby="notification-email-password-help"
              :disabled="!providerDraft.enabled"
              hide-details="auto"
              placeholder="Password"
              type="password"
              variant="outlined"
            />
          </div>
          <div class="service-field">
            <label for="notification-email-from-address" class="service-field-label">
              From Address
            </label>
            <p id="notification-email-from-address-help" class="notification-field-help">
              Sender email address (e.g "Kaeru &lt;kaeru@example.com&gt;")
            </p>
            <v-text-field
              id="notification-email-from-address"
              v-model="providerDraft.fromAddress"
              aria-describedby="notification-email-from-address-help"
              :disabled="!providerDraft.enabled"
              hide-details="auto"
              placeholder="Kaeru &lt;kaeru@example.com&gt;"
              variant="outlined"
            />
          </div>
        </template>
      </v-card-text>
      <v-card-actions>
        <v-btn variant="text" @click="cancelEditor">Cancel</v-btn>
        <div class="notification-provider-primary-actions">
          <v-btn variant="text" @click="openTest">Test</v-btn>
          <v-btn color="primary" variant="flat" @click="saveNotificationService">
            Save
          </v-btn>
        </div>
      </v-card-actions>
    </v-card>
  </v-dialog>

  <v-dialog
    :model-value="testOpen"
    max-width="520"
    @update:model-value="(open) => !open && cancelTest()"
  >
    <v-card v-if="serviceToEdit" class="notification-test-dialog" rounded="lg">
      <form @submit.prevent="sendTest">
        <v-card-title>
          {{ serviceToEdit.id === "email" ? "Send Test Email" : "Send Test Notification" }}
        </v-card-title>
        <v-card-text class="notification-test-fields">
          <div v-if="serviceToEdit.id === 'email'" class="service-field">
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
            <label for="test-device" class="service-field-label">Device</label>
            <p id="test-device-help" class="notification-field-help">
              Choose a device to send the test notification to
            </p>
            <v-select
              id="test-device"
              v-model="testDevice"
              aria-describedby="test-device-help"
              :items="users.deviceNames"
              hide-details="auto"
              no-data-text="No registered devices"
              placeholder="Select a device"
              variant="outlined"
            />
          </div>
        </v-card-text>
        <v-card-actions>
          <v-spacer />
          <v-btn variant="text" @click="cancelTest">Cancel</v-btn>
          <v-btn
            :disabled="serviceToEdit.id === 'email'
              ? testEmailAddress.trim() === ''
              : testDevice === null"
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
</template>

<style scoped>
.notification-services-section {
  margin-top: 48px;
}

.notification-service-grid {
  display: grid;
  grid-template-columns: repeat(4, minmax(0, 1fr));
  gap: 20px;
}

.notification-service-card {
  display: flex;
  position: relative;
  min-height: 72px;
  align-items: center;
  gap: 14px;
  padding: 16px 20px;
  background: color-mix(
    in srgb,
    rgb(var(--v-theme-surface)) 75%,
    rgb(var(--v-theme-background))
  );
  cursor: pointer;
  user-select: none;
  transition: background-color 150ms ease, border-color 150ms ease;
}

.notification-service-card:hover,
.notification-service-card:focus-within {
  background: rgb(var(--v-theme-surface));
  border-color: rgba(var(--v-theme-on-surface), 0.16);
}

.notification-service-card:focus-visible {
  outline: 2px solid rgb(var(--v-theme-primary));
  outline-offset: 2px;
}

.notification-service-icon {
  width: 36px;
  height: 36px;
  flex: 0 0 36px;
  object-fit: contain;
}

.notification-service-name {
  margin: 0;
  color: rgb(var(--v-theme-on-surface));
  font-size: 1rem;
  font-weight: 700;
  line-height: 1.4;
}

.notification-service-status {
  margin: 2px 0 0;
  color: rgb(var(--v-theme-primary));
  font-size: 0.8rem;
  font-weight: 600;
  line-height: 1.4;
}

.notification-service-status--disabled {
  color: rgba(var(--v-theme-on-surface), 0.58);
}

.notification-provider-dialog :deep(.v-card-title) {
  padding: 24px 24px 0;
  white-space: normal;
}

.notification-provider-fields {
  display: grid;
  gap: 18px;
  padding: 24px;
}

.notification-provider-dialog .service-field-label {
  color: rgb(var(--v-theme-primary));
}

.notification-field-help {
  margin: 0 0 10px;
  color: rgba(var(--v-theme-on-surface), 0.65);
  font-size: 0.875rem;
  line-height: 1.4;
}

.notification-provider-dialog :deep(.v-card-actions) {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
  justify-content: space-between;
  padding: 8px 16px 16px;
}

.notification-provider-primary-actions {
  display: flex;
  gap: 4px;
  margin-left: auto;
}

.notification-test-dialog :deep(.v-card-title) {
  padding: 24px 24px 0;
  white-space: normal;
}

.notification-test-fields {
  padding: 24px;
}

.notification-test-dialog .service-field-label {
  color: rgb(var(--v-theme-primary));
}

.notification-test-dialog :deep(.v-card-actions) {
  padding: 8px 16px 16px;
}

@media (max-width: 1200px) {
  .notification-service-grid {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }
}

@media (max-width: 600px) {
  .notification-service-grid {
    grid-template-columns: 1fr;
  }
}
</style>
