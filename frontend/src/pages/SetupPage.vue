<script setup>
import { mdiBackupRestore, mdiShieldAccount } from "@mdi/js";
</script>

<template>
  <v-container class="setup-page">
    <section class="setup-panel" aria-labelledby="setup-title">
      <div class="setup-heading">
        <span class="setup-app-icon" aria-hidden="true" />
        <div>
          <h1 id="setup-title">Set up Kaeru</h1>
          <p>Configure a new installation or restore an existing Kaeru backup.</p>
        </div>
      </div>

      <div class="setup-options">
        <v-card class="setup-option" rounded="lg" variant="outlined">
          <v-card-text>
            <v-icon :icon="mdiShieldAccount" color="primary" size="36" />
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
