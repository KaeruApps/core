<script setup>
import { computed, reactive, ref } from "vue";
import { mdiBackupRestore, mdiCalendarClock, mdiChevronDown, mdiDatabaseArrowUpOutline, mdiDownload } from "@mdi/js";

// MOCK-BACKED: Kaeru Core has no backup API yet, so nothing in this section
// reaches the server. See src/mocks/README.md.
import { availableBackups, backupScheduleOptions, backupSummary as initialSummary } from "../../mocks/backup.js";
import { useServicesStore } from "../../stores/services.js";

const services = useServicesStore();

const summary = reactive({ ...initialSummary });

const includedServices = ref([]);
const includedServicesLoading = ref(false);
const includedServicesError = ref("");

/**
 * Loads the services that can take part in a backup and seeds each selection
 * from the option the service marks as its default.
 */
async function loadIncludedServices() {
  includedServicesLoading.value = true;
  includedServicesError.value = "";
  try {
    const entries = await services.loadBackupOptions();
    includedServices.value = entries.map((entry) => ({
      ...entry,
      included: entry.available,
      selectedOptionId: entry.options.find((option) => option.default)?.id
        ?? entry.options[0]?.id
        ?? null,
    }));
  } catch (error) {
    includedServicesError.value = error instanceof Error
      ? error.message
      : "Unable to load backup options.";
    includedServices.value = [];
  } finally {
    includedServicesLoading.value = false;
  }
}

function backupOptionItems(entry) {
  return entry.options.map((option) => ({ title: option.option, value: option.id }));
}

const configurationOpen = ref(false);
const downloadOpen = ref(false);
const restoreOpen = ref(false);
const selectedBackup = ref(summary.file);
const restoreBackupFile = ref(null);
const selectedRestoreServiceIds = ref([]);
const configurationDraft = ref({
  automatic: true,
  schedule: "Every day",
  scheduledTime: summary.scheduledTime,
  retention: Number(summary.retention),
});

const restoreFileSelected = computed(() => (
  Array.isArray(restoreBackupFile.value)
    ? restoreBackupFile.value.length > 0
    : Boolean(restoreBackupFile.value)
));

function openConfiguration() {
  loadIncludedServices();
  configurationDraft.value = {
    automatic: summary.automatic === "Enabled",
    schedule: summary.schedule,
    scheduledTime: summary.scheduledTime,
    retention: summary.retention === "" ? null : Number(summary.retention),
  };
  configurationOpen.value = true;
}

function cancelConfiguration() {
  configurationOpen.value = false;
}

/** MOCK: updates the local summary only. */
function saveConfiguration() {
  summary.automatic = configurationDraft.value.automatic ? "Enabled" : "Disabled";
  summary.schedule = configurationDraft.value.schedule;
  summary.scheduledTime = configurationDraft.value.scheduledTime;
  summary.retention = configurationDraft.value.retention == null
    || configurationDraft.value.retention === ""
    ? ""
    : String(configurationDraft.value.retention);
  configurationOpen.value = false;
}

function openDownload() {
  selectedBackup.value = summary.file;
  downloadOpen.value = true;
}

function cancelDownload() {
  downloadOpen.value = false;
}

/** MOCK: no backup is downloaded. */
function downloadSelectedBackup() {
  downloadOpen.value = false;
}

function openRestore() {
  restoreBackupFile.value = null;
  selectedRestoreServiceIds.value = services.services.map((service) => service.id);
  restoreOpen.value = true;
}

function cancelRestore() {
  restoreOpen.value = false;
  restoreBackupFile.value = null;
}

/** MOCK: no restore is performed. */
function restoreSelectedBackup() {
  restoreOpen.value = false;
  restoreBackupFile.value = null;
}
</script>

