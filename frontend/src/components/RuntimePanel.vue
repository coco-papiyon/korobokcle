<script setup lang="ts">
import { onBeforeUnmount, ref, watch } from 'vue'
import type { RuntimeLogResponse, RuntimeStatus } from '../types'

const props = defineProps<{
  active: boolean
  jobId: string
}>()

const runtimeStatus = ref<RuntimeStatus | null>(null)
const runtimeLogs = ref<RuntimeLogResponse | null>(null)
const loading = ref(false)
const actionLoading = ref(false)
const error = ref('')
let refreshTimer: number | undefined

async function loadRuntime() {
  const showLoading = runtimeStatus.value === null && runtimeLogs.value === null
  if (showLoading) {
    loading.value = true
  }
  error.value = ''
  try {
    const [statusResponse, logsResponse] = await Promise.all([
      fetch(`/api/jobs/${encodeURIComponent(props.jobId)}/runtime`, { cache: 'no-store' }),
      fetch(`/api/jobs/${encodeURIComponent(props.jobId)}/runtime/logs`, { cache: 'no-store' }),
    ])
    if (!statusResponse.ok) {
      throw new Error((await statusResponse.text()) || `HTTP ${statusResponse.status}`)
    }
    if (!logsResponse.ok) {
      throw new Error((await logsResponse.text()) || `HTTP ${logsResponse.status}`)
    }
    runtimeStatus.value = (await statusResponse.json()) as RuntimeStatus
    runtimeLogs.value = (await logsResponse.json()) as RuntimeLogResponse
  } catch (err) {
    error.value = err instanceof Error ? err.message : 'unknown error'
  } finally {
    if (showLoading) {
      loading.value = false
    }
  }
}

async function startRuntime() {
  if (actionLoading.value) {
    return
  }
  if (!props.jobId) {
    return
  }
  actionLoading.value = true
  error.value = ''
  try {
    const response = await fetch(`/api/jobs/${encodeURIComponent(props.jobId)}/runtime`, {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
      },
      body: JSON.stringify({ action: 'start' }),
    })
    if (!response.ok) {
      throw new Error((await response.text()) || `HTTP ${response.status}`)
    }
    runtimeStatus.value = (await response.json()) as RuntimeStatus
    await loadRuntime()
  } catch (err) {
    error.value = err instanceof Error ? err.message : 'unknown error'
  } finally {
    actionLoading.value = false
  }
}

async function stopRuntime() {
  if (actionLoading.value) {
    return
  }
  if (!props.jobId) {
    return
  }
  actionLoading.value = true
  error.value = ''
  try {
    const response = await fetch(`/api/jobs/${encodeURIComponent(props.jobId)}/runtime`, {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
      },
      body: JSON.stringify({ action: 'stop' }),
    })
    if (!response.ok) {
      throw new Error((await response.text()) || `HTTP ${response.status}`)
    }
    runtimeStatus.value = (await response.json()) as RuntimeStatus
    await loadRuntime()
  } catch (err) {
    error.value = err instanceof Error ? err.message : 'unknown error'
  } finally {
    actionLoading.value = false
  }
}

function startPolling() {
  if (refreshTimer !== undefined) {
    return
  }
  refreshTimer = window.setInterval(() => {
    void loadRuntime()
  }, 4000)
}

function stopPolling() {
  if (refreshTimer === undefined) {
    return
  }
  window.clearInterval(refreshTimer)
  refreshTimer = undefined
}

watch(
  () => [props.active, props.jobId] as const,
  ([active, jobId]) => {
    if (!active || !jobId) {
      stopPolling()
      return
    }
    void loadRuntime()
    startPolling()
  },
  { immediate: true },
)

onBeforeUnmount(() => {
  stopPolling()
})

defineExpose({
  loadRuntime,
  startRuntime,
  stopRuntime,
  runtimeStatus,
})
</script>

<template>
  <p v-if="error" class="error">{{ error }}</p>

  <div class="runtime-panel">
    <section class="settings-section runtime-panel__config">
        <div class="panel__title-row">
          <h2>起動状態</h2>
        <span class="panel__hint">GET /api/jobs/:id/runtime</span>
      </div>

      <div v-if="loading" class="empty-state">読み込み中...</div>
      <template v-else>
        <div class="runtime-panel__summary">
          <span class="chip" :class="runtimeStatus?.running ? 'chip--generated' : 'chip--missing'">
            {{ runtimeStatus?.running ? '起動中' : '停止中' }}
          </span>
          <span class="runtime-panel__command">{{ runtimeStatus?.command || '起動コマンド未設定' }}</span>
        </div>

        <div class="runtime-panel__meta">
          <div class="runtime-panel__meta-item">
            <span class="runtime-panel__meta-label">PID</span>
            <span class="runtime-panel__meta-value">{{ runtimeStatus?.pid ?? '-' }}</span>
          </div>
          <div class="runtime-panel__meta-item">
            <span class="runtime-panel__meta-label">常駐モード</span>
            <span class="runtime-panel__meta-value">{{ runtimeStatus?.residentMode ? '有効' : '無効' }}</span>
          </div>
          <div class="runtime-panel__meta-item">
            <span class="runtime-panel__meta-label">起動時刻</span>
            <span class="runtime-panel__meta-value">{{ runtimeStatus?.startedAt ?? '-' }}</span>
          </div>
          <div class="runtime-panel__meta-item">
            <span class="runtime-panel__meta-label">終了時刻</span>
            <span class="runtime-panel__meta-value">{{ runtimeStatus?.stoppedAt ?? '-' }}</span>
          </div>
          <div class="runtime-panel__meta-item runtime-panel__meta-item--wide">
            <span class="runtime-panel__meta-label">作業ディレクトリ</span>
            <span class="runtime-panel__meta-value runtime-panel__meta-value--mono">{{ runtimeStatus?.workingDir ?? '-' }}</span>
          </div>
        </div>

        <p v-if="runtimeStatus?.error" class="error runtime-panel__inline-error">{{ runtimeStatus.error }}</p>

        <div class="modal__actions">
          <div class="modal__actions-group">
            <button
              v-if="runtimeStatus?.running"
              class="button button--danger"
              type="button"
              :disabled="actionLoading"
              @click="stopRuntime"
            >
              {{ actionLoading ? '停止中' : '停止' }}
            </button>
            <button
              v-else
              class="button"
              type="button"
              :disabled="actionLoading"
              @click="startRuntime"
            >
              {{ actionLoading ? '起動中' : '起動' }}
            </button>
          </div>
        </div>
      </template>
    </section>

    <section class="settings-section runtime-panel__logs">
      <div class="panel__title-row">
        <h2>起動ログ</h2>
        <span class="panel__hint">{{ runtimeLogs?.path || 'logs/runtime/startup.log' }}</span>
      </div>
      <pre class="runtime-panel__log">{{ runtimeLogs?.content || 'ログはまだありません。' }}</pre>
    </section>
  </div>
</template>
