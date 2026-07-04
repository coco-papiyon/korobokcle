<script setup lang="ts">
import { computed, ref, watch, onBeforeUnmount } from 'vue'
import type { Job, JobArtifact, JobDetailResponse, JobLogGroup } from '../types'
import { jobStateChipClass, jobStateLabel as formatJobStateLabel } from '../utils/jobState'
import { formatJobTimestampValue } from '../utils/jobTime'

const props = defineProps<{
  active: boolean
  jobId: string
  refreshKey: number
}>()

const detailLoading = ref(false)
const detailError = ref('')
const detailJob = ref<Job | null>(null)
const detailUpdatedAt = ref('')
const detailBranch = ref('')
const detailLogs = ref<JobLogGroup[]>([])
const artifactLoading = ref(false)
const artifactError = ref('')
const artifact = ref<JobArtifact | null>(null)
const artifactJobId = ref('')
const artifactUserComment = ref('')
const artifactActionLoading = ref(false)
const deleteLoading = ref(false)
let detailRequestSequence = 0
let artifactRequestSequence = 0
let detailRefreshTimer: number | undefined
const emit = defineEmits<{
  (event: 'close'): void
  (event: 'refresh'): void
  (event: 'deleted', jobId: string): void
}>()

const inspectableStates = new Set([
  'design_ready',
  'design_approved',
  'implementation_ready',
  'implementation_approved',
  'pr_created',
  'review_ready',
  'review_approved',
  'review_fixed',
  'review_fix_implementation_ready',
  'review_fix_implementation_approved',
  'review_fix_design_approved',
  'pr_conflict_ready',
  'pr_conflict_resolved',
  'completed',
])

const detailTitle = computed(() => {
  if (!detailJob.value) {
    return 'ジョブ詳細'
  }
  return detailJob.value.title || `#${detailJob.value.number}`
})

const showIssueContext = computed(() => detailJob.value?.kind === 'issue_design' || detailJob.value?.kind === 'issue_implementation')

const issueContext = computed(() => detailJob.value?.issueContext ?? '')
const detailSubStatus = computed(() =>
  detailJob.value?.kind === 'issue_implementation' ? detailJob.value?.subStatus?.trim() ?? '' : '',
)
const hasLogs = computed(() => detailLogs.value.length > 0)

const relatedLink = computed(() => {
  const job = detailJob.value
  if (!job || !job.repository || !job.number) {
    return null
  }

  let pathType: 'issues' | 'pull' | null = null
  if (job.kind === 'issue_design' || job.kind === 'issue_implementation') {
    pathType = 'issues'
  } else if (job.kind === 'pr_review' || job.kind === 'pr_feedback' || job.kind === 'pr_conflict') {
    pathType = 'pull'
  }

  if (!pathType) {
    return null
  }

  return {
    label: pathType === 'issues' ? 'Issue を開く' : 'PR を開く',
    href: `https://github.com/${job.repository}/${pathType}/${job.number}`,
  }
})

function canInspectArtifact(job: Job | null) {
  return job != null && inspectableStates.has(job.state)
}

function jobStateClass(state: string) {
  return jobStateChipClass(state)
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
    return 'レビュー指摘修正結果'
  }
  if (job.kind === 'pr_conflict') {
    return 'コンフリクト解消結果'
  }
  return '結果'
}

function logGroupTitle(group: JobLogGroup) {
  return `${group.roleLabel} / 試行 ${group.attempt}`
}

