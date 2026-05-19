<script setup lang="ts">
import AppShell from '@/components/AppShell.vue'
import AsyncState from '@/components/AsyncState.vue'
import DataTable from '@/components/DataTable.vue'
import PanelCard from '@/components/PanelCard.vue'
import StateBadge from '@/components/StateBadge.vue'
import { useAsyncData } from '@/composables/useAsyncData'
import {
  deleteJob,
  fetchAppConfig,
  fetchJobDetail,
  purgeJob,
  restoreJob,
  submitDesignApproval,
  submitDesignRerun,
  submitFinalApproval,
  submitImplementationRerun,
  submitPRRerun,
  submitReviewComment,
  submitReviewApproval,
  submitReviewRerun,
} from '@/lib/api'
import { formatDateTime, formatPayloadDisplay } from '@/lib/format'
import { rerunActionFromEvent, rerunButtonLabel, type RerunAction } from '@/lib/rerun-actions'
import type { JobEvent } from '@/types'
import { computed, onUnmounted, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'

const route = useRoute()
const router = useRouter()
const jobID = computed(() => String(route.params.id))
const { data: appConfig } = useAsyncData(fetchAppConfig)
const { data, isLoading, isRefreshing, error, reload } = useAsyncData(() => fetchJobDetail(jobID.value))
let refreshTimer: number | null = null
const refreshIntervalMs = computed(() => {
  const seconds = appConfig.value?.screenRefreshInterval ?? 0
  return seconds > 0 ? seconds * 1000 : 0
})

function isPollingState(state?: string) {
  if (!state) {
    return false
  }
  return (
    state === 'detected' ||
    state.includes('running') ||
    state === 'pr_creating' ||
    state === 'design_ready' ||
    state === 'implementation_ready' ||
    state === 'review_ready'
  )
}

function stopPolling() {
  if (refreshTimer !== null) {
    window.clearInterval(refreshTimer)
    refreshTimer = null
  }
}

watch(
  [() => data.value?.job.state, refreshIntervalMs],
  ([state, intervalMs]) => {
    stopPolling()
    if (!isPollingState(state) || !intervalMs || intervalMs <= 0) {
      return
    }
    refreshTimer = window.setInterval(() => {
      void reload({ silent: true })
    }, intervalMs)
  },
  { immediate: true },
)

onUnmounted(() => {
  stopPolling()
})
const approvalState = ref<'idle' | 'saving' | 'error'>('idle')
const finalApprovalState = ref<'idle' | 'saving' | 'error'>('idle')
const jobArchiveState = ref<'idle' | 'saving' | 'error'>('idle')
const jobPurgeState = ref<'idle' | 'saving' | 'error'>('idle')
const approvalError = ref<string | null>(null)
const finalApprovalError = ref<string | null>(null)
const jobArchiveError = ref<string | null>(null)
const jobPurgeError = ref<string | null>(null)
const flowRerunComment = ref('')
const designRerunState = ref<'idle' | 'saving' | 'error'>('idle')
const implementationRerunState = ref<'idle' | 'saving' | 'error'>('idle')
const prRerunState = ref<'idle' | 'saving' | 'error'>('idle')
const reviewRerunState = ref<'idle' | 'saving' | 'error'>('idle')
const reviewSubmitState = ref<'idle' | 'saving' | 'saved' | 'error'>('idle')
const reviewApproveState = ref<'idle' | 'saving' | 'saved' | 'error'>('idle')
const designRerunError = ref<string | null>(null)
const implementationRerunError = ref<string | null>(null)
const prRerunError = ref<string | null>(null)
const reviewRerunError = ref<string | null>(null)
const reviewSubmitError = ref<string | null>(null)
const reviewApproveError = ref<string | null>(null)
const reviewSubmitComment = ref('')

const prCreateInfo = computed(() => {
  const raw = data.value?.prCreateArtifact?.content
  if (!raw) {
    return null
  }
  try {
    return JSON.parse(raw) as { url?: string; repository?: string; branchName?: string; title?: string }
  } catch {
    return null
  }
})

const latestEvent = computed(() => {
  const events = data.value?.events ?? []
  return events.length > 0 ? events[events.length - 1] : null
})

const eventRows = computed(() =>
  (data.value?.events ?? []).map((event) => ({
    ...event,
    payloadDisplay: formatPayloadDisplay(event.payload),
  })),
)

const flowRerunEvent = computed<JobEvent | null>(() => {
  const events = data.value?.events ?? []
  for (let i = events.length - 1; i >= 0; i--) {
    const ev = events[i]
    if (ev.availableActions && ev.availableActions.length > 0) {
      return ev
    }
  }
  return null
})

const canReviewDesign = computed(() => data.value?.job.state === 'waiting_design_approval')
const isDeletedJob = computed(() => !!data.value?.job.deletedAt)
const canReviewImplementation = computed(() => {
  if (isDeletedJob.value) {
    return false
  }
  const state = data.value?.job.state
  if (state === 'waiting_final_approval') {
    return true
  }
  return state === 'failed' && latestEvent.value?.eventType === 'test_failed'
})
const flowRerunAction = computed<RerunAction | null>(() => rerunActionFromEvent(flowRerunEvent.value))
const finalApprovalWarning = computed(() => {
  if (data.value?.job.state === 'failed' && latestEvent.value?.eventType === 'test_failed') {
    return 'Tests failed, but you can still approve and continue to PR creation.'
  }
  return ''
})
const testReportMarkdown = computed(() => formatTestReportMarkdown(data.value?.testReport?.content))
const groupedLogs = computed(() => {
  const logs = data.value?.logs ?? []
  return [
    {
      phase: 'design',
      title: 'Design Logs',
      items: logs.filter((log) => log.phase === 'design'),
    },
    {
      phase: 'implementation',
      title: 'Implementation Logs',
      items: logs.filter((log) => log.phase === 'implementation'),
    },
    {
      phase: 'fix',
      title: 'Fix Logs',
      items: logs.filter((log) => log.phase === 'fix'),
    },
    {
      phase: 'review',
      title: 'Review Logs',
      items: logs.filter((log) => log.phase === 'review'),
    },
    {
      phase: 'pr',
      title: 'PR Logs',
      items: logs.filter((log) => log.phase === 'pr'),
    },
  ].filter((group) => group.items.length > 0)
})
const canSubmitReviewComment = computed(() => data.value?.job.type === 'pr_review' && data.value?.job.state === 'review_ready' && !!data.value?.reviewArtifact)
const canApproveReview = computed(() => data.value?.job.type === 'pr_review' && data.value?.job.state === 'review_ready' && !!data.value?.reviewArtifact)
const isPRFeedbackJob = computed(() => data.value?.job.type === 'pr_feedback')
const hasReviewComments = computed(() => (data.value?.reviewComments?.length ?? 0) > 0)

watch(
  () => data.value?.reviewArtifact?.content,
  (content) => {
    if (typeof content === 'string' && reviewSubmitComment.value.trim() === '') {
      reviewSubmitComment.value = content
    }
  },
  { immediate: true },
)

function rerunState(action: RerunAction) {
  if (action === 'retry_design') {
    return designRerunState
  }
  if (action === 'retry_implementation') {
    return implementationRerunState
  }
  if (action === 'retry_review') {
    return reviewRerunState
  }
  return prRerunState
}

function rerunError(action: RerunAction) {
  if (action === 'retry_design') {
    return designRerunError
  }
  if (action === 'retry_implementation') {
    return implementationRerunError
  }
  if (action === 'retry_review') {
    return reviewRerunError
  }
  return prRerunError
}

function rerunErrorLabel(action: RerunAction) {
  if (action === 'retry_design') {
    return 'Design rerun'
  }
  if (action === 'retry_implementation') {
    return 'Implementation rerun'
  }
  if (action === 'retry_review') {
    return 'Review rerun'
  }
  return 'PR rerun'
}

function formatTestReportMarkdown(raw?: string) {
  if (!raw) {
    return ''
  }
  try {
    const report = JSON.parse(raw) as {
      profile?: string
      success?: boolean
      startedAt?: string
      finishedAt?: string
      results?: Array<{
        command?: string
        exitCode?: number
        durationMs?: number
        stdout?: string
        stderr?: string
        success?: boolean
      }>
    }

    const lines: string[] = []
    lines.push('# Test Report')
    lines.push('')
    lines.push(`- Profile: ${report.profile ?? '-'}`)
    lines.push(`- Success: ${report.success ? 'true' : 'false'}`)
    lines.push(`- Started At: ${report.startedAt ?? '-'}`)
    lines.push(`- Finished At: ${report.finishedAt ?? '-'}`)
    lines.push('')
    lines.push('## Results')
    lines.push('')

    const results = report.results ?? []
    if (results.length === 0) {
      lines.push('- No commands were executed.')
      return lines.join('\n')
    }

    results.forEach((result, index) => {
      lines.push(`### Command ${index + 1}`)
      lines.push('')
      lines.push(`- Command: ${result.command ?? '-'}`)
      lines.push(`- Exit Code: ${result.exitCode ?? '-'}`)
      lines.push(`- Duration: ${result.durationMs ?? '-'} ms`)
      lines.push(`- Success: ${result.success ? 'true' : 'false'}`)
      lines.push('')
      lines.push('#### Stdout')
      lines.push('')
      lines.push('```text')
      lines.push(result.stdout?.trimEnd() || '')
      lines.push('```')
      lines.push('')
      lines.push('#### Stderr')
      lines.push('')
      lines.push('```text')
      lines.push(result.stderr?.trimEnd() || '')
      lines.push('```')
      lines.push('')
    })

    return lines.join('\n').trimEnd()
  } catch {
    return raw
  }
}

function formatLogName(name: string) {
  if (name === 'stdout.log') {
    return 'AI stdout'
  }
  if (name === 'stderr.log') {
    return 'AI stderr'
  }
  if (name === 'git-push.log') {
    return 'git push'
  }
  if (name === 'gh-pr-create.log') {
    return 'gh pr create'
  }
  return name
}

function formatIssueBody(body?: string) {
  return body && body.trim().length > 0 ? body : 'Issue body is empty.'
}

function formatReviewCommentLocation(path?: string, line?: number) {
  if (path && line) {
    return `${path}:${line}`
  }
  if (path) {
    return path
  }
  return 'General review comment'
}

function defaultRerunCommentForEvent(action: RerunAction, event?: JobEvent | null) {
  if (!event || action !== 'retry_implementation') {
    return ''
  }
  try {
    const payload = JSON.parse(event.payload) as { error?: unknown }
    return typeof payload.error === 'string' ? payload.error.trim() : ''
  } catch {
    return ''
  }
}

async function submitRerun(action: RerunAction, eventId?: number) {
  rerunState(action).value = 'saving'
  rerunError(action).value = null
  try {
    const event =
      eventId === undefined ? latestEvent.value : data.value?.events.find((candidate) => candidate.id === eventId)
    const typedComment = flowRerunComment.value.trim()
    const comment = typedComment.length > 0 ? flowRerunComment.value : defaultRerunCommentForEvent(action, event)
    if (action === 'retry_design') {
      data.value = await submitDesignRerun(jobID.value, comment, eventId)
    } else if (action === 'retry_implementation') {
      data.value = await submitImplementationRerun(jobID.value, comment, eventId)
    } else if (action === 'retry_review') {
      data.value = await submitReviewRerun(jobID.value, comment, eventId)
    } else {
      data.value = await submitPRRerun(jobID.value, comment, eventId)
    }
    rerunState(action).value = 'idle'
    await reload()
  } catch (err) {
    rerunState(action).value = 'error'
    rerunError(action).value = err instanceof Error ? err.message : 'Unknown error'
  }
}

async function sendApproval(status: 'approved' | 'rejected') {
  approvalState.value = 'saving'
  approvalError.value = null
  try {
    data.value = await submitDesignApproval(jobID.value, status, '')
    approvalState.value = 'idle'
    await reload()
  } catch (err) {
    approvalState.value = 'error'
    approvalError.value = err instanceof Error ? err.message : 'Unknown error'
  }
}

async function sendFinalApproval(status: 'approved' | 'rejected') {
  finalApprovalState.value = 'saving'
  finalApprovalError.value = null
  try {
    data.value = await submitFinalApproval(jobID.value, status, '')
    finalApprovalState.value = 'idle'
    await reload()
  } catch (err) {
    finalApprovalState.value = 'error'
    finalApprovalError.value = err instanceof Error ? err.message : 'Unknown error'
  }
}

async function sendReviewComment() {
  reviewSubmitState.value = 'saving'
  reviewSubmitError.value = null
  try {
    data.value = await submitReviewComment(jobID.value, reviewSubmitComment.value)
    reviewSubmitState.value = 'saved'
    await reload()
  } catch (err) {
    reviewSubmitState.value = 'error'
    reviewSubmitError.value = err instanceof Error ? err.message : 'Unknown error'
  }
}

async function sendReviewApproval() {
  reviewApproveState.value = 'saving'
  reviewApproveError.value = null
  try {
    data.value = await submitReviewApproval(jobID.value)
    reviewApproveState.value = 'saved'
    await reload()
  } catch (err) {
    reviewApproveState.value = 'error'
    reviewApproveError.value = err instanceof Error ? err.message : 'Unknown error'
  }
}

async function archiveJob() {
  if (!window.confirm('このジョブを削除済みとして非表示にしますか？')) {
    return
  }
  jobArchiveState.value = 'saving'
  jobArchiveError.value = null
  try {
    data.value = await deleteJob(jobID.value)
    jobArchiveState.value = 'idle'
    await reload()
  } catch (err) {
    jobArchiveState.value = 'error'
    jobArchiveError.value = err instanceof Error ? err.message : 'Unknown error'
  }
}

async function unarchiveJob() {
  jobArchiveState.value = 'saving'
  jobArchiveError.value = null
  try {
    data.value = await restoreJob(jobID.value)
    jobArchiveState.value = 'idle'
    await reload()
  } catch (err) {
    jobArchiveState.value = 'error'
    jobArchiveError.value = err instanceof Error ? err.message : 'Unknown error'
  }
}

async function purgeArchivedJob() {
  if (!window.confirm('このジョブをDBから完全削除しますか？復元できません。アーティファクトは残ります。')) {
    return
  }
  jobPurgeState.value = 'saving'
  jobPurgeError.value = null
  try {
    await purgeJob(jobID.value)
    jobPurgeState.value = 'idle'
    await router.push('/')
  } catch (err) {
    jobPurgeState.value = 'error'
    jobPurgeError.value = err instanceof Error ? err.message : 'Unknown error'
  }
}
</script>

<template>
  <AppShell
    title="Job Detail"
    description="自動処理の詳細情報(設計結果、実装結果、レビュー結果等)やイベント履歴を確認できます。"
  >
    <AsyncState :is-loading="isLoading" :error="error">
      <p v-if="isRefreshing" class="text-muted">Syncing job detail...</p>
      <template v-if="data">
        <section class="hero-grid">
          <PanelCard title="Job summary">
            <div class="stack-sm job-summary">
              <h3 class="job-summary__title">{{ data.job.title || '-' }}</h3>
              <p class="job-summary__id text-muted">ID: <code>{{ data.job.id || '-' }}</code></p>
              <StateBadge :state="data.job.state" />
              <p v-if="isDeletedJob" class="notice notice-danger">このジョブは削除済みです。dashboard の通常表示には出ません。</p>
              <p class="text-muted">{{ data.job.repository }} #{{ data.job.githubNumber }}</p>
              <p class="text-muted">Branch: <code>{{ data.job.branchName }}</code></p>
              <p class="text-muted">Watch Rule: <code>{{ data.job.watchRuleId }}</code></p>
            </div>
          </PanelCard>
          <PanelCard title="Flow" description="設計承認、実装成果物確認、最終承認をここから行えます。">
            <div class="stack-sm">
              <p class="text-muted">Current state: <code>{{ data.job.state }}</code></p>
              <div class="button-row">
                <button
                  v-if="!isDeletedJob"
                  class="button button-secondary"
                  type="button"
                  :disabled="jobArchiveState === 'saving'"
                  @click="archiveJob"
                >
                  削除
                </button>
                <template v-else>
                  <button
                    class="button button-primary"
                    type="button"
                    :disabled="jobArchiveState === 'saving'"
                    @click="unarchiveJob"
                  >
                    復元
                  </button>
                  <button
                    class="button button-danger"
                    type="button"
                    :disabled="jobPurgeState === 'saving'"
                    @click="purgeArchivedJob"
                  >
                    完全削除
                  </button>
                </template>
              </div>
              <template v-if="!isDeletedJob && (flowRerunAction || canReviewDesign || canReviewImplementation)">
                <label v-if="flowRerunAction" class="field field-full">
                  <span class="field__label">Rerun Comment</span>
                  <textarea
                    v-model="flowRerunComment"
                    class="field__control field__control--textarea"
                    rows="4"
                    placeholder="再実行時に AI へ伝えたい指示を入力してください。"
                    spellcheck="false"
                  />
                  <p class="text-muted">入力したコメントは rerun の prompt に渡されます。未入力なら従来通りの挙動です。</p>
                </label>
                <div class="button-row">
                  <button
                    v-if="flowRerunAction"
                    class="button button-secondary"
                    type="button"
                    :disabled="approvalState === 'saving' || finalApprovalState === 'saving' || rerunState(flowRerunAction) === 'saving'"
                    @click="submitRerun(flowRerunAction, flowRerunEvent?.id)"
                  >
                    {{ rerunButtonLabel(flowRerunAction, flowRerunEvent?.eventType, flowRerunEvent?.sourceEventType) }}
                  </button>
                  <template v-if="canReviewDesign">
                    <button class="button button-secondary" type="button" :disabled="approvalState === 'saving'" @click="sendApproval('rejected')">
                      Reject Design
                    </button>
                    <button class="button button-primary" type="button" :disabled="approvalState === 'saving'" @click="sendApproval('approved')">
                      Approve Design
                    </button>
                  </template>
                  <template v-if="canReviewImplementation">
                    <button class="button button-secondary" type="button" :disabled="finalApprovalState === 'saving'" @click="sendFinalApproval('rejected')">
                      Reject Final
                    </button>
                    <button class="button button-primary" type="button" :disabled="finalApprovalState === 'saving'" @click="sendFinalApproval('approved')">
                      Approve Final
                    </button>
                  </template>
                </div>
              </template>
              <p v-if="jobArchiveState === 'error'" class="notice notice-danger">{{ jobArchiveError }}</p>
              <p v-if="jobPurgeState === 'error'" class="notice notice-danger">{{ jobPurgeError }}</p>
              <p v-if="approvalState === 'error'" class="notice notice-danger">{{ approvalError }}</p>
              <template v-if="canReviewImplementation">
                <p v-if="finalApprovalWarning" class="notice notice-danger">{{ finalApprovalWarning }}</p>
                <p v-if="finalApprovalState === 'error'" class="notice notice-danger">{{ finalApprovalError }}</p>
              </template>
            </div>
          </PanelCard>
        </section>

        <PanelCard
          v-if="!isPRFeedbackJob"
          title="Issue"
          description="元の issue 内容です。"
        >
          <details class="stack-sm">
            <summary class="text-muted">Open issue body</summary>
            <pre class="artifact-view">{{ formatIssueBody(data.issueBody) }}</pre>
          </details>
        </PanelCard>

        <PanelCard
          v-if="isPRFeedbackJob && hasReviewComments"
          title="PR Review Comments"
          description="修正対象の PR review コメントです。"
        >
          <div class="stack-sm">
            <details v-for="(comment, index) in data.reviewComments" :key="`${comment.url ?? index}`" class="stack-sm">
              <summary class="text-muted">
                {{ comment.author || 'unknown' }} / {{ formatReviewCommentLocation(comment.path, comment.line) }}
              </summary>
              <pre class="artifact-view">{{ comment.body }}</pre>
              <p v-if="comment.url">
                <a class="table-link" :href="comment.url" target="_blank" rel="noreferrer">Open on GitHub</a>
              </p>
            </details>
          </div>
        </PanelCard>

        <p v-if="designRerunState === 'error'" class="notice notice-danger">{{ rerunErrorLabel('retry_design') }}: {{ designRerunError }}</p>
        <p v-if="implementationRerunState === 'error'" class="notice notice-danger">{{ rerunErrorLabel('retry_implementation') }}: {{ implementationRerunError }}</p>
        <p v-if="reviewRerunState === 'error'" class="notice notice-danger">{{ rerunErrorLabel('retry_review') }}: {{ reviewRerunError }}</p>
        <p v-if="prRerunState === 'error'" class="notice notice-danger">{{ rerunErrorLabel('retry_pr') }}: {{ prRerunError }}</p>

        <PanelCard
          v-if="data.designArtifact"
          title="Design Artifact"
          description="生成された設計成果物です。承認前に内容を確認します。"
        >
          <div class="stack-sm">
            <details class="stack-sm">
              <summary class="text-muted">{{ data.designArtifact.path }}</summary>
              <pre class="artifact-view">{{ data.designArtifact.content }}</pre>
            </details>
          </div>
        </PanelCard>

        <PanelCard
          v-if="data.implementationArtifact"
          :title="isPRFeedbackJob ? '修正結果' : 'Implementation Artifact'"
          :description="isPRFeedbackJob ? 'PR review コメントに対する修正結果です。' : '実装フェーズの成果物サマリです。最終承認前に確認します。'"
        >
          <div class="stack-sm">
            <details class="stack-sm">
              <summary class="text-muted">{{ data.implementationArtifact.path }}</summary>
              <pre class="artifact-view">{{ data.implementationArtifact.content }}</pre>
            </details>
          </div>
        </PanelCard>

        <PanelCard
          v-if="data.fixArtifact"
          :title="isPRFeedbackJob ? '追加修正結果' : 'Fix Artifact'"
          :description="isPRFeedbackJob ? '再実行やテスト失敗後の追加修正結果です。' : 'test_failed 後の修正フェーズで生成された成果物です。'"
        >
          <div class="stack-sm">
            <details class="stack-sm">
              <summary class="text-muted">{{ data.fixArtifact.path }}</summary>
              <pre class="artifact-view">{{ data.fixArtifact.content }}</pre>
            </details>
          </div>
        </PanelCard>

        <PanelCard
          v-if="data.reviewArtifact"
          title="Review Artifact"
          description="PR review フェーズの成果物です。総評コメントとして GitHub へ返せます。"
        >
          <div class="stack-sm">
            <label v-if="canSubmitReviewComment" class="field field-full">
              <span class="field__label">Review Comment</span>
              <textarea
                v-model="reviewSubmitComment"
                class="field__control field__control--textarea"
                rows="10"
                placeholder="GitHub に返すレビューコメント"
                spellcheck="false"
              />
            </label>
            <div v-if="canApproveReview" class="button-row">
              <button class="button button-primary" type="button" :disabled="reviewSubmitState === 'saving'" @click="sendReviewComment">
                {{ reviewSubmitState === 'saving' ? 'Submitting Review...' : 'Submit Review Comment' }}
              </button>
              <button class="button button-secondary" type="button" :disabled="reviewApproveState === 'saving'" @click="sendReviewApproval">
                {{ reviewApproveState === 'saving' ? 'Approving Review...' : 'Approve Review' }}
              </button>
            </div>
            <p v-if="reviewSubmitState === 'saved'" class="notice notice-success">レビューコメントを GitHub に送信しました。</p>
            <p v-if="reviewSubmitState === 'error'" class="notice notice-danger">{{ reviewSubmitError }}</p>
            <p v-if="reviewApproveState === 'saved'" class="notice notice-success">レビューを承認しました。</p>
            <p v-if="reviewApproveState === 'error'" class="notice notice-danger">{{ reviewApproveError }}</p>
            <details class="stack-sm">
              <summary class="text-muted">{{ data.reviewArtifact.path }}</summary>
              <pre class="artifact-view">{{ data.reviewArtifact.content }}</pre>
            </details>
          </div>
        </PanelCard>

        <PanelCard
          v-if="data.testReport"
          title="Test Report"
          description="設定された test profile の実行結果です。"
        >
          <div class="stack-sm">
            <details class="stack-sm">
              <summary class="text-muted">{{ data.testReport.path }}</summary>
              <pre class="artifact-view">{{ testReportMarkdown }}</pre>
            </details>
          </div>
        </PanelCard>

        <PanelCard
          v-if="data.prCreateArtifact"
          title="Pull Request"
          description="作成された PR の記録です。"
        >
          <div class="stack-sm">
            <details class="stack-sm">
              <summary class="text-muted">{{ data.prCreateArtifact.path }}</summary>
              <template v-if="prCreateInfo">
                <p v-if="prCreateInfo.title"><strong>{{ prCreateInfo.title }}</strong></p>
                <p v-if="prCreateInfo.repository" class="text-muted">Repository: <code>{{ prCreateInfo.repository }}</code></p>
                <p v-if="prCreateInfo.branchName" class="text-muted">Branch: <code>{{ prCreateInfo.branchName }}</code></p>
                <p v-if="prCreateInfo.url">
                  <a class="table-link" :href="prCreateInfo.url" target="_blank" rel="noreferrer">Open Pull Request</a>
                </p>
              </template>
              <pre class="artifact-view">{{ data.prCreateArtifact.content }}</pre>
            </details>
          </div>
        </PanelCard>

        <PanelCard
          v-if="groupedLogs.length > 0"
          title="Logs"
          description="各フェーズの実行ログです。"
        >
          <div class="stack-md">
            <section v-for="group in groupedLogs" :key="group.phase" class="stack-sm">
              <h3>{{ group.title }}</h3>
              <details v-for="log in group.items" :key="log.path" class="stack-sm">
                <summary class="text-muted">{{ formatLogName(log.name) }} <code>{{ log.path }}</code></summary>
                <pre class="artifact-view">{{ log.content }}</pre>
              </details>
            </section>
          </div>
        </PanelCard>

        <DataTable :columns="['When', 'Event', 'State', 'Payload', 'Action']">
          <tr v-for="event in eventRows" :key="event.id">
            <td>{{ formatDateTime(event.createdAt) }}</td>
            <td>{{ event.eventType }}</td>
            <td>{{ event.stateTo || '-' }}</td>
            <td class="payload-cell">
              <details class="payload-details">
                <summary class="payload-summary">
                  <span class="payload-summary__label">クリックして展開</span>
                  <span class="payload-summary__preview">{{ event.payloadDisplay.preview }}</span>
                </summary>
                <pre class="payload-view artifact-view">{{ event.payloadDisplay.content }}</pre>
              </details>
            </td>
            <td>
              <div v-if="event.availableActions.length > 0" class="button-row">
                <button
                  v-if="event.availableActions.includes('retry_design')"
                  class="button button-secondary"
                  type="button"
                  :disabled="designRerunState === 'saving'"
                  @click="submitRerun('retry_design', event.id)"
                >
                  {{ rerunButtonLabel('retry_design', event.eventType, event.sourceEventType) }}
                </button>
                <button
                  v-if="event.availableActions.includes('retry_implementation')"
                  class="button button-secondary"
                  type="button"
                  :disabled="implementationRerunState === 'saving'"
                  @click="submitRerun('retry_implementation', event.id)"
                >
                  {{ rerunButtonLabel('retry_implementation', event.eventType, event.sourceEventType) }}
                </button>
                <button
                  v-if="event.availableActions.includes('retry_review')"
                  class="button button-secondary"
                  type="button"
                  :disabled="reviewRerunState === 'saving'"
                  @click="submitRerun('retry_review', event.id)"
                >
                  {{ rerunButtonLabel('retry_review', event.eventType, event.sourceEventType) }}
                </button>
                <button
                  v-if="event.availableActions.includes('retry_pr')"
                  class="button button-secondary"
                  type="button"
                  :disabled="prRerunState === 'saving'"
                  @click="submitRerun('retry_pr', event.id)"
                >
                  {{ rerunButtonLabel('retry_pr', event.eventType, event.sourceEventType) }}
                </button>
              </div>
              <span v-else>-</span>
            </td>
          </tr>
          <tr v-if="data.events.length === 0">
            <td colspan="5" class="text-muted">イベントはまだありません。</td>
          </tr>
        </DataTable>
      </template>
    </AsyncState>
  </AppShell>
</template>
