<script setup>
import { computed, ref, watch } from "vue";
import { useRoute } from "vue-router";
import { mdiMagnify } from "@mdi/js";

const route = useRoute();
const searchQuery = ref("");
const selectedService = ref("Any Service");
const selectedUser = ref("Any User");

const recentEvents = [
  {
    id: 1,
    timestamp: "August 9, 2026 at 14:32:08",
    timestampAt: "2026-08-09T14:32:08+02:00",
    message: "Upload Archiver came online",
    service: "Upload Archiver",
    user: null,
    tone: "normal",
  },
  {
    id: 2,
    timestamp: "August 9, 2026 at 14:28:41",
    timestampAt: "2026-08-09T14:28:41+02:00",
    message: "Alex Morgan uploaded a file",
    service: "Upload Archiver",
    user: "Alex Morgan",
    tone: "normal",
  },
  {
    id: 3,
    timestamp: "August 9, 2026 at 13:54:12",
    timestampAt: "2026-08-09T13:54:12+02:00",
    message: "Relay went offline",
    service: "Relay",
    user: null,
    tone: "error",
  },
  {
    id: 4,
    timestamp: "August 9, 2026 at 09:15:03",
    timestampAt: "2026-08-09T09:15:03+02:00",
    message: "Sam Rivera connected",
    service: "Core",
    user: "Sam Rivera",
    tone: "normal",
  },
  {
    id: 5,
    timestamp: "August 9, 2026 at 02:02:19",
    timestampAt: "2026-08-09T02:02:19+02:00",
    message: "Platform backup completed",
    service: "Core",
    user: null,
    tone: "normal",
  },
  {
    id: 6,
    timestamp: "August 8, 2026 at 21:08:47",
    timestampAt: "2026-08-08T21:08:47+02:00",
    message: "Jamie Chen queried the service registry",
    service: "Core",
    user: "Jamie Chen",
    tone: "normal",
  },
];

const serviceOptions = [
  "Any Service",
  ...new Set(recentEvents.map((event) => event.service)),
];
const userOptions = [
  "Any User",
  ...new Set(recentEvents.map((event) => event.user).filter(Boolean)),
];

watch(
  () => route.query.user,
  (user) => {
    selectedUser.value = Array.isArray(user) ? user[0] : user ?? "Any User";
  },
  { immediate: true },
);

const filteredEvents = computed(() => {
  const normalizedSearch = searchQuery.value.trim().toLocaleLowerCase();

  return recentEvents.filter((event) => {
    const matchesSearch = normalizedSearch === ""
      || event.message.toLocaleLowerCase().includes(normalizedSearch);
    const matchesService = selectedService.value === "Any Service"
      || event.service === selectedService.value;
    const matchesUser = selectedUser.value === "Any User"
      || event.user === selectedUser.value;

    return matchesSearch && matchesService && matchesUser;
  });
});
</script>

<template>
  <v-container class="page-content">
    <h1 class="page-title">Event Log</h1>

    <v-sheet class="event-log-panel" border rounded="lg">
      <form class="event-log-filters" @submit.prevent>
        <v-text-field
          v-model="searchQuery"
          :append-inner-icon="mdiMagnify"
          class="event-log-search"
          clearable
          density="compact"
          hide-details="auto"
          label="Query"
          placeholder="Search..."
          variant="outlined"
        />
        <v-select
          v-model="selectedService"
          :items="serviceOptions"
          density="compact"
          hide-details="auto"
          label="Service"
          variant="outlined"
        />
        <v-select
          v-model="selectedUser"
          :items="userOptions"
          density="compact"
          hide-details="auto"
          label="User"
          variant="outlined"
        />
      </form>

      <div class="event-log" role="log">
        <ol class="event-log-list">
          <li
            v-for="event in filteredEvents"
            :key="event.id"
            :class="['event-log-entry', `event-log-entry--${event.tone}`]"
          >
            <time :datetime="event.timestampAt" class="event-log-timestamp">
              {{ event.timestamp }}
            </time>
            <span class="event-log-message">{{ event.message }}</span>
          </li>
          <li v-if="filteredEvents.length === 0" class="event-log-empty">
            No events match the selected filters.
          </li>
        </ol>
      </div>
    </v-sheet>
  </v-container>
</template>
