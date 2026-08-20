import { defineStore } from "pinia";
import { ref } from "vue";

import { apiFetch, apiErrorMessage, fetchJSON, fetchPublicJSON, sendForm, sendJSON } from "../api/client.js";

const preferenceDefaults = {
  username: "",
  name: "",
  email: "",
  time_format: "24h",
  timezone: "automatic",
  theme: "dark",
};

/**
 * Owns everything about who is signed in: installation state, the current
 * principal, and that user's stored preferences. The router guard and the app
 * shell both read from here so the session is fetched once per navigation
 * rather than once per consumer.
 */
export const useSessionStore = defineStore("session", () => {
  const initialized = ref(false);
  const developmentMode = ref(false);
  const statusLoaded = ref(false);
  const user = ref(null);
  const authenticated = ref(false);
  const preferences = ref({ ...preferenceDefaults });
  const preferencesBaseline = ref(null);
  const preferencesLoading = ref(false);

  /** True when the signed-in user holds the Kaeru Core administrator role. */
  function isCoreAdministrator() {
    return user.value?.service_roles?.core === "admin";
  }

  async function refreshStatus() {
    const status = await fetchPublicJSON("/api/v1/setup/status");
    initialized.value = status.initialized === true;
    developmentMode.value = status.development_mode === true;
    statusLoaded.value = true;
    return status;
  }

  async function refreshSession() {
    if (!initialized.value) {
      user.value = null;
      authenticated.value = false;
      return false;
    }
    try {
      const session = await fetchPublicJSON("/api/v1/session");
      user.value = session.user;
      authenticated.value = true;
    } catch {
      user.value = null;
      authenticated.value = false;
    }
    return authenticated.value;
  }

  async function loadPreferences() {
    preferencesLoading.value = true;
    try {
      const loaded = await fetchJSON(
        "/api/v1/users/me/preferences",
        "User preferences could not be loaded.",
      );
      preferences.value = {
        ...loaded,
        name: loaded.name || "",
        email: loaded.email || "",
      };
      preferencesBaseline.value = { ...preferences.value };
      return preferences.value;
    } finally {
      preferencesLoading.value = false;
    }
  }

  /**
   * Saves only the named fields on top of the last known server state so an
   * unopened settings tab can never overwrite values the user did not edit.
   */
  async function savePreferences(draft, fields) {
    const payload = { ...(preferencesBaseline.value ?? preferences.value) };
    for (const field of fields) {
      payload[field] = draft[field];
    }
    const saved = await sendJSON(
      "/api/v1/users/me/preferences",
      "PUT",
      payload,
      "User preferences could not be saved.",
    );
    const normalized = { ...saved, name: saved.name || "", email: saved.email || "" };
    preferences.value = normalized;
    preferencesBaseline.value = { ...normalized };
    if (fields.includes("username")) {
      user.value = {
        ...user.value,
        name: normalized.username,
        display_name: normalized.name,
        email: normalized.email,
      };
    }
    return normalized;
  }

  async function uploadAvatar(file) {
    const form = new FormData();
    form.append("avatar", file);
    const body = await sendForm(
      "/api/v1/users/me/avatar",
      "PUT",
      form,
      "Avatar could not be uploaded.",
    );
    user.value = { ...user.value, avatar_url: body.avatar_url };
    return body;
  }

  async function logout() {
    const response = await apiFetch("/api/v1/session/logout", {
      method: "POST",
      headers: { Accept: "application/json" },
    });
    if (!response.ok) {
      throw new Error(await apiErrorMessage(response, "Logout failed."));
    }
    user.value = null;
    authenticated.value = false;
  }

  /** Clears the session without surfacing an error, used by the router guard. */
  async function logoutQuietly() {
    try {
      await logout();
    } catch {
      user.value = null;
      authenticated.value = false;
    }
  }

  return {
    initialized,
    developmentMode,
    statusLoaded,
    user,
    authenticated,
    preferences,
    preferencesBaseline,
    preferencesLoading,
    isCoreAdministrator,
    refreshStatus,
    refreshSession,
    loadPreferences,
    savePreferences,
    uploadAvatar,
    logout,
    logoutQuietly,
  };
});
