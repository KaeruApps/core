<script setup>
import { mdiBackupRestore } from "@mdi/js";
import oidcIcon from "../assets/oidc.png";
import SetupHeading from "../components/layout/SetupHeading.vue";
</script>

<template>
  <v-container class="setup-page">
    <section class="setup-panel" aria-labelledby="setup-title">
      <SetupHeading
        title-id="setup-title"
        title="Set up Kaeru"
        subtitle="Configure a new installation or restore an existing Kaeru backup."
      />

      <div class="setup-options">
        <v-card class="setup-option" rounded="lg" variant="outlined">
          <v-card-text>
            <img :src="oidcIcon" alt="" class="setup-option-icon" />
            <h2>Set up OIDC</h2>
            <p>
              Connect Kaeru to your identity provider and create the initial administrator. OIDC is required.
            </p>
          </v-card-text>
          <v-card-actions>
            <v-btn color="primary" to="/setup/oidc" variant="flat">Set up OIDC</v-btn>
          </v-card-actions>
        </v-card>

        <v-card class="setup-option" rounded="lg" variant="outlined">
          <v-card-text>
            <v-icon :icon="mdiBackupRestore" color="primary" size="36" />
            <h2>Restore from backup</h2>
            <p>
              Restore Core configuration and platform services from an existing Kaeru backup.
            </p>
          </v-card-text>
          <v-card-actions>
            <v-btn disabled variant="outlined">Restore backup</v-btn>
          </v-card-actions>
        </v-card>
      </div>

      <p class="setup-next-step">Backup restoration will be enabled in a later setup step.</p>
    </section>
  </v-container>
</template>

<style scoped>
.setup-option p,
.setup-next-step {
  color: rgba(var(--v-theme-on-background), 0.68);
}

.setup-option p {
  margin: 0;
}

.setup-options {
  display: grid;
  gap: 1rem;
  grid-template-columns: repeat(2, minmax(0, 1fr));
}

.setup-option {
  display: flex;
  flex-direction: column;
}

.setup-option :deep(.v-card-text) {
  flex: 1;
}

.setup-option-icon {
  border-radius: 4px;
  display: block;
  height: 36px;
  object-fit: cover;
  width: 36px;
}

.setup-option h2 {
  font-size: 1.35rem;
  margin: 1rem 0 0.5rem;
}

.setup-option :deep(.v-card-actions) {
  padding: 0 1rem 1rem;
}

.setup-next-step {
  font-size: 0.9rem;
  margin-top: 1.25rem;
  text-align: center;
}

@media (max-width: 600px) {
  .setup-options {
    grid-template-columns: 1fr;
  }
}
</style>
