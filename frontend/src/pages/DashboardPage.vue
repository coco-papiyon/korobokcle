<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import AppShell from '@/components/AppShell.vue'
import AsyncState from '@/components/AsyncState.vue'
import DataTable from '@/components/DataTable.vue'
import StateBadge from '@/components/StateBadge.vue'
import { useAsyncData } from '@/composables/useAsyncData'
import { deleteJob, fetchAppConfig, fetchJobs, restoreJob } from '@/lib/api'
import { formatDateTime } from '@/lib/format'
import type { Job } from '@/types'

function mergeJobs(current: Job[] | null, incoming: Job[]) {
  if (!current) {
    return incoming
  }

  const currentByID = new Map(current.map((job) => [job.id, job]))
  return incoming.map((job) => {
    const existing = currentByID.get(job.id)
    if (!existing) {
      return job
    }
    if (
      existing.type === job.type &&
      existing.repository === job.repository &&
      existing.githubNumber === job.githubNumber &&
      existing.state === job.state &&
      existing.title === job.title &&
      existing.branchName === job.branchName &&
      existing.watchRuleId === job.watchRuleId &&
      existing.deletedAt === job.deletedAt &&
      existing.createdAt === job.createdAt &&
      existing.updatedAt === job.updatedAt
    ) {
      return existing
    }
    return {
      ...existing,
      ...job,
    }
  })
}

const { data: appConfig } = useAsyncData(fetchAppConfig)
const refreshIntervalMs = computed(() => {
  const seconds = appConfig.value?.screenRefreshInterval ?? 0
  return seconds > 0 ? seconds * 1000 : 0
})

const showDeletedOnly = ref(false)
const selectedJobIds = ref<string[]>([])
const bulkActionState = ref<'idle' | 'saving' | 'error'>('idle')
const bulkActionError = ref<string | null>(null)

const { data, isLoading, isRefreshing, error, reload } = useAsyncData(() => fetchJobs(showDeletedOnly.value ? 'only' : 'exclude'), {
  pollIntervalMs: refreshIntervalMs,
  mergeData: mergeJobs,
})

watch(showDeletedOnly, () => {
  selectedJobIds.value = []
  bulkActionError.value = null
  void reload()
})

const jobs = computed(() => data.value ?? [])
const visibleJobs = computed(() =>
  jobs.value.filter((job) => (showDeletedOnly.value ? !!job.deletedAt : !job.deletedAt)),
)
const visibleJobIds = computed(() => visibleJobs.value.map((job) => job.id))
const selectedVisibleJobs = computed(() =>
  visibleJobs.value.filter((job) => selectedJobIds.value.includes(job.id)),
)
const selectedVisibleJobCount = computed(() => selectedVisibleJobs.value.length)
const visibleJobCount = computed(() => visibleJobs.value.length)
const allVisibleJobsSelected = computed(
  () => visibleJobCount.value > 0 && selectedVisibleJobCount.value === visibleJobCount.value,
)
const selectedJobCountLabel = computed(() => `${selectedVisibleJobCount.value}件`)
const isBulkActionRunning = computed(() => bulkActionState.value === 'saving')

function getJobTitle(job: Job) {
  return job.title?.trim() || 'タイトルなし'
}

function syncSelectedJobIds() {
  const visibleJobIdSet = new Set(visibleJobIds.value)
  selectedJobIds.value = selectedJobIds.value.filter((selectedId) => visibleJobIdSet.has(selectedId))
}

watch(visibleJobs, syncSelectedJobIds, { immediate: true })

function isJobSelected(jobId: string) {
  return selectedJobIds.value.includes(jobId)
}

function toggleJobSelection(jobId: string) {
  if (isBulkActionRunning.value) {
    return
  }
  if (isJobSelected(jobId)) {
    selectedJobIds.value = selectedJobIds.value.filter((selectedId) => selectedId !== jobId)
    return
  }
  selectedJobIds.value = [...selectedJobIds.value, jobId]
}

