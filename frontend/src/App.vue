<script setup>
import { mdiFormatListBulleted } from "@mdi/js";
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
      <router-view />
    </v-main>
  </v-app>
</template>
