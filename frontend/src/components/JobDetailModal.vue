<script setup lang="ts">
import { onBeforeUnmount, onMounted } from 'vue'
import JobDetailPanel from './JobDetailPanel.vue'

const props = defineProps<{
  jobId: string
  refreshKey: number
}>()

const emit = defineEmits<{
  (event: 'close'): void
  (event: 'refresh'): void
  (event: 'deleted', jobId: string): void
}>()

function close() {
  emit('close')
}

function handleRefresh() {
  emit('refresh')
}

function handleDeleted(jobId: string) {
  emit('deleted', jobId)
}

function handleKeydown(event: KeyboardEvent) {
  if (event.key === 'Escape') {
    event.preventDefault()
    close()
  }
}

onMounted(() => {
  document.body.classList.add('modal-open')
  window.addEventListener('keydown', handleKeydown)
})

onBeforeUnmount(() => {
  document.body.classList.remove('modal-open')
  window.removeEventListener('keydown', handleKeydown)
})
</script>

<template>
  <div class="modal-overlay" @click.self="close">
    <section class="modal-dialog" role="dialog" aria-modal="true" aria-label="ジョブ詳細">
      <div class="modal-dialog__header">
        <div class="modal-dialog__title">
          <p class="panel__hint">詳細確認と操作をモーダル内で完結します</p>
          <h2>ジョブ詳細</h2>
        </div>
        <button class="button button--ghost modal-dialog__close" type="button" aria-label="閉じる" @click="close">
          閉じる
        </button>
      </div>

      <div class="modal-dialog__body">
        <JobDetailPanel
          :active="true"
          :job-id="props.jobId"
          :refresh-key="props.refreshKey"
          @close="close"
          @refresh="handleRefresh"
          @deleted="handleDeleted"
        />
      </div>
    </section>
  </div>
</template>
