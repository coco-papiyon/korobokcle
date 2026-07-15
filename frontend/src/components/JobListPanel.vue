<script setup lang="ts">
import { computed, onBeforeUnmount, ref, watch } from 'vue'
import type { Job, JobListResponse } from '../types'
import {
  jobStateChipClass,
  jobStateFilterDefinitions,
  jobStateLabel as formatJobStateLabel,
  jobStateMatchesFilter,
  type JobStateFilterValue,
} from '../utils/jobState'
import { formatJobTimestampValue } from '../utils/jobTime'

const props = defineProps<{
  active: boolean
  selectedJobId: string
  refreshKey: number
}>()

const emit = defineEmits<{
  (event: 'select', jobId: string): void
}>()

const kindFilterDefinitions = [
  { value: 'all', label: 'すべて' },
  { value: 'issue_design', label: 'Issue 設計' },
  { value: 'issue_implementation', label: 'Issue 実装' },
  { value: 'pr_review', label: 'PR レビュー' },
  { value: 'pr_acceptance', label: 'PR 受入確認' },
  { value: 'pr_feedback', label: 'PR 指摘対応' },
  { value: 'pr_conflict', label: 'PR コンフリクト' },
] as const

const sortOrderDefinitions = [
  { value: 'kindAsc', label: 'Kind 昇順' },
  { value: 'kindDesc', label: 'Kind 降順' },
  { value: 'titleAsc', label: 'タイトル 昇順' },
  { value: 'titleDesc', label: 'タイトル 降順' },
  { value: 'fetchedAtAsc', label: '取得時間 昇順' },
  { value: 'fetchedAtDesc', label: '取得時間 降順' },
  { value: 'stateAsc', label: 'ステータス 昇順' },
  { value: 'stateDesc', label: 'ステータス 降順' },
] as const

const jobs = ref<Job[]>([])
const jobsUpdatedAt = ref('')
const loadingJobs = ref(false)
const error = ref('')
const selectedKindFilter = ref<(typeof kindFilterDefinitions)[number]['value']>('all')
const selectedStateFilter = ref<JobStateFilterValue>('unfinished')
const selectedSortOrder = ref<(typeof sortOrderDefinitions)[number]['value']>('fetchedAtDesc')
let refreshTimer: number | undefined

const filteredJobs = computed(() => {
  return jobs.value
    .map((job, index) => ({ job, index }))
    .filter(({ job }) => {
      const kindMatches = selectedKindFilter.value === 'all' || job.kind === selectedKindFilter.value
      const stateMatches = jobStateMatchesFilter(job.state, selectedStateFilter.value)
      return kindMatches && stateMatches
    })
})

function compareFetchedAt(a: Job, b: Job) {
  const fetchedAtA = Date.parse(a.fetchedAt ?? '')
  const fetchedAtB = Date.parse(b.fetchedAt ?? '')
  const hasFetchedAtA = Number.isFinite(fetchedAtA)
  const hasFetchedAtB = Number.isFinite(fetchedAtB)

  if (hasFetchedAtA && hasFetchedAtB && fetchedAtA !== fetchedAtB) {
    return fetchedAtA - fetchedAtB
  }

  if (hasFetchedAtA && !hasFetchedAtB) {
    return 1
  }
  if (!hasFetchedAtA && hasFetchedAtB) {
    return -1
  }

  return 0
}

function compareText(a: string, b: string) {
  return a.localeCompare(b, 'ja', { sensitivity: 'base', numeric: true })
}

function compareJobs(a: Job, b: Job) {
  const direction = selectedSortOrder.value.endsWith('Desc') ? -1 : 1
  const sortKey = selectedSortOrder.value.replace(/(Asc|Desc)$/, '') as 'kind' | 'title' | 'fetchedAt' | 'state'

  let comparison = 0
  if (sortKey === 'kind') {
    comparison = compareText(a.kind, b.kind)
  } else if (sortKey === 'title') {
    comparison = compareText(a.title || `#${a.number}`, b.title || `#${b.number}`)
  } else if (sortKey === 'fetchedAt') {
    comparison = compareFetchedAt(a, b)
  } else {
    comparison = compareText(a.state, b.state)
  }

  if (comparison !== 0) {
    return comparison * direction
  }

  if (a.number !== b.number) {
    return a.number - b.number
  }
  return compareText(a.id, b.id)
}

const visibleJobs = computed(() => {
  return [...filteredJobs.value]
    .sort((a, b) => {
      const order = compareJobs(a.job, b.job)
      if (order !== 0) {
        return order
      }
      return a.index - b.index
    })
    .map(({ job }) => job)
})

const jobsSummary = computed(() => `${visibleJobs.value.length} / ${jobs.value.length} 件`)