<template>
  <section aria-labelledby="backup-title" class="home-section backup-section">
    <div class="home-section-header home-section-header--stacked">
      <div class="home-section-heading">
        <div class="home-section-title-row">
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
              <v-list-item :prepend-icon="mdiDatabaseArrowUpOutline" link title="Backup now" />
              <v-list-item
                :prepend-icon="mdiDownload"
                link
                title="Download"
                @click="openDownload"
              />
              <v-list-item
                :prepend-icon="mdiBackupRestore"
                link
                title="Restore"
                @click="openRestore"
              />
              <v-list-item
                :prepend-icon="mdiCalendarClock"
                link
                title="Automatic Backups"
                @click="openConfiguration"
              />
            </v-list>
          </v-menu>
        </div>
        <p class="home-section-subtitle">
          Configure automatic backups, download or restore backups, or make a new backup
        </p>
      </div>
    </div>

    <v-sheet class="backup-summary" border rounded="lg">
      <dl class="backup-details">
        <div class="backup-detail">
          <dt>Automatic backups</dt>
          <dd>
            {{ summary.automatic }} ({{ summary.schedule }} at {{ summary.scheduledTime }}, {{ summary.retention === "" ? "Unlimited" : `${summary.retention} days` }} retention)
          </dd>
        </div>
        <div class="backup-detail">
          <dt>Last backup</dt>
          <dd>
            <time :datetime="summary.lastBackupAt">{{ summary.lastBackup }}</time>
          </dd>
        </div>
        <div class="backup-detail">
          <dt>Backup directory</dt>
          <dd>{{ summary.path }}</dd>
        </div>
        <div class="backup-detail">
          <dt>Last backup file</dt>
          <dd>{{ summary.file }} ({{ summary.fileSize }})</dd>
        </div>
        <div class="backup-detail">
          <dt>Next backup</dt>
          <dd>
            <time :datetime="summary.nextBackupAt">{{ summary.nextBackup }}</time>
          </dd>
        </div>
      </dl>
    </v-sheet>
  </section>

  <v-dialog
    :model-value="configurationOpen"
    max-width="520"
    @update:model-value="(open) => !open && cancelConfiguration()"
  >
    <v-card class="backup-configuration-dialog" rounded="lg">
      <form @submit.prevent="saveConfiguration">
        <v-card-title>Automatic Backups</v-card-title>
        <v-card-text class="backup-configuration-fields">
          <v-switch
            v-model="configurationDraft.automatic"
            color="primary"
            hide-details
            label="Enabled"
          />


          <div class="backup-configuration-field">
            <label for="backup-schedule" class="service-field-label">Schedule</label>
            <p id="backup-schedule-help" class="backup-field-help">
              Choose how often automatic backups run
            </p>
            <v-select
              id="backup-schedule"
              v-model="configurationDraft.schedule"
              aria-describedby="backup-schedule-help"
              :disabled="!configurationDraft.automatic"
              :items="backupScheduleOptions"
              hide-details="auto"
              variant="outlined"
            />
          </div>
          <div class="backup-configuration-field">
            <label for="backup-scheduled-time" class="service-field-label">Time</label>
            <p id="backup-scheduled-time-help" class="backup-field-help">
              Choose the time of day for automatic backups
            </p>
            <v-text-field
              id="backup-scheduled-time"
              v-model="configurationDraft.scheduledTime"
              aria-describedby="backup-scheduled-time-help"
              :disabled="!configurationDraft.automatic"
              hide-details="auto"
              step="60"
              type="time"
              variant="outlined"
            />
          </div>
          <div class="backup-configuration-field">
            <label for="backup-retention" class="service-field-label">Retention</label>
            <p id="backup-retention-help" class="backup-field-help">
              Leave empty for unlimited retention. Manual backups are never
              deleted
            </p>
            <v-text-field
              id="backup-retention"
              v-model.number="configurationDraft.retention"
              aria-describedby="backup-retention-help"
              :disabled="!configurationDraft.automatic"
              hide-details="auto"
              min="1"
              placeholder="Unlimited"
              step="1"
              suffix="days"
              type="number"
              variant="outlined"
            />
          </div>

          <fieldset class="included-services">
            <legend class="service-field-label">Included Services</legend>
            <p class="backup-field-help">
              Select services and backup type included in automatic backups
            </p>

            <p v-if="includedServicesLoading" class="included-services-message">
              Loading services…
            </p>
            <p
              v-else-if="includedServicesError"
              class="included-services-message included-services-message--error"
              role="alert"
            >
              {{ includedServicesError }}
            </p>
            <ul v-else class="included-service-list">
              <li
                v-for="entry in includedServices"
                :key="entry.service_id"
                class="included-service"
              >
                <v-checkbox
                  v-model="entry.included"
                  class="included-service-checkbox"
                  color="primary"
                  density="compact"
                  :disabled="!configurationDraft.automatic || !entry.available"
                  hide-details
                  :aria-label="`Include ${entry.service_name} in automatic backups`"
                />
                <div class="included-service-identity">
                  <span class="included-service-name">{{ entry.service_name }}</span>
                  <small v-if="!entry.available" class="included-service-unavailable">
                    {{ entry.unavailable_reason }}
                  </small>
                </div>
                <v-select
                  v-model="entry.selectedOptionId"
                  class="included-service-option"
                  density="compact"
                  :disabled="!configurationDraft.automatic || !entry.available || !entry.included"
                  hide-details
                  :items="backupOptionItems(entry)"
                  :aria-label="`${entry.service_name} backup type`"
                  no-data-text="No backup options"
                  variant="outlined"
                />
              </li>
            </ul>
          </fieldset>
        </v-card-text>
        <v-card-actions>
          <v-spacer />
          <v-btn variant="text" @click="cancelConfiguration">Cancel</v-btn>
          <v-btn color="primary" type="submit" variant="flat">Save</v-btn>
        </v-card-actions>
      </form>
    </v-card>
  </v-dialog>

  <v-dialog
    :model-value="restoreOpen"
    max-width="560"
    @update:model-value="(open) => !open && cancelRestore()"
  >
    <v-card class="restore-backup-dialog" rounded="lg">
      <form @submit.prevent="restoreSelectedBackup">
        <v-card-title>Restore Backup</v-card-title>
        <v-card-text class="restore-backup-fields">
          <div class="backup-configuration-field">
            <label for="restore-backup-file" class="service-field-label">Backup</label>
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

          <fieldset v-if="restoreFileSelected" class="restore-service-selection">
            <legend class="service-field-label">Services</legend>
            <p class="backup-field-help">
              Select the services you wish to restore
            </p>
            <div class="restore-service-list">
              <v-checkbox
                v-for="service in services.services"
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
          <v-btn variant="text" @click="cancelRestore">Cancel</v-btn>
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
    :model-value="downloadOpen"
    max-width="520"
    @update:model-value="(open) => !open && cancelDownload()"
  >
    <v-card class="download-backup-dialog" rounded="lg">
      <form @submit.prevent="downloadSelectedBackup">
        <v-card-title>Download Backup</v-card-title>
        <v-card-text class="download-backup-fields">
          <div class="backup-configuration-field">
            <label for="download-backup-selection" class="service-field-label">Backup</label>
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
          <v-btn variant="text" @click="cancelDownload">Cancel</v-btn>
          <v-btn color="primary" type="submit" variant="flat">Download</v-btn>
        </v-card-actions>
      </form>
    </v-card>
  </v-dialog>
