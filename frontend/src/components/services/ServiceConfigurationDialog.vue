<script setup>
import { computed, nextTick, ref, watch } from "vue";
import { mdiClose, mdiPlus } from "@mdi/js";

import defaultServiceIcon from "../../assets/app-icon.svg";

import { normalizeService, useServicesStore } from "../../stores/services.js";
import { useOIDCStore } from "../../stores/oidc.js";

const props = defineProps({
  service: { type: Object, default: null },
});
const emit = defineEmits(["close"]);

const services = useServicesStore();
const oidc = useOIDCStore();

const loading = ref(false);
const saving = ref(false);
const error = ref("");
const fieldsScroll = ref(null);
const serviceToDelete = ref(null);
const deletionSaving = ref(false);
const draft = ref({
  applicationUrl: "",
  defaultUserRole: null,
  roleMappings: [],
  alternateUrls: [],
});

let nextAlternateUrlId = 1;

const isCore = computed(() => props.service?.service_type === "core");

/**
 * Kaeru Core defines the groups, so its config can add and rename them. Every
 * other service is handed those groups read-only and only fills in its own URL.
 */
const alternateUrlHelp = computed(() => (isCore.value
  ? "Define alternate URLs. Each Alternate URL has a group it belongs to. Every service can provide it's own alternate URL for the group which will be used if accessing Kaeru via that alternate URL."
  : "Add alternate URLs corresponding to your alternate URL groups. If not provided, the Application URL will be used by default. Groups are managed in the Kaeru Core service config."));

/**
 * Core sees the groups it has defined; every other service sees the same groups
 * with whichever URL it has supplied for them.
 */
function alternateUrlRowsFrom(alternates) {
  return (alternates ?? []).map((alternate) => ({
    id: `alternate-url-${nextAlternateUrlId++}`,
    groupId: alternate.group_id,
    group: alternate.group,
    url: alternate.url ?? "",
  }));
}

function addAlternateUrl() {
  draft.value.alternateUrls.push({
    id: `alternate-url-${nextAlternateUrlId++}`,
    groupId: 0,
    group: "",
    url: "",
  });
}

function removeAlternateUrl(rowID) {
  draft.value.alternateUrls = draft.value.alternateUrls.filter((row) => row.id !== rowID);
}


const activeRoles = computed(() => (props.service?.roles ?? []).filter((role) => role.active));
const roleOptions = computed(() => [
  { title: "No Access", value: null },
  ...activeRoles.value.map((role) => ({ title: role.name, value: role.key })),
]);
/**
 * One row per role the service provides, in the order the service published
 * them. Supplying OIDC groups is optional: a role left blank simply has no
 * mapping.
 */
function roleMappingRowsFor(service, mappings) {
  const groupsByRole = new Map(
    (mappings ?? []).map((mapping) => [mapping.role_key, mapping.oidc_groups.join(", ")]),
  );
  return (service.roles ?? [])
    .filter((role) => role.active)
    .map((role) => ({
      roleKey: role.key,
      roleName: role.name,
      oidcGroups: groupsByRole.get(role.key) ?? "",
    }));
}

async function scrollToTop() {
  await nextTick();
  fieldsScroll.value?.scrollTo({ top: 0, behavior: "smooth" });
}

async function reportError(message) {
  error.value = message;
  await scrollToTop();
}

watch(() => props.service, async (service) => {
  if (!service) return;
  loading.value = true;
  error.value = "";
  draft.value = {
    applicationUrl: service.public_url ?? "",
    defaultUserRole: null,
    roleMappings: roleMappingRowsFor(service, service.role_mappings),
    alternateUrls: alternateUrlRowsFrom(service.alternate_urls),
  };

  try {
    const configuration = await services.getConfiguration(service.id);
    if (props.service?.id !== service.id) return;
    Object.assign(service, configuration);
    draft.value = {
      applicationUrl: configuration.public_url ?? "",
      defaultUserRole: configuration.default_role_key ?? null,
      alternateUrls: alternateUrlRowsFrom(configuration.alternate_urls),
      roleMappings: roleMappingRowsFor(configuration, configuration.role_mappings),
    };
  } catch (loadError) {
    await reportError(loadError instanceof Error
      ? loadError.message
      : "Unable to load service configuration.");
  } finally {
    loading.value = false;
  }
}, { immediate: true });

function close() {
  error.value = "";
  emit("close");
}

function groupsForRole(mappings, roleKey) {
  return [...new Set((mappings ?? [])
    .filter((mapping) => mapping.role_key === roleKey)
    .flatMap((mapping) => mapping.oidc_groups ?? []))]
    .sort();
}