const hasLoadedJobs = computed(() => jobsUpdatedAt.value !== '' || jobs.value.length > 0)

async function loadJobs() {
  const showLoading = jobsUpdatedAt.value === '' && jobs.value.length === 0
  if (showLoading) {
    loadingJobs.value = true
  }
  error.value = ''
  try {
    const res = await fetch('/api/jobs')
    if (!res.ok) {
      throw new Error(`HTTP ${res.status}`)
    }
    const payload = (await res.json()) as JobListResponse
    if (payload.updatedAt !== jobsUpdatedAt.value) {
      jobs.value = payload.jobs ?? []
      jobsUpdatedAt.value = payload.updatedAt
    }
  } catch (err) {
    error.value = err instanceof Error ? err.message : 'unknown error'
  } finally {
    if (showLoading) {
      loadingJobs.value = false
    }
  }
}

function startPolling() {
  if (refreshTimer !== undefined) {
    return
  }
  refreshTimer = window.setInterval(() => {
    void loadJobs()
  }, 5000)
}

function stopPolling() {
  if (refreshTimer === undefined) {
    return
  }
  window.clearInterval(refreshTimer)
  refreshTimer = undefined
}

function jobStateClass(state: string) {
  return jobStateChipClass(state)
}

watch(
  () => [props.active, props.refreshKey] as const,
  ([active]) => {
    if (!active) {
      stopPolling()
      return
    }
    void loadJobs()
    startPolling()
  },
  { immediate: true },
)

onBeforeUnmount(() => {
  stopPolling()
})
</script>

<template>
  <div>
    <div class="panel__title-row">
      <h2>現在のジョブ</h2>
      <div class="job-list-panel__meta">
        <span class="panel__hint">表示 {{ jobsSummary }}</span>
      </div>
    </div>

    <div class="job-list-panel__filters" aria-label="ジョブ一覧フィルター">
      <section class="job-list-panel__filter-group">
        <div class="job-list-panel__filter-row">
          <h3>Kind</h3>
          <select v-model="selectedKindFilter" class="control job-list-panel__filter-select" aria-label="Kindフィルター">
            <option v-for="option in kindFilterDefinitions" :key="option.value" :value="option.value">
              {{ option.label }}
            </option>
          </select>
        </div>
      </section>

      <section class="job-list-panel__filter-group">
        <div class="job-list-panel__filter-row">
          <h3>ステータス</h3>
          <select v-model="selectedStateFilter" class="control job-list-panel__filter-select" aria-label="ステータスフィルター">
            <option v-for="option in jobStateFilterDefinitions" :key="option.value" :value="option.value">
              {{ option.label }}
            </option>
          </select>
        </div>
      </section>

      <section class="job-list-panel__filter-group">
        <div class="job-list-panel__filter-row">
          <h3>並び順</h3>
          <select v-model="selectedSortOrder" class="control job-list-panel__filter-select" aria-label="並び順">
            <option v-for="option in sortOrderDefinitions" :key="option.value" :value="option.value">
              {{ option.label }}
            </option>
          </select>
        </div>
      </section>
    </div>

    <p v-if="error" class="error">{{ error }}</p>

    <div v-if="loadingJobs" class="empty-state">読み込み中...</div>
    <div v-else-if="hasLoadedJobs && jobs.length === 0" class="empty-state">まだジョブがありません。</div>
    <div v-else-if="visibleJobs.length === 0" class="empty-state">条件に一致するジョブがありません。</div>

    <div v-else class="job-table-wrap">
      <table class="job-table">
        <thead>
          <tr>
            <th scope="col">Kind</th>
            <th scope="col">Number</th>
            <th scope="col">ID</th>
            <th scope="col">タイトル</th>
            <th scope="col">取得時間</th>
            <th scope="col">ステータス</th>
          </tr>
        </thead>
        <tbody>
          <tr
            v-for="job in visibleJobs"
            :key="job.id"
            class="job-table__row"
            :class="{ 'job-table__row--active': selectedJobId === job.id }"
            tabindex="0"
            @click="emit('select', job.id)"
            @keydown.enter="emit('select', job.id)"
            @keydown.space.prevent="emit('select', job.id)"
          >
            <td><code>{{ job.kind }}</code></td>
            <td>#{{ job.number }}</td>
            <td><code>{{ job.id }}</code></td>
            <td class="job-table__title">{{ job.title || `#${job.number}` }}</td>
            <td class="job-table__time-cell">{{ formatJobTimestampValue(job.fetchedAt) }}</td>
            <td>
              <span :class="jobStateClass(job.state)">{{ formatJobStateLabel(job.state) }}</span>
            </td>
          </tr>
        </tbody>
      </table>
    </div>
  </div>
</template>
