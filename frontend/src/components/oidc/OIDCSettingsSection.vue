<script setup>
import { nextTick, onMounted, onUnmounted, ref, watch } from "vue";
import { mdiAlertCircleOutline, mdiAlertOutline, mdiCheck, mdiChevronDown, mdiChevronUp, mdiClose } from "@mdi/js";

import OIDCConfigurationForm from "../OIDCConfigurationForm.vue";
import { useOIDCStore } from "../../stores/oidc.js";

const oidc = useOIDCStore();

const section = ref(null);
const errorElement = ref(null);

onMounted(async () => {
  await oidc.load();
  oidc.handleVerificationReturn();
});

onUnmounted(() => {
  oidc.dispose();
});

watch(() => oidc.scrollTarget, async (target) => {
  if (target !== "oidc") return;
  await nextTick();
  section.value?.scrollIntoView({ behavior: "smooth", block: "start" });
  oidc.scrollTarget = null;
}, { immediate: true });

async function scrollToError() {
  await nextTick();
  errorElement.value?.scrollIntoView({ behavior: "smooth", block: "center" });
}

async function save() {
  try {
    await oidc.save();
  } catch {
    await scrollToError();
  }
}

async function startVerification() {
  try {
    await oidc.startVerification();
  } catch {
    await scrollToError();
  }
}

function close() {
  oidc.beginClose(() => {
    if (!section.value) return false;
    section.value.scrollIntoView({ behavior: "smooth", block: "start" });
    return true;
  });
}
</script>

<template>
  <section
    ref="section"
    aria-labelledby="oidc-configuration-title"
    class="home-section oidc-configuration-section"
  >
    <div class="home-section-header">
      <div class="home-section-heading">
        <h2 id="oidc-configuration-title" class="home-section-title">
          OIDC Configuration
        </h2>
        <p class="home-section-subtitle">Manage your OIDC settings</p>
      </div>
    </div>

    <v-sheet class="oidc-configuration-card" border rounded="lg">
      <button
        class="oidc-configuration-summary"
        type="button"
        :aria-expanded="oidc.expanded"
        @click="oidc.expanded = !oidc.expanded"
      >
        <span class="oidc-configuration-summary-content">
          <img
            :src="oidc.summaryIcon"
            alt=""
            class="oidc-configuration-icon"
            @error="oidc.imageFailed = true"
          />
          <span>
            <strong>{{ oidc.settings?.name || "OIDC" }}</strong>
            <small>
              {{ oidc.settings?.issuer_url || (oidc.loading ? "Loading…" : "Not configured") }}
            </small>
          </span>
        </span>
        <v-icon :icon="oidc.expanded ? mdiChevronUp : mdiChevronDown" />
      </button>

      <v-expand-transition>
        <form v-show="oidc.expanded" class="oidc-configuration-form" @submit.prevent="save">
          <div
            v-if="oidc.verificationStatus"
            :class="['oidc-verification-status', `oidc-verification-status--${oidc.verificationStatus.type}`]"
            role="status"
          >
            <v-icon :icon="oidc.verificationStatus.type === 'success' ? mdiCheck : mdiAlertCircleOutline" />
            <div>
              <strong>{{ oidc.verificationStatus.message }}</strong>
              <p v-if="oidc.verificationStatus.detail">{{ oidc.verificationStatus.detail }}</p>
            </div>
            <v-btn
              aria-label="Dismiss message"
              :icon="mdiClose"
              size="small"
              type="button"
              variant="text"
              @click="oidc.verificationStatus = null"
            />
          </div>
          <p v-if="oidc.error" ref="errorElement" class="oidc-setup-error" role="alert">
            {{ oidc.error }}
          </p>
          <OIDCConfigurationForm
            v-model:button-image="oidc.buttonImage"
            v-model:configuration="oidc.draft"
            :existing-button-image="oidc.settings?.button_image_configured"
            access-urls-mode
          />

          <div class="oidc-configuration-actions">
            <v-btn :disabled="oidc.closing" type="button" variant="text" @click="close">
              Close
            </v-btn>
            <v-btn
              :aria-label="oidc.saved ? 'OIDC settings saved' : 'Save OIDC settings'"
              color="primary"
              :loading="oidc.saving"
              type="submit"
              variant="flat"
              width="80"
            >
              <v-icon v-if="oidc.saved" :icon="mdiCheck" />
              <span v-else>Save</span>
            </v-btn>
          </div>
        </form>
      </v-expand-transition>
    </v-sheet>
  </section>

  <v-dialog v-model="oidc.verificationDialog" max-width="560" persistent>
    <v-card class="oidc-verification-dialog" rounded="lg">
      <v-card-title>
        {{ oidc.verificationSource === "core-admin"
          ? "Verify Core admin access changes?"
          : "Verify OIDC changes?" }}
      </v-card-title>
      <v-card-text class="oidc-verification-dialog-content">
        <p v-if="oidc.verificationSource === 'core-admin'">
          These changes affect who can administer Kaeru Core and must be verified before they are saved.
        </p>
        <p v-else>
          These changes affect how users sign in and must be verified before they are saved.
        </p>
        <div class="oidc-verification-session-warning">
          <v-icon color="warning" :icon="mdiAlertOutline" size="20" />
          <p v-if="oidc.verificationRevokesSessions">
            After successful verification, all users will be logged out on every device. Your newly verified session will remain signed in.
          </p>
          <p v-else>
            Administrator access will update immediately. Existing user sessions will remain signed in.
          </p>
        </div>
        <div>
          <strong class="oidc-verification-changes-title">Changes requiring verification</strong>
          <ul>
            <li v-for="change in oidc.verificationChanges" :key="change">{{ change }}</li>
          </ul>
        </div>
      </v-card-text>
      <v-card-actions>
        <v-spacer />
        <v-btn
          :disabled="oidc.verificationStarting"
          variant="text"
          @click="oidc.cancelVerification"
        >
          Cancel
        </v-btn>
        <v-btn
          color="primary"
          :loading="oidc.verificationStarting"
          variant="flat"
          @click="startVerification"
        >
          Verify Changes
        </v-btn>
      </v-card-actions>
    </v-card>
  </v-dialog>
