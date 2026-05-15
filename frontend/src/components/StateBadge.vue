<script setup lang="ts">
import { computed } from 'vue'
import { formatStateLabel } from '@/lib/format'

const props = defineProps<{
  state: string
}>()

const toneClass = computed(() => {
  if (props.state.includes('failed') || props.state.includes('rejected')) {
    return 'state-badge--danger'
  }
  if (props.state.includes('waiting')) {
    return 'state-badge--warning'
  }
  if (props.state.includes('completed') || props.state.includes('ready')) {
    return 'state-badge--success'
  }
  return 'state-badge--neutral'
})
</script>

<template>
  <span class="state-badge" :class="toneClass">{{ formatStateLabel(state) }}</span>
</template>