function setAllVisibleJobsSelected(checked: boolean) {
  if (isBulkActionRunning.value) {
    return
  }
  if (!checked) {
    const visibleJobIdSet = new Set(visibleJobIds.value)
    selectedJobIds.value = selectedJobIds.value.filter((selectedId) => !visibleJobIdSet.has(selectedId))
    return
  }
  selectedJobIds.value = Array.from(new Set([...selectedJobIds.value, ...visibleJobIds.value]))
}

function toggleAllVisibleJobs(event: Event) {
  const target = event.target as HTMLInputElement
  setAllVisibleJobsSelected(target.checked)
}

function clearBulkSelection() {
  selectedJobIds.value = []
}

function formatBulkError(actionLabel: string, failures: Array<{ jobId: string; reason: unknown }>) {
  const [firstFailure] = failures
  const reason = firstFailure?.reason
  const message = reason instanceof Error ? reason.message : 'Unknown error'
  const failedJobIds = failures.slice(0, 3).map((failure) => failure.jobId)
  const failedJobIdLabel = failedJobIds.length > 0 ? ` (対象: ${failedJobIds.join(', ')})` : ''
  return `${actionLabel}に失敗しました: ${message}${failedJobIdLabel}`
}

async function runBulkJobAction(
  targets: Job[],
  action: (jobId: string) => Promise<unknown>,
): Promise<Array<{ jobId: string; reason: unknown }>> {
  const failures: Array<{ jobId: string; reason: unknown }> = []
  for (const job of targets) {
    try {
      await action(job.id)
    } catch (reason) {
      failures.push({ jobId: job.id, reason })
    }
  }
  return failures
}

async function submitBulkDelete() {
  const targets = selectedVisibleJobs.value
  if (targets.length === 0) {
    bulkActionState.value = 'error'
    bulkActionError.value = showDeletedOnly.value
      ? '削除済み表示で復元対象のジョブを選択してください。'
      : '削除対象のジョブを選択してください。'
    return
  }
  if (!window.confirm(`選択した ${targets.length} 件のジョブを削除済みにしますか？`)) {
    return
  }

  bulkActionState.value = 'saving'
  bulkActionError.value = null
  try {
    const failures = await runBulkJobAction(targets, deleteJob)
    await reload()
    clearBulkSelection()
    if (failures.length > 0) {
      bulkActionState.value = 'error'
      bulkActionError.value = `${targets.length}件中${failures.length}件の削除に失敗しました。${formatBulkError('削除', failures)}`
      return
    }
    bulkActionState.value = 'idle'
  } catch (err) {
    bulkActionState.value = 'error'
    bulkActionError.value = err instanceof Error ? err.message : 'Unknown error'
  }
}

async function submitBulkRestore() {
  const targets = selectedVisibleJobs.value
  if (targets.length === 0) {
    bulkActionState.value = 'error'
    bulkActionError.value = showDeletedOnly.value
      ? '復元対象のジョブを選択してください。'
      : '通常表示では復元を実行できません。'
    return
  }
  if (!window.confirm(`選択した ${targets.length} 件のジョブを復元しますか？`)) {
    return
  }

  bulkActionState.value = 'saving'
  bulkActionError.value = null
  try {
    const failures = await runBulkJobAction(targets, restoreJob)
    await reload()
    clearBulkSelection()
    if (failures.length > 0) {
      bulkActionState.value = 'error'
      bulkActionError.value = `${targets.length}件中${failures.length}件の復元に失敗しました。${formatBulkError('復元', failures)}`
      return
    }
    bulkActionState.value = 'idle'
  } catch (err) {
    bulkActionState.value = 'error'
    bulkActionError.value = err instanceof Error ? err.message : 'Unknown error'
  }
}
</script>

