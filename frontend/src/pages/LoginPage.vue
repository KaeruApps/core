<script setup>
import { computed, onMounted, ref } from "vue";
import { useRoute } from "vue-router";
import SetupHeading from "../components/layout/SetupHeading.vue";

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
// The provider button shows a configured branding image or nothing at all.
// There is no generic fallback icon: a missing or broken image leaves a
// text-only button rather than branding the provider incorrectly.
const brandingImageUrl = computed(() => (
  branding.value.button_image_configured && !brandingImageFailed.value
    ? "/api/v1/auth/oidc/button-image"
    : null
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
      <SetupHeading
        title-id="login-title"
        title="Log in to Kaeru"
        subtitle="Use your configured identity provider to continue."
      />

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
            v-if="brandingImageUrl"
            :src="brandingImageUrl"
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

<style scoped>
.login-panel {
  width: min(100%, 34rem);
}

.login-actions {
  display: grid;
  gap: 20px;
}

.login-provider-icon {
  border-radius: 4px;
  height: 24px;
  margin-right: 10px;
  object-fit: cover;
  width: 24px;
}
</style>
