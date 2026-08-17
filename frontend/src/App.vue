<script setup>
import { mdiCamera, mdiFormatListBulleted, mdiLogout, mdiTune } from "@mdi/js";
import { nextTick, onMounted, ref } from "vue";
import { useRoute, useRouter } from "vue-router";
import { useTheme } from "vuetify";
import defaultUserIcon from "./assets/default-user.png";
import { apiFetch } from "./api.js";

const developmentMode = ref(false);
const currentUser = ref(null);
const avatarFailed = ref(false);
const loggingOut = ref(false);
const preferencesOpen = ref(false);
const preferencesLoading = ref(false);
const preferencesSaving = ref(false);
const preferencesError = ref("");
const preferencesCard = ref(null);
const avatarInput = ref(null);
const avatarUploading = ref(false);
const preferencesDraft = ref({
  username: "",
  name: "",
  email: "",
  time_format: "24h",
  timezone: "automatic",
  theme: "dark",
});
const route = useRoute();
const router = useRouter();
const theme = useTheme();

const timeFormatOptions = [
  { title: "12-hour", value: "12h" },
  { title: "24-hour", value: "24h" },
];
const themeOptions = [
  { title: "Dark", value: "dark" },
  { title: "Light", value: "light" },
];
const fallbackTimezones = [
  "UTC",
  "America/Chicago",
  "America/Denver",
  "America/Los_Angeles",
  "America/New_York",
  "Asia/Tokyo",
  "Australia/Sydney",
  "Europe/Amsterdam",
  "Europe/London",
];
const supportedTimezones = typeof Intl.supportedValuesOf === "function"
  ? Intl.supportedValuesOf("timeZone")
  : fallbackTimezones;
const timezoneOptions = [
  { title: "Automatic", value: "automatic" },
  ...supportedTimezones.map((timezone) => ({ title: timezone, value: timezone })),
];

onMounted(async () => {
  try {
    const response = await fetch("/api/v1/health", {
      headers: { Accept: "application/json" },
    });
    if (response.ok) {
      const health = await response.json();
      developmentMode.value = health.development_mode === true;
      if (health.initialized === true) {
        const sessionResponse = await fetch("/api/v1/session", {
          headers: { Accept: "application/json" },
        });
        if (sessionResponse.ok) {
          const session = await sessionResponse.json();
          currentUser.value = session.user;
          await loadPreferences();
        }
      }
    }
  } catch {
    // Availability errors are handled by the pages that depend on the API.
  }
});

async function logout() {
  loggingOut.value = true;
  try {
    const response = await fetch("/api/v1/session/logout", {
      method: "POST",
      headers: { Accept: "application/json" },
    });
    if (!response.ok) {
      throw new Error("Logout failed");
    }
    currentUser.value = null;
    await router.replace({ name: "login" });
  } finally {
    loggingOut.value = false;
  }
}

function applyTheme(preference) {
  theme.global.name.value = preference === "light" ? "kaeruLight" : "kaeruDark";
}

async function loadPreferences(reportErrors = false) {
  preferencesLoading.value = true;
  try {
    const response = await apiFetch("/api/v1/users/me/preferences", {
      headers: { Accept: "application/json" },
    });
    if (!response.ok) {
      throw new Error("User preferences could not be loaded.");
    }
    const preferences = await response.json();
    preferencesDraft.value = {
      ...preferences,
      name: preferences.name || "",
      email: preferences.email || "",
    };
    applyTheme(preferences.theme);
  } catch (error) {
    if (reportErrors) {
      preferencesError.value = error.message;
      await resetPreferencesScroll();
    }
  } finally {
    preferencesLoading.value = false;
  }
}

async function openPreferences() {
  preferencesOpen.value = true;
  preferencesError.value = "";
  await resetPreferencesScroll();
  await loadPreferences(true);
  await resetPreferencesScroll();
}

async function resetPreferencesScroll() {
  await nextTick();
  const card = preferencesCard.value?.$el || preferencesCard.value;
  card?.scrollTo({ top: 0 });
}

function closePreferences() {
  if (!preferencesSaving.value) {
    preferencesOpen.value = false;
  }
}

