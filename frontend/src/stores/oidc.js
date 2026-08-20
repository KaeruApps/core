import { defineStore } from "pinia";
import { computed, ref } from "vue";

import { fetchJSON, sendForm } from "../api/client.js";
import oidcFallbackIcon from "../assets/oidc.png";

const verificationDraftKey = "kaeru.oidc-settings-verification-draft";
const verificationMetadataKey = "kaeru.oidc-settings-verification-metadata";

function emptyDraft() {
  return {
    name: "",
    accessUrls: [],
    issuerUrl: "",
    clientId: "",
    clientSecret: "",
    additionalScopes: "",
    usernameClaim: "",
    displayNameClaim: "",
    avatarClaim: "",
    groupsClaim: "",
    adminGroups: "",
    buttonText: "",
  };
}

function normalizedValues(value, separator = /[\s,]+/) {
  return [...new Set(value.split(separator).map((item) => item.trim()).filter(Boolean))].sort();
}

function sameValues(left, right, separator) {
  return JSON.stringify(normalizedValues(left, separator))
    === JSON.stringify(normalizedValues(right, separator));
}

/**
 * Owns OIDC settings and the provider round-trip that verifies them.
 *
 * Verification is shared state on purpose: changing Kaeru Core administrator
 * group mappings from the service configuration dialog has to raise the same
 * dialog and report its result back in the services section, so neither
 * section can own the flow alone.
 */