</template>

<style scoped>
.oidc-configuration-section {
  margin-top: 48px;
  scroll-margin-top: 88px;
}

.oidc-configuration-card {
  overflow: hidden;
}

.oidc-configuration-summary {
  align-items: center;
  background: transparent;
  border: 0;
  color: inherit;
  cursor: pointer;
  display: flex;
  justify-content: space-between;
  padding: 20px 24px;
  text-align: left;
  width: 100%;
}

.oidc-configuration-summary:hover {
  background: rgb(var(--v-theme-surface));
}

.oidc-configuration-summary-content {
  align-items: center;
  display: flex;
  gap: 16px;
  min-width: 0;
}

.oidc-configuration-summary-content > span {
  display: grid;
  gap: 4px;
  min-width: 0;
}

.oidc-configuration-icon {
  border-radius: 8px;
  flex: 0 0 48px;
  height: 48px;
  object-fit: cover;
  width: 48px;
}

.oidc-configuration-summary strong {
  font-size: 1.05rem;
}

.oidc-configuration-summary small {
  color: rgba(var(--v-theme-on-surface), 0.68);
  overflow-wrap: anywhere;
}

.oidc-configuration-form {
  border-top: 1px solid rgba(var(--v-border-color), var(--v-border-opacity));
  display: grid;
  gap: 24px;
  padding: 24px;
}

.oidc-configuration-actions {
  display: flex;
  justify-content: space-between;
}

.oidc-verification-dialog-content {
  display: grid;
  gap: 16px;
  padding: 20px 24px 24px;
}

.oidc-verification-dialog :deep(.v-card-title) {
  padding: 24px 24px 0;
  white-space: normal;
}

.oidc-verification-dialog :deep(.v-card-actions) {
  padding: 0 16px 16px;
}

.oidc-verification-dialog-content p {
  margin: 0;
}

.oidc-verification-session-warning {
  align-items: flex-start;
  display: flex;
  gap: 10px;
}

.oidc-verification-session-warning :deep(.v-icon) {
  flex: 0 0 auto;
  margin-top: 2px;
}

.oidc-verification-changes-title {
  color: rgb(var(--v-theme-primary));
}

.oidc-verification-dialog-content ul {
  margin: 8px 0 0;
  padding-left: 22px;
}

@media (max-width: 600px) {
  .oidc-verification-dialog :deep(.v-card-title) {
    padding: 20px 20px 0;
  }

  .oidc-verification-dialog-content {
    padding: 18px 20px 22px;
  }

  .oidc-verification-dialog :deep(.v-card-actions) {
    padding: 0 12px 12px;
  }

  .oidc-configuration-form {
    padding-right: 24px;
    padding-left: 24px;
  }
}
</style>
