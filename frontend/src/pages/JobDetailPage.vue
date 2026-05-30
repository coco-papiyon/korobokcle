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
  fetchToolCommands,
  purgeJob,
  restoreJob,
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
import { formatDateTime, formatPayloadDisplay } from '@/lib/format'
import { rerunActionFromEvent, rerunButtonLabel, type RerunAction } from '@/lib/rerun-actions'
import type { JobEvent, JobLog } from '@/types'
import { computed, onUnmounted, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'

const route = useRoute()
const router = useRouter()
const jobID = computed(() => String(route.params.id))
const { data: appConfig } = useAsyncData(fetchAppConfig)
const { data: toolCommands } = useAsyncData(fetchToolCommands)
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
const reviewArtifactModalOpen = ref(false)
const issueBodyModalOpen = ref(false)
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
const toolCommandState = ref<'idle' | 'saving' | 'error'>('idle')
const toolCommandAction = ref<'start' | 'stop' | null>(null)
const toolCommandError = ref<string | null>(null)
const selectedToolCommandName = ref('')
const popupLoadingState = ref<'idle' | 'loading' | 'error'>('idle')
const popupLoadingError = ref<string | null>(null)
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
      title?: string
      [key: string]: unknown
    }
  } catch {
    return null
  }
})
const prCreateRawContent = computed(() => data.value?.prCreateArtifact?.content ?? '')

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
const flowRerunActionInFlow = computed<RerunAction | null>(() => {
  return flowRerunAction.value === 'retry_pr' ? flowRerunAction.value : null
})
const finalApprovalWarning = computed(() => {
  if (data.value?.job.state === 'failed' && latestEvent.value?.eventType === 'test_failed') {
    return 'Tests failed, but you can still approve and continue to PR creation.'
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
const hasIssueBody = computed(() => (data.value?.issueBody?.trim().length ?? 0) > 0)

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
    return `${tool.resident ? 'resident' : 'one-shot'} command`
  }

  const parts: string[] = []
  parts.push(tool.resident ? 'resident' : 'one-shot')
  parts.push(execution.running ? 'running' : 'stopped')
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
    rerunError(action).value = err instanceof Error ? err.message : 'Unknown error'
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
    designRerunError.value = err instanceof Error ? err.message : 'Unknown error'
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
    implementationRerunError.value = err instanceof Error ? err.message : 'Unknown error'
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
    implementationRerunError.value = err instanceof Error ? err.message : 'Unknown error'
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
    reviewRerunError.value = err instanceof Error ? err.message : 'Unknown error'
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
    approvalError.value = err instanceof Error ? err.message : 'Unknown error'
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
    finalApprovalError.value = err instanceof Error ? err.message : 'Unknown error'
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
    reviewSubmitError.value = err instanceof Error ? err.message : 'Unknown error'
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
    toolCommandError.value = err instanceof Error ? err.message : 'Unknown error'
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
    toolCommandError.value = err instanceof Error ? err.message : 'Unknown error'
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
    popupLoadingError.value = err instanceof Error ? err.message : 'Unknown error'
  }
}

