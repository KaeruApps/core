<script setup>
import { onMounted, ref } from "vue";

import { fetchPublicJSON } from "../../api/client.js";

const about = ref(null);
const loading = ref(true);
const error = ref("");

onMounted(async () => {
  try {
    about.value = await fetchPublicJSON("/api/v1/about");
  } catch {
    error.value = "Unable to load application information.";
  } finally {
    loading.value = false;
  }
});
</script>

<template>
  <section aria-labelledby="about-title" class="home-section about-section">
    <div class="home-section-header">
      <h2 id="about-title" class="home-section-title">About</h2>
    </div>

    <v-sheet class="about-summary" border rounded="lg">
      <p v-if="loading" class="about-message">
        Loading application information…
      </p>
      <p v-else-if="error" class="about-message text-error">
        {{ error }}
      </p>
      <div v-else-if="about" class="about-details">
        <span class="app-icon about-icon" aria-hidden="true" />
        <div>
          <p class="about-name">{{ about.name }} v{{ about.version }}</p>
          <p class="about-description">{{ about.description }}</p>
        </div>
      </div>
    </v-sheet>
  </section>
</template>

<style scoped>
.about-section {
  margin-top: 48px;
}

.about-summary {
  padding: 24px;
  background: rgb(var(--v-theme-surface));
}

.about-details,
.about-message {
  margin: 0;
}

.about-details {
  display: flex;
  align-items: center;
  gap: 16px;
}

.about-icon {
  width: 52px;
  height: 52px;
}

.about-name {
  margin: 0 0 4px;
  color: rgb(var(--v-theme-on-surface));
  font-size: 1.05rem;
  font-weight: 600;
}

.about-description {
  margin: 0;
}
</style>
