<script setup lang="ts">
defineProps<{
  title: string
  description: string
}>()
</script>

<template>
  <div class="app-shell">
    <header class="hero panel panel-hero">
      <div class="hero__content">
        <p class="eyebrow">Automation Control Plane</p>
        <h1>{{ title }}</h1>
        <p class="hero__description">{{ description }}</p>
      </div>
      <nav class="nav-tabs" aria-label="Main">
        <RouterLink class="nav-tabs__link" to="/">Dashboard</RouterLink>
        <RouterLink class="nav-tabs__link" to="/settings">Settings</RouterLink>
        <RouterLink class="nav-tabs__link" to="/settings/watch-rules">Watch Rules</RouterLink>
      </nav>
    </header>
    <main class="stack-lg">
      <slot />
    </main>
  </div>
</template>