/** Roles left without OIDC groups are simply not mapped. */
function collectRoleMappings() {
  const roleMappings = [];
  for (const row of draft.value.roleMappings) {
    const groups = [...new Set(row.oidcGroups
      .split(",")
      .map((group) => group.trim())
      .filter(Boolean))];
    if (groups.length === 0) {
      continue;
    }
    roleMappings.push({ role_key: row.roleKey, oidc_groups: groups });
  }
  return roleMappings;
}

/** Core defines the group names, so only Core can produce an invalid set. */
async function collectAlternateUrls() {
  const rows = draft.value.alternateUrls;
  if (!isCore.value) {
    return rows;
  }
  const groups = new Set();
  for (const row of rows) {
    const group = row.group.trim();
    if (!group) {
      await reportError("Give every alternate URL a group, or remove it.");
      return null;
    }
    if (groups.has(group)) {
      await reportError(`More than one alternate URL uses the group "${group}". Group names must be unique.`);
      return null;
    }
    groups.add(group);
  }
  return rows;
}

async function save() {
  if (!props.service) return;
  const alternateUrls = await collectAlternateUrls();
  if (alternateUrls === null) return;
  const roleMappings = collectRoleMappings();

  const proposedCoreAdminGroups = groupsForRole(roleMappings, "admin");
  const currentCoreAdminGroups = groupsForRole(props.service.role_mappings, "admin");
  const coreAdminGroupsChanged = isCore.value
    && JSON.stringify(proposedCoreAdminGroups) !== JSON.stringify(currentCoreAdminGroups);

  // Core administrator mappings only take effect after an OIDC round-trip, so
  // this save keeps the existing mappings and the verification applies the new
  // ones on the way back.
  const mappingsToSave = coreAdminGroupsChanged
    ? (props.service.role_mappings ?? []).map((mapping) => ({
      role_key: mapping.role_key,
      oidc_groups: [...mapping.oidc_groups],
    }))
    : roleMappings;

  saving.value = true;
  error.value = "";
  try {
    const updated = await services.updateConfiguration(props.service.id, {
      public_url: draft.value.applicationUrl.trim(),
      default_role_key: isCore.value ? null : draft.value.defaultUserRole,
      role_mappings: mappingsToSave,
      alternate_urls: alternateUrls.map((row) => ({
        group_id: row.groupId ?? 0,
        group: row.group.trim(),
        url: row.url.trim(),
      })),
    });
    Object.assign(props.service, normalizeService(updated), { roles: updated.roles });
    draft.value.alternateUrls = alternateUrlRowsFrom(updated.alternate_urls);
    if (updated.service_type === "core") {
      await oidc.load();
    }
    close();
    if (coreAdminGroupsChanged) {
      oidc.requestCoreAdminVerification(proposedCoreAdminGroups);
    }
  } catch (saveError) {
    await reportError(saveError instanceof Error
      ? saveError.message
      : "Unable to save service configuration.");
  } finally {
    saving.value = false;
  }
}

function requestDeletion() {
  serviceToDelete.value = props.service;
}

function cancelDeletion() {
  serviceToDelete.value = null;
}

async function confirmDeletion() {
  if (!serviceToDelete.value) return;
  deletionSaving.value = true;
  try {
    const unregistered = await services.unregister(serviceToDelete.value.id);
    Object.assign(serviceToDelete.value, normalizeService(unregistered));
    serviceToDelete.value = null;
    close();
  } catch (deleteError) {
    serviceToDelete.value = null;
    await reportError(deleteError instanceof Error
      ? deleteError.message
      : "Unable to unregister the service.");
  } finally {
    deletionSaving.value = false;
  }
}

const canDelete = computed(() => (
  props.service?.status === "offline"
  && props.service?.registration_status === "registered"
  && props.service?.service_type !== "core"
));
</script>

