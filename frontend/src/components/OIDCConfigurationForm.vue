<script setup>
import { computed } from "vue";
import { mdiClose, mdiPaperclip, mdiPlus, mdiCamera } from "@mdi/js";

const configuration = defineModel("configuration", { required: true });
const buttonImage = defineModel("buttonImage");
const props = defineProps({
  clientSecretRequired: { type: Boolean, default: false },
  existingButtonImage: { type: Boolean, default: false },
  accessUrlsMode: { type: Boolean, default: false },
});

const callbackUrl = computed(() => {
  const publicUrl = configuration.value.publicUrl.trim().replace(/\/+$/, "");
  return publicUrl ? `${publicUrl}/api/v1/auth/oidc/callback` : "";
});

const imageRules = [
  (value) => {
    const file = Array.isArray(value) ? value[0] : value;
    return !file || ["image/jpeg", "image/png"].includes(file.type)
      || "Button image must be a JPG or PNG file.";
  },
  (value) => {
    const file = Array.isArray(value) ? value[0] : value;
    return !file || file.size <= 1024 * 1024
      || "Button image must be 1 MB or smaller.";
  },
];

function addAccessUrl() {
  configuration.value.accessUrls.push("");
}

function removeAccessUrl(index) {
  if (index > 0) {
    configuration.value.accessUrls.splice(index, 1);
  }
}
</script>

