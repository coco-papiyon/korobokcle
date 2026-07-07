<script setup lang="ts">
import { computed, nextTick, ref, watch, onBeforeUnmount, onMounted } from 'vue'
import MarkdownIt from 'markdown-it'
import { html as diff2Html } from 'diff2html'
import 'diff2html/bundles/css/diff2html.min.css'
import type { Job, JobArtifact, JobDetailResponse, JobLogGroup, JobSourceDiff } from '../types'
import { jobStateChipClass, jobStateLabel as formatJobStateLabel } from '../utils/jobState'
import { formatJobTimestampValue } from '../utils/jobTime'

let chatComponentRegistered = false

async function registerChatComponent() {
  if (chatComponentRegistered || typeof window === 'undefined' || import.meta.env.MODE === 'test') {
    return
  }
  const module = await import('vue-advanced-chat')
  module.register()
  chatComponentRegistered = true
}

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
const artifactEditContent = ref('')
const sourceDiffLoading = ref(false)
const sourceDiffError = ref('')
const sourceDiff = ref<JobSourceDiff | null>(null)
const sourceDiffJobId = ref('')
const detailViewMode = ref<'chat' | 'detail' | 'diff' | 'logs' | 'edit'>('chat')
const artifactUserComment = ref('')
const chatDraftMessages = ref<ChatMessage[]>([])
const artifactActionLoading = ref(false)
const artifactEditSaving = ref(false)
const deleteLoading = ref(false)
const chatReady = ref(false)
const chatMessagesLoaded = ref(false)
const chatComponentReady = ref(false)
const chatRenderKey = ref(0)
const isTestMode = import.meta.env.MODE === 'test'
const detailScrollRef = ref<HTMLElement | null>(null)
const markdown = new MarkdownIt({
  html: true,
  breaks: true,
  linkify: true,
})
let detailRequestSequence = 0
let artifactRequestSequence = 0
let sourceDiffRequestSequence = 0
let detailRefreshTimer: number | undefined
let chatReadyTimer: number | undefined
let detailRevisionKey = ''

type ChatRoomUser = {
  _id: string
  username: string
  avatar: string
  status: {
    state: 'online' | 'offline'
    lastChanged: string
  }
}

type ChatRoom = {
  roomId: string
  roomName: string
  avatar: string
  users: ChatRoomUser[]
  lastMessage?: {
    content: string
    senderId: string
    timestamp?: string
  }
}

