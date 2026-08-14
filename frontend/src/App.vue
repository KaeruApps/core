<script setup>
import { mdiFormatListBulleted } from "@mdi/js";
import { onMounted, ref } from "vue";

const developmentMode = ref(false);

onMounted(async () => {
  try {
    const response = await fetch("/api/v1/health", {
      headers: { Accept: "application/json" },
    });
    if (response.ok) {
      const health = await response.json();
      developmentMode.value = health.development_mode === true;
    }
  } catch {
    // Availability errors are handled by the pages that depend on the API.
  }
});
</script>

<template>
  <v-app>
    <v-app-bar color="appbar" elevation="1">
      <router-link
        v-ripple
        class="app-brand"
        to="/"
        aria-label="Kaeru home"
      >
        <span class="app-icon" aria-hidden="true" />
        <v-app-bar-title class="app-title">Kaeru</v-app-bar-title>
      </router-link>

      <nav class="app-navigation" aria-label="Primary navigation">
        <v-btn
          :active="false"
          aria-label="Event Log"
          class="nav-button nav-button--event-log"
          to="/events"
          variant="text"
        >
          <v-icon :icon="mdiFormatListBulleted" />
          <span class="nav-button-label">Event Log</span>
        </v-btn>
      </nav>
    </v-app-bar>

    <v-main>
      <div v-if="developmentMode" class="development-mode-banner" role="status">
        Development mode: authentication is bypassed
      </div>
      <router-view />
    </v-main>
  </v-app>
</template>
