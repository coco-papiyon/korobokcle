<script setup lang="ts">
defineProps<{
  title: string
  description?: string
}>()
</script>

<template>
  <section class="panel">
    <div class="stack-sm">
      <div class="stack-xs">
        <h2>{{ title }}</h2>
        <p v-if="description" class="text-muted">{{ description }}</p>
      </div>
      <slot />
    </div>
  </section>
</template>
