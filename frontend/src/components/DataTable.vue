<script setup lang="ts">
defineProps<{
  columns: string[]
}>()
</script>

<template>
  <div class="table-wrap panel">
    <table class="data-table">
      <thead>
        <tr>
          <th v-for="column in columns" :key="column">{{ column }}</th>
        </tr>
      </thead>
      <tbody>
        <slot />
      </tbody>
    </table>
  </div>
</template>
