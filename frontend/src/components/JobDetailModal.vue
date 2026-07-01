<script setup lang="ts">
import { nextTick, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import JobDetailPanel from './JobDetailPanel.vue'

const props = defineProps<{
  open: boolean
  jobId: string
  refreshKey: number
}>()

const emit = defineEmits<{
  (event: 'close'): void
  (event: 'deleted', jobId: string): void
  (event: 'refresh'): void
}>()

const dialogRef = ref<HTMLElement | null>(null)

function focusDialog() {
  void nextTick(() => {
    dialogRef.value?.focus()
  })
}

function handleKeydown(event: KeyboardEvent) {
  if (props.open && event.key === 'Escape') {
    emit('close')
  }
}

function handleBackdropClick() {
  emit('close')
}

function handlePanelClose() {
  emit('close')
}

function handlePanelDeleted(jobId: string) {
  emit('deleted', jobId)
}

function handlePanelRefresh() {
  emit('refresh')
}

watch(
  () => props.open,
  (open) => {
    if (open) {
      focusDialog()
    }
  },
)

onMounted(() => {
  window.addEventListener('keydown', handleKeydown)
  document.body.classList.add('modal-open')
  if (props.open) {
    focusDialog()
  }
})

onBeforeUnmount(() => {
  window.removeEventListener('keydown', handleKeydown)
  document.body.classList.remove('modal-open')
})
</script>

<template>
  <div v-if="open" class="modal-overlay" @click.self="handleBackdropClick">
    <div
      ref="dialogRef"
      class="modal"
      role="dialog"
      aria-modal="true"
      aria-labelledby="job-detail-modal-title"
      tabindex="-1"
    >
      <div class="modal__header">
        <div>
          <p class="modal__eyebrow">Job Details</p>
          <h2 id="job-detail-modal-title">ジョブ詳細</h2>
        </div>
        <button class="button button--ghost modal__close" type="button" aria-label="閉じる" @click="handleBackdropClick">
          閉じる
        </button>
      </div>

      <div class="modal__content">
        <JobDetailPanel
          :job-id="jobId"
          :refresh-key="refreshKey"
          @close="handlePanelClose"
          @deleted="handlePanelDeleted"
          @refresh="handlePanelRefresh"
        />
      </div>
    </div>
  </div>
</template>
