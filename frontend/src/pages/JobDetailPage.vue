<script setup lang="ts">
import AppShell from '@/components/AppShell.vue'
import AsyncState from '@/components/AsyncState.vue'
import DataTable from '@/components/DataTable.vue'
import PanelCard from '@/components/PanelCard.vue'
import StateBadge from '@/components/StateBadge.vue'
import { useAsyncData } from '@/composables/useAsyncData'
import {
  generateImprovement,
  analyzePRComment,
  deleteJob,
  fetchAppConfig,
  fetchJobDetail,
  fetchPRComments,
  fetchToolCommands,
  fetchWatchRules,
  purgeJob,
  restoreJob,
  refreshIssueBody,
  startToolCommand,
  submitDesignApproval,
  submitDesignRerun,
  submitFinalApproval,
  submitImplementationRerun,
  submitPRRerun,
  submitReviewApproval,
  submitReviewComment,
  submitReviewRerun,
  stopToolCommand,
} from '@/lib/api'
import {
  formatDateTime,
  formatEventTypeLabel,
  formatIssueBody,
  formatJobTypeLabel,
  formatLogName,
  formatPayloadDisplay,
  formatStateLabel,
  formatToolExecutionStatusLabel,
} from '@/lib/format'
import { rerunActionFromEvent, rerunButtonLabel, type RerunAction } from '@/lib/rerun-actions'
import { UNKNOWN_ERROR_MESSAGE } from '@/lib/ui-text'
import type { JobEvent, JobLog, ReviewComment } from '@/types'
import { computed, onUnmounted, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'

const route = useRoute()
const router = useRouter()
const jobID = computed(() => String(route.params.id))
const { data: appConfig } = useAsyncData(fetchAppConfig)
const { data: toolCommands } = useAsyncData(fetchToolCommands)
const { data: watchRules } = useAsyncData(fetchWatchRules)
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
const designArtifactModalOpen = ref(false)
const implementationArtifactModalOpen = ref(false)
const testReportModalOpen = ref(false)
const toolLogModalOpen = ref(false)
const prCreateModalOpen = ref(false)
const prCommentAnalysisModalOpen = ref(false)
const prCommentsModalOpen = ref(false)
const reviewArtifactModalOpen = ref(false)
const issueBodyModalOpen = ref(false)
const improvementModalOpen = ref(false)
const selectedLog = ref<{ groupTitle: string; log: JobLog } | null>(null)
const designArtifactComment = ref('')
const implementationArtifactComment = ref('')
const testReportComment = ref('')
const reviewArtifactComment = ref('')
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
const prCommentsState = ref<'idle' | 'loading' | 'error'>('idle')
const prCommentsError = ref<string | null>(null)
const prCommentsPullNumber = ref<number | null>(null)
const prComments = ref<ReviewComment[]>([])
const prCommentAnalyzeState = ref<'idle' | 'saving' | 'error'>('idle')
const prCommentAnalyzeError = ref<string | null>(null)
const prCommentAnalyzingKey = ref<string | null>(null)
const selectedPRCommentForAnalysis = ref<ReviewComment | null>(null)
const prCommentAnalysisComment = ref('')
const prCommentAnalysisActionState = ref<'idle' | 'saving' | 'error'>('idle')
const prCommentAnalysisActionError = ref<string | null>(null)
const toolCommandState = ref<'idle' | 'saving' | 'error'>('idle')
const toolCommandAction = ref<'start' | 'stop' | null>(null)
const toolCommandError = ref<string | null>(null)
const issueBodyRefreshState = ref<'idle' | 'loading' | 'error'>('idle')
const issueBodyRefreshError = ref<string | null>(null)
const selectedToolCommandName = ref('')
const popupLoadingState = ref<'idle' | 'loading' | 'error'>('idle')
const popupLoadingError = ref<string | null>(null)
const improvementCreateState = ref<'idle' | 'saving' | 'error'>('idle')
const improvementCreateError = ref<string | null>(null)
const improvementComment = ref('')
const prCreateInfo = computed(() => {
  const raw = data.value?.prCreateArtifact?.content
  if (!raw) {
    return null
  }
  try {
    return JSON.parse(raw) as {
      url?: string
      repository?: string
      branchName?: string
      pullNumber?: number
      title?: string
      [key: string]: unknown
    }
  } catch {
    return null
  }
})
const prCreateRawContent = computed(() => data.value?.prCreateArtifact?.content ?? '')
const prCommentAnalysisRawContent = computed(() => data.value?.prCommentAnalysisArtifact?.content ?? '')
const hasPRCommentsArtifact = computed(() => {
  if (prCreateInfo.value?.pullNumber) {
    return true
  }
  return (data.value?.prComments?.length ?? 0) > 0
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

const hasPRCommentAnalysis = computed(() => (data.value?.prCommentAnalysisArtifact?.content ?? '').trim().length > 0)
const canReviewDesign = computed(() => data.value?.job.state === 'waiting_design_approval' && !hasPRCommentAnalysis.value)
const canReviewPRCommentAnalysis = computed(() => data.value?.job.state === 'waiting_design_approval' && hasPRCommentAnalysis.value)
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
const canShowToolFlow = computed(() => {
  if (isDeletedJob.value) {
    return false
  }
  if (toolExecution.value?.running) {
    return true
  }
  const state = data.value?.job.state
  if (!state) {
    return false
  }
  if (state === 'implementation_ready' || state === 'waiting_final_approval') {
    return true
  }
  return state === 'failed' && ['implementation_failed', 'test_failed'].includes(latestEvent.value?.eventType ?? '')
})
const flowRerunAction = computed<RerunAction | null>(() => rerunActionFromEvent(flowRerunEvent.value))
const flowRerunActionInFlow = computed<RerunAction | null>(() => {
  if (!flowRerunAction.value) {
    return null
  }
  if (data.value?.job.state === 'interrupted') {
    return flowRerunAction.value
  }
  return flowRerunAction.value === 'retry_pr' ? flowRerunAction.value : null
})
const finalApprovalWarning = computed(() => {
  if (data.value?.job.state === 'failed' && latestEvent.value?.eventType === 'test_failed') {
    return 'テストに失敗していますが、承認して PR 作成へ進むことはできます。'
  }
  return ''
})
const testReportMarkdown = computed(() => formatTestReportMarkdown(data.value?.testReport?.content))
const configuredToolCommand = computed(() => data.value?.toolCommand ?? null)
const availableToolCommands = computed(() => toolCommands.value ?? [])
const selectedToolCommand = computed(() => {
  const name = selectedToolCommandName.value.trim()
  if (!name) {
    return null
  }
  return availableToolCommands.value.find((command) => command.name === name) ?? null
})
const toolExecution = computed(() => data.value?.toolExecution ?? null)
const toolStdout = computed(() => data.value?.toolExecution?.stdout ?? null)
const toolStderr = computed(() => data.value?.toolExecution?.stderr ?? null)
const toolStdoutContent = computed(() => data.value?.toolExecution?.stdout?.content ?? '')
const toolStderrContent = computed(() => data.value?.toolExecution?.stderr?.content ?? '')
const groupedLogs = computed(() => {
  const logs = data.value?.logs ?? []
  return [
    {
      phase: 'design',
      title: '設計ログ',
      items: logs.filter((log) => log.phase === 'design'),
    },
    {
      phase: 'implementation',
      title: '実装ログ',
      items: logs.filter((log) => log.phase === 'implementation'),
    },
    {
      phase: 'implement_fix',
      title: '実装修正ログ',
      items: logs.filter((log) => log.phase === 'implement_fix'),
    },
    {
      phase: 'review',
      title: 'レビューログ',
      items: logs.filter((log) => log.phase === 'review'),
    },
    {
      phase: 'pr',
      title: 'PR ログ',
      items: logs.filter((log) => log.phase === 'pr'),
    },
  ].filter((group) => group.items.length > 0)
})
const canSubmitReviewComment = computed(() => data.value?.job.type === 'pr_review' && data.value?.job.state === 'review_ready' && !!data.value?.reviewArtifact)
const canApproveReview = computed(() => data.value?.job.type === 'pr_review' && data.value?.job.state === 'review_ready' && !!data.value?.reviewArtifact)
const isPRFeedbackJob = computed(() => data.value?.job.type === 'pr_feedback')
const isIssueJob = computed(() => data.value?.job.type === 'issue')
const hasReviewComments = computed(() => (data.value?.reviewComments?.length ?? 0) > 0)
const hasIssueBody = computed(() => (data.value?.issueBody?.trim().length ?? 0) > 0)
const watchRuleNameByID = computed(() => {
  return new Map((watchRules.value ?? []).map((rule) => [rule.id, rule.name.trim() || rule.id]))
})
const watchRuleDisplayName = computed(() => {
  const watchRuleId = data.value?.job.watchRuleId?.trim() ?? ''
  if (!watchRuleId) {
    return '-'
  }
  return watchRuleNameByID.value.get(watchRuleId) ?? watchRuleId
})

watch(
  () => data.value?.reviewArtifact?.content,
  (content) => {
    if (typeof content === 'string' && reviewArtifactComment.value.trim() === '') {
      reviewArtifactComment.value = content
    }
  },
  { immediate: true },
)

watch(
  [configuredToolCommand, toolExecution],
  ([configured]) => {
    if (selectedToolCommandName.value.trim()) {
      return
    }
    selectedToolCommandName.value = configured?.name ?? ''
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
    return '設計の再実行'
  }
  if (action === 'retry_implementation') {
    return '実装の再実行'
  }
  if (action === 'retry_review') {
    return 'レビューの再実行'
  }
  return 'PR作成の再実行'
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
    lines.push('# テスト結果')
    lines.push('')
    lines.push(`- プロファイル: ${report.profile ?? '-'}`)
    lines.push(`- 成功: ${report.success ? 'はい' : 'いいえ'}`)
    lines.push(`- 開始時刻: ${report.startedAt ?? '-'}`)
    lines.push(`- 終了時刻: ${report.finishedAt ?? '-'}`)
    lines.push('')
    lines.push('## 実行結果')
    lines.push('')

    const results = report.results ?? []
    if (results.length === 0) {
      lines.push('- 実行されたコマンドはありません。')
      return lines.join('\n')
    }

    results.forEach((result, index) => {
      lines.push(`### コマンド ${index + 1}`)
      lines.push('')
      lines.push(`- コマンド: ${result.command ?? '-'}`)
      lines.push(`- 終了コード: ${result.exitCode ?? '-'}`)
      lines.push(`- 所要時間: ${result.durationMs ?? '-'} ms`)
      lines.push(`- 成功: ${result.success ? 'はい' : 'いいえ'}`)
      lines.push('')
      lines.push('#### 標準出力')
      lines.push('')
      lines.push('```text')
      lines.push(result.stdout?.trimEnd() || '')
      lines.push('```')
      lines.push('')
      lines.push('#### 標準エラー')
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

function formatToolButtonLabel(action: 'start' | 'stop') {
  if (toolCommandState.value === 'saving' && toolCommandAction.value === action) {
    return action === 'start' ? '起動中...' : '停止中...'
  }
  return action === 'start' ? '起動' : '停止'
}

function formatToolExecutionSummary() {
  const execution = toolExecution.value
  const tool = execution ?? selectedToolCommand.value ?? configuredToolCommand.value
  if (!tool) {
    return ''
  }
  if (!execution) {
    return `${formatToolExecutionStatusLabel(tool.resident ? 'resident' : 'one-shot')} コマンド`
  }

  const parts: string[] = []
  parts.push(formatToolExecutionStatusLabel(tool.resident ? 'resident' : 'one-shot'))
  parts.push(formatToolExecutionStatusLabel(execution.running ? 'running' : 'stopped'))
  if (typeof execution.exitCode === 'number') {
    parts.push(`exit=${execution.exitCode}`)
  }
  return parts.join(' / ')
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
    const comment = defaultRerunCommentForEvent(action, event)
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
    rerunError(action).value = err instanceof Error ? err.message : UNKNOWN_ERROR_MESSAGE
  }
}

async function submitDesignArtifactRerun() {
  designRerunState.value = 'saving'
  designRerunError.value = null
  try {
    data.value = await submitDesignRerun(jobID.value, designArtifactComment.value)
    designRerunState.value = 'idle'
    designArtifactComment.value = ''
    designArtifactModalOpen.value = false
    await reload()
  } catch (err) {
    designRerunState.value = 'error'
    designRerunError.value = err instanceof Error ? err.message : UNKNOWN_ERROR_MESSAGE
  }
}

async function submitImplementationArtifactRerun() {
  implementationRerunState.value = 'saving'
  implementationRerunError.value = null
  try {
    const comment = implementationArtifactComment.value.trim().length > 0
      ? implementationArtifactComment.value
      : defaultRerunCommentForEvent('retry_implementation', latestEvent.value)
    data.value = await submitImplementationRerun(jobID.value, comment)
    implementationRerunState.value = 'idle'
    implementationArtifactComment.value = ''
    implementationArtifactModalOpen.value = false
    await reload()
  } catch (err) {
    implementationRerunState.value = 'error'
    implementationRerunError.value = err instanceof Error ? err.message : UNKNOWN_ERROR_MESSAGE
  }
}

async function submitTestReportRerun() {
  implementationRerunState.value = 'saving'
  implementationRerunError.value = null
  try {
    const comment = testReportComment.value.trim().length > 0
      ? testReportComment.value
      : defaultRerunCommentForEvent('retry_implementation', latestEvent.value)
    data.value = await submitImplementationRerun(jobID.value, comment)
    implementationRerunState.value = 'idle'
    testReportComment.value = ''
    testReportModalOpen.value = false
    await reload()
  } catch (err) {
    implementationRerunState.value = 'error'
    implementationRerunError.value = err instanceof Error ? err.message : UNKNOWN_ERROR_MESSAGE
  }
}

async function submitReviewArtifactRerun() {
  reviewRerunState.value = 'saving'
  reviewRerunError.value = null
  try {
    data.value = await submitReviewRerun(jobID.value, reviewArtifactComment.value)
    reviewRerunState.value = 'idle'
    reviewArtifactModalOpen.value = false
    await reload()
  } catch (err) {
    reviewRerunState.value = 'error'
    reviewRerunError.value = err instanceof Error ? err.message : UNKNOWN_ERROR_MESSAGE
  }
}

async function sendApproval(status: 'approved' | 'rejected', comment = '') {
  approvalState.value = 'saving'
  approvalError.value = null
  try {
    data.value = await submitDesignApproval(jobID.value, status, comment)
    approvalState.value = 'idle'
    designArtifactComment.value = ''
    designArtifactModalOpen.value = false
    await reload()
  } catch (err) {
    approvalState.value = 'error'
    approvalError.value = err instanceof Error ? err.message : UNKNOWN_ERROR_MESSAGE
  }
}

async function sendFinalApproval(status: 'approved' | 'rejected', comment = '', source: 'implementation' | 'test-report' | null = null) {
  finalApprovalState.value = 'saving'
  finalApprovalError.value = null
  try {
    data.value = await submitFinalApproval(jobID.value, status, comment)
    finalApprovalState.value = 'idle'
    if (source === 'implementation') {
      implementationArtifactComment.value = ''
      implementationArtifactModalOpen.value = false
    }
    if (source === 'test-report') {
      testReportComment.value = ''
      testReportModalOpen.value = false
    }
    await reload()
  } catch (err) {
    finalApprovalState.value = 'error'
    finalApprovalError.value = err instanceof Error ? err.message : UNKNOWN_ERROR_MESSAGE
  }
}

async function sendReviewComment() {
  reviewSubmitState.value = 'saving'
  reviewSubmitError.value = null
  try {
    data.value = await submitReviewComment(jobID.value, reviewArtifactComment.value)
    reviewSubmitState.value = 'saved'
    await reload()
  } catch (err) {
    reviewSubmitState.value = 'error'
    reviewSubmitError.value = err instanceof Error ? err.message : UNKNOWN_ERROR_MESSAGE
  }
}

async function sendReviewApproval() {
  reviewApproveState.value = 'saving'
  reviewApproveError.value = null
  try {
    data.value = await submitReviewApproval(jobID.value)
    reviewApproveState.value = 'saved'
    reviewArtifactModalOpen.value = false
    await reload()
  } catch (err) {
    reviewApproveState.value = 'error'
    reviewApproveError.value = err instanceof Error ? err.message : UNKNOWN_ERROR_MESSAGE
  }
}

function prCommentKey(comment: ReviewComment, index: number) {
  return `${comment.url ?? ''}::${comment.createdAt ?? ''}::${comment.author ?? ''}::${index}`
}

async function loadPRComments() {
  prCommentsState.value = 'loading'
  prCommentsError.value = null
  try {
    const response = await fetchPRComments(jobID.value)
    prCommentsPullNumber.value = response.pullNumber
    prComments.value = response.comments
    if (data.value) {
      data.value = {
        ...data.value,
        prComments: response.comments,
      }
    }
    prCommentsState.value = 'idle'
  } catch (err) {
    prCommentsState.value = 'error'
    prCommentsError.value = err instanceof Error ? err.message : UNKNOWN_ERROR_MESSAGE
  }
}

function openPRCommentsModal() {
  prCommentsModalOpen.value = true
  prCommentsState.value = 'loading'
  prCommentsError.value = null
  prCommentAnalyzeState.value = 'idle'
  prCommentAnalyzeError.value = null
  prCommentAnalyzingKey.value = null
  prCommentsPullNumber.value = null
  prComments.value = []
  void loadPRComments()
}

function closePRCommentsModal() {
  prCommentsModalOpen.value = false
  prCommentsError.value = null
  prCommentsState.value = 'idle'
  prCommentAnalyzeState.value = 'idle'
  prCommentAnalyzeError.value = null
  prCommentAnalyzingKey.value = null
}

async function analyzePRCommentItem(comment: ReviewComment, index: number) {
  prCommentAnalyzeState.value = 'saving'
  prCommentAnalyzeError.value = null
  prCommentAnalyzingKey.value = prCommentKey(comment, index)
  try {
    selectedPRCommentForAnalysis.value = comment
    prCommentAnalysisComment.value = ''
    data.value = await analyzePRComment(jobID.value, comment)
    prCommentAnalyzeState.value = 'idle'
    prCommentAnalyzingKey.value = null
    prCommentsModalOpen.value = false
    await reload()
  } catch (err) {
    prCommentAnalyzeState.value = 'error'
    prCommentAnalyzingKey.value = null
    prCommentAnalyzeError.value = err instanceof Error ? err.message : UNKNOWN_ERROR_MESSAGE
  }
}

function buildPRCommentAnalysisPayload() {
  const analysis = prCommentAnalysisRawContent.value.trim()
  const note = prCommentAnalysisComment.value.trim()
  if (!note) {
    return analysis
  }
  if (!analysis) {
    return note
  }
  return `${analysis}\n\n## コメント\n\n${note}`
}

async function rerunPRCommentAnalysis() {
  const selectedComment = selectedPRCommentForAnalysis.value
  if (!selectedComment) {
    return
  }
  prCommentAnalysisActionState.value = 'saving'
  prCommentAnalysisActionError.value = null
  try {
    const note = prCommentAnalysisComment.value.trim()
    const rerunComment = note.length > 0
      ? {
          ...selectedComment,
          body: `${selectedComment.body}\n\n## コメント\n\n${note}`,
        }
      : selectedComment
    data.value = await analyzePRComment(jobID.value, rerunComment)
    prCommentAnalysisActionState.value = 'idle'
    await reload()
  } catch (err) {
    prCommentAnalysisActionState.value = 'error'
    prCommentAnalysisActionError.value = err instanceof Error ? err.message : UNKNOWN_ERROR_MESSAGE
  }
}

async function sendPRCommentAnalysisApproval(status: 'approved' | 'rejected') {
  prCommentAnalysisActionState.value = 'saving'
  prCommentAnalysisActionError.value = null
  try {
    data.value = await submitDesignApproval(jobID.value, status, buildPRCommentAnalysisPayload())
    prCommentAnalysisActionState.value = 'idle'
    prCommentAnalysisModalOpen.value = false
    await reload()
  } catch (err) {
    prCommentAnalysisActionState.value = 'error'
    prCommentAnalysisActionError.value = err instanceof Error ? err.message : UNKNOWN_ERROR_MESSAGE
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
    jobArchiveError.value = err instanceof Error ? err.message : UNKNOWN_ERROR_MESSAGE
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
    jobArchiveError.value = err instanceof Error ? err.message : UNKNOWN_ERROR_MESSAGE
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
    jobPurgeError.value = err instanceof Error ? err.message : UNKNOWN_ERROR_MESSAGE
  }
}

async function createImprovementDraft() {
  improvementCreateState.value = 'saving'
  improvementCreateError.value = null
  try {
    await generateImprovement(jobID.value, improvementComment.value)
    improvementCreateState.value = 'idle'
    improvementComment.value = ''
    improvementModalOpen.value = false
  } catch (err) {
    improvementCreateState.value = 'error'
    improvementCreateError.value = err instanceof Error ? err.message : UNKNOWN_ERROR_MESSAGE
  }
}

async function startSelectedToolCommand() {
  if (!selectedToolCommand.value || toolExecution.value?.running) {
    return
  }
  toolCommandState.value = 'saving'
  toolCommandAction.value = 'start'
  toolCommandError.value = null
  try {
    data.value = await startToolCommand(jobID.value, selectedToolCommand.value.name)
    toolCommandState.value = 'idle'
    toolCommandAction.value = null
    await reload()
  } catch (err) {
    toolCommandState.value = 'error'
    toolCommandAction.value = null
    toolCommandError.value = err instanceof Error ? err.message : UNKNOWN_ERROR_MESSAGE
  }
}

async function stopRunningToolCommand() {
  if (!toolExecution.value?.running) {
    return
  }
  toolCommandState.value = 'saving'
  toolCommandAction.value = 'stop'
  toolCommandError.value = null
  try {
    data.value = await stopToolCommand(jobID.value)
    toolCommandState.value = 'idle'
    toolCommandAction.value = null
    await reload()
  } catch (err) {
    toolCommandState.value = 'error'
    toolCommandAction.value = null
    toolCommandError.value = err instanceof Error ? err.message : UNKNOWN_ERROR_MESSAGE
  }
}

async function openPopup(setOpen: (value: boolean) => void) {
  popupLoadingState.value = 'loading'
  popupLoadingError.value = null
  try {
    await reload({ silent: true })
    setOpen(true)
    popupLoadingState.value = 'idle'
  } catch (err) {
    popupLoadingState.value = 'error'
    popupLoadingError.value = err instanceof Error ? err.message : UNKNOWN_ERROR_MESSAGE
  }
}

function openIssueBodyModal() {
  issueBodyRefreshState.value = 'idle'
  issueBodyRefreshError.value = null
  void openPopup((value) => {
    issueBodyModalOpen.value = value
  })
}

async function refreshIssueBodyContent() {
  const current = data.value
  if (!current) {
    return
  }
  issueBodyRefreshState.value = 'loading'
  issueBodyRefreshError.value = null
  try {
    const response = await refreshIssueBody(jobID.value)
    data.value = {
      ...(data.value ?? current),
      issueBody: response.issueBody,
    }
    issueBodyRefreshState.value = 'idle'
  } catch (err) {
    issueBodyRefreshState.value = 'error'
    issueBodyRefreshError.value = err instanceof Error ? err.message : UNKNOWN_ERROR_MESSAGE
  }
}

function openDesignArtifactModal() {
  void openPopup((value) => {
    designArtifactModalOpen.value = value
  })
}

function openImplementationArtifactModal() {
  void openPopup((value) => {
    implementationArtifactModalOpen.value = value
  })
}

function openReviewArtifactModal() {
  void openPopup((value) => {
    reviewArtifactModalOpen.value = value
  })
}

function openTestReportModal() {
  void openPopup((value) => {
    testReportModalOpen.value = value
  })
}

function openToolLogModal() {
  void openPopup((value) => {
    toolLogModalOpen.value = value
  })
}

function openPRCreateModal() {
  void openPopup((value) => {
    prCreateModalOpen.value = value
  })
}
</script>

<template>
  <AppShell
    title="ジョブ詳細"
    description="自動処理の詳細情報（設計結果、実装結果、レビュー結果など）やイベント履歴を確認できます。"
  >
    <AsyncState :is-loading="isLoading" :error="error">
      <p v-if="isRefreshing" class="text-muted">ジョブ詳細を同期しています...</p>
      <template v-if="data">
        <section class="hero-grid">
          <PanelCard title="ジョブ概要">
            <div class="stack-sm job-summary">
              <h3 class="job-summary__title">{{ data.job.title || '-' }}</h3>
              <p class="job-summary__id text-muted">ID: <code>{{ data.job.id || '-' }}</code></p>
              <p class="text-muted">種別: {{ formatJobTypeLabel(data.job.type) }}</p>
              <p class="text-muted">{{ data.job.repository }} #{{ data.job.githubNumber }}</p>
              <p v-if="prCreateInfo?.pullNumber" class="text-muted">
                PR: <code>#{{ prCreateInfo.pullNumber }}</code>
                <a v-if="prCreateInfo.url" class="table-link" :href="prCreateInfo.url" target="_blank" rel="noreferrer">開く</a>
              </p>
              <p v-if="isIssueJob" class="text-muted">ブランチ: <code>{{ data.job.branchName || '-' }}</code></p>
              <p class="text-muted">監視ルール: <code>{{ watchRuleDisplayName }}</code></p>
            </div>
          </PanelCard>
          <PanelCard title="フロー">
            <div class="stack-sm">
              <div v-if="!isDeletedJob && flowRerunActionInFlow" class="status-inline">
                <StateBadge :state="data.job.state" />
                <button
                  class="button button-secondary"
                  type="button"
                  :disabled="rerunState(flowRerunActionInFlow) === 'saving'"
                  @click="submitRerun(flowRerunActionInFlow, flowRerunEvent?.id)"
                >
                  {{ rerunButtonLabel(flowRerunActionInFlow, flowRerunEvent?.eventType, flowRerunEvent?.sourceEventType) }}
                </button>
              </div>
              <StateBadge v-else :state="data.job.state" />
              <template v-if="canShowToolFlow">
                <div class="stack-sm">
                  <p class="text-muted">
                    <span v-if="toolExecution">
                      実行中: <code>{{ toolExecution.name }}</code> / 
                    </span>
                    {{ formatToolExecutionSummary() }}
                  </p>
                  <label class="field field-full">
                    <span class="field__label">ツールコマンド</span>
                    <select
                      v-model="selectedToolCommandName"
                      class="field__control"
                      :disabled="toolExecution?.running || toolCommandState === 'saving' || availableToolCommands.length === 0"
                    >
                      <option value="" disabled>ツールコマンドを選択</option>
                      <option v-for="command in availableToolCommands" :key="command.name" :value="command.name">
                        {{ command.name }}
                      </option>
                    </select>
                  </label>
                  <div class="button-row">
                    <button
                      class="button button-primary"
                      type="button"
                      :disabled="toolCommandState === 'saving' || toolExecution?.running || !selectedToolCommand"
                      @click="startSelectedToolCommand"
                    >
                      {{ formatToolButtonLabel('start') }}
                    </button>
                    <button
                      class="button button-secondary"
                      type="button"
                      :disabled="toolCommandState === 'saving' || !toolExecution?.running"
                      @click="stopRunningToolCommand"
                    >
                      {{ formatToolButtonLabel('stop') }}
                    </button>
                  </div>
                </div>
              </template>
              <template v-if="isDeletedJob">
                <p class="notice notice-danger">このジョブは削除済みです。</p>
                <div class="flow-delete-row">
                  <div class="button-row">
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
                  </div>
                </div>
              </template>
              <p v-if="jobArchiveState === 'error'" class="notice notice-danger">{{ jobArchiveError }}</p>
              <p v-if="jobPurgeState === 'error'" class="notice notice-danger">{{ jobPurgeError }}</p>
              <p v-if="approvalState === 'error'" class="notice notice-danger">{{ approvalError }}</p>
              <p v-if="toolCommandState === 'error'" class="notice notice-danger">{{ toolCommandError }}</p>
              <template v-if="canReviewImplementation">
                <p v-if="finalApprovalWarning" class="notice notice-danger">{{ finalApprovalWarning }}</p>
                <p v-if="finalApprovalState === 'error'" class="notice notice-danger">{{ finalApprovalError }}</p>
              </template>
              <div v-if="!isDeletedJob" class="flow-delete-row">
                <button
                  class="button button-secondary"
                  type="button"
                  :disabled="jobArchiveState === 'saving'"
                  @click="archiveJob"
                >
                  ジョブを削除
                </button>
              </div>
            </div>
          </PanelCard>
        </section>

        <PanelCard
          v-if="!isPRFeedbackJob && hasIssueBody"
          title="Issue"
        >
          <div class="artifact-headline">
            <button class="log-entry-button" type="button" @click="openIssueBodyModal">Issue 本文を開く</button>
            <p class="text-muted">元の issue 内容です。</p>
          </div>
        </PanelCard>

        <PanelCard v-if="!isDeletedJob" title="改善案生成">
          <div class="artifact-headline">
            <button class="log-entry-button" type="button" @click="improvementModalOpen = true">改善案を生成</button>
            <p class="text-muted">コメントを入力して改善 draft を生成し、改善点画面で編集と承認を行います。</p>
          </div>
        </PanelCard>

        <PanelCard
          v-if="isPRFeedbackJob && hasReviewComments"
          title="PR レビューコメント"
          description="修正対象の PR レビューコメントです。"
        >
          <div class="stack-sm">
            <details v-for="(comment, index) in data.reviewComments" :key="`${comment.url ?? index}`" class="stack-sm">
              <summary class="text-muted">
                {{ comment.author || 'unknown' }} / {{ formatReviewCommentLocation(comment.path, comment.line) }}
              </summary>
              <pre class="artifact-view">{{ comment.body }}</pre>
              <p v-if="comment.url">
                <a class="table-link" :href="comment.url" target="_blank" rel="noreferrer">GitHub で開く</a>
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
          title="設計成果物"
        >
          <div class="artifact-headline">
            <button class="log-entry-button" type="button" @click="openDesignArtifactModal">設計結果を開く</button>
            <p class="text-muted">生成された設計成果物です。承認前に内容を確認します。</p>
          </div>
        </PanelCard>

        <PanelCard
          v-if="data.implementationArtifact"
          :title="isPRFeedbackJob ? '修正結果' : '実装成果物'"
        >
          <div class="artifact-headline">
            <button class="log-entry-button" type="button" @click="openImplementationArtifactModal">{{ isPRFeedbackJob ? '修正結果を開く' : '実装結果を開く' }}</button>
            <p class="text-muted">{{ isPRFeedbackJob ? 'PR レビューコメントに対する修正結果です。' : '実装フェーズの成果物サマリです。最終承認前に確認します。' }}</p>
          </div>
        </PanelCard>

        <PanelCard
          v-if="data.reviewArtifact"
          title="レビュー成果物"
        >
          <div class="artifact-headline">
            <button class="log-entry-button" type="button" @click="openReviewArtifactModal">レビュー結果を開く</button>
            <p class="text-muted">PR レビューフェーズの成果物です。総評コメントとして GitHub へ返せます。</p>
          </div>
          <div class="stack-sm">
            <p v-if="reviewSubmitState === 'saved'" class="notice notice-success">レビューコメントを GitHub に送信しました。</p>
            <p v-if="reviewSubmitState === 'error'" class="notice notice-danger">{{ reviewSubmitError }}</p>
            <p v-if="reviewApproveState === 'saved'" class="notice notice-success">レビューを承認しました。</p>
            <p v-if="reviewApproveState === 'error'" class="notice notice-danger">{{ reviewApproveError }}</p>
          </div>
        </PanelCard>

        <PanelCard
          v-if="data.testReport"
          title="テスト結果"
        >
          <div class="artifact-headline">
            <button class="log-entry-button" type="button" @click="openTestReportModal">テスト結果を開く</button>
            <p class="text-muted">設定された test profile の実行結果です。</p>
          </div>
        </PanelCard>

        <PanelCard
          v-if="toolExecution || toolStdout || toolStderr"
          title="ツールログ"
        >
          <div class="artifact-headline">
            <button class="log-entry-button" type="button" @click="openToolLogModal">ツールログを開く</button>
            <p class="text-muted">
              <code>{{ toolExecution?.name ?? selectedToolCommand?.name ?? configuredToolCommand?.name ?? '-' }}</code>
              / {{ formatToolExecutionSummary() }}
            </p>
          </div>
        </PanelCard>

        <PanelCard
          v-if="data.prCreateArtifact"
          title="PR 作成"
        >
          <div class="artifact-headline">
            <button class="log-entry-button" type="button" @click="openPRCreateModal">PR 作成結果を開く</button>
            <p class="text-muted">作成された PR の情報を確認できます。</p>
          </div>
        </PanelCard>

        <PanelCard
          v-if="hasPRCommentsArtifact"
          title="PR コメント"
        >
          <div class="artifact-headline">
            <button class="log-entry-button" type="button" :disabled="prCommentsState === 'loading'" @click="openPRCommentsModal">
              {{ prCommentsState === 'loading' ? '取得中...' : 'PRコメントを開く' }}
            </button>
            <p class="text-muted">取得済みの PR コメントを確認し、必要なら修正案の検討を開始します。</p>
          </div>
        </PanelCard>

        <PanelCard
          v-if="hasPRCommentAnalysis"
          title="PR コメント分析結果"
        >
          <div class="artifact-headline">
            <button class="log-entry-button" type="button" @click="prCommentAnalysisModalOpen = true">分析結果を開く</button>
            <p class="text-muted">取得した PR コメントに対する修正案の検討結果です。</p>
          </div>
        </PanelCard>

        <PanelCard
          v-if="groupedLogs.length > 0"
          title="ログ"
          description="各フェーズの実行ログです。"
        >
          <div class="stack-md">
            <section v-for="group in groupedLogs" :key="group.phase" class="stack-sm">
              <h3>{{ group.title }}</h3>
              <div class="stack-sm">
                <button
                  v-for="log in group.items"
                  :key="log.path"
                  class="log-entry-button"
                  type="button"
                  @click="selectedLog = { groupTitle: group.title, log }"
                >
                  {{ formatLogName(log.name) }} <code>{{ log.path }}</code>
                </button>
              </div>
            </section>
          </div>
        </PanelCard>

        <DataTable :columns="['日時', 'イベント', '状態', 'ペイロード']">
          <tr v-for="event in eventRows" :key="event.id">
            <td>{{ formatDateTime(event.createdAt) }}</td>
            <td>{{ formatEventTypeLabel(event.eventType) }}</td>
            <td>{{ formatStateLabel(event.stateTo || '-') }}</td>
            <td class="payload-cell">
              <details class="payload-details">
                <summary class="payload-summary">
                  <span class="payload-summary__label">クリックして展開</span>
                  <span class="payload-summary__preview">{{ event.payloadDisplay.preview }}</span>
                </summary>
                <pre class="payload-view artifact-view">{{ event.payloadDisplay.content }}</pre>
              </details>
            </td>
          </tr>
          <tr v-if="data.events.length === 0">
            <td colspan="4" class="text-muted">イベントはまだありません。</td>
          </tr>
        </DataTable>

        <div v-if="designArtifactModalOpen && data.designArtifact" class="modal-backdrop" @click.self="designArtifactModalOpen = false">
          <div class="modal-panel">
            <div class="modal-panel__header">
              <div>
                <h3 class="modal-panel__title">設計成果物</h3>
                <p class="text-muted"><code>{{ data.designArtifact.path }}</code></p>
              </div>
              <button class="button button-secondary" type="button" @click="designArtifactModalOpen = false">閉じる</button>
            </div>
            <div class="stack-sm">
              <pre class="artifact-view">{{ data.designArtifact.content }}</pre>
              <label class="field field-full">
                <span class="field__label">コメント</span>
                <textarea
                  v-model="designArtifactComment"
                  class="field__control field__control--textarea"
                  rows="4"
                  placeholder="再実行や承認時のコメントを入力してください。"
                  spellcheck="false"
                />
              </label>
              <div class="modal-actions">
                <div class="button-row">
                  <button v-if="canReviewDesign" class="button button-primary" type="button" :disabled="approvalState === 'saving'" @click="sendApproval('approved', designArtifactComment)">
                    {{ approvalState === 'saving' ? '承認中...' : '承認' }}
                  </button>
                  <button class="button button-secondary" type="button" :disabled="designRerunState === 'saving'" @click="submitDesignArtifactRerun">
                    {{ designRerunState === 'saving' ? '再実行中...' : '再実行' }}
                  </button>
                </div>
                <div v-if="canReviewDesign" class="modal-actions__right">
                  <button class="button button-secondary" type="button" :disabled="approvalState === 'saving'" @click="sendApproval('rejected', designArtifactComment)">
                    {{ approvalState === 'saving' ? '却下中...' : '却下' }}
                  </button>
                </div>
              </div>
              <p v-if="designRerunState === 'error'" class="notice notice-danger">{{ rerunErrorLabel('retry_design') }}: {{ designRerunError }}</p>
              <p v-if="approvalState === 'error'" class="notice notice-danger">{{ approvalError }}</p>
            </div>
          </div>
        </div>

        <div v-if="issueBodyModalOpen" class="modal-backdrop" @click.self="issueBodyModalOpen = false">
          <div class="modal-panel">
            <div class="modal-panel__header">
              <div>
                <h3 class="modal-panel__title">Issue</h3>
              </div>
              <button class="button button-secondary" type="button" @click="issueBodyModalOpen = false">閉じる</button>
            </div>
            <div class="stack-sm">
              <pre class="artifact-view">{{ formatIssueBody(data.issueBody) }}</pre>
              <div class="button-row">
                <button
                  class="button button-primary"
                  type="button"
                  :disabled="issueBodyRefreshState === 'loading'"
                  @click="refreshIssueBodyContent"
                >
                  {{ issueBodyRefreshState === 'loading' ? '取得中...' : '最新を取得' }}
                </button>
              </div>
              <p v-if="issueBodyRefreshState === 'loading'" class="text-muted">GitHub から最新の Issue 本文を取得しています...</p>
              <p v-if="issueBodyRefreshState === 'error'" class="notice notice-danger">{{ issueBodyRefreshError }}</p>
            </div>
          </div>
        </div>

        <div v-if="implementationArtifactModalOpen && data.implementationArtifact" class="modal-backdrop" @click.self="implementationArtifactModalOpen = false">
          <div class="modal-panel">
            <div class="modal-panel__header">
              <div>
                <h3 class="modal-panel__title">{{ isPRFeedbackJob ? '修正結果' : '実装成果物' }}</h3>
                <p class="text-muted"><code>{{ data.implementationArtifact.path }}</code></p>
              </div>
              <button class="button button-secondary" type="button" @click="implementationArtifactModalOpen = false">閉じる</button>
            </div>
            <div class="stack-sm">
              <pre class="artifact-view">{{ data.implementationArtifact.content }}</pre>
              <label class="field field-full">
                <span class="field__label">コメント</span>
                <textarea
                  v-model="implementationArtifactComment"
                  class="field__control field__control--textarea"
                  rows="4"
                  placeholder="再実行や承認時のコメントを入力してください。"
                  spellcheck="false"
                />
              </label>
              <div class="modal-actions">
                <div class="button-row">
                  <button v-if="canReviewImplementation" class="button button-primary" type="button" :disabled="finalApprovalState === 'saving'" @click="sendFinalApproval('approved', implementationArtifactComment, 'implementation')">
                    {{ finalApprovalState === 'saving' ? '承認中...' : '承認' }}
                  </button>
                  <button class="button button-secondary" type="button" :disabled="implementationRerunState === 'saving'" @click="submitImplementationArtifactRerun">
                    {{ implementationRerunState === 'saving' ? '再実行中...' : '再実行' }}
                  </button>
                </div>
                <div v-if="canReviewImplementation" class="modal-actions__right">
                  <button class="button button-secondary" type="button" :disabled="finalApprovalState === 'saving'" @click="sendFinalApproval('rejected', implementationArtifactComment, 'implementation')">
                    {{ finalApprovalState === 'saving' ? '却下中...' : '却下' }}
                  </button>
                </div>
              </div>
              <p v-if="implementationRerunState === 'error'" class="notice notice-danger">{{ rerunErrorLabel('retry_implementation') }}: {{ implementationRerunError }}</p>
              <p v-if="finalApprovalWarning && canReviewImplementation" class="notice notice-danger">{{ finalApprovalWarning }}</p>
              <p v-if="finalApprovalState === 'error'" class="notice notice-danger">{{ finalApprovalError }}</p>
            </div>
          </div>
        </div>

        <div v-if="selectedLog" class="modal-backdrop" @click.self="selectedLog = null">
          <div class="modal-panel">
            <div class="modal-panel__header">
              <div>
                <h3 class="modal-panel__title">{{ selectedLog.groupTitle }}</h3>
                <p class="text-muted">{{ formatLogName(selectedLog.log.name) }} <code>{{ selectedLog.log.path }}</code></p>
              </div>
              <button class="button button-secondary" type="button" @click="selectedLog = null">閉じる</button>
            </div>
            <pre class="artifact-view">{{ selectedLog.log.content }}</pre>
          </div>
        </div>

        <div v-if="toolLogModalOpen && (toolExecution || toolStdout || toolStderr)" class="modal-backdrop" @click.self="toolLogModalOpen = false">
          <div class="modal-panel">
            <div class="modal-panel__header">
              <div>
                <h3 class="modal-panel__title">ツールログ</h3>
                <p class="text-muted">
                  <code>{{ toolExecution?.name ?? selectedToolCommand?.name ?? configuredToolCommand?.name ?? '-' }}</code>
                  / {{ formatToolExecutionSummary() }}
                </p>
                <p v-if="toolExecution?.startedAt" class="text-muted">
                  開始: {{ formatDateTime(toolExecution.startedAt) }}
                  <span v-if="toolExecution.finishedAt"> / 終了: {{ formatDateTime(toolExecution.finishedAt) }}</span>
                </p>
              </div>
              <button class="button button-secondary" type="button" @click="toolLogModalOpen = false">閉じる</button>
            </div>
            <div class="stack-sm">
              <section class="stack-sm">
                <h4>標準出力</h4>
                <p v-if="toolExecution?.stdout?.path" class="text-muted"><code>{{ toolExecution.stdout.path }}</code></p>
                <pre class="artifact-view">{{ toolStdoutContent || 'まだ標準出力ログはありません。' }}</pre>
              </section>
              <section class="stack-sm">
                <h4>標準エラー</h4>
                <p v-if="toolExecution?.stderr?.path" class="text-muted"><code>{{ toolExecution.stderr.path }}</code></p>
                <pre class="artifact-view">{{ toolStderrContent || 'まだ標準エラーログはありません。' }}</pre>
              </section>
            </div>
          </div>
        </div>

        <p v-if="popupLoadingState === 'loading'" class="text-muted">最新ファイルを取得しています...</p>
        <p v-if="popupLoadingState === 'error'" class="notice notice-danger">{{ popupLoadingError }}</p>

        <div v-if="testReportModalOpen && data.testReport" class="modal-backdrop" @click.self="testReportModalOpen = false">
          <div class="modal-panel">
            <div class="modal-panel__header">
              <div>
                <h3 class="modal-panel__title">テスト結果</h3>
                <p class="text-muted"><code>{{ data.testReport.path }}</code></p>
              </div>
              <button class="button button-secondary" type="button" @click="testReportModalOpen = false">閉じる</button>
            </div>
            <div class="stack-sm">
              <pre class="artifact-view">{{ testReportMarkdown }}</pre>
              <label class="field field-full">
                <span class="field__label">コメント</span>
                <textarea
                  v-model="testReportComment"
                  class="field__control field__control--textarea"
                  rows="4"
                  placeholder="再実行や承認時のコメントを入力してください。"
                  spellcheck="false"
                />
              </label>
              <div class="modal-actions">
                <div class="button-row">
                  <button v-if="canReviewImplementation" class="button button-primary" type="button" :disabled="finalApprovalState === 'saving'" @click="sendFinalApproval('approved', testReportComment, 'test-report')">
                    {{ finalApprovalState === 'saving' ? '承認中...' : '承認' }}
                  </button>
                  <button class="button button-secondary" type="button" :disabled="implementationRerunState === 'saving'" @click="submitTestReportRerun">
                    {{ implementationRerunState === 'saving' ? '再実行中...' : '再実行' }}
                  </button>
                </div>
                <div v-if="canReviewImplementation" class="modal-actions__right">
                  <button class="button button-secondary" type="button" :disabled="finalApprovalState === 'saving'" @click="sendFinalApproval('rejected', testReportComment, 'test-report')">
                    {{ finalApprovalState === 'saving' ? '却下中...' : '却下' }}
                  </button>
                </div>
              </div>
              <p v-if="implementationRerunState === 'error'" class="notice notice-danger">{{ rerunErrorLabel('retry_implementation') }}: {{ implementationRerunError }}</p>
              <p v-if="finalApprovalWarning && canReviewImplementation" class="notice notice-danger">{{ finalApprovalWarning }}</p>
              <p v-if="finalApprovalState === 'error'" class="notice notice-danger">{{ finalApprovalError }}</p>
            </div>
          </div>
        </div>

        <div v-if="prCreateModalOpen && data.prCreateArtifact" class="modal-backdrop" @click.self="prCreateModalOpen = false">
          <div class="modal-panel">
            <div class="modal-panel__header">
              <div>
                <h3 class="modal-panel__title">PR 作成</h3>
                <p class="text-muted"><code>{{ data.prCreateArtifact.path }}</code></p>
              </div>
              <button class="button button-secondary" type="button" @click="prCreateModalOpen = false">閉じる</button>
            </div>
            <div class="stack-sm">
              <template v-if="prCreateInfo">
                <p v-if="prCreateInfo.title"><strong>{{ prCreateInfo.title }}</strong></p>
                <p v-if="prCreateInfo.repository" class="text-muted">リポジトリ: <code>{{ prCreateInfo.repository }}</code></p>
                <p v-if="prCreateInfo.branchName" class="text-muted">ブランチ: <code>{{ prCreateInfo.branchName }}</code></p>
                <p v-if="prCreateInfo.pullNumber" class="text-muted">PR 番号: <code>{{ prCreateInfo.pullNumber }}</code></p>
                <p v-if="prCreateInfo.url">
                  <a class="table-link" :href="prCreateInfo.url" target="_blank" rel="noreferrer">PR を開く</a>
                </p>
              </template>
              <p v-else class="text-muted">PR 作成結果を構造化表示できなかったため、生データを表示しています。</p>
              <pre class="artifact-view">{{ prCreateRawContent }}</pre>
            </div>
          </div>
        </div>

        <div v-if="prCommentsModalOpen" class="modal-backdrop" @click.self="closePRCommentsModal">
          <div class="modal-panel">
            <div class="modal-panel__header">
              <div>
                <h3 class="modal-panel__title">PR コメント</h3>
                <p class="text-muted">
                  <span v-if="prCommentsPullNumber">PR #{{ prCommentsPullNumber }}</span>
                  <span v-else>PR コメントを取得しています。</span>
                </p>
              </div>
              <button class="button button-secondary" type="button" @click="closePRCommentsModal">戻る</button>
            </div>
            <div class="stack-sm">
              <p v-if="prCommentsState === 'loading'" class="text-muted">PR コメントを取得しています...</p>
              <p v-if="prCommentsState === 'error'" class="notice notice-danger">{{ prCommentsError }}</p>
              <p v-if="prCommentAnalyzeState === 'error'" class="notice notice-danger">{{ prCommentAnalyzeError }}</p>
              <p v-if="prCommentsState !== 'loading' && prComments.length === 0 && !prCommentsError" class="text-muted">コメントはありません。</p>
              <section v-for="(comment, index) in prComments" :key="prCommentKey(comment, index)" class="stack-sm">
                <div class="text-muted">
                  {{ comment.author || 'unknown' }}
                  <span v-if="comment.createdAt"> / {{ formatDateTime(comment.createdAt) }}</span>
                  <span v-if="comment.path || comment.line"> / {{ formatReviewCommentLocation(comment.path, comment.line) }}</span>
                </div>
                <pre class="artifact-view">{{ comment.body }}</pre>
                <div class="button-row">
                  <button
                    class="button button-primary"
                    type="button"
                    :disabled="prCommentAnalyzeState === 'saving'"
                    @click="analyzePRCommentItem(comment, index)"
                  >
                    {{ prCommentAnalyzeState === 'saving' && prCommentAnalyzingKey === prCommentKey(comment, index) ? '分析中...' : 'PRコメント分析' }}
                  </button>
                  <button class="button button-secondary" type="button" @click="closePRCommentsModal">戻る</button>
                </div>
                <p v-if="comment.url">
                  <a class="table-link" :href="comment.url" target="_blank" rel="noreferrer">GitHub で開く</a>
                </p>
              </section>
            </div>
          </div>
        </div>

        <div v-if="prCommentAnalysisModalOpen && data.prCommentAnalysisArtifact" class="modal-backdrop" @click.self="prCommentAnalysisModalOpen = false">
          <div class="modal-panel">
              <div class="modal-panel__header">
                <div>
                  <h3 class="modal-panel__title">PR コメント分析結果</h3>
                  <p class="text-muted"><code>{{ data.prCommentAnalysisArtifact.path }}</code></p>
                </div>
              <button class="button button-secondary" type="button" @click="prCommentAnalysisModalOpen = false">閉じる</button>
            </div>
            <div class="stack-sm">
              <pre class="artifact-view">{{ prCommentAnalysisRawContent }}</pre>
              <label class="field field-full">
                <span class="field__label">コメント</span>
                <textarea
                  v-model="prCommentAnalysisComment"
                  class="field__control field__control--textarea"
                  rows="4"
                  placeholder="再実行や承認時のコメントを入力してください。"
                  spellcheck="false"
                />
              </label>
              <div class="modal-actions">
                <div class="button-row">
                  <button v-if="canReviewPRCommentAnalysis" class="button button-primary" type="button" :disabled="prCommentAnalysisActionState === 'saving'" @click="sendPRCommentAnalysisApproval('approved')">
                    {{ prCommentAnalysisActionState === 'saving' ? '承認中...' : '承認' }}
                  </button>
                  <button class="button button-secondary" type="button" :disabled="prCommentAnalysisActionState === 'saving'" @click="rerunPRCommentAnalysis">
                    {{ prCommentAnalysisActionState === 'saving' ? '再実行中...' : '再実行' }}
                  </button>
                </div>
                <div v-if="canReviewPRCommentAnalysis" class="modal-actions__right">
                  <button class="button button-secondary" type="button" :disabled="prCommentAnalysisActionState === 'saving'" @click="sendPRCommentAnalysisApproval('rejected')">
                    {{ prCommentAnalysisActionState === 'saving' ? '却下中...' : '却下' }}
                  </button>
                </div>
              </div>
              <p v-if="prCommentAnalysisActionState === 'error'" class="notice notice-danger">{{ prCommentAnalysisActionError }}</p>
            </div>
          </div>
        </div>
        <div v-if="reviewArtifactModalOpen && data.reviewArtifact" class="modal-backdrop" @click.self="reviewArtifactModalOpen = false">
          <div class="modal-panel">
            <div class="modal-panel__header">
              <div>
                <h3 class="modal-panel__title">レビュー成果物</h3>
                <p class="text-muted"><code>{{ data.reviewArtifact.path }}</code></p>
              </div>
              <button class="button button-secondary" type="button" @click="reviewArtifactModalOpen = false">閉じる</button>
            </div>
            <div class="stack-sm">
              <pre class="artifact-view">{{ data.reviewArtifact.content }}</pre>
              <label class="field field-full">
                <span class="field__label">コメント</span>
                <textarea
                  v-model="reviewArtifactComment"
                  class="field__control field__control--textarea"
                  rows="10"
                  placeholder="GitHub に返すレビューコメント"
                  spellcheck="false"
                />
              </label>
              <div class="button-row review-button-row">
                <button v-if="canApproveReview" class="button button-primary" type="button" :disabled="reviewApproveState === 'saving'" @click="sendReviewApproval">
                  {{ reviewApproveState === 'saving' ? '承認中...' : '承認' }}
                </button>
                <button class="button button-secondary" type="button" :disabled="reviewRerunState === 'saving'" @click="submitReviewArtifactRerun">
                  {{ reviewRerunState === 'saving' ? '再実行中...' : '再実行' }}
                </button>
                <button v-if="canSubmitReviewComment" class="button button-secondary review-button-row__submit" type="button" :disabled="reviewSubmitState === 'saving'" @click="sendReviewComment">
                  {{ reviewSubmitState === 'saving' ? 'レビューコメントを送信中...' : 'レビューコメントを送信' }}
                </button>
              </div>
              <p v-if="reviewRerunState === 'error'" class="notice notice-danger">{{ rerunErrorLabel('retry_review') }}: {{ reviewRerunError }}</p>
              <p v-if="reviewSubmitState === 'error'" class="notice notice-danger">{{ reviewSubmitError }}</p>
              <p v-if="reviewApproveState === 'error'" class="notice notice-danger">{{ reviewApproveError }}</p>
            </div>
          </div>
        </div>

        <div v-if="improvementModalOpen" class="modal-backdrop" @click.self="improvementModalOpen = false">
          <div class="modal-panel">
            <div class="modal-panel__header">
              <div>
                <h3 class="modal-panel__title">改善案生成</h3>
                <p class="text-muted">改善したい点を入力すると、改善点画面で扱う draft を自動生成します。</p>
              </div>
              <button class="button button-secondary" type="button" @click="improvementModalOpen = false">閉じる</button>
            </div>
            <div class="stack-sm">
              <label class="field field-full">
                <span class="field__label">コメント</span>
                <textarea
                  v-model="improvementComment"
                  class="field__control field__control--textarea"
                  rows="8"
                  placeholder="例: ボタンは左、説明文は右に配置したい"
                  spellcheck="false"
                />
              </label>
              <div class="modal-actions">
                <div class="modal-actions__right">
                  <button class="button button-primary" type="button" :disabled="improvementCreateState === 'saving' || !improvementComment.trim()" @click="createImprovementDraft">
                    {{ improvementCreateState === 'saving' ? '生成中...' : '改善案を生成' }}
                  </button>
                </div>
              </div>
              <p v-if="improvementCreateState === 'error'" class="notice notice-danger">{{ improvementCreateError }}</p>
            </div>
          </div>
        </div>
      </template>
    </AsyncState>
  </AppShell>
</template>