<template>
  <div class="setup-form-field">
    <label for="oidc-provider-name" class="service-field-label">
      Name <span class="required-field-marker" aria-hidden="true">*</span>
    </label>
    <p class="backup-field-help">Name used to identify this OIDC configuration</p>
    <v-text-field id="oidc-provider-name" v-model="configuration.name" hide-details="auto" placeholder="Identity provider name" required variant="outlined" />
  </div>

  <div v-if="!props.accessUrlsMode" class="setup-form-field">
    <label for="oidc-public-url" class="service-field-label">
      Public URL <span class="required-field-marker" aria-hidden="true">*</span>
    </label>
    <p class="backup-field-help">URL used to access Kaeru Core</p>
    <v-text-field id="oidc-public-url" v-model="configuration.publicUrl" autocomplete="url" hide-details="auto" placeholder="https://kaeru.example.com" required type="url" variant="outlined" />
    <p class="oidc-callback-url-help">Your callback URL is <strong>{{ callbackUrl }}</strong></p>
  </div>

  <div v-else class="setup-form-field">
    <label for="oidc-access-url-0" class="service-field-label">
      Access URLs <span class="required-field-marker" aria-hidden="true">*</span>
    </label>
    <p class="backup-field-help">URLs allowed to initiate OIDC login. Each URL should be an allowed redirect URL in your IdP provider with the format <b>{url}/api/v1/auth/oidc/callback</b>. The first URL is always your configured public URL for Kaeru Core, the redirect URL for it is <b>{{ configuration.accessUrls[0] }}/api/v1/auth/oidc/callback</b></p>
    <div class="oidc-access-url-list">
      <div
        v-for="(_, index) in configuration.accessUrls"
        :key="index"
        class="oidc-access-url-row"
      >
        <v-text-field
          :id="`oidc-access-url-${index}`"
          v-model="configuration.accessUrls[index]"
          autocomplete="url"
          hide-details="auto"
          :placeholder="index === 0 ? 'Kaeru Core public URL' : 'https://kaeru.example.com'"
          :readonly="index === 0"
          required
          type="url"
          variant="outlined"
        />
        <v-btn
          v-if="index > 0"
          :aria-label="`Remove access URL ${index + 1}`"
          class="oidc-access-url-remove"
          color="error"
          :icon="mdiClose"
          size="small"
          type="button"
          variant="text"
          @click="removeAccessUrl(index)"
        />
      </div>
    </div>
    <v-btn
      class="oidc-add-access-url"
      color="primary"
      :prepend-icon="mdiPlus"
      type="button"
      variant="text"
      @click="addAccessUrl"
    >
      Add access URL
    </v-btn>
  </div>

  <div class="setup-form-field">
    <label for="oidc-issuer-url" class="service-field-label">
      Issuer URL <span class="required-field-marker" aria-hidden="true">*</span>
    </label>
    <p class="backup-field-help">URL used to discover your OIDC provider<span v-if="props.accessUrlsMode">. Warning: if you change this you are changing OIDC providers and current users will lose access to their accounts.</span><span v-else> (e.g https://auth.example.com/application/o/kaeru/)</span></p>
    <v-text-field id="oidc-issuer-url" v-model="configuration.issuerUrl" autocomplete="url" hide-details="auto" placeholder="https://auth.example.com/application/o/kaeru/" required type="url" variant="outlined" />
  </div>

  <div class="setup-form-field">
    <label for="oidc-client-id" class="service-field-label">
      Client ID <span class="required-field-marker" aria-hidden="true">*</span>
    </label>
    <p class="backup-field-help">Client identifier assigned to Kaeru by your OIDC provider</p>
    <v-text-field id="oidc-client-id" v-model="configuration.clientId" autocomplete="off" hide-details="auto" placeholder="Client ID" required variant="outlined" />
  </div>

  <div class="setup-form-field">
    <label for="oidc-client-secret" class="service-field-label">
      Client Secret <span v-if="props.clientSecretRequired" class="required-field-marker" aria-hidden="true">*</span>
    </label>
    <p class="backup-field-help">
      {{ props.clientSecretRequired ? "Secret assigned to Kaeru by your OIDC provider" : "Leave empty to keep the currently configured secret" }}
    </p>
    <v-text-field id="oidc-client-secret" v-model="configuration.clientSecret" autocomplete="new-password" hide-details="auto" :placeholder="props.clientSecretRequired ? 'Client secret' : 'Leave empty to keep current secret'" :required="props.clientSecretRequired" type="password" variant="outlined" />
  </div>

  <div class="setup-form-field">
    <label for="oidc-additional-scopes" class="service-field-label">Additional Scopes</label>
    <p class="backup-field-help">Optional space-separated scopes requested in addition to openid, profile, and email</p>
    <v-text-field id="oidc-additional-scopes" v-model="configuration.additionalScopes" hide-details="auto" placeholder="groups" variant="outlined" />
  </div>

  <div class="setup-form-field">
    <label for="oidc-username-claim" class="service-field-label">Username Claim <span class="required-field-marker" aria-hidden="true">*</span></label>
    <p class="backup-field-help">OIDC claim used as the user's username</p>
    <v-text-field id="oidc-username-claim" v-model="configuration.usernameClaim" hide-details="auto" placeholder="preferred_username" required variant="outlined" />
  </div>

  <div class="setup-form-field">
    <label for="oidc-display-name-claim" class="service-field-label">Display Name Claim</label>
    <p class="backup-field-help">Optional claim containing the user's display name, defaults to username if not provided</p>
    <v-text-field id="oidc-display-name-claim" v-model="configuration.displayNameClaim" hide-details="auto" placeholder="name" variant="outlined" />
  </div>

  <div class="setup-form-field">
    <label for="oidc-avatar-claim" class="service-field-label">Avatar Picture Claim</label>
    <p class="backup-field-help">Optional claim containing the URL for the user's avatar</p>
    <v-text-field id="oidc-avatar-claim" v-model="configuration.avatarClaim" hide-details="auto" placeholder="picture" variant="outlined" />
  </div>

  <div class="setup-form-field">
    <label for="oidc-groups-claim" class="service-field-label">Groups Claim <span class="required-field-marker" aria-hidden="true">*</span></label>
    <p class="backup-field-help">OIDC claim containing the user's group memberships. Access to Kaeru Services is managed by user groups and is therefore a required claim.</p>
    <v-text-field id="oidc-groups-claim" v-model="configuration.groupsClaim" hide-details="auto" placeholder="groups" required variant="outlined" />
  </div>

  <div class="setup-form-field">
    <label for="oidc-admin-groups" class="service-field-label">Admin Groups <span class="required-field-marker" aria-hidden="true">*</span></label>
    <p class="backup-field-help">OIDC groups (comma separated) whose members receive administrator access to Kaeru Core</p>
    <v-text-field id="oidc-admin-groups" v-model="configuration.adminGroups" hide-details="auto" placeholder="OIDC groups (comma separated)" required variant="outlined" />
  </div>

  <div class="setup-form-field">
    <label for="oidc-button-text" class="service-field-label">Button Text <span class="required-field-marker" aria-hidden="true">*</span></label>
    <p class="backup-field-help">Text displayed on the login button</p>
    <v-text-field id="oidc-button-text" v-model="configuration.buttonText" hide-details="auto" placeholder="Sign in with OIDC" required variant="outlined" />
  </div>

  <div class="setup-form-field">
    <label for="oidc-button-image" class="service-field-label">Button Image</label>
    <p class="backup-field-help">
      Optional JPG or PNG image, maximum 1 MB. Shown in the log in button<span v-if="props.existingButtonImage">. Leave empty to keep the current image.</span>
    </p>
    <v-file-input id="oidc-button-image" v-model="buttonImage" accept=".jpg,.jpeg,.png,image/jpeg,image/png" clearable hide-details="auto" :placeholder="props.existingButtonImage ? 'Leave empty to keep current picture' : 'Choose an image'" prepend-icon="" :prepend-inner-icon="mdiCamera" :rules="imageRules" variant="outlined" show-size persistent-placeholder />
  </div>
</template>