type ChatMessage = {
  _id: string
  senderId: string
  content: string
  username?: string
  date?: string
  timestamp?: string
  system?: boolean
  saved?: boolean
  distributed?: boolean
  seen?: boolean
  disableActions?: boolean
  disableReactions?: boolean
}
const emit = defineEmits<{
  (event: 'close'): void
  (event: 'refresh'): void
  (event: 'deleted', jobId: string): void
  (event: 'source-diff-availability', available: boolean): void
  (event: 'artifact-edit-availability', available: boolean): void
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
const issueContextMarkdown = computed(() =>
  issueContext.value.replace(/^#(\d+)\s+/m, '# $1 '),
)
const issueContextHtml = computed(() => markdown.render(issueContextMarkdown.value))
const detailSubStatus = computed(() =>
  detailJob.value?.kind === 'issue_implementation' ? detailJob.value?.subStatus?.trim() ?? '' : '',
)
const hasLogs = computed(() => detailLogs.value.length > 0)
const artifactHtml = computed(() => (artifact.value ? markdown.render(artifact.value.content) : ''))
const chatRoomId = computed(() => detailJob.value?.id ?? 'job-detail')
const chatUsers = computed<ChatRoomUser[]>(() => [
  {
    _id: 'user',
    username: 'User',
    avatar: '',
    status: { state: 'online', lastChanged: detailUpdatedAt.value || new Date(0).toISOString() },
  },
  {
    _id: 'assistant',
    username: 'AI',
    avatar: '',
    status: {
      state: detailJob.value?.state?.includes('running') ? 'online' : 'offline',
      lastChanged: detailUpdatedAt.value || new Date(0).toISOString(),
    },
  },
  {
    _id: 'system',
    username: 'System',
    avatar: '',
    status: { state: 'online', lastChanged: detailUpdatedAt.value || new Date(0).toISOString() },
  },
  {
    _id: 'tool',
    username: 'Tool',
    avatar: '',
    status: { state: 'offline', lastChanged: detailUpdatedAt.value || new Date(0).toISOString() },
  },
])
const chatRooms = computed<ChatRoom[]>(() => {
  const lastMessage = chatMessages.value.at(-1)
  return [
    {
      roomId: chatRoomId.value,
      roomName: detailTitle.value,
      avatar: '',
      users: chatUsers.value,
      lastMessage: lastMessage
        ? {
            content: lastMessage.content,
            senderId: lastMessage.senderId,
            timestamp: lastMessage.timestamp,
          }
        : undefined,
    },
  ]
})
const chatMessages = computed<ChatMessage[]>(() => {
  const job = detailJob.value
  if (!job || !chatReady.value) {
    return []
  }
  const messages: ChatMessage[] = []
  if (issueContextMarkdown.value.trim()) {
    messages.push(createChatMessage('context', 'user', issueContextMarkdown.value.trim()))
  }
  if (artifactLoading.value && artifact.value == null) {
    messages.push(createChatMessage('artifact-loading', 'system', `${artifactTitle(job)}を取得中です。`, true))
  }
  if (artifact.value) {
    messages.push(createChatMessage('artifact-title', 'system', artifactTitle(job), true))
    messages.push(createChatMessage('artifact', 'assistant', artifact.value.content))
  }
  if (sourceDiff.value) {
    messages.push(
      createChatMessage(
        'diff',
        'tool',
        `ソース差分を取得しました。\n\n対象: ${sourceDiff.value.path}${sourceDiff.value.baseRef ? `\n比較基準: ${sourceDiff.value.baseRef}` : ''}`,
      ),
    )
  }
  if (job.state === 'failed' && job.errorMessage) {
    messages.push(createChatMessage('failed', 'system', `エラー: ${job.errorMessage}`, true))
  }
  if (artifactError.value) {
    messages.push(createChatMessage('artifact-error', 'system', `結果取得エラー: ${artifactError.value}`, true))
  }
  return [
    ...messages,
    ...chatDraftMessages.value.filter((message) => message._id.startsWith(`${job.id}-draft-`)),
  ]
})
const chatRoomsJson = computed(() => JSON.stringify(chatRooms.value))
const chatMessagesJson = computed(() => JSON.stringify(chatMessages.value))
const chatMessageActionsJson = computed(() => JSON.stringify([]))
const chatTextMessagesJson = computed(() => JSON.stringify({
    ROOMS_EMPTY: 'ジョブが選択されていません',
    ROOM_EMPTY: '会話はまだありません',
    NEW_MESSAGES: '新しいメッセージ',
    MESSAGE_DELETED: 'このメッセージは削除されました',
    MESSAGES_EMPTY: '会話はまだありません',
    CONVERSATION_STARTED: '会話を開始しました',
    TYPE_MESSAGE: 'AIへの修正指示を入力',
    SEARCH: '検索',
    IS_ONLINE: 'オンライン',
    LAST_SEEN: '最終確認',
    IS_TYPING: '入力中',
    CANCEL_SELECT_MESSAGE: '選択を解除',
  }))
const chatStylesJson = computed(() => JSON.stringify({
    general: {
      color: '#122033',
      colorSpinner: '#2f5bea',
      borderStyle: 'none',
    },
    footer: {
      background: '#ffffff',
      backgroundReply: '#f8fafc',
    },
    message: {
      backgroundMe: '#dce9ff',
      background: '#f8fafc',
      color: '#122033',
      colorMe: '#10284f',
    },
  }))
const sourceDiffHtml = computed(() => {
  if (!sourceDiff.value) {
    return ''
  }
  return diff2Html(sourceDiff.value.content, {
    drawFileList: false,
    matching: 'lines',
    outputFormat: 'side-by-side',
    renderNothingWhenEmpty: true,
    synchronisedScroll: true,
  })
})

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

function canInspectSourceDiff(job: Job | null) {
  return (
    job != null &&
    (job.kind === 'issue_implementation' ||
      job.kind === 'pr_conflict' ||
      (job.kind === 'pr_feedback' && job.state.startsWith('review_fix_implementation_'))) &&
    inspectableStates.has(job.state)
  )
}

function canEditArtifact(job: Job | null) {
  if (!job || !canInspectArtifact(job)) {
    return false
  }
  if (job.kind === 'issue_design') {
    return job.state === 'design_ready'
  }
  return job.kind === 'pr_feedback' && job.state === 'review_fix_design_ready'
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

function sourceDiffTitle(job: Job | null) {
  if (job?.kind === 'pr_conflict') {
    return 'ソース差分'
  }
  return 'ソース差分'
}

function artifactEditorTitle(job: Job | null) {
  return `${artifactTitle(job)}の編集`
}

function logGroupTitle(group: JobLogGroup) {
  return `${group.roleLabel} / 試行 ${group.attempt}`
}

function createChatMessage(id: string, senderId: string, content: string, system = false): ChatMessage {
  return {
    _id: `${detailJob.value?.id ?? 'job'}-${id}`,
    senderId,
    content,
    username:
      senderId === 'assistant' ? 'AI' : senderId === 'tool' ? 'Tool' : senderId === 'system' ? 'System' : 'User',
    date: detailJob.value?.updatedAt ? formatJobTimestampValue(detailJob.value.updatedAt) : '',
    timestamp: detailJob.value?.updatedAt ? formatJobTimestampValue(detailJob.value.updatedAt) : '',
    system,
    saved: true,
    distributed: true,
    seen: true,
    disableActions: true,
    disableReactions: true,
  }
}

async function scrollDetailToTop() {
  await nextTick()
  detailScrollRef.value?.scrollTo({ top: 0, behavior: 'smooth' })
}

function appendChatInstruction(content: string) {
  const normalized = content.trim()
  if (!normalized || !detailJob.value) {
    return
  }
  chatDraftMessages.value = [
    ...chatDraftMessages.value,
    createChatMessage(`draft-${Date.now()}`, 'user', normalized),
    createChatMessage(
      `draft-response-${Date.now()}`,
      'assistant',
      'AIへの修正指示を受け付けました。変更結果はこの画面で更新されます。',
      true,
    ),
  ]
}

function handleChatSendMessage(event: Event) {
  const detail = (event as CustomEvent<{ content?: string }>).detail
  appendChatInstruction(detail?.content ?? '')
}

function clearChatReadyTimer() {
  if (chatReadyTimer === undefined) {
    return
  }
  window.clearTimeout(chatReadyTimer)
  chatReadyTimer = undefined
}

function refreshChatMessages(options: { remount?: boolean } = {}) {
  const remount = options.remount ?? true
  clearChatReadyTimer()
  chatReady.value = false
  chatMessagesLoaded.value = false
  if (remount) {
    chatRenderKey.value += 1
  }
  void nextTick(() => {
    if (detailViewMode.value !== 'chat' || !chatComponentReady.value) {
      return
    }
    chatReadyTimer = window.setTimeout(() => {
      chatReadyTimer = undefined
      if (detailViewMode.value !== 'chat' || !chatComponentReady.value) {
        return
      }
      chatReady.value = true
      void nextTick(() => {
        if (detailViewMode.value === 'chat') {
          chatMessagesLoaded.value = true
        }
      })
    }, 0)
  })
}

function handleChatFetchMessages() {
  chatReady.value = true
  chatMessagesLoaded.value = true
}

function jobRevisionKey(job: Job, branch: string) {
  return JSON.stringify({
    id: job.id,
    kind: job.kind,
    state: job.state,
    subStatus: job.subStatus ?? '',
    title: job.title,
    branch,
    issueContext: job.issueContext ?? '',
    errorMessage: job.errorMessage ?? '',
    failedFromState: job.failedFromState ?? '',
    fetchedAt: job.fetchedAt ?? '',
    updatedAt: job.updatedAt ?? '',
  })
}

async function loadJobDetail(id: string, options: { refreshArtifact?: boolean } = {}) {
  const refreshArtifact = options.refreshArtifact ?? true
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
    artifactEditContent.value = ''
    artifactEditSaving.value = false
    sourceDiffRequestSequence += 1
    sourceDiffLoading.value = false
    sourceDiffError.value = ''
    sourceDiff.value = null
    sourceDiffJobId.value = ''
    detailViewMode.value = 'chat'
    chatReady.value = false
    chatMessagesLoaded.value = false
    detailRevisionKey = ''
    chatDraftMessages.value = []
    emit('source-diff-availability', false)
    emit('artifact-edit-availability', false)
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
      const nextRevisionKey = jobRevisionKey(payload.job, branch)
      const isSameRevision =
        detailJob.value?.id === payload.job.id &&
        (payload.updatedAt === detailUpdatedAt.value || nextRevisionKey === detailRevisionKey)
      detailUpdatedAt.value = payload.updatedAt
      if (!isSameRevision) {
        detailRevisionKey = nextRevisionKey
        detailJob.value = payload.job
        detailBranch.value = branch
        artifactUserComment.value = ''
        artifactEditContent.value = ''
        if (detailViewMode.value === 'chat') {
          refreshChatMessages()
        }
      }
      if (refreshArtifact && canInspectArtifact(payload.job)) {
        void loadArtifact(payload.job.id)
      } else if (refreshArtifact) {
        artifactRequestSequence += 1
        artifactLoading.value = false
        artifactError.value = ''
        artifact.value = null
        artifactJobId.value = ''
      }
      if (!canInspectSourceDiff(payload.job)) {
        sourceDiffRequestSequence += 1
        sourceDiffLoading.value = false
        sourceDiffError.value = ''
        sourceDiff.value = null
        sourceDiffJobId.value = ''
        if (detailViewMode.value === 'diff') {
          detailViewMode.value = 'chat'
        }
      }
      if (!canEditArtifact(payload.job) && detailViewMode.value === 'edit') {
        detailViewMode.value = 'chat'
      }
      emit('source-diff-availability', canInspectSourceDiff(payload.job))
      emit('artifact-edit-availability', canEditArtifact(payload.job))
    }
  } catch (err) {
    if (requestSequence === detailRequestSequence) {
      if (detailViewMode.value === 'logs') {
        detailError.value = ''
        if (showLoading) {
          detailLoading.value = false
        }
        return
      }
      detailError.value = err instanceof Error ? err.message : 'unknown error'
      detailJob.value = null
      detailBranch.value = ''
      detailLogs.value = []
      artifactRequestSequence += 1
      artifactLoading.value = false
      artifactError.value = ''
      artifact.value = null
      artifactJobId.value = ''
      artifactEditContent.value = ''
      artifactEditSaving.value = false
      sourceDiffRequestSequence += 1
      sourceDiffLoading.value = false
      sourceDiffError.value = ''
      sourceDiff.value = null
      sourceDiffJobId.value = ''
      detailViewMode.value = 'chat'
      chatReady.value = false
      chatMessagesLoaded.value = false
      detailRevisionKey = ''
      emit('source-diff-availability', false)
      emit('artifact-edit-availability', false)
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
    if (!props.active || !props.jobId || detailViewMode.value === 'edit') {
      return
    }
    void loadJobDetail(props.jobId, {
      refreshArtifact: detailViewMode.value !== 'logs',
    })
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

async function loadSourceDiff(jobId: string) {
  if (!jobId) {
    return
  }
  const requestSequence = ++sourceDiffRequestSequence
  sourceDiffError.value = ''
  const hasCurrentDiff = sourceDiffJobId.value === jobId && sourceDiff.value !== null
  if (!hasCurrentDiff) {
    sourceDiffLoading.value = true
    sourceDiff.value = null
  }
  try {
    const res = await fetch(`/api/jobs/${encodeURIComponent(jobId)}/diff`, { cache: 'no-store' })
    if (!res.ok) {
      const message = await res.text()
      throw new Error(message || `HTTP ${res.status}`)
    }
    const payload = (await res.json()) as JobSourceDiff
    if (requestSequence === sourceDiffRequestSequence) {
      sourceDiff.value = payload
      sourceDiffJobId.value = jobId
    }
  } catch (err) {
    if (requestSequence === sourceDiffRequestSequence) {
      sourceDiffError.value = err instanceof Error ? err.message : 'unknown error'
    }
  } finally {
    if (requestSequence === sourceDiffRequestSequence) {
      sourceDiffLoading.value = false
    }
  }
}

async function openSourceDiff() {
  if (!detailJob.value) {
    return
  }
  detailViewMode.value = 'diff'
  await loadSourceDiff(detailJob.value.id)
}

async function openEditView() {
  if (!detailJob.value || !canEditArtifact(detailJob.value)) {
    return
  }
  detailViewMode.value = 'edit'
  if (artifact.value !== null && artifactJobId.value === detailJob.value.id) {
    artifactEditContent.value = artifact.value.content
    return
  }
  await loadArtifact(detailJob.value.id)
  artifactEditContent.value = artifact.value?.content ?? ''
}

function openResultView() {
  detailViewMode.value = 'detail'
}

function openChatView() {
  detailViewMode.value = 'chat'
  refreshChatMessages()
}

function openLogsView() {
  detailViewMode.value = 'logs'
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
    void scrollDetailToTop()
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
    void scrollDetailToTop()
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
    void scrollDetailToTop()
  } finally {
    artifactActionLoading.value = false
  }
}

async function saveArtifactEdit() {
  if (!detailJob.value) {
    return
  }
  artifactEditSaving.value = true
  artifactError.value = ''
  try {
    const res = await fetch(`/api/jobs/${encodeURIComponent(detailJob.value.id)}/artifact/content`, {
      method: 'PUT',
      headers: {
        'Content-Type': 'application/json',
      },
      body: JSON.stringify({ content: artifactEditContent.value }),
    })
    if (!res.ok) {
      const message = await res.text()
      throw new Error(message || `HTTP ${res.status}`)
    }
    const payload = (await res.json()) as JobArtifact
    artifact.value = payload
    artifactJobId.value = detailJob.value.id
    artifactEditContent.value = payload.content
    detailViewMode.value = 'chat'
    chatDraftMessages.value = [
      ...chatDraftMessages.value,
      createChatMessage(`edit-saved-${Date.now()}`, 'system', '編集内容を保存しました。', true),
    ]
    emit('refresh')
    refreshChatMessages({ remount: false })
  } catch (err) {
    artifactError.value = err instanceof Error ? err.message : 'unknown error'
    void scrollDetailToTop()
  } finally {
    artifactEditSaving.value = false
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
    void scrollDetailToTop()
  } finally {
    deleteLoading.value = false
  }
}

watch(
  () => [props.active, props.jobId, props.refreshKey] as const,
  ([active, jobId], previous) => {
    const [prevActive, prevJobId, prevRefreshKey] = previous ?? [undefined, undefined, undefined]
    if (!active) {
      stopPolling()
      return
    }
    if (!jobId) {
      stopPolling()
      void loadJobDetail(jobId)
      return
    }
    if (
      detailViewMode.value === 'edit' &&
      prevActive &&
      prevJobId === jobId &&
      prevRefreshKey !== props.refreshKey
    ) {
      return
    }
    if (detailViewMode.value === 'edit') {
      stopPolling()
      return
    }
    void loadJobDetail(jobId, {
      refreshArtifact: detailViewMode.value !== 'logs',
    })
    startPolling()
  },
  { immediate: true },
)

onMounted(() => {
  void (async () => {
    await registerChatComponent()
    chatComponentReady.value = true
    refreshChatMessages()
  })()
})

onBeforeUnmount(() => {
  clearChatReadyTimer()
  stopPolling()
})

defineExpose({
  openChatView,
  openResultView,
  openSourceDiff,
  openLogsView,
  openEditView,
  detailViewMode,
})
</script>

<template>
  <div>
    <div v-if="detailLoading" class="empty-state">読み込み中...</div>
    <div v-else-if="detailError" class="error">{{ detailError }}</div>
    <div v-else-if="detailJob" ref="detailScrollRef" class="detail" :class="{ 'detail--chat': detailViewMode === 'chat' }">
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

      <section v-if="detailViewMode === 'chat'" class="detail-chat" aria-label="AI実行チャット">
        <div class="detail-chat__body">
          <vue-advanced-chat
            v-if="chatComponentReady && !isTestMode"
            class="detail-chat__component"
            :key="`${chatRoomId}-${chatRenderKey}`"
            height="100%"
            current-user-id="user"
            :rooms="chatRoomsJson"
            :messages="chatMessagesJson"
            :room-id="chatRoomId"
            :rooms-loaded="true"
            :messages-loaded="chatMessagesLoaded"
            :single-room="true"
            :show-files="false"
            :show-audio="false"
            :show-emojis="false"
            :show-reaction-emojis="false"
            :show-add-room="false"
            :show-search="false"
            :show-new-messages-divider="false"
            :room-info-enabled="false"
            :message-actions="chatMessageActionsJson"
            :text-messages="chatTextMessagesJson"
            :styles="chatStylesJson"
            @fetch-messages="handleChatFetchMessages"
            @send-message="handleChatSendMessage"
          />
          <div v-else-if="isTestMode" class="detail-chat__fallback">
            <article
              v-for="message in chatMessages"
              :key="message._id"
              class="detail-chat__fallback-message"
              :class="{
                'detail-chat__fallback-message--system': message.system,
                'detail-chat__fallback-message--assistant': message.senderId === 'assistant',
                'detail-chat__fallback-message--user': message.senderId === 'user',
              }"
            >
              <strong>{{ message.username }}</strong>
              <pre>{{ message.content }}</pre>
            </article>
          </div>
          <div v-else class="empty-state">チャットを読み込み中...</div>
        </div>

        <div v-if="canInspectArtifact(detailJob)" class="detail-chat__actions">
          <label class="field detail-chat__comment">
            <span>ユーザコメント</span>
            <textarea
              v-model="artifactUserComment"
              class="control artifact-comment"
              rows="4"
              placeholder="承認や再実行に添えるコメントを入力"
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
          </div>
        </div>
      </section>

      <details v-if="detailViewMode === 'detail' && showIssueContext && issueContext" class="detail-context">
        <summary>Issue の内容</summary>
        <div class="markdown-body detail-context__body" v-html="issueContextHtml"></div>
      </details>

      <section v-if="detailViewMode === 'diff'" class="detail-diff">
        <div class="panel__title-row">
          <h3>{{ sourceDiffTitle(detailJob) }}</h3>
          <span class="panel__hint">GET /api/jobs/:id/diff</span>
        </div>

        <div class="detail-diff__viewer">
          <div v-if="sourceDiffLoading && !sourceDiff" class="empty-state">読み込み中...</div>
          <div v-else-if="sourceDiffError" class="error">{{ sourceDiffError }}</div>
          <div v-else-if="sourceDiff" class="detail-diff__diff" v-html="sourceDiffHtml"></div>
          <p v-if="sourceDiff" class="detail-diff__meta">
            <span v-if="sourceDiff.baseRef">比較基準: {{ sourceDiff.baseRef }}</span>
            <span>対象: {{ sourceDiff.path }}</span>
          </p>
        </div>
      </section>

      <div v-if="detailViewMode === 'detail' && canInspectArtifact(detailJob)" class="detail-artifact">
        <div class="panel__title-row">
          <h3>{{ artifactTitle(detailJob) }}</h3>
          <span class="panel__hint">GET /api/jobs/:id/artifact</span>
        </div>

        <div v-if="artifactLoading && !artifact" class="empty-state">読み込み中...</div>
        <div v-if="artifactError" class="error">{{ artifactError }}</div>
        <div v-if="artifact">
          <div class="artifact-view markdown-body" v-html="artifactHtml"></div>

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

      <div v-if="detailViewMode === 'edit'" class="detail-artifact detail-artifact--edit">
        <div class="panel__title-row">
          <h3>{{ artifactEditorTitle(detailJob) }}</h3>
          <span class="panel__hint">PUT /api/jobs/:id/artifact/content</span>
        </div>

        <div v-if="artifactLoading && !artifact" class="empty-state">読み込み中...</div>
        <div v-if="artifactError" class="error">{{ artifactError }}</div>
        <div v-if="artifact">
          <textarea
            v-model="artifactEditContent"
            class="control artifact-comment detail-artifact__editor"
            rows="16"
            spellcheck="false"
          ></textarea>

          <div class="modal__actions">
            <div class="modal__actions-group">
              <button class="button" type="button" @click="saveArtifactEdit" :disabled="artifactEditSaving">
                保存
              </button>
              <button class="button button--ghost" type="button" @click="openChatView" :disabled="artifactEditSaving">
                チャットへ戻る
              </button>
            </div>
          </div>
        </div>
      </div>

      <section v-if="detailViewMode === 'logs'" class="detail-logs" aria-label="ログ">
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

      <section v-if="detailViewMode === 'detail' && relatedLink" class="detail-links" aria-label="関連リンク">
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
