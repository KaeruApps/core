<script setup>
import { mdiFormatListBulleted, mdiLogout, mdiAccountCog } from "@mdi/js";
import { onMounted, ref, watch } from "vue";
import { useRoute, useRouter } from "vue-router";
import { useTheme } from "vuetify";

import defaultUserIcon from "./assets/default-user.png";
import UserPreferencesDialog from "./components/users/UserPreferencesDialog.vue";
import { useSessionStore } from "./stores/session.js";

const route = useRoute();
const router = useRouter();
const theme = useTheme();
const session = useSessionStore();

const avatarFailed = ref(false);
const loggingOut = ref(false);
const preferencesOpen = ref(false);

// The router guard has already loaded the session for this navigation, so the
// shell only fills in the pieces it needs rather than refetching.
onMounted(async () => {
  if (session.authenticated) {
    try {
      await session.loadPreferences();
    } catch {
      // The preferences dialog reports its own load failures.
    }
  }
});

watch(() => session.preferences.theme, (preference) => {
  theme.change(preference === "light" ? "kaeruLight" : "kaeruDark");
}, { immediate: true });

async function logout() {
  loggingOut.value = true;
  try {
    await session.logout();
    await router.replace({ name: "login" });
  } finally {
    loggingOut.value = false;
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

      <nav
        v-if="!route.meta.setup && !route.meta.authentication"
        class="app-navigation"
        aria-label="Primary navigation"
      >
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

        <v-menu v-if="session.user" location="bottom end">
          <template #activator="{ props: activatorProps }">
            <v-btn
              v-bind="activatorProps"
              aria-label="User menu"
              class="user-menu-button"
              icon
              variant="text"
            >
              <img
                :src="session.user.avatar_url && !avatarFailed ? session.user.avatar_url : defaultUserIcon"
                alt=""
                class="user-menu-avatar"
                referrerpolicy="no-referrer"
                @error="avatarFailed = true"
              >
            </v-btn>
          </template>

          <v-list class="user-menu-list" density="compact">
            <v-list-item class="user-menu-identity">
              <div class="user-menu-identity-content">
                <img
                  :src="session.user.avatar_url && !avatarFailed ? session.user.avatar_url : defaultUserIcon"
                  alt=""
                  class="user-menu-identity-avatar"
                  referrerpolicy="no-referrer"
                  @error="avatarFailed = true"
                >
                <div class="user-menu-identity-text">
                  <p class="user-menu-real-name">
                    {{ session.user.display_name || session.user.name }}
                  </p>
                  <p class="user-menu-username">
                    @{{ session.user.name }}
                  </p>
                </div>
              </div>
            </v-list-item>
            <v-list-item
              :prepend-icon="mdiAccountCog"
              link
              title="Profile"
              @click="preferencesOpen = true"
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

    <UserPreferencesDialog v-model="preferencesOpen" />

    <v-main>
      <div v-if="session.developmentMode" class="development-mode-banner" role="status">
        Development mode: authentication is bypassed
      </div>
      <router-view />
    </v-main>
  </v-app>
</template>

<style scoped>
.development-mode-banner {
  width: 100%;
  padding: 6px 16px;
  background: rgba(var(--v-theme-warning), 0.16);
  color: rgb(var(--v-theme-warning));
  font-size: 0.8rem;
  font-weight: 700;
  text-align: center;
}

.v-application .app-brand,
.v-application .app-brand:active,
.v-application .app-brand:hover,
.v-application .app-brand:visited {
  display: inline-flex;
  align-items: center;
  position: relative;
  overflow: hidden;
  margin-left: 8px;
  padding: 6px 8px;
  color: inherit;
  border-radius: 4px;
  text-decoration: none;
}

.v-application .app-brand:focus-visible {
  outline: 2px solid rgb(var(--v-theme-primary));
  outline-offset: 2px;
}

.app-title {
  flex: 0 0 auto;
  font-family: "Roboto Condensed Variable", "Arial Narrow", sans-serif;
  font-size: 1.3rem;
  font-weight: 700;
  letter-spacing: -0.025em;
  margin-inline-start: 12px;
}

.app-navigation {
  align-items: center;
  display: flex;
  gap: 4px;
  margin-right: 16px;
  margin-left: auto;
}

.user-menu-button {
  flex: 0 0 auto;
}

.user-menu-avatar {
  display: block;
  width: 36px;
  height: 36px;
  border-radius: 50%;
  object-fit: cover;
}

.user-menu-list {
  min-width: 260px;
}

.user-menu-identity {
  margin-bottom: 8px;
  padding-bottom: 12px !important;
  border-bottom: 1px solid rgba(var(--v-theme-on-surface), 0.12);
}

.user-menu-identity-content {
  display: flex;
  gap: 12px;
  align-items: center;
}

.user-menu-identity-avatar {
  flex: 0 0 auto;
  width: 42px;
  height: 42px;
  border-radius: 50%;
  object-fit: cover;
}

.user-menu-identity-text {
  min-width: 0;
}

.user-menu-identity-text p {
  margin: 0;
  line-height: 1.35;
  overflow-wrap: anywhere;
}

.user-menu-real-name {
  color: rgb(var(--v-theme-on-surface));
  font-weight: 600;
}

.user-menu-username {
  color: rgba(var(--v-theme-on-surface), 0.68);
  font-size: 0.8125rem;
}

.nav-button {
  font-weight: 600;
  letter-spacing: 0;
  text-transform: none;
}

.nav-button :deep(.v-btn__content) {
  gap: 8px;
}

.nav-button :deep(.v-btn__overlay) {
  opacity: 0 !important;
}

@media (max-width: 600px) {
  .nav-button {
    min-width: 48px;
    padding-inline: 12px;
  }

  .nav-button-label {
    display: none;
  }

  .nav-button--event-log .nav-button-label {
    display: inline;
  }

  .nav-button :deep(.v-btn__content) {
    gap: 0;
  }

  .nav-button--event-log :deep(.v-btn__content) {
    gap: 8px;
  }
}
</style>
