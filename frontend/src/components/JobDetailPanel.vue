<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import type { Job, JobArtifact } from '../types'
import { jobStateChipClass } from '../utils/jobState'

const props = defineProps<{
  jobId: string
  refreshKey: number
}>()

const detailLoading = ref(false)
const detailError = ref('')
const detailJob = ref<Job | null>(null)
const artifactLoading = ref(false)
const artifactError = ref('')
const artifact = ref<JobArtifact | null>(null)
const artifactUserComment = ref('')
const artifactActionLoading = ref(false)
let detailRequestSequence = 0

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
  pr_review_comment: 'レビュー指摘あり',
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

const inspectableStates = new Set([
  'design_ready',
  'implementation_ready',
  'review_ready',
  'review_fix_design_ready',
  'review_fix_implementation_ready',
])

const detailTitle = computed(() => {
  if (!detailJob.value) {
    return 'ジョブ詳細'
  }
  return detailJob.value.title || `#${detailJob.value.number}`
})

function jobStateLabel(state: string) {
  return stateLabels[state] ?? state
}

function jobStateClass(state: string) {
  return jobStateChipClass(state)
}

function canInspectArtifact(job: Job | null) {
  return job != null && inspectableStates.has(job.state)
}

function canRequestChanges(job: Job | null) {
  return job?.kind === 'pr_review' && job.state === 'review_ready'
}

function artifactTitle(job: Job | null) {
  if (!job) {
    return '結果'
  }
  if (job.kind === 'issue_design') {
    return '設計結果'
  }
  if (job.kind === 'issue_implementation') {
    return '実装結果'
  }
  if (job.kind === 'pr_review') {
    return 'レビュー結果'
  }
  if (job.kind === 'pr_feedback' && job.state === 'review_fix_implementation_ready') {
    return 'レビュー指摘修正結果'
  }
  if (job.kind === 'pr_feedback') {
    return 'レビュー指摘検討結果'
  }
  return '結果'
}

async function loadJobDetail(id: string) {
	const requestSequence = ++detailRequestSequence
  if (!id) {
    detailJob.value = null
    return
  }
  detailLoading.value = true
  detailError.value = ''
  try {
    const res = await fetch(`/api/jobs/${encodeURIComponent(id)}`, { cache: 'no-store' })
    if (!res.ok) {
      throw new Error(`HTTP ${res.status}`)
    }
    const job = (await res.json()) as Job
    if (requestSequence === detailRequestSequence) {
      detailJob.value = job
    }
  } catch (err) {
    if (requestSequence === detailRequestSequence) {
      detailError.value = err instanceof Error ? err.message : 'unknown error'
      detailJob.value = null
    }
  } finally {
    if (requestSequence === detailRequestSequence) {
      detailLoading.value = false
    }
  }
}

async function loadArtifact() {
  if (!detailJob.value) {
    return
  }
  artifactLoading.value = true
  artifactError.value = ''
  artifact.value = null
  try {
    const res = await fetch(`/api/jobs/${encodeURIComponent(detailJob.value.id)}/artifact`, { cache: 'no-store' })
    if (!res.ok) {
      const message = await res.text()
      throw new Error(message || `HTTP ${res.status}`)
    }
    artifact.value = (await res.json()) as JobArtifact
  } catch (err) {
    artifactError.value = err instanceof Error ? err.message : 'unknown error'
  } finally {
    artifactLoading.value = false
  }
}

async function approveArtifact() {
  if (!detailJob.value) {
    return
  }
  artifactActionLoading.value = true
  artifactError.value = ''
  try {
    const res = await fetch(`/api/jobs/${encodeURIComponent(detailJob.value.id)}/artifact`, {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
      },
      body: JSON.stringify({ comment: artifactUserComment.value }),
    })
    if (!res.ok) {
      const message = await res.text()
      throw new Error(message || `HTTP ${res.status}`)
    }
    await loadJobDetail(detailJob.value.id)
  } catch (err) {
    artifactError.value = err instanceof Error ? err.message : 'unknown error'
  } finally {
    artifactActionLoading.value = false
  }
}

