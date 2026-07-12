<script setup lang="ts">
import { nextTick, onBeforeUnmount, onMounted, ref } from 'vue'
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

const detailPanel = ref<InstanceType<typeof JobDetailPanel> | null>(null)
const sourceDiffAvailable = ref(false)
const artifactEditAvailable = ref(false)
const runtimeAvailable = ref(false)
const activeView = ref<'chat' | 'diff' | 'edit' | 'logs' | 'runtime'>('chat')

function close() {
  emit('close')
}

function handleRefresh() {
  emit('refresh')
}

function handleDeleted(jobId: string) {
  emit('deleted', jobId)
}

function handleSourceDiffAvailability(available: boolean) {
  sourceDiffAvailable.value = available
  if (!available) {
    if (activeView.value === 'diff') {
      activeView.value = 'chat'
      detailPanel.value?.openChatView()
    }
  }
}

function handleArtifactEditAvailability(available: boolean) {
  artifactEditAvailable.value = available
  if (!available && activeView.value === 'edit') {
    activeView.value = 'chat'
    detailPanel.value?.openChatView()
  }
}

function handleRuntimeAvailability(available: boolean) {
  runtimeAvailable.value = available
  if (!available && activeView.value === 'runtime') {
    activeView.value = 'chat'
    detailPanel.value?.openChatView()
  }
}

function showChat() {
  activeView.value = 'chat'
  detailPanel.value?.openChatView()
}

function openSourceDiff() {
  activeView.value = 'diff'
  detailPanel.value?.openSourceDiff()
}

function openEditView() {
  activeView.value = 'edit'
  detailPanel.value?.openEditView()
}

function showLogs() {
  activeView.value = 'logs'
  detailPanel.value?.openLogsView()
}

function showRuntime() {
  activeView.value = 'runtime'
  detailPanel.value?.openRuntimeView()
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
  void nextTick(() => {
    detailPanel.value?.openChatView?.()
  })
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
        <div class="modal-dialog__header-actions">
          <button
            class="button button--ghost modal-dialog__action"
            :class="{ 'modal-dialog__action--active': activeView === 'chat' }"
            type="button"
            @click="showChat"
          >
            チャット
          </button>
          <button
            v-if="sourceDiffAvailable"
            class="button button--ghost modal-dialog__action"
            :class="{ 'modal-dialog__action--active': activeView === 'diff' }"
            type="button"
            @click="openSourceDiff"
          >
            差分確認
          </button>
          <button
            v-if="artifactEditAvailable"
            class="button button--ghost modal-dialog__action"
            :class="{ 'modal-dialog__action--active': activeView === 'edit' }"
            type="button"
            @click="openEditView"
          >
            編集
          </button>
          <button
            v-if="runtimeAvailable"
            class="button button--ghost modal-dialog__action"
            :class="{ 'modal-dialog__action--active': activeView === 'runtime' }"
            type="button"
            @click="showRuntime"
          >
            動作確認
          </button>
          <button
            class="button button--ghost modal-dialog__action"
            :class="{ 'modal-dialog__action--active': activeView === 'logs' }"
            type="button"
            @click="showLogs"
          >
            ログ
          </button>
          <button class="button button--ghost modal-dialog__close" type="button" aria-label="閉じる" @click="close">
            閉じる
          </button>
        </div>
      </div>

      <div class="modal-dialog__body" :class="{ 'modal-dialog__body--chat': activeView === 'chat' }">
        <JobDetailPanel
          ref="detailPanel"
          :active="true"
          :job-id="props.jobId"
          :refresh-key="props.refreshKey"
          @close="close"
          @refresh="handleRefresh"
          @deleted="handleDeleted"
          @source-diff-availability="handleSourceDiffAvailability"
          @artifact-edit-availability="handleArtifactEditAvailability"
          @runtime-availability="handleRuntimeAvailability"
        />
      </div>
    </section>
  </div>
</template>
