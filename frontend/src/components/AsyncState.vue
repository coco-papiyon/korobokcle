<script setup lang="ts">
import { ERROR_MESSAGE, LOADING_MESSAGE } from '@/lib/ui-text'

defineProps<{
  isLoading: boolean
  error: string | null
}>()
</script>

<template>
  <div v-if="isLoading" class="panel status-card">
    <p class="status-card__title">{{ LOADING_MESSAGE }}</p>
    <p class="text-muted">データを取得しています。</p>
  </div>
  <div v-else-if="error" class="panel status-card status-card--error">
    <p class="status-card__title">{{ ERROR_MESSAGE }}</p>
    <p>{{ error }}</p>
  </div>
  <slot v-else />
</template>