async function savePreferences() {
  preferencesSaving.value = true;
  preferencesError.value = "";
  try {
    const response = await apiFetch("/api/v1/users/me/preferences", {
      method: "PUT",
      headers: {
        Accept: "application/json",
        "Content-Type": "application/json",
      },
      body: JSON.stringify(preferencesDraft.value),
    });
    const body = await response.json();
    if (!response.ok) {
      throw new Error(body?.error?.message || "User preferences could not be saved.");
    }
    currentUser.value = {
      ...currentUser.value,
      name: body.username,
      display_name: body.name,
      email: body.email,
    };
    applyTheme(body.theme);
    preferencesOpen.value = false;
  } catch (error) {
    preferencesError.value = error.message;
    await resetPreferencesScroll();
  } finally {
    preferencesSaving.value = false;
  }
}

function chooseAvatar() {
  avatarInput.value?.click();
}

async function uploadAvatar(event) {
  const file = event.target.files?.[0];
  if (!file) return;
  preferencesError.value = "";
  if (!["image/jpeg", "image/png"].includes(file.type) || file.size > 5 * 1024 * 1024) {
    preferencesError.value = "Avatar must be a JPG or PNG file no larger than 5 MB.";
    event.target.value = "";
    await resetPreferencesScroll();
    return;
  }

  avatarUploading.value = true;
  try {
    const form = new FormData();
    form.append("avatar", file);
    const response = await apiFetch("/api/v1/users/me/avatar", {
      method: "PUT",
      headers: { Accept: "application/json" },
      body: form,
    });
    const body = await response.json();
    if (!response.ok) {
      throw new Error(body?.error?.message || "Avatar could not be uploaded.");
    }
    avatarFailed.value = false;
    currentUser.value = { ...currentUser.value, avatar_url: body.avatar_url };
  } catch (error) {
    preferencesError.value = error.message;
    await resetPreferencesScroll();
  } finally {
    avatarUploading.value = false;
    event.target.value = "";
  }
}
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

      <nav v-if="!route.meta.setup && !route.meta.authentication" class="app-navigation" aria-label="Primary navigation">
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

        <v-menu v-if="currentUser" location="bottom end">
          <template #activator="{ props: activatorProps }">
            <v-btn
              v-bind="activatorProps"
              aria-label="User menu"
              class="user-menu-button"
              icon
              variant="text"
            >
              <img
                v-if="currentUser.avatar_url && !avatarFailed"
                :src="currentUser.avatar_url"
                alt=""
                class="user-menu-avatar"
                referrerpolicy="no-referrer"
                @error="avatarFailed = true"
              >
              <img
                v-else
                :src="defaultUserIcon"
                alt=""
                class="user-menu-avatar"
              >
            </v-btn>
          </template>

          <v-list class="user-menu-list" density="compact">
            <v-list-item class="user-menu-identity">
              <div class="user-menu-identity-content">
                <img
                  :src="currentUser.avatar_url && !avatarFailed ? currentUser.avatar_url : defaultUserIcon"
                  alt=""
                  class="user-menu-identity-avatar"
                  referrerpolicy="no-referrer"
                  @error="avatarFailed = true"
                >
                <div class="user-menu-identity-text">
                  <p class="user-menu-real-name">
                    {{ currentUser.display_name || currentUser.name }}
                  </p>
                  <p class="user-menu-username">
                    @{{ currentUser.name }}
                  </p>
                  <p v-if="currentUser.email" class="user-menu-email">
                    {{ currentUser.email }}
                  </p>
                </div>
              </div>
            </v-list-item>
            <v-list-item
              :prepend-icon="mdiTune"
              link
              title="Preferences"
              @click="openPreferences"
            />
            <v-list-item
              :disabled="loggingOut"
              :prepend-icon="mdiLogout"
              link
              title="Logout"
              @click="logout"
            />
          </v-list>
        </v-menu>
      </nav>
    </v-app-bar>

    <v-dialog
      v-model="preferencesOpen"
      aria-label="User preferences"
      max-width="620"
      @update:model-value="(open) => !open && closePreferences()"
    >
      <v-card ref="preferencesCard" class="user-preferences-dialog" rounded="lg">
        <form @submit.prevent="savePreferences">
          <v-card-text class="user-preferences-fields">
            <div class="user-preferences-profile">
              <button
                aria-label="Upload a new avatar"
                class="user-preferences-avatar-button"
                :disabled="avatarUploading"
                type="button"
                @click="chooseAvatar"
              >
                <img
                  :src="currentUser.avatar_url && !avatarFailed ? currentUser.avatar_url : defaultUserIcon"
                  alt=""
                  class="user-preferences-avatar"
                  referrerpolicy="no-referrer"
                  @error="avatarFailed = true"
                >
                <span class="user-preferences-avatar-overlay">
                  <v-icon :icon="mdiCamera" size="22" />
                  <span>{{ avatarUploading ? "Uploading" : "Change" }}</span>
                </span>
              </button>
              <input
                ref="avatarInput"
                accept=".jpg,.jpeg,.png,image/jpeg,image/png"
                class="user-preferences-avatar-input"
                type="file"
                @change="uploadAvatar"
              >

              <div class="user-preferences-profile-text">
                <p class="user-preferences-real-name">
                  {{ currentUser.display_name || currentUser.name }}
                </p>
                <p class="user-preferences-username">
                  @{{ currentUser.name }}
                </p>
                <p v-if="currentUser.email" class="user-preferences-email">
                  {{ currentUser.email }}
                </p>
              </div>
            </div>

            <p v-if="preferencesError" class="user-preferences-error" role="alert">
              {{ preferencesError }}
            </p>

            <div class="user-preferences-field">
              <label for="preference-username" class="service-field-label">Username</label>
              <p id="preference-username-help" class="service-field-help">
                Choose the username shown across Kaeru apps
              </p>
              <v-text-field
                id="preference-username"
                v-model="preferencesDraft.username"
                aria-describedby="preference-username-help"
                :disabled="preferencesLoading || preferencesSaving"
                hide-details="auto"
                variant="outlined"
              />
            </div>

            <div class="user-preferences-field">
              <label for="preference-name" class="service-field-label">Display Name</label>
              <p id="preference-name-help" class="service-field-help">
                Add an optional real or display name
              </p>
              <v-text-field
                id="preference-name"
                v-model="preferencesDraft.name"
                aria-describedby="preference-name-help"
                :disabled="preferencesLoading || preferencesSaving"
                hide-details="auto"
                placeholder="Name"
                variant="outlined"
              />
            </div>

            <div class="user-preferences-field">
              <label for="preference-email" class="service-field-label">Email</label>
              <p id="preference-email-help" class="service-field-help">
                Add an optional email address
              </p>
              <v-text-field
                id="preference-email"
                v-model="preferencesDraft.email"
                aria-describedby="preference-email-help"
                :disabled="preferencesLoading || preferencesSaving"
                hide-details="auto"
                placeholder="name@example.com"
                type="email"
                variant="outlined"
              />
            </div>

            <div class="user-preferences-field">
              <label for="preference-time-format" class="service-field-label">Time format</label>
              <p id="preference-time-format-help" class="service-field-help">
                Choose how times are displayed across Kaeru apps
              </p>
              <v-select
                id="preference-time-format"
                v-model="preferencesDraft.time_format"
                aria-describedby="preference-time-format-help"
                :disabled="preferencesLoading || preferencesSaving"
                :items="timeFormatOptions"
                hide-details="auto"
                variant="outlined"
              />
            </div>

            <div class="user-preferences-field">
              <label for="preference-timezone" class="service-field-label">Timezone</label>
              <p id="preference-timezone-help" class="service-field-help">
                Use your device timezone automatically or select a specific timezone
              </p>
              <v-autocomplete
                id="preference-timezone"
                v-model="preferencesDraft.timezone"
                aria-describedby="preference-timezone-help"
                :disabled="preferencesLoading || preferencesSaving"
                :items="timezoneOptions"
                hide-details="auto"
                variant="outlined"
              />
            </div>

            <div class="user-preferences-field">
              <label for="preference-theme" class="service-field-label">Theme</label>
              <p id="preference-theme-help" class="service-field-help">
                Choose a light or dark appearance
              </p>
              <v-select
                id="preference-theme"
                v-model="preferencesDraft.theme"
                aria-describedby="preference-theme-help"
                :disabled="preferencesLoading || preferencesSaving"
                :items="themeOptions"
                hide-details="auto"
                variant="outlined"
              />
            </div>
          </v-card-text>
          <v-card-actions>
            <v-spacer />
            <v-btn :disabled="preferencesSaving" variant="text" @click="closePreferences">
              Cancel
            </v-btn>
            <v-btn
              color="primary"
              :disabled="preferencesLoading"
              :loading="preferencesSaving"
              type="submit"
              variant="flat"
            >
              Save
            </v-btn>
          </v-card-actions>
        </form>
      </v-card>
    </v-dialog>

    <v-main>
      <div v-if="developmentMode" class="development-mode-banner" role="status">
        Development mode: authentication is bypassed
      </div>
      <router-view />
    </v-main>
  </v-app>
</template>