async function loadJobDetail(id: string) {
  const requestSequence = ++detailRequestSequence
  if (!id) {
    detailLoading.value = false
    detailError.value = ''
    detailJob.value = null
    detailBranch.value = ''
    detailLogs.value = []
    artifactRequestSequence += 1
    artifactLoading.value = false
    artifactError.value = ''
    artifact.value = null
    artifactJobId.value = ''
    artifactUserComment.value = ''
    return
  }
  const showLoading = detailJob.value?.id !== id || detailJob.value == null
  if (showLoading) {
    detailLoading.value = true
  }
  detailError.value = ''
  try {
    const res = await fetch(`/api/jobs/${encodeURIComponent(id)}`, { cache: 'no-store' })
    if (!res.ok) {
      throw new Error(`HTTP ${res.status}`)
    }
    const payload = (await res.json()) as JobDetailResponse
    const branch = payload.branch || payload.job.branch || ''
    if (requestSequence === detailRequestSequence) {
      detailLogs.value = payload.logs ?? []
      const isSameRevision =
        detailJob.value?.id === payload.job.id && payload.updatedAt === detailUpdatedAt.value
      if (!isSameRevision) {
        detailUpdatedAt.value = payload.updatedAt
        detailJob.value = payload.job
        detailBranch.value = branch
        artifactUserComment.value = ''
      }
      if (canInspectArtifact(payload.job)) {
        void loadArtifact(payload.job.id)
      } else {
        artifactRequestSequence += 1
        artifactLoading.value = false
        artifactError.value = ''
        artifact.value = null
        artifactJobId.value = ''
      }
    }
  } catch (err) {
    if (requestSequence === detailRequestSequence) {
      detailError.value = err instanceof Error ? err.message : 'unknown error'
      detailJob.value = null
      detailBranch.value = ''
      detailLogs.value = []
      artifactRequestSequence += 1
      artifactLoading.value = false
      artifactError.value = ''
      artifact.value = null
      artifactJobId.value = ''
      artifactUserComment.value = ''
    }
  } finally {
    if (requestSequence === detailRequestSequence && showLoading) {
      detailLoading.value = false
    }
  }
}

function startPolling() {
  if (detailRefreshTimer !== undefined) {
    return
  }
  detailRefreshTimer = window.setInterval(() => {
    if (!props.active || !props.jobId) {
      return
    }
    void loadJobDetail(props.jobId)
  }, 5000)
}

function stopPolling() {
  if (detailRefreshTimer === undefined) {
    return
  }
  window.clearInterval(detailRefreshTimer)
  detailRefreshTimer = undefined
}

async function loadArtifact(jobId: string) {
  if (!jobId) {
    return
  }
  const requestSequence = ++artifactRequestSequence
  artifactError.value = ''
  const hasCurrentArtifact = artifactJobId.value === jobId && artifact.value !== null
  if (!hasCurrentArtifact) {
    artifactLoading.value = true
    artifact.value = null
  }
  try {
    const res = await fetch(`/api/jobs/${encodeURIComponent(jobId)}/artifact`, { cache: 'no-store' })
    if (!res.ok) {
      const message = await res.text()
      throw new Error(message || `HTTP ${res.status}`)
    }
    const payload = (await res.json()) as JobArtifact
    if (requestSequence === artifactRequestSequence) {
      artifact.value = payload
      artifactJobId.value = jobId
    }
  } catch (err) {
    if (requestSequence === artifactRequestSequence) {
      artifactError.value = err instanceof Error ? err.message : 'unknown error'
    }
  } finally {
    if (requestSequence === artifactRequestSequence) {
      artifactLoading.value = false
    }
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
    emit('refresh')
    emit('close')
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
    emit('refresh')
    emit('close')
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
    emit('refresh')
    emit('close')
  } catch (err) {
    artifactError.value = err instanceof Error ? err.message : 'unknown error'
  } finally {
    artifactActionLoading.value = false
  }
}

async function deleteJob() {
  if (!detailJob.value) {
    return
  }
  const confirmed = window.confirm(`ジョブ ${detailJob.value.id} を削除します。よろしいですか?`)
  if (!confirmed) {
    return
  }
  deleteLoading.value = true
  artifactError.value = ''
  try {
    const res = await fetch(`/api/jobs/${encodeURIComponent(detailJob.value.id)}`, {
      method: 'DELETE',
    })
    if (!res.ok && res.status !== 204) {
      const message = await res.text()
      throw new Error(message || `HTTP ${res.status}`)
    }
    emit('refresh')
    emit('deleted', detailJob.value.id)
    emit('close')
    detailUpdatedAt.value = ''
    detailBranch.value = ''
    detailJob.value = null
    artifact.value = null
  } catch (err) {
    artifactError.value = err instanceof Error ? err.message : 'unknown error'
  } finally {
    deleteLoading.value = false
  }
}