function openIssueBodyModal() {
  void openPopup((value) => {
    issueBodyModalOpen.value = value
  })
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
              <p class="text-muted">{{ data.job.repository }} #{{ data.job.githubNumber }}</p>
              <p class="text-muted">Branch: <code>{{ data.job.branchName }}</code></p>
              <p class="text-muted">Watch Rule: <code>{{ data.job.watchRuleId }}</code></p>
            </div>
          </PanelCard>
          <PanelCard title="Flow">
            <div class="stack-sm">
              <StateBadge :state="data.job.state" />
              <template v-if="!isDeletedJob && flowRerunActionInFlow">
                <div class="button-row">
                  <button
                    v-if="flowRerunActionInFlow"
                    class="button button-secondary"
                    type="button"
                    :disabled="rerunState(flowRerunActionInFlow) === 'saving'"
                    @click="submitRerun(flowRerunActionInFlow, flowRerunEvent?.id)"
                  >
                    {{ rerunButtonLabel(flowRerunActionInFlow, flowRerunEvent?.eventType, flowRerunEvent?.sourceEventType) }}
                  </button>
                </div>
              </template>
              <template v-if="!isDeletedJob">
                <div class="stack-sm">
                  <p class="text-muted">
                    Tool Default: <code>{{ configuredToolCommand?.name ?? '-' }}</code>
                    <span v-if="toolExecution">
                      / Running: <code>{{ toolExecution.name }}</code>
                    </span>
                    / {{ formatToolExecutionSummary() }}
                  </p>
                  <label class="field field-full">
                    <span class="field__label">Tool Commands</span>
                    <select
                      v-model="selectedToolCommandName"
                      class="field__control"
                      :disabled="toolExecution?.running || toolCommandState === 'saving' || availableToolCommands.length === 0"
                    >
                      <option value="" disabled>Tool Commands を選択</option>
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
            <button class="log-entry-button" type="button" @click="openIssueBodyModal">Issue body を開く</button>
            <p class="text-muted">元の issue 内容です。</p>
          </div>
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
        >
          <div class="artifact-headline">
            <button class="log-entry-button" type="button" @click="openDesignArtifactModal">設計結果を開く</button>
            <p class="text-muted">生成された設計成果物です。承認前に内容を確認します。</p>
          </div>
        </PanelCard>

        <PanelCard
          v-if="data.implementationArtifact"
          :title="isPRFeedbackJob ? '修正結果' : 'Implementation Artifact'"
        >
          <div class="artifact-headline">
            <button class="log-entry-button" type="button" @click="openImplementationArtifactModal">{{ isPRFeedbackJob ? '修正結果を開く' : '実装結果を開く' }}</button>
            <p class="text-muted">{{ isPRFeedbackJob ? 'PR review コメントに対する修正結果です。' : '実装フェーズの成果物サマリです。最終承認前に確認します。' }}</p>
          </div>
        </PanelCard>

        <PanelCard
          v-if="data.fixArtifact"
          :title="isPRFeedbackJob ? '追加修正結果' : 'Fix Artifact'"
          :description="isPRFeedbackJob ? '再実行やテスト失敗後の追加修正結果です。' : 'test_failed 後の修正フェーズで生成された成果物です。'"
        >
          <details class="stack-sm">
            <summary class="text-muted">{{ data.fixArtifact.path }}</summary>
            <pre class="artifact-view">{{ data.fixArtifact.content }}</pre>
          </details>
        </PanelCard>

        <PanelCard
          v-if="data.reviewArtifact"
          title="Review Artifact"
        >
          <div class="artifact-headline">
            <button class="log-entry-button" type="button" @click="openReviewArtifactModal">レビュー結果を開く</button>
            <p class="text-muted">PR review フェーズの成果物です。総評コメントとして GitHub へ返せます。</p>
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
          title="Test Report"
        >
          <div class="artifact-headline">
            <button class="log-entry-button" type="button" @click="openTestReportModal">テスト結果を開く</button>
            <p class="text-muted">設定された test profile の実行結果です。</p>
          </div>
        </PanelCard>

        <PanelCard
          v-if="toolExecution || toolStdout || toolStderr"
          title="Tool Log"
        >
          <div class="artifact-headline">
            <button class="log-entry-button" type="button" @click="openToolLogModal">Tool log を開く</button>
            <p class="text-muted">
              <code>{{ toolExecution?.name ?? selectedToolCommand?.name ?? configuredToolCommand?.name ?? '-' }}</code>
              / {{ formatToolExecutionSummary() }}
            </p>
          </div>
        </PanelCard>

        <PanelCard
          v-if="data.prCreateArtifact"
          title="Pull Request"
        >
          <div class="artifact-headline">
            <button class="log-entry-button" type="button" @click="openPRCreateModal">PR作成結果を開く</button>
            <p class="text-muted">作成された Pull Request の情報を確認できます。</p>
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

        <DataTable :columns="['When', 'Event', 'State', 'Payload']">
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
          </tr>
          <tr v-if="data.events.length === 0">
            <td colspan="4" class="text-muted">イベントはまだありません。</td>
          </tr>
        </DataTable>

        <div v-if="designArtifactModalOpen && data.designArtifact" class="modal-backdrop" @click.self="designArtifactModalOpen = false">
          <div class="modal-panel">
            <div class="modal-panel__header">
              <div>
                <h3 class="modal-panel__title">Design Artifact</h3>
                <p class="text-muted"><code>{{ data.designArtifact.path }}</code></p>
              </div>
              <button class="button button-secondary" type="button" @click="designArtifactModalOpen = false">閉じる</button>
            </div>
            <div class="stack-sm">
              <pre class="artifact-view">{{ data.designArtifact.content }}</pre>
              <label class="field field-full">
                <span class="field__label">Comment</span>
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
            <pre class="artifact-view">{{ formatIssueBody(data.issueBody) }}</pre>
          </div>
        </div>

        <div v-if="implementationArtifactModalOpen && data.implementationArtifact" class="modal-backdrop" @click.self="implementationArtifactModalOpen = false">
          <div class="modal-panel">
            <div class="modal-panel__header">
              <div>
                <h3 class="modal-panel__title">{{ isPRFeedbackJob ? '修正結果' : 'Implementation Artifact' }}</h3>
                <p class="text-muted"><code>{{ data.implementationArtifact.path }}</code></p>
              </div>
              <button class="button button-secondary" type="button" @click="implementationArtifactModalOpen = false">閉じる</button>
            </div>
            <div class="stack-sm">
              <pre class="artifact-view">{{ data.implementationArtifact.content }}</pre>
              <label class="field field-full">
                <span class="field__label">Comment</span>
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
                <h3 class="modal-panel__title">Tool Log</h3>
                <p class="text-muted">
                  <code>{{ toolExecution?.name ?? selectedToolCommand?.name ?? configuredToolCommand?.name ?? '-' }}</code>
                  / {{ formatToolExecutionSummary() }}
                </p>
                <p v-if="toolExecution?.startedAt" class="text-muted">
                  Started: {{ formatDateTime(toolExecution.startedAt) }}
                  <span v-if="toolExecution.finishedAt"> / Finished: {{ formatDateTime(toolExecution.finishedAt) }}</span>
                </p>
              </div>
              <button class="button button-secondary" type="button" @click="toolLogModalOpen = false">閉じる</button>
            </div>
            <div class="stack-sm">
              <section class="stack-sm">
                <h4>Stdout</h4>
                <p v-if="toolExecution?.stdout?.path" class="text-muted"><code>{{ toolExecution.stdout.path }}</code></p>
                <pre class="artifact-view">{{ toolStdoutContent || 'No stdout log yet.' }}</pre>
              </section>
              <section class="stack-sm">
                <h4>Stderr</h4>
                <p v-if="toolExecution?.stderr?.path" class="text-muted"><code>{{ toolExecution.stderr.path }}</code></p>
                <pre class="artifact-view">{{ toolStderrContent || 'No stderr log yet.' }}</pre>
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
                <h3 class="modal-panel__title">Test Report</h3>
                <p class="text-muted"><code>{{ data.testReport.path }}</code></p>
              </div>
              <button class="button button-secondary" type="button" @click="testReportModalOpen = false">閉じる</button>
            </div>
            <div class="stack-sm">
              <pre class="artifact-view">{{ testReportMarkdown }}</pre>
              <label class="field field-full">
                <span class="field__label">Comment</span>
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
                <h3 class="modal-panel__title">Pull Request</h3>
                <p class="text-muted"><code>{{ data.prCreateArtifact.path }}</code></p>
              </div>
              <button class="button button-secondary" type="button" @click="prCreateModalOpen = false">閉じる</button>
            </div>
            <div class="stack-sm">
              <template v-if="prCreateInfo">
                <p v-if="prCreateInfo.title"><strong>{{ prCreateInfo.title }}</strong></p>
                <p v-if="prCreateInfo.repository" class="text-muted">Repository: <code>{{ prCreateInfo.repository }}</code></p>
                <p v-if="prCreateInfo.branchName" class="text-muted">Branch: <code>{{ prCreateInfo.branchName }}</code></p>
                <p v-if="prCreateInfo.url">
                  <a class="table-link" :href="prCreateInfo.url" target="_blank" rel="noreferrer">Open Pull Request</a>
                </p>
              </template>
              <p v-else class="text-muted">PR 作成結果を構造化表示できなかったため、生データを表示しています。</p>
              <pre class="artifact-view">{{ prCreateRawContent }}</pre>
            </div>
          </div>
        </div>

        <div v-if="reviewArtifactModalOpen && data.reviewArtifact" class="modal-backdrop" @click.self="reviewArtifactModalOpen = false">
          <div class="modal-panel">
            <div class="modal-panel__header">
              <div>
                <h3 class="modal-panel__title">Review Artifact</h3>
                <p class="text-muted"><code>{{ data.reviewArtifact.path }}</code></p>
              </div>
              <button class="button button-secondary" type="button" @click="reviewArtifactModalOpen = false">閉じる</button>
            </div>
            <div class="stack-sm">
              <pre class="artifact-view">{{ data.reviewArtifact.content }}</pre>
              <label class="field field-full">
                <span class="field__label">Comment</span>
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
      </template>
    </AsyncState>
  </AppShell>
</template>
