<script setup>
import { nextTick, onMounted, onUnmounted, ref, watch } from "vue";
import { mdiAlertCircleOutline, mdiCheck, mdiChevronDown, mdiChevronUp, mdiClose } from "@mdi/js";

import defaultServiceIcon from "../../assets/app-icon.svg";
import { serviceStatusLabel, useServicesStore } from "../../stores/services.js";
import { useOIDCStore } from "../../stores/oidc.js";
import ServiceConfigurationDialog from "./ServiceConfigurationDialog.vue";

const services = useServicesStore();
const oidc = useOIDCStore();

const section = ref(null);
const serviceToConfigure = ref(null);

onMounted(() => {
  services.load();
  services.startPolling();
});

onUnmounted(() => {
  services.stopPolling();
});

// A Core administrator verification round-trip reports its result here rather
// than in the OIDC section, so scroll this section back into view on return.
watch(() => oidc.scrollTarget, async (target) => {
  if (target !== "services") return;
  await nextTick();
  section.value?.scrollIntoView({ behavior: "smooth", block: "start" });
  oidc.scrollTarget = null;
}, { immediate: true });

function openConfiguration(service) {
  serviceToConfigure.value = service;
}

function closeConfiguration() {
  serviceToConfigure.value = null;
}
</script>

<template>
  <section ref="section" aria-labelledby="services-title" class="home-section">
    <div class="home-section-header home-section-header--stacked">
      <div class="home-section-heading">
        <div class="home-section-title-row">
          <h1 id="services-title" class="home-section-title">Services</h1>
          <v-btn
            :aria-expanded="services.showDetails"
            color="primary"
            variant="text"
            @click="services.showDetails = !services.showDetails"
          >
            {{ services.showDetails ? "Less Details" : "More Details" }}
            <v-icon :icon="services.showDetails ? mdiChevronUp : mdiChevronDown" end />
          </v-btn>
        </div>
        <p class="home-section-subtitle">
          Configure URLs and access for Kaeru Services. Any Kaeru service you
          install will automatically appear here.
        </p>
      </div>
    </div>

    <div
      v-if="oidc.coreAdminVerificationStatus"
      :class="[
        'oidc-verification-status',
        'core-admin-verification-status',
        `oidc-verification-status--${oidc.coreAdminVerificationStatus.type}`,
      ]"
      role="status"
    >
      <v-icon :icon="oidc.coreAdminVerificationStatus.type === 'success' ? mdiCheck : mdiAlertCircleOutline" />
      <div>
        <strong>{{ oidc.coreAdminVerificationStatus.message }}</strong>
        <p v-if="oidc.coreAdminVerificationStatus.detail">
          {{ oidc.coreAdminVerificationStatus.detail }}
        </p>
      </div>
      <v-btn
        aria-label="Dismiss message"
        :icon="mdiClose"
        size="small"
        type="button"
        variant="text"
        @click="oidc.coreAdminVerificationStatus = null"
      />
    </div>

    <p v-if="services.loading" class="service-state-message">
      Loading services…
    </p>
    <div v-else-if="services.error" class="service-state-message service-state-message--error">
      <span>{{ services.error }}</span>
      <v-btn color="primary" size="small" variant="text" @click="services.load()">
        Try again
      </v-btn>
    </div>
    <p v-else-if="services.services.length === 0" class="service-state-message">
      No services are registered.
    </p>
    <div v-else class="service-card-grid">
      <v-sheet
        v-for="service in services.services"
        :key="service.id"
        :class="['service-card', { 'service-card--offline': service.status === 'offline' }]"
        border
        role="button"
        rounded="lg"
        tabindex="0"
        @click="openConfiguration(service)"
        @keydown.enter="openConfiguration(service)"
        @keydown.space.prevent="openConfiguration(service)"
      >
        <div :class="['service-card-status', `service-card-status--${service.status}`]">
          <span class="service-card-status-dot" aria-hidden="true" />
          {{ serviceStatusLabel(service.status) }}
        </div>

        <div class="service-card-summary">
          <img
            :src="service.iconFailed ? defaultServiceIcon : service.iconUrl"
            alt=""
            class="service-card-icon"
            @error="service.iconFailed = true"
          />

          <div>
            <h2 class="service-card-name">{{ service.name }}</h2>
            <p class="service-card-description">{{ service.description }}</p>
          </div>
        </div>

        <v-expand-transition>
          <dl v-show="services.showDetails" class="service-card-details">
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

  <ServiceConfigurationDialog :service="serviceToConfigure" @close="closeConfiguration" />