watch(
  () => [props.active, props.jobId, props.refreshKey] as const,
  ([active, jobId]) => {
    if (!active) {
      stopPolling()
      return
    }
    if (!jobId) {
      stopPolling()
      void loadJobDetail(jobId)
      return
    }
    void loadJobDetail(jobId)
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
          <span :class="jobStateClass(detailJob.state)">{{ formatJobStateLabel(detailJob.state) }}</span>
          <span v-if="detailSubStatus" class="detail__substatus">{{ detailSubStatus }}</span>
        </div>
      </div>

      <div v-if="detailJob.state === 'failed' && detailJob.errorMessage" class="error detail__error">
        <strong>エラー内容</strong>
        <pre>{{ detailJob.errorMessage }}</pre>
        <button class="button button--danger detail__retry" type="button" @click="rerunArtifact" :disabled="artifactActionLoading">
          再実行
        </button>
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
          <div class="detail__meta-value detail__meta-value--mono">#{{ detailJob.number }}</div>
        </div>
        <div class="detail__meta-item">
          <div class="detail__meta-label">ブランチ</div>
          <div class="detail__meta-value detail__meta-value--mono">{{ detailBranch || detailJob.branch || '-' }}</div>
        </div>
        <div class="detail__meta-item">
          <div class="detail__meta-label">取得時間</div>
          <div class="detail__meta-value detail__meta-value--mono">
            {{ formatJobTimestampValue(detailJob.fetchedAt) }}
          </div>
        </div>
      </div>

      <details v-if="showIssueContext && issueContext" class="detail-context">
        <summary>Issue の内容</summary>
        <pre class="detail-context__body">{{ issueContext }}</pre>
      </details>

      <div v-if="canInspectArtifact(detailJob)" class="detail-artifact">
        <div class="panel__title-row">
          <h3>{{ artifactTitle(detailJob) }}</h3>
          <span class="panel__hint">GET /api/jobs/:id/artifact</span>
        </div>

        <div v-if="artifactLoading && !artifact" class="empty-state">読み込み中...</div>
        <div v-if="artifactError" class="error">{{ artifactError }}</div>
        <div v-if="artifact">
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
            <div class="modal__actions-group">
              <button class="button" type="button" @click="approveArtifact" :disabled="artifactActionLoading">
                承認
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
              <button class="button button--ghost" type="button" @click="rerunArtifact" :disabled="artifactActionLoading">
                再実行
              </button>
            </div>
            <button class="button button--danger" type="button" @click="deleteJob" :disabled="artifactActionLoading || deleteLoading">
              削除
            </button>
          </div>
        </div>
      </div>

      <section class="detail-logs" aria-label="ログ">
        <div class="panel__title-row">
          <h3>ログ</h3>
          <span class="panel__hint">役割別 / 試行別</span>
        </div>
        <div v-if="hasLogs" class="detail-logs__list">
          <details
            v-for="group in detailLogs"
            :key="`${group.attempt}-${group.role}`"
            class="detail-log-card"
          >
            <summary class="detail-log-card__summary">
              <span>{{ logGroupTitle(group) }}</span>
              <span class="detail-log-card__summary-count">{{ group.files.length }}ファイル</span>
            </summary>
            <div class="detail-log-card__files">
              <article
                v-for="file in group.files"
                :key="file.path"
                class="detail-log-card__file"
              >
                <div class="detail-log-card__file-header">
                  <strong>{{ file.label }}</strong>
                  <code>{{ file.path }}</code>
                </div>
                <pre class="detail-log-card__content">{{ file.content || '（空）' }}</pre>
              </article>
            </div>
          </details>
        </div>
        <div v-else class="empty-state">ログはまだありません。</div>
      </section>

      <section v-if="relatedLink" class="detail-links" aria-label="関連リンク">
        <div class="detail-links__header">
          <h3>関連リンク</h3>
          <span class="panel__hint">GitHub</span>
        </div>
        <a
          class="detail-links__link"
          :href="relatedLink.href"
          target="_blank"
          rel="noreferrer"
        >
          {{ relatedLink.label }}
        </a>
      </section>
    </div>

    <div v-else class="empty-state">一覧からジョブを選択してください。</div>
  </div>
</template>