async function requestChanges() {
  if (!detailJob.value) {
    return
  }
  artifactActionLoading.value = true
  artifactError.value = ''
  try {
    const res = await fetch(`/api/jobs/${encodeURIComponent(detailJob.value.id)}/artifact/request-changes`, {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
      },
      body: JSON.stringify({ comment: artifactUserComment.value }),
    })
    if (!res.ok) {
      const message = await res.text()
      throw new Error(message || `HTTP ${res.status}`)
    }
    await loadJobDetail(detailJob.value.id)
  } catch (err) {
    artifactError.value = err instanceof Error ? err.message : 'unknown error'
  } finally {
    artifactActionLoading.value = false
  }
}

async function rerunArtifact() {
  if (!detailJob.value) {
    return
  }
  artifactActionLoading.value = true
  artifactError.value = ''
  try {
    const res = await fetch(`/api/jobs/${encodeURIComponent(detailJob.value.id)}/artifact`, {
      method: 'PATCH',
      headers: {
        'Content-Type': 'application/json',
      },
      body: JSON.stringify({ comment: artifactUserComment.value }),
    })
    if (!res.ok) {
      const message = await res.text()
      throw new Error(message || `HTTP ${res.status}`)
    }
    await loadJobDetail(detailJob.value.id)
  } catch (err) {
    artifactError.value = err instanceof Error ? err.message : 'unknown error'
  } finally {
    artifactActionLoading.value = false
  }
}

watch(
  () => [props.jobId, props.refreshKey] as const,
  ([jobId]) => {
    void loadJobDetail(jobId)
  },
  { immediate: true },
)

watch(
  detailJob,
  (job) => {
    artifact.value = null
    artifactError.value = ''
    artifactLoading.value = false
    artifactUserComment.value = ''
    if (job && canInspectArtifact(job)) {
      void loadArtifact()
    }
  },
  { immediate: true },
)
</script>

<template>
  <div>
    <div class="panel__title-row">
      <h2>ジョブ詳細</h2>
      <span class="panel__hint">GET /api/jobs/:id</span>
    </div>

    <div v-if="detailLoading" class="empty-state">読み込み中...</div>
    <div v-else-if="detailError" class="error">{{ detailError }}</div>
    <div v-else-if="detailJob" class="detail">
      <div class="detail__header">
        <div>
          <p class="job-card__repo">{{ detailJob.repository }}</p>
          <h3>{{ detailTitle }}</h3>
        </div>
        <div class="detail__header-actions">
          <span :class="jobStateClass(detailJob.state)">{{ jobStateLabel(detailJob.state) }}</span>
        </div>
      </div>

      <div class="detail__meta" aria-label="ジョブ詳細の要約">
        <div class="detail__meta-item">
          <div class="detail__meta-label">Kind</div>
          <div class="detail__meta-value detail__meta-value--mono">{{ detailJob.kind }}</div>
        </div>
        <div class="detail__meta-item">
          <div class="detail__meta-label">ID</div>
          <div class="detail__meta-value detail__meta-value--mono">{{ detailJob.id }}</div>
        </div>
        <div class="detail__meta-item">
          <div class="detail__meta-label">Number</div>
          <div class="detail__meta-value detail__meta-value--number">#{{ detailJob.number }}</div>
        </div>
      </div>

      <div v-if="canInspectArtifact(detailJob)" class="detail-artifact">
        <div class="panel__title-row">
          <h3>{{ artifactTitle(detailJob) }}</h3>
          <span class="panel__hint">GET /api/jobs/:id/artifact</span>
        </div>

        <div v-if="artifactLoading" class="empty-state">読み込み中...</div>
        <div v-else-if="artifactError" class="error">{{ artifactError }}</div>
        <div v-else>
          <pre class="artifact-view">{{ artifact?.content }}</pre>

          <label class="field">
            <span>ユーザコメント</span>
            <textarea
              v-model="artifactUserComment"
              class="control artifact-comment"
              rows="5"
              placeholder="修正したいポイントを入力"
            ></textarea>
          </label>

          <div class="modal__actions">
            <button class="button button--ghost" type="button" @click="rerunArtifact" :disabled="artifactActionLoading">
              再実行
            </button>
            <button
              v-if="canRequestChanges(detailJob)"
              class="button button--ghost"
              type="button"
              @click="requestChanges"
              :disabled="artifactActionLoading"
            >
              修正依頼
            </button>
            <button class="button" type="button" @click="approveArtifact" :disabled="artifactActionLoading">
              承認
            </button>
          </div>
        </div>
      </div>
    </div>

    <div v-else class="empty-state">一覧からジョブを選択してください。</div>
  </div>
</template>
