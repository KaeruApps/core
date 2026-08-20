<script setup>
import { nextTick, onMounted, ref } from "vue";
import { useRoute } from "vue-router";
import OIDCConfigurationForm from "../components/OIDCConfigurationForm.vue";
import SetupHeading from "../components/layout/SetupHeading.vue";

const route = useRoute();
const setupDraftStorageKey = "kaeru.oidc-setup-draft";

const oidcConfiguration = ref({
  publicUrl: window.location.origin,
  name: "",
  issuerUrl: "",
  clientId: "",
  clientSecret: "",
  additionalScopes: "",
  usernameClaim: "preferred_username",
  displayNameClaim: "",
  avatarClaim: "",
  groupsClaim: "groups",
  adminGroups: "",
  buttonText: "Log in with OIDC",
});
const buttonImage = ref(null);
const setupError = ref("");
const setupErrorElement = ref(null);
const setupSubmitting = ref(false);

const maxButtonImageBytes = 1024 * 1024;
const allowedButtonImageTypes = new Set(["image/jpeg", "image/png"]);

function selectedFile(value) {
  return Array.isArray(value) ? value[0] : value;
}

function saveSetupDraft() {
  try {
    window.sessionStorage.setItem(
      setupDraftStorageKey,
      JSON.stringify(oidcConfiguration.value),
    );
  } catch {
    // Setup can continue if browser storage is unavailable.
  }
}

function restoreSetupDraft() {
  try {
    const savedDraft = window.sessionStorage.getItem(setupDraftStorageKey);
    window.sessionStorage.removeItem(setupDraftStorageKey);
    if (!savedDraft) return;

    const parsedDraft = JSON.parse(savedDraft);
    for (const field of Object.keys(oidcConfiguration.value)) {
      if (typeof parsedDraft[field] === "string") {
        oidcConfiguration.value[field] = parsedDraft[field];
      }
    }
  } catch {
    window.sessionStorage.removeItem(setupDraftStorageKey);
  }
}

async function showSetupError(message) {
  setupError.value = message;
  await nextTick();
  setupErrorElement.value?.focus({ preventScroll: true });
  setupErrorElement.value?.scrollIntoView({
    behavior: "smooth",
    block: "center",
  });
}

onMounted(() => {
  if (typeof route.query.error === "string" && route.query.error) {
    restoreSetupDraft();
    showSetupError(route.query.error);
  } else {
    window.sessionStorage.removeItem(setupDraftStorageKey);
  }
});

async function beginOIDCLogin() {
  setupError.value = "";
  const requiredFields = [
    [oidcConfiguration.value.publicUrl, "Public URL"],
    [oidcConfiguration.value.name, "Name"],
    [oidcConfiguration.value.issuerUrl, "Issuer URL"],
    [oidcConfiguration.value.clientId, "Client ID"],
    [oidcConfiguration.value.clientSecret, "Client Secret"],
    [oidcConfiguration.value.usernameClaim, "Username Claim"],
    [oidcConfiguration.value.groupsClaim, "Groups Claim"],
    [oidcConfiguration.value.adminGroups, "Admin Groups"],
    [oidcConfiguration.value.buttonText, "Button Text"],
  ];
  const missing = requiredFields.find(([value]) => !value.trim());
  if (missing) {
    await showSetupError(`${missing[1]} is required.`);
    return;
  }

  const image = selectedFile(buttonImage.value);
  if (image && !allowedButtonImageTypes.has(image.type)) {
    await showSetupError("Button image must be a JPG or PNG file.");
    return;
  }
  if (image && image.size > maxButtonImageBytes) {
    await showSetupError("Button image must be 1 MB or smaller.");
    return;
  }

  const form = new FormData();
  form.set("public_url", oidcConfiguration.value.publicUrl);
  form.set("name", oidcConfiguration.value.name);
  form.set("issuer_url", oidcConfiguration.value.issuerUrl);
  form.set("client_id", oidcConfiguration.value.clientId);
  form.set("client_secret", oidcConfiguration.value.clientSecret);
  form.set("additional_scopes", oidcConfiguration.value.additionalScopes);
  form.set("username_claim", oidcConfiguration.value.usernameClaim);
  form.set("display_name_claim", oidcConfiguration.value.displayNameClaim);
  form.set("avatar_claim", oidcConfiguration.value.avatarClaim);
  form.set("groups_claim", oidcConfiguration.value.groupsClaim);
  form.set("admin_groups", oidcConfiguration.value.adminGroups);
  form.set("button_text", oidcConfiguration.value.buttonText);
  if (image) {
    form.set("button_image", image);
  }

  setupSubmitting.value = true;
  try {
    const response = await fetch("/api/v1/setup/oidc", {
      method: "POST",
      headers: { Accept: "application/json" },
      body: form,
    });
    const body = await response.json().catch(() => ({}));
    if (!response.ok) {
      throw new Error(body.error?.message || "OIDC setup could not be saved.");
    }
    if (!body.authorization_url) {
      throw new Error("The OIDC provider did not return an authorization URL.");
    }
    saveSetupDraft();
    window.location.assign(body.authorization_url);
  } catch (error) {
    setupSubmitting.value = false;
    await showSetupError(error instanceof Error
      ? error.message
      : "OIDC setup could not be saved.");
  }
}
</script>

<template>
  <v-container class="setup-page">
    <section class="setup-panel" aria-labelledby="oidc-setup-title">
      <SetupHeading
        title-id="oidc-setup-title"
        title="Set up OIDC"
        subtitle="Connect Kaeru to your OpenID Connect identity provider."
      />

      <v-card class="oidc-setup-card" rounded="lg" variant="outlined">
        <form class="oidc-setup-form" novalidate @submit.prevent="beginOIDCLogin">
          <v-card-text class="oidc-setup-fields">
            <p
              v-if="setupError"
              ref="setupErrorElement"
              class="oidc-setup-error"
              role="alert"
              tabindex="-1"
            >
              {{ setupError }}
            </p>
            <OIDCConfigurationForm
              v-model:button-image="buttonImage"
              v-model:configuration="oidcConfiguration"
              client-secret-required
            />
          </v-card-text>
          <v-card-actions class="oidc-setup-actions">
            <v-btn :disabled="setupSubmitting" to="/setup" variant="text">Back</v-btn>
            <v-spacer />
            <v-btn color="primary" :loading="setupSubmitting" type="submit" variant="flat">
              Login
            </v-btn>
          </v-card-actions>
        </form>
      </v-card>
    </section>
  </v-container>
</template>

<style scoped>
.oidc-setup-card {
  background: rgb(var(--v-theme-surface));
}

.oidc-setup-fields {
  display: grid;
  gap: 1.5rem;
  padding: clamp(1.25rem, 4vw, 2rem);
}

.oidc-setup-actions {
  padding: 0 clamp(1.25rem, 4vw, 2rem) clamp(1.25rem, 4vw, 2rem);
}

@media (max-width: 600px) {
  .oidc-setup-fields {
    padding-right: 24px;
    padding-left: 24px;
  }

  .oidc-setup-actions {
    padding-right: 24px;
    padding-left: 24px;
  }
}
</style>
