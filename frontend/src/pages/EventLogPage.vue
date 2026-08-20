<script setup>
import { computed, ref, watch } from "vue";
import { useRoute } from "vue-router";
import { mdiMagnify } from "@mdi/js";

// MOCK-BACKED: Kaeru Core has no event log API yet. See src/mocks/README.md.
import { recentEvents } from "../mocks/eventLog.js";

const route = useRoute();
const searchQuery = ref("");
const selectedService = ref("Any Service");
const selectedUser = ref("Any User");

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

<style scoped>
.event-log {
  max-height: min(960px, calc(100vh - 160px));
  overflow-y: auto;
  background: rgb(var(--v-theme-surface));
}

.event-log-panel {
  overflow: hidden;
  background: rgb(var(--v-theme-surface));
}

.event-log-filters {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 16px;
  padding: 20px 24px;
  border-bottom: 1px solid rgba(var(--v-theme-on-surface), 0.1);
}

.event-log-search {
  grid-column: 1 / -1;
}

.event-log-list {
  margin: 0;
  padding: 0;
  list-style: none;
}

.event-log-entry {
  display: grid;
  grid-template-columns: minmax(220px, 0.3fr) minmax(0, 1fr);
  gap: 24px;
  padding: 16px 24px;
  border-bottom: 1px solid rgba(var(--v-theme-on-surface), 0.08);
  font-size: 0.875rem;
  line-height: 1.5;
}

.event-log-entry:last-child {
  border-bottom: 0;
}

.event-log-timestamp {
  color: rgba(var(--v-theme-on-surface), 0.58);
  font-variant-numeric: tabular-nums;
}

.event-log-message {
  color: rgba(var(--v-theme-on-surface), 0.9);
}

.event-log-entry--error .event-log-message {
  color: rgb(var(--v-theme-error));
}

.event-log-empty {
  padding: 24px;
  color: rgba(var(--v-theme-on-surface), 0.62);
  font-size: 0.875rem;
  text-align: center;
}

.page-title {
  margin: 0 0 24px;
  color: rgb(var(--v-theme-on-background));
  font-size: 1.5rem;
  font-weight: 700;
  line-height: 1.3;
}

@media (max-width: 600px) {
  .event-log {
    max-height: calc(100vh - 120px);
  }

  .event-log-filters {
    grid-template-columns: 1fr;
  }

  .event-log-entry {
    grid-template-columns: 1fr;
    gap: 4px;
  }
}
</style>