<template>
  <v-dialog
    :model-value="service !== null"
    aria-label="Service configuration"
    max-width="680"
    @update:model-value="(open) => !open && close()"
  >
    <v-card v-if="service" class="service-configuration-dialog" rounded="lg">
      <form @submit.prevent="save">
        <v-card-text class="service-configuration-fields">
          <div class="service-configuration-header">
            <img
              :src="service.iconFailed ? defaultServiceIcon : service.iconUrl"
              alt=""
              class="service-configuration-icon"
              @error="service.iconFailed = true"
            />
            <p class="service-configuration-title">{{ service.name }} Config</p>
          </div>

          <div ref="fieldsScroll" class="service-configuration-form-scroll">
            <p v-if="error" class="service-configuration-error" role="alert">
              {{ error }}
            </p>
            <p v-if="loading" class="service-state-message">
              Loading configuration…
            </p>

            <div class="service-configuration-field">
              <label for="service-application-url" class="service-field-label">
                Application URL
              </label>
              <p id="service-application-url-help" class="service-field-help">
                URL for {{ service.name }} access
              </p>
              <v-text-field
                id="service-application-url"
                v-model="draft.applicationUrl"
                aria-describedby="service-application-url-help"
                :disabled="loading || saving"
                hide-details="auto"
                placeholder="https://service.example.com"
                type="url"
                variant="outlined"
              />
            </div>

            <div
              v-if="draft.alternateUrls.length > 0"
              class="service-alternate-urls-heading"
            >
              <p class="service-field-label">Alternate URLs</p>
              <p class="service-field-help">{{ alternateUrlHelp }}</p>
            </div>

            <div
              v-for="(row, index) in draft.alternateUrls"
              :key="row.id"
              class="service-alternate-url"
            >
              <v-btn
                v-if="isCore"
                :aria-label="`Delete alternate URL ${index + 1}`"
                class="service-alternate-url-delete"
                color="error"
                icon
                type="button"
                variant="text"
                @click="removeAlternateUrl(row.id)"
              >
                <v-icon :icon="mdiClose" />
              </v-btn>

              <div class="service-alternate-url-inputs">
                <v-text-field
                  :class="{ 'no-interaction': !isCore }"
                  v-model="row.group"
                  :aria-label="`Alternate URL ${index + 1} group`"
                  :disabled="saving"
                  hide-details="auto"
                  placeholder="Group"
                  :readonly="!isCore"
                  variant="outlined"
                />
                <v-text-field
                  v-model="row.url"
                  :aria-label="`Alternate URL ${index + 1}`"
                  :disabled="saving"
                  hide-details="auto"
                  placeholder="https://service.example.com"
                  type="url"
                  variant="outlined"
                />
              </div>
            </div>

            <v-btn
              v-if="isCore"
              class="add-alternate-url-button"
              color="primary"
              :disabled="loading || saving"
              type="button"
              variant="text"
              @click="addAlternateUrl"
            >
              <v-icon :icon="mdiPlus" start />
              Alternate URL
            </v-btn>

            <div class="service-configuration-field">
              <label for="service-default-role" class="service-field-label">
                Default User Role
              </label>
              <p id="service-default-role-help" class="service-field-help">
                Default access level for users not belonging to a higher access
                group
              </p>
              <v-select
                v-if="service.service_type !== 'core'"
                id="service-default-role"
                v-model="draft.defaultUserRole"
                aria-describedby="service-default-role-help"
                :disabled="loading || saving"
                :items="roleOptions"
                hide-details="auto"
                placeholder="No Access"
                variant="outlined"
              />
              <v-text-field
                v-else
                id="service-default-role"
                aria-describedby="service-default-role-help"
                class="no-interaction"
                hide-details="auto"
                model-value="No Access"
                readonly
                variant="outlined"
              />
            </div>

            <div
              v-if="draft.roleMappings.length > 0"
              class="service-role-mappings-heading"
            >
              <p class="service-field-label">Role Mappings</p>
              <p class="service-field-help">
                Choose OIDC groups to grant access to {{ service.name }}. Roles are listed by priority, so the first group match determines the role (highest priority wins). If no match, default role is used. If no group set, that role will never be used unless it's the default role.
              </p>
            </div>

            <div
              v-for="row in draft.roleMappings"
              :key="row.roleKey"
              class="service-role-mapping"
            >
              <div class="service-role-mapping-inputs">
                <v-text-field
                  class="no-interaction"
                  :aria-label="`${row.roleName} role`"
                  hide-details="auto"
                  :model-value="row.roleName"
                  readonly
                  variant="outlined"
                />
                <v-text-field
                  v-model="row.oidcGroups"
                  :aria-label="`OIDC groups for the ${row.roleName} role`"
                  :disabled="saving"
                  hide-details="auto"
                  placeholder="OIDC groups (comma separated)"
                  variant="outlined"
                />
              </div>
            </div>
          </div>
        </v-card-text>
        <v-card-actions>
          <v-btn
            v-if="canDelete"
            color="error"
            :disabled="saving"
            variant="text"
            @click="requestDeletion"
          >
            Delete
          </v-btn>
          <v-spacer />
          <v-btn :disabled="saving" variant="text" @click="close">
            Cancel
          </v-btn>
          <v-btn
            color="primary"
            :disabled="loading"
            :loading="saving"
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
    @update:model-value="(open) => !open && cancelDeletion()"
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
        <v-btn variant="text" @click="cancelDeletion">
          Cancel
        </v-btn>
        <v-btn
          color="error"
          :loading="deletionSaving"
          variant="flat"
          @click="confirmDeletion"
        >
          Confirm
        </v-btn>
      </v-card-actions>
    </v-card>
  </v-dialog>