</template>

<style scoped>
.backup-section {
  margin-top: 48px;
}

.backup-options-menu :deep(.v-list-item) {
  cursor: pointer;
  transition: background-color 150ms ease;
}

.backup-options-menu :deep(.v-list-item):hover,
.backup-options-menu :deep(.v-list-item):focus-visible {
  background: rgba(var(--v-theme-on-surface), 0.1);
}

.backup-configuration-dialog :deep(.v-card-title) {
  padding: 24px 24px 0;
  white-space: normal;
}

.backup-configuration-fields {
  display: grid;
  gap: 16px;
  padding: 20px 24px 24px;
}

.backup-configuration-field {
  min-width: 0;
}

.backup-configuration-field .service-field-label {
  color: rgb(var(--v-theme-primary));
}

.backup-configuration-dialog :deep(.v-card-actions) {
  padding: 8px 16px 16px;
}

.download-backup-dialog :deep(.v-card-title) {
  padding: 24px 24px 0;
  white-space: normal;
}

.download-backup-fields {
  padding: 24px;
}

.download-backup-dialog :deep(.v-card-actions) {
  padding: 8px 16px 16px;
}

.restore-backup-dialog :deep(.v-card-title) {
  padding: 24px 24px 0;
  white-space: normal;
}

.restore-backup-fields {
  display: grid;
  gap: 22px;
  padding: 24px;
}

.restore-service-selection {
  min-width: 0;
  margin: 0;
  padding: 0;
  border: 0;
}

.restore-service-selection .service-field-label {
  color: rgb(var(--v-theme-primary));
}

.restore-service-list {
  display: grid;
  gap: 2px;
}

.restore-backup-dialog :deep(.v-card-actions) {
  padding: 8px 16px 16px;
}

.backup-summary {
  padding: 24px;
  background: rgb(var(--v-theme-surface));
  cursor: default;
  user-select: none;
}

.backup-details {
  display: grid;
  gap: 0;
  margin: 0;
}

.backup-detail {
  display: grid;
  grid-template-columns: minmax(220px, 0.3fr) minmax(0, 1fr);
  gap: 24px;
  padding: 14px 0;
  border-bottom: 1px solid rgba(var(--v-theme-on-surface), 0.08);
  font-size: 0.875rem;
  line-height: 1.5;
}

.backup-detail:first-child {
  padding-top: 0;
}

.backup-detail:last-child {
  padding-bottom: 0;
  border-bottom: 0;
}

.backup-detail dt {
  color: rgba(var(--v-theme-on-surface), 0.62);
  font-weight: 600;
}

.backup-detail dd {
  min-width: 0;
  margin: 0;
  color: rgb(var(--v-theme-on-surface));
  overflow-wrap: anywhere;
}

@media (max-width: 600px) {
  .backup-detail {
    grid-template-columns: 1fr;
    gap: 4px;
  }
}

.included-services {
  min-width: 0;
  margin: 0;
  padding: 0;
  border: 0;
}

.included-services .service-field-label {
  color: rgb(var(--v-theme-primary));
}

.included-service-list {
  display: grid;
  gap: 4px;
  margin: 6px 0 0;
  padding: 0;
  list-style: none;
}

.included-service {
  display: grid;
  grid-template-columns: auto minmax(0, 1fr) minmax(0, 15rem);
  gap: 12px;
  align-items: center;
}

.included-service-checkbox {
  flex: 0 0 auto;
}

.included-service-identity {
  display: grid;
  min-width: 0;
}

.included-service-name {
  color: rgb(var(--v-theme-on-surface));
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.included-service-unavailable {
  color: rgba(var(--v-theme-on-surface), 0.6);
  font-size: 0.8125rem;
  line-height: 1.3;
}

.included-services-message {
  margin: 6px 0 0;
  color: rgba(var(--v-theme-on-surface), 0.68);
  font-size: 0.875rem;
}

.included-services-message--error {
  color: rgb(var(--v-theme-error));
}

@media (max-width: 600px) {
  .included-service {
    grid-template-columns: auto minmax(0, 1fr);
  }

  .included-service-option {
    grid-column: 2;
  }
}
</style>