export const useOIDCStore = defineStore("oidc", () => {
  const settings = ref(null);
  const draft = ref(emptyDraft());
  const baseline = ref(null);
  const buttonImage = ref(null);

  const expanded = ref(false);
  const closing = ref(false);
  const loading = ref(true);
  const saving = ref(false);
  const saved = ref(false);
  const error = ref("");

  const imageFailed = ref(false);
  const imageVersion = ref(Date.now());

  const verificationDialog = ref(false);
  const verificationChanges = ref([]);
  const verificationSource = ref("oidc");
  const verificationStarting = ref(false);
  const verificationStatus = ref(null);
  const coreAdminVerificationStatus = ref(null);

  // Set to "oidc" or "services" after a verification round-trip so the matching
  // section can scroll itself into view, then cleared by that section.
  const scrollTarget = ref(null);

  let savedTimer = null;
  let closeTimer = null;
  let verificationStatusTimer = null;
  let coreAdminStatusTimer = null;

  const summaryIcon = computed(() => (
    settings.value?.button_image_configured && !imageFailed.value
      ? `/api/v1/oidc/settings/button-image?v=${imageVersion.value}`
      : oidcFallbackIcon
  ));

  /** Admin group changes alone do not invalidate existing sessions. */
  const verificationRevokesSessions = computed(() => (
    verificationChanges.value.some((change) => change !== "Admin Groups")
  ));

  function applySettings(loaded) {
    settings.value = loaded;
    imageFailed.value = false;
    const publicUrl = loaded.public_url ?? "";
    const accessUrls = (loaded.access_urls ?? [])
      .filter((accessUrl) => accessUrl && accessUrl !== publicUrl);
    draft.value = {
      name: loaded.name,
      accessUrls: [publicUrl, ...accessUrls],
      issuerUrl: loaded.issuer_url,
      clientId: loaded.client_id,
      clientSecret: "",
      additionalScopes: (loaded.additional_scopes ?? []).join(" "),
      usernameClaim: loaded.username_claim,
      displayNameClaim: loaded.display_name_claim ?? "",
      avatarClaim: loaded.avatar_claim ?? "",
      groupsClaim: loaded.groups_claim,
      adminGroups: (loaded.admin_groups ?? []).join(", "),
      buttonText: loaded.button_text,
    };
    baseline.value = JSON.parse(JSON.stringify(draft.value));
  }

  /** Settings that change how users sign in and therefore need verification. */
  function authenticationChanges() {
    const current = baseline.value;
    const proposed = draft.value;
    if (!current) return [];
    const changes = [];
    if (proposed.issuerUrl.trim() !== current.issuerUrl.trim()) changes.push("Issuer URL");
    if (proposed.clientId.trim() !== current.clientId.trim()) changes.push("Client ID");
    if (proposed.clientSecret) changes.push("Client Secret");
    if (!sameValues(proposed.additionalScopes, current.additionalScopes, /\s+/)) {
      changes.push("Additional Scopes");
    }
    if (proposed.usernameClaim.trim() !== current.usernameClaim.trim()) changes.push("Username Claim");
    if (proposed.groupsClaim.trim() !== current.groupsClaim.trim()) changes.push("Groups Claim");
    if (!sameValues(proposed.adminGroups, current.adminGroups)) changes.push("Admin Groups");
    const proposedURLs = proposed.accessUrls.map((url) => url.trim()).filter(Boolean);
    const currentURLs = current.accessUrls.map((url) => url.trim()).filter(Boolean);
    if (JSON.stringify(proposedURLs) !== JSON.stringify(currentURLs)) changes.push("Access URLs");
    return changes;
  }

  function buildSettingsForm() {
    const form = new FormData();
    const proposed = draft.value;
    form.set("name", proposed.name);
    form.set("access_urls", proposed.accessUrls.join(","));
    form.set("issuer_url", proposed.issuerUrl);
    form.set("client_id", proposed.clientId);
    form.set("client_secret", proposed.clientSecret);
    form.set("additional_scopes", proposed.additionalScopes);
    form.set("username_claim", proposed.usernameClaim);
    form.set("display_name_claim", proposed.displayNameClaim);
    form.set("avatar_claim", proposed.avatarClaim);
    form.set("groups_claim", proposed.groupsClaim);
    form.set("admin_groups", proposed.adminGroups);
    form.set("button_text", proposed.buttonText);
    const image = Array.isArray(buttonImage.value) ? buttonImage.value[0] : buttonImage.value;
    if (image) form.set("button_image", image);
    return form;
  }

  function clearMessages() {
    error.value = "";
    verificationStatus.value = null;
    if (verificationStatusTimer !== null) {
      window.clearTimeout(verificationStatusTimer);
      verificationStatusTimer = null;
    }
  }

  async function load() {
    loading.value = true;
    error.value = "";
    try {
      applySettings(await fetchJSON("/api/v1/oidc/settings", "Unable to load OIDC settings."));
      imageVersion.value = Date.now();
    } catch (loadError) {
      error.value = loadError instanceof Error ? loadError.message : "Unable to load OIDC settings.";
    } finally {
      loading.value = false;
    }
  }

  /**
   * Saves directly when nothing security-relevant changed, otherwise opens the
   * verification dialog. Returns true when a save actually happened.
   */
  async function save() {
    clearMessages();
    const changes = authenticationChanges();
    if (changes.length > 0) {
      verificationSource.value = "oidc";
      verificationChanges.value = changes;
      verificationDialog.value = true;
      return false;
    }
    saving.value = true;
    saved.value = false;
    error.value = "";
    try {
      applySettings(await sendForm(
        "/api/v1/oidc/settings",
        "PUT",
        buildSettingsForm(),
        "Unable to save OIDC settings.",
      ));
      imageVersion.value = Date.now();
      buttonImage.value = null;
      saved.value = true;
      if (savedTimer !== null) window.clearTimeout(savedTimer);
      savedTimer = window.setTimeout(() => {
        saved.value = false;
        savedTimer = null;
      }, 2500);
      return true;
    } catch (saveError) {
      error.value = saveError instanceof Error ? saveError.message : "Unable to save OIDC settings.";
      throw saveError;
    } finally {
      saving.value = false;
    }
  }

  /** Raised from the service configuration dialog for Core admin group edits. */
  function requestCoreAdminVerification(adminGroups) {
    draft.value.adminGroups = adminGroups.join(", ");
    expanded.value = true;
    verificationSource.value = "core-admin";
    verificationChanges.value = ["Admin Groups"];
    verificationDialog.value = true;
  }

  function cancelVerification() {
    verificationDialog.value = false;
  }

  /**
   * Stashes the pending draft, then hands the browser to the provider. The
   * draft is restored on the way back so a failed verification does not lose
   * what the administrator typed.
   */
  async function startVerification() {
    verificationStarting.value = true;
    error.value = "";
    try {
      const body = await sendForm(
        "/api/v1/oidc/settings/verify",
        "POST",
        buildSettingsForm(),
        "OIDC verification could not be started.",
      );
      window.sessionStorage.setItem(verificationDraftKey, JSON.stringify(draft.value));
      window.sessionStorage.setItem(verificationMetadataKey, JSON.stringify({
        source: verificationSource.value,
        revokesSessions: verificationRevokesSessions.value,
      }));
      window.location.assign(body.authorization_url);
    } catch (startError) {
      verificationStarting.value = false;
      verificationDialog.value = false;
      error.value = startError instanceof Error
        ? startError.message
        : "OIDC verification could not be started.";
      throw startError;
    }
  }

  /** Reads the ?oidc_verification result the provider redirect leaves behind. */
  function handleVerificationReturn() {
    const query = new URLSearchParams(window.location.search);
    const result = query.get("oidc_verification");
    if (!result) return;

    let metadata = null;
    try {
      const savedMetadata = window.sessionStorage.getItem(verificationMetadataKey);
      if (savedMetadata) metadata = JSON.parse(savedMetadata);
    } catch {
      // Fall back to the conservative message when metadata is unavailable.
    }
    const coreAdminVerification = metadata?.source === "core-admin";
    if (!coreAdminVerification) expanded.value = true;

    if (result === "success") {
      window.sessionStorage.removeItem(verificationDraftKey);
      if (coreAdminVerification) {
        coreAdminVerificationStatus.value = {
          type: "success",
          message: "Core administrator access changes verified and saved. Existing sessions remain signed in.",
        };
      } else {
        verificationStatus.value = {
          type: "success",
          message: metadata?.revokesSessions === false
            ? "OIDC settings verified and saved. Existing sessions remain signed in."
            : "OIDC settings verified and saved. All existing sessions have been revoked.",
        };
        verificationStatusTimer = window.setTimeout(() => {
          verificationStatus.value = null;
          verificationStatusTimer = null;
        }, 5000);
      }
    } else {
      if (!coreAdminVerification) {
        try {
          const savedDraft = window.sessionStorage.getItem(verificationDraftKey);
          if (savedDraft) draft.value = JSON.parse(savedDraft);
        } catch {
          // Keep the active configuration when the draft cannot be restored.
        }
      }
      window.sessionStorage.removeItem(verificationDraftKey);
      const failureStatus = {
        type: "error",
        message: coreAdminVerification
          ? "Core administrator access changes could not be verified. Your existing access configuration remains active."
          : "OIDC settings could not be verified. Your existing configuration remains active.",
        detail: query.get("error") || "Please review the proposed settings and try again.",
      };
      if (coreAdminVerification) coreAdminVerificationStatus.value = failureStatus;
      else verificationStatus.value = failureStatus;
    }

    if (coreAdminVerification) {
      if (coreAdminStatusTimer !== null) window.clearTimeout(coreAdminStatusTimer);
      coreAdminStatusTimer = window.setTimeout(() => {
        coreAdminVerificationStatus.value = null;
        coreAdminStatusTimer = null;
      }, 5000);
    }
    window.sessionStorage.removeItem(verificationMetadataKey);
    window.history.replaceState({}, "", window.location.pathname);
    scrollTarget.value = coreAdminVerification ? "services" : "oidc";
  }

  /** Collapses the panel after scrolling it back into view. */
  function beginClose(onScrollIntoView) {
    clearMessages();
    if (closing.value) return;
    const reduceMotion = window.matchMedia("(prefers-reduced-motion: reduce)").matches;
    if (reduceMotion || !onScrollIntoView?.()) {
      expanded.value = false;
      return;
    }
    closing.value = true;
    closeTimer = window.setTimeout(() => {
      expanded.value = false;
      closing.value = false;
      closeTimer = null;
    }, 500);
  }

  function dispose() {
    for (const timer of [savedTimer, closeTimer, verificationStatusTimer, coreAdminStatusTimer]) {
      if (timer !== null) window.clearTimeout(timer);
    }
    savedTimer = null;
    closeTimer = null;
    verificationStatusTimer = null;
    coreAdminStatusTimer = null;
  }

  return {
    settings,
    draft,
    baseline,
    buttonImage,
    expanded,
    closing,
    loading,
    saving,
    saved,
    error,
    imageFailed,
    imageVersion,
    verificationDialog,
    verificationChanges,
    verificationSource,
    verificationStarting,
    verificationStatus,
    coreAdminVerificationStatus,
    scrollTarget,
    summaryIcon,
    verificationRevokesSessions,
    applySettings,
    authenticationChanges,
    clearMessages,
    load,
    save,
    requestCoreAdminVerification,
    cancelVerification,
    startVerification,
    handleVerificationReturn,
    beginClose,
    dispose,
  };
});
