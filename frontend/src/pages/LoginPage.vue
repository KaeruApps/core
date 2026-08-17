<script setup>
import { computed, onMounted, ref } from "vue";
import { useRoute } from "vue-router";
import oidcFallbackIcon from "../assets/oidc.png";

const route = useRoute();
const startingLogin = ref(false);
const loginError = ref("");
const branding = ref({
  name: "OIDC",
  button_text: "Login with OIDC",
  button_image_configured: false,
});
const brandingImageFailed = ref(false);
const providerError = computed(() => typeof route.query.error === "string" ? route.query.error : "");
const brandingImage = computed(() => (
  branding.value.button_image_configured && !brandingImageFailed.value
    ? "/api/v1/auth/oidc/button-image"
    : oidcFallbackIcon
));

onMounted(async () => {
  try {
    const response = await fetch("/api/v1/auth/oidc/branding", {
      headers: { Accept: "application/json" },
    });
    if (response.ok) branding.value = await response.json();
  } catch {
    // The fallback branding keeps the login action available.
  }
});

async function login() {
  startingLogin.value = true;
  loginError.value = "";
  try {
    const response = await fetch("/api/v1/auth/oidc/login", {
      method: "POST",
      headers: { Accept: "application/json" },
    });
    const body = await response.json();
    if (!response.ok) {
      throw new Error(body?.error?.message || "Login could not be started.");
    }
    window.location.assign(body.authorization_url);
  } catch (error) {
    loginError.value = error.message;
    startingLogin.value = false;
  }
}
</script>

<template>
  <v-container class="setup-page login-page">
    <section class="setup-panel login-panel" aria-labelledby="login-title">
      <div class="setup-heading">
        <span class="setup-app-icon" aria-hidden="true" />
        <div>
          <h1 id="login-title">Log in to Kaeru</h1>
          <p>Use your configured identity provider to continue.</p>
        </div>
      </div>

      <div class="login-actions">
        <p v-if="providerError || loginError" class="oidc-setup-error" role="alert">
          {{ loginError || providerError }}
        </p>
        <v-btn
          block
          color="primary"
          :loading="startingLogin"
          size="large"
          variant="outlined"
          @click="login"
        >
          <img
            :src="brandingImage"
            alt=""
            class="login-provider-icon"
            @error="brandingImageFailed = true"
          />
          {{ branding.button_text }}
        </v-btn>
      </div>
    </section>
  </v-container>
</template>
