<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref } from 'vue'
import type { Job } from '../types'

const props = defineProps<{
  selectedJobId: string
}>()

const emit = defineEmits<{
  (event: 'select', jobId: string): void
}>()

const jobs = ref<Job[]>([])
const loadingJobs = ref(false)
const error = ref('')
const showCompletedJobs = ref(true)
let refreshTimer: number | undefined

const stateLabels: Record<string, string> = {
  detected: '検知済み',
  design_running: '設計中',
  design_ready: '設計完了',
  design_approved: '設計承認済み',
  completed: '完了',
  implementation_running: '実装中',
  implementation_ready: '実装完了',
  implementation_approved: '実装承認済み',
  pr_created: 'PR済み',
  pr_review_comment: 'PRレビューコメント状態',
  review_fix_design_running: 'レビュー指摘検討中',
  review_fix_design_ready: 'レビュー指摘検討済み',
  review_fix_design_approved: 'レビュー検討承認済み',
  review_fix_implementation_running: 'レビュー指摘修正中',
  review_fix_implementation_ready: 'レビュー指摘修正完了',
  review_fix_implementation_approved: 'レビュー指摘修正承認済み',
  review_fixed: 'レビュー指摘修正済み',
  review_running: 'レビュー中',
  review_ready: 'レビュー完了',
  review_approved: 'レビュー承認済み',
  failed: '失敗',
}

function compareJobs(a: Job, b: Job) {
  if (a.repository !== b.repository) return a.repository.localeCompare(b.repository)
  if (a.number !== b.number) return a.number - b.number
  return a.kind.localeCompare(b.kind)
}

function isJobVisible(job: Job) {
  if (showCompletedJobs.value) {
    return true
  }
  return job.state !== 'completed'
}

const sortedJobs = computed(() => {
  return jobs.value.slice().sort(compareJobs)
})

const visibleJobs = computed(() => {
  return sortedJobs.value.filter((job) => isJobVisible(job))
})

async function loadJobs() {
  loadingJobs.value = true
  error.value = ''
  try {
    const res = await fetch('/api/jobs')
    if (!res.ok) {
      throw new Error(`HTTP ${res.status}`)
    }
    const payload = (await res.json()) as { jobs?: Job[] }
    jobs.value = payload.jobs ?? []
  } catch (err) {
    error.value = err instanceof Error ? err.message : 'unknown error'
  } finally {
    loadingJobs.value = false
  }
}

function jobStateLabel(state: string) {
  return stateLabels[state] ?? state
}

onMounted(() => {
  void loadJobs()
  refreshTimer = window.setInterval(() => {
    void loadJobs()
  }, 5000)
})

onBeforeUnmount(() => {
  if (refreshTimer !== undefined) {
    window.clearInterval(refreshTimer)
  }
})
</script>

<template>
  <div>
    <div class="panel__title-row">
      <h2>現在のジョブ</h2>
      <span class="panel__hint">{{ visibleJobs.length }} 件</span>
    </div>

    <div class="job-list-toolbar">
      <label class="job-list-filter">
        <input v-model="showCompletedJobs" type="checkbox" />
        <span>完了ジョブを表示</span>
      </label>
    </div>

    <p v-if="error" class="error">{{ error }}</p>

    <div v-if="loadingJobs" class="empty-state">読み込み中...</div>
    <div v-else-if="visibleJobs.length === 0" class="empty-state">まだジョブがありません。</div>

    <div v-else class="job-table-wrap">
      <table class="job-table">
        <thead>
          <tr>
            <th scope="col">Kind</th>
            <th scope="col">Number</th>
            <th scope="col">ID</th>
            <th scope="col">タイトル</th>
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
            <td><span class="chip">{{ jobStateLabel(job.state) }}</span></td>
          </tr>
        </tbody>
      </table>
    </div>
  </div>
</template>