<template>
  <AppShell
    title="korobokcle"
    description="Watch Ruleに一致するGitHub Issue/PRの一覧と自動処理の状況を確認できます。"
  >
    <AsyncState :is-loading="isLoading" :error="error">
      <p v-if="isRefreshing" class="text-muted">Syncing jobs...</p>
      <div class="dashboard-toolbar">
        <div class="button-row">
          <button
            class="button button-secondary"
            type="button"
            :disabled="isBulkActionRunning"
            @click="showDeletedOnly = !showDeletedOnly"
          >
            {{ showDeletedOnly ? '表示を通常に戻す' : '削除済みジョブを表示' }}
          </button>
        </div>
        <div class="dashboard-toolbar__bulk">
          <label class="dashboard-toolbar__select-all">
            <input
              :checked="allVisibleJobsSelected"
              :disabled="isBulkActionRunning || visibleJobCount === 0"
              type="checkbox"
              @change="toggleAllVisibleJobs"
            >
            <span>表示中を全選択</span>
          </label>
          <p class="text-muted dashboard-toolbar__selection">選択中: {{ selectedJobCountLabel }}</p>
          <div class="button-row">
            <button
              v-if="!showDeletedOnly"
              class="button button-danger"
              type="button"
              :disabled="isBulkActionRunning || selectedVisibleJobCount === 0"
              @click="submitBulkDelete"
            >
              {{ selectedVisibleJobCount > 0 ? `選択した ${selectedJobCountLabel} を削除` : '削除するジョブを選択してください' }}
            </button>
            <button
              v-else
              class="button button-primary"
              type="button"
              :disabled="isBulkActionRunning || selectedVisibleJobCount === 0"
              @click="submitBulkRestore"
            >
              {{ selectedVisibleJobCount > 0 ? `選択した ${selectedJobCountLabel} を復元` : '復元するジョブを選択してください' }}
            </button>
          </div>
        </div>
      </div>
      <p v-if="bulkActionState === 'error' && bulkActionError" class="notice notice-danger">{{ bulkActionError }}</p>
      <DataTable :columns="['選択', 'Title', 'Type', 'Repository', 'State', 'Updated']">
        <tr v-for="job in visibleJobs" :key="job.id">
          <td class="dashboard-select-cell">
            <input
              :checked="isJobSelected(job.id)"
              :aria-label="`${getJobTitle(job)} を選択`"
              :disabled="isBulkActionRunning"
              type="checkbox"
              @change="toggleJobSelection(job.id)"
            >
          </td>
          <td class="dashboard-job-cell">
            <RouterLink class="dashboard-job-title" :to="`/jobs/${job.id}`">
              {{ getJobTitle(job) }}
            </RouterLink>
            <span class="dashboard-job-id text-muted">{{ job.id }}</span>
          </td>
          <td>{{ job.type }}</td>
          <td>{{ job.repository }} #{{ job.githubNumber }}</td>
          <td><StateBadge :state="job.state" /></td>
          <td>{{ formatDateTime(job.updatedAt) }}</td>
        </tr>
        <tr v-if="visibleJobs.length === 0">
          <td colspan="6" class="text-muted">
            {{ showDeletedOnly ? '削除済みジョブはまだありません。' : 'ジョブはまだありません。' }}
          </td>
        </tr>
      </DataTable>
    </AsyncState>
  </AppShell>
</template>

<style scoped>
.dashboard-toolbar {
  display: flex;
  flex-wrap: wrap;
  justify-content: space-between;
  gap: var(--space-4);
  margin-bottom: var(--space-4);
}

.dashboard-toolbar__bulk {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: var(--space-3);
}

.dashboard-toolbar__select-all {
  display: inline-flex;
  align-items: center;
  gap: 0.5rem;
  color: var(--ink-muted);
  font-size: 0.9rem;
}

.dashboard-toolbar__selection {
  margin: 0;
}

.dashboard-select-cell {
  width: 3.25rem;
}

.dashboard-select-cell input {
  margin-top: 0.25rem;
  width: 1rem;
  height: 1rem;
  accent-color: var(--accent);
}

.dashboard-job-cell {
  display: flex;
  flex-direction: column;
  gap: 0.15rem;
}

.dashboard-job-title {
  color: var(--ink-primary);
  font-weight: 700;
  line-height: 1.45;
  text-decoration: none;
  word-break: break-word;
}

.dashboard-job-title:hover {
  color: var(--accent-deep);
}

.dashboard-job-id {
  display: block;
  font-size: 0.76rem;
  line-height: 1.35;
  word-break: break-word;
}
</style>
