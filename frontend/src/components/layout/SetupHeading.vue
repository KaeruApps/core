<script setup>
defineProps({
  titleId: { type: String, required: true },
  title: { type: String, required: true },
  subtitle: { type: String, required: true },
});
</script>

<template>
  <div class="setup-heading">
    <span class="setup-app-icon" aria-hidden="true" />
    <div>
      <h1 :id="titleId">{{ title }}</h1>
      <p>{{ subtitle }}</p>
    </div>
  </div>
</template>

<style scoped>
.setup-heading {
  align-items: center;
  display: flex;
  gap: 1.25rem;
  margin-bottom: 2rem;
}

.setup-heading h1 {
  font-size: clamp(2rem, 5vw, 3rem);
  line-height: 1;
  margin: 0 0 0.65rem;
}

.setup-heading p {
  color: rgba(var(--v-theme-on-background), 0.68);
}

.setup-heading p {
  margin: 0;
}

.setup-app-icon {
  background: url("../../assets/app-icon.svg") center / contain no-repeat;
  flex: 0 0 4rem;
  height: 4rem;
}

@media (max-width: 600px) {
  .setup-heading {
    align-items: flex-start;
  }

  .setup-app-icon {
    flex-basis: 3rem;
    height: 3rem;
  }
}
</style>
