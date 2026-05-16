<script setup lang="ts">
import { computed, onUnmounted, ref, watch } from 'vue'
import { useRoute } from 'vue-router'
import AppShell from '@/components/AppShell.vue'
import AsyncState from '@/components/AsyncState.vue'
import DataTable from '@/components/DataTable.vue'
import PanelCard from '@/components/PanelCard.vue'
import StateBadge from '@/components/StateBadge.vue'
import { useAsyncData } from '@/composables/useAsyncData'
import {
  fetchJobDetail,
  submitDesignApproval,
  submitDesignRerun,
  submitFinalApproval,
  submitImplementationRerun,
  submitPRRerun,
  submitReviewRerun,
} from '@/lib/api'
import { formatDateTime } from '@/lib/format'

const route = useRoute()
const jobID = computed(() => String(route.params.id))
const { data, isLoading, error, reload } = useAsyncData(() => fetchJobDetail(jobID.value))
let refreshTimer: number | null = null

function isPollingState(state?: string) {
  if (!state) {
    return false
  }
  return state === 'detected' || state.includes('running') || state === 'pr_creating'
}

function stopPolling() {
  if (refreshTimer !== null) {
    window.clearInterval(refreshTimer)
    refreshTimer = null
  }
}

function startPolling() {
  if (refreshTimer !== null) {
    return
  }
  refreshTimer = window.setInterval(() => {
    void reload()
  }, 5000)
}

watch(
  () => data.value?.job.state,
  (state) => {
    if (isPollingState(state)) {
      startPolling()
      return
    }
    stopPolling()
  },
  { immediate: true },
)

onUnmounted(() => {
  stopPolling()
})
const approvalState = ref<'idle' | 'saving' | 'error'>('idle')
const finalApprovalState = ref<'idle' | 'saving' | 'error'>('idle')
const approvalError = ref<string | null>(null)
const finalApprovalError = ref<string | null>(null)
const designRerunState = ref<'idle' | 'saving' | 'error'>('idle')
const implementationRerunState = ref<'idle' | 'saving' | 'error'>('idle')
const prRerunState = ref<'idle' | 'saving' | 'error'>('idle')
const reviewRerunState = ref<'idle' | 'saving' | 'error'>('idle')
const designRerunError = ref<string | null>(null)
const implementationRerunError = ref<string | null>(null)
const prRerunError = ref<string | null>(null)
const reviewRerunError = ref<string | null>(null)

type RerunAction = 'retry_design' | 'retry_implementation' | 'retry_pr' | 'retry_review'

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

const canReviewDesign = computed(() => data.value?.job.state === 'waiting_design_approval')
const canReviewImplementation = computed(() => {
  const state = data.value?.job.state
  if (state === 'waiting_final_approval') {
    return true
  }
  return state === 'failed' && latestEvent.value?.eventType === 'test_failed'
})
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

function actionButtonLabel(action: RerunAction, eventType: string, sourceEventType?: string) {
  if (action === 'retry_implementation' && (eventType === 'test_failed' || sourceEventType === 'test_failed')) {
    return 'Fix Implementation'
  }
  if (action === 'retry_design') {
    return 'Rerun Design'
  }
  if (action === 'retry_implementation') {
    return 'Rerun Implementation'
  }
  if (action === 'retry_review') {
    return 'Rerun Review'
  }
  return 'Retry PR'
}