</template>

<style scoped>
.service-configuration-dialog {
  display: flex;
  height: min(780px, calc(100vh - 32px));
  height: min(780px, calc(100dvh - 32px));
  min-height: min(780px, calc(100vh - 32px));
  min-height: min(780px, calc(100dvh - 32px));
  max-height: min(780px, calc(100vh - 32px));
  max-height: min(780px, calc(100dvh - 32px));
  overflow: hidden;
}

.service-configuration-dialog form {
  display: flex;
  flex: 1 1 auto;
  min-height: 0;
  flex-direction: column;
}

.service-configuration-header {
  display: flex;
  flex: 0 0 auto;
  gap: 20px;
  align-items: center;
  padding-bottom: 22px;
  border-bottom: 1px solid rgba(var(--v-theme-on-surface), 0.1);
}

.service-configuration-icon {
  flex: 0 0 auto;
  width: 42px;
  height: 42px;
  object-fit: contain;
}

.service-configuration-title {
  min-width: 0;
  margin: 0;
  color: rgb(var(--v-theme-on-surface));
  font-size: 1.3rem;
  font-weight: 600;
  line-height: 1.4;
  overflow-wrap: anywhere;
}

.service-configuration-fields {
  display: flex;
  flex: 1 1 auto;
  gap: 22px;
  min-height: 0;
  flex-direction: column;
  overflow: hidden;
  padding: 24px;
}

/* Keeps the scrollbar at the card edge while the content keeps its padding. */
.service-configuration-form-scroll {
  display: grid;
  flex: 1 1 auto;
  gap: 22px;
  align-content: start;
  min-height: 0;
  margin-right: -24px;
  overflow-y: auto;
  padding-right: 24px;
}

.service-configuration-field {
  min-width: 0;
}

.service-configuration-dialog .service-field-label {
  color: rgb(var(--v-theme-primary));
}

.service-configuration-dialog .service-field-help {
  margin: 0 0 10px;
  color: rgba(var(--v-theme-on-surface), 0.65);
  font-size: 0.875rem;
  line-height: 1.4;
}

.service-alternate-urls-heading .service-field-label,
.service-role-mappings-heading .service-field-label {
  margin: 0 0 4px;
}

.service-alternate-urls-heading .service-field-help,
.service-role-mappings-heading .service-field-help {
  margin-bottom: 0;
}

.service-alternate-urls-heading + .service-alternate-url,
.service-role-mappings-heading + .service-role-mapping {
  margin-top: -12px;
}

/* Consecutive rows read as one list, so they sit closer than the form's
   spacing between separate fields. */
.service-alternate-url + .service-alternate-url,
.service-role-mapping + .service-role-mapping {
  margin-top: -12px;
}

/* The add button belongs to the list above it, not to the next field. It keeps
   the normal field spacing when the list is empty and it follows a field. */
.service-alternate-url + .add-alternate-url-button {
  margin-top: -12px;
}

.service-alternate-url,
.service-role-mapping {
  display: grid;
  grid-template-columns: minmax(0, 1fr) auto;
  gap: 12px;
  align-items: center;
}

.service-role-mapping {
  grid-template-columns: minmax(0, 1fr);
}

/* Services other than Core cannot remove a group, so the row has no delete
   button and the inputs take the full width. */
.service-alternate-url:not(:has(.service-alternate-url-delete)) {
  grid-template-columns: minmax(0, 1fr);
}

.service-alternate-url-delete {
  grid-column: 2;
  grid-row: 1;
}

/* The name on the left is short; the value beside it needs the room. */
.service-alternate-url-inputs,
.service-role-mapping-inputs {
  display: grid;
  grid-column: 1;
  grid-row: 1;
  grid-template-columns: minmax(0, 1fr) minmax(0, 2fr);
  gap: 12px;
}

.add-alternate-url-button {
  justify-self: start;
}

.service-configuration-dialog :deep(.v-card-actions) {
  flex: 0 0 auto;
  padding: 8px 16px 16px;
}

.service-configuration-error {
  margin: 0;
  padding: 12px 14px;
  border: 1px solid rgba(var(--v-theme-error), 0.45);
  border-radius: 6px;
  background: rgba(var(--v-theme-error), 0.1);
  color: rgb(var(--v-theme-error));
  font-size: 0.875rem;
}

@media (max-width: 600px) {
  .service-alternate-url-inputs,
  .service-role-mapping-inputs {
    grid-template-columns: 1fr;
  }

  .service-alternate-url + .service-alternate-url,
  .service-role-mapping + .service-role-mapping {
    margin-top: 0;
    padding-top: 22px;
    border-top: 1px solid rgba(var(--v-theme-on-surface), 0.1);
  }
}
</style>
