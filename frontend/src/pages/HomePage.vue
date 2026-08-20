<script setup>
import AboutSection from "../components/about/AboutSection.vue";
import BackupSection from "../components/backup/BackupSection.vue";
import NotificationServicesSection from "../components/notifications/NotificationServicesSection.vue";
import OIDCSettingsSection from "../components/oidc/OIDCSettingsSection.vue";
import ServicesSection from "../components/services/ServicesSection.vue";
import UsersSection from "../components/users/UsersSection.vue";
</script>

<template>
  <v-container class="page-content">
    <ServicesSection />
    <UsersSection />
    <NotificationServicesSection />
    <OIDCSettingsSection />
    <BackupSection />
    <AboutSection />
  </v-container>
</template>