</template>

<style scoped>
.core-admin-verification-status {
  margin-bottom: 16px;
}

.service-card-grid {
  display: grid;
  grid-template-columns: repeat(4, minmax(0, 1fr));
  gap: 20px;
}

.service-card {
  display: block;
  position: relative;
  min-height: 112px;
  padding: 24px;
  background: rgb(var(--v-theme-surface));
  cursor: pointer;
  user-select: none;
  transition: background-color 150ms ease, border-color 150ms ease;
}

.service-card:focus-visible {
  outline: 2px solid rgb(var(--v-theme-primary));
  outline-offset: 2px;
}

.service-card-summary {
  display: flex;
  min-height: 62px;
  gap: 16px;
  align-items: center;
}

.service-card:hover,
.service-card:focus-within {
  background: color-mix(
    in srgb,
    rgb(var(--v-theme-surface)) 88%,
    rgb(var(--v-theme-primary))
  );
  border-color: rgba(var(--v-theme-primary), 0.7);
}

.service-card--offline:hover,
.service-card--offline:focus-within {
  background: color-mix(
    in srgb,
    rgb(var(--v-theme-surface)) 88%,
    rgb(var(--v-theme-error))
  );
  border-color: rgba(var(--v-theme-error), 0.7);
}

.service-card-status {
  display: flex;
  position: absolute;
  top: 14px;
  right: 16px;
  gap: 6px;
  align-items: center;
  color: rgba(var(--v-theme-on-surface), 0.72);
  font-size: 0.75rem;
  font-weight: 600;
  line-height: 1;
  text-transform: capitalize;
  cursor: default;
  user-select: none;
  transition: opacity 120ms ease;
}

.service-card-status-dot {
  display: block;
  width: 8px;
  height: 8px;
  border-radius: 50%;
  background: rgba(var(--v-theme-on-surface), 0.45);
}

.service-card-status--online .service-card-status-dot {
  background: rgb(var(--v-theme-primary));
}

.service-card-status--offline .service-card-status-dot {
  background: rgb(var(--v-theme-error));
}

.service-card-icon {
  display: flex;
  flex: 0 0 auto;
  width: 42px;
  height: 42px;
  font-size: 42px;
  object-fit: contain;
}

.service-card-name {
  margin: 0 0 4px;
  color: rgb(var(--v-theme-primary));
  font-size: 1.05rem;
  font-weight: 700;
  line-height: 1.4;
}

.service-card-description {
  margin: 0;
  color: rgba(var(--v-theme-on-surface), 0.72);
  font-size: 0.9rem;
  line-height: 1.45;
}

.service-card-details {
  display: grid;
  gap: 12px;
  margin: 20px 0 0;
  padding: 20px 0 0;
  border-top: 1px solid rgba(var(--v-theme-on-surface), 0.1);
}

.service-card-detail {
  display: grid;
  grid-template-columns: 96px minmax(0, 1fr);
  gap: 12px;
  font-size: 0.825rem;
  line-height: 1.4;
}

.service-card-detail dt {
  min-width: 0;
  color: rgba(var(--v-theme-on-surface), 0.58);
  overflow-wrap: anywhere;
}

.service-card-detail dd {
  min-width: 0;
  margin: 0;
  color: rgba(var(--v-theme-on-surface), 0.86);
  overflow-wrap: anywhere;
}

@media (max-width: 1200px) {
  .service-card-grid {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }
}

@media (max-width: 600px) {
  .service-card-grid {
    grid-template-columns: 1fr;
  }
}
</style>