function formatPayloadPreview(payload: string) {
  try {
    const parsed = JSON.parse(payload) as unknown
    if (parsed && typeof parsed === 'object' && !Array.isArray(parsed)) {
      const copy = { ...(parsed as Record<string, unknown>) }
      if ('body' in copy) {
        delete copy.body
      }
      return JSON.stringify(copy)
    }
    return JSON.stringify(parsed)
  } catch {
    return payload
  }
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
  if (name === 'ai-stdout.log') {
    return 'AI stdout'
  }
  if (name === 'ai-stderr.log') {
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

async function submitRerun(action: RerunAction, eventId: number) {
  rerunState(action).value = 'saving'
  rerunError(action).value = null
  try {
    if (action === 'retry_design') {
      data.value = await submitDesignRerun(jobID.value, '', eventId)
    } else if (action === 'retry_implementation') {
      data.value = await submitImplementationRerun(jobID.value, '', eventId)
    } else if (action === 'retry_review') {
      data.value = await submitReviewRerun(jobID.value, '', eventId)
    } else {
      data.value = await submitPRRerun(jobID.value, '', eventId)
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
</script>

<template>
  <AppShell
    title="Job Detail"
    description="ジョブ状態、関連ブランチ、イベント履歴を確認するページです。"
  >
    <AsyncState :is-loading="isLoading" :error="error">
      <template v-if="data">
        <section class="hero-grid">
          <PanelCard :title="data.job.id" description="Job summary">
            <div class="stack-sm">
              <StateBadge :state="data.job.state" />
              <p class="text-muted">{{ data.job.repository }} #{{ data.job.githubNumber }}</p>
              <p>{{ data.job.title }}</p>
              <p class="text-muted">Branch: <code>{{ data.job.branchName }}</code></p>
              <p class="text-muted">Watch Rule: <code>{{ data.job.watchRuleId }}</code></p>
            </div>
          </PanelCard>
          <PanelCard title="Flow" description="設計承認、実装成果物確認、最終承認をここから行えます。">
            <div class="stack-sm">
              <p class="text-muted">Current state: <code>{{ data.job.state }}</code></p>
              <template v-if="canReviewDesign">
                <div class="button-row">
                  <button class="button button-secondary" type="button" :disabled="approvalState === 'saving'" @click="sendApproval('rejected')">
                    Reject Design
                  </button>
                  <button class="button button-primary" type="button" :disabled="approvalState === 'saving'" @click="sendApproval('approved')">
                    Approve Design
                  </button>
                </div>
              </template>
              <p v-if="approvalState === 'error'" class="notice notice-danger">{{ approvalError }}</p>
              <template v-if="canReviewImplementation">
                <p v-if="finalApprovalWarning" class="notice notice-danger">{{ finalApprovalWarning }}</p>
                <div class="button-row">
                  <button class="button button-secondary" type="button" :disabled="finalApprovalState === 'saving'" @click="sendFinalApproval('rejected')">
                    Reject Final
                  </button>
                  <button class="button button-primary" type="button" :disabled="finalApprovalState === 'saving'" @click="sendFinalApproval('approved')">
                    Approve Final
                  </button>
                </div>
                <p v-if="finalApprovalState === 'error'" class="notice notice-danger">{{ finalApprovalError }}</p>
              </template>
            </div>
          </PanelCard>
        </section>

        <PanelCard title="Issue" description="元の issue 内容です。">
          <details class="stack-sm">
            <summary class="text-muted">Open issue body</summary>
            <pre class="artifact-view">{{ formatIssueBody(data.issueBody) }}</pre>
          </details>
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
          title="Implementation Artifact"
          description="実装フェーズの成果物サマリです。最終承認前に確認します。"
        >
          <div class="stack-sm">
            <details class="stack-sm">
              <summary class="text-muted">{{ data.implementationArtifact.path }}</summary>
              <pre class="artifact-view">{{ data.implementationArtifact.content }}</pre>
            </details>
          </div>
        </PanelCard>

        <PanelCard
          v-if="data.reviewArtifact"
          title="Review Artifact"
          description="PR review フェーズの成果物です。"
        >
          <div class="stack-sm">
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
          <tr v-for="event in data.events" :key="event.id">
            <td>{{ formatDateTime(event.createdAt) }}</td>
            <td>{{ event.eventType }}</td>
            <td>{{ event.stateTo || '-' }}</td>
            <td><pre class="artifact-view">{{ formatPayloadPreview(event.payload) }}</pre></td>
            <td>
              <div v-if="event.availableActions.length > 0" class="button-row">
                <button
                  v-if="event.availableActions.includes('retry_design')"
                  class="button button-secondary"
                  type="button"
                  :disabled="designRerunState === 'saving'"
                  @click="submitRerun('retry_design', event.id)"
                >
                  {{ actionButtonLabel('retry_design', event.eventType, event.sourceEventType) }}
                </button>
                <button
                  v-if="event.availableActions.includes('retry_implementation')"
                  class="button button-secondary"
                  type="button"
                  :disabled="implementationRerunState === 'saving'"
                  @click="submitRerun('retry_implementation', event.id)"
                >
                  {{ actionButtonLabel('retry_implementation', event.eventType, event.sourceEventType) }}
                </button>
                <button
                  v-if="event.availableActions.includes('retry_review')"
                  class="button button-secondary"
                  type="button"
                  :disabled="reviewRerunState === 'saving'"
                  @click="submitRerun('retry_review', event.id)"
                >
                  {{ actionButtonLabel('retry_review', event.eventType, event.sourceEventType) }}
                </button>
                <button
                  v-if="event.availableActions.includes('retry_pr')"
                  class="button button-secondary"
                  type="button"
                  :disabled="prRerunState === 'saving'"
                  @click="submitRerun('retry_pr', event.id)"
                >
                  {{ actionButtonLabel('retry_pr', event.eventType, event.sourceEventType) }}
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
