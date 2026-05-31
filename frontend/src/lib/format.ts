import { EMPTY_ISSUE_BODY_MESSAGE, EMPTY_PAYLOAD_LABEL, EVENT_TYPE_LABELS, JOB_TYPE_LABELS, TOOL_EXECUTION_STATUS_LABELS } from '@/lib/ui-text'

export function formatDateTime(value: string): string {
  return new Intl.DateTimeFormat('ja-JP', {
    year: 'numeric',
    month: '2-digit',
    day: '2-digit',
    hour: '2-digit',
    minute: '2-digit',
    second: '2-digit',
  }).format(new Date(value))
}

export function formatStateLabel(value: string): string {
  const translated = {
    enabled: '有効',
    disabled: '無効',
    detected: '検知済み',
    running: '実行中',
    stopped: '停止中',
    completed: '完了',
    waiting: '待機中',
    ready: '完了',
    failed: '失敗',
    rejected: '却下',
    interrupted: '中断',
    waiting_design_approval: '設計承認待ち',
    waiting_final_approval: '最終承認待ち',
    design_ready: '設計完了',
    implementation_ready: '実装完了',
    review_ready: 'レビュー完了',
    pr_creating: 'PR 作成中',
    pr_created: 'PR 作成完了',
    pr_updated: 'PR 更新完了',
    review_completed: 'レビュー完了',
  } as const

  if (value in translated) {
    return translated[value as keyof typeof translated]
  }
  if (value.startsWith('provider:')) {
    return `プロバイダー ${value.slice('provider:'.length)}`
  }
  if (value.startsWith('model:')) {
    return `モデル ${value.slice('model:'.length)}`
  }
  return value.replaceAll('_', ' ')
}

export function formatJobTypeLabel(value: string): string {
  return JOB_TYPE_LABELS[value] ?? value.replaceAll('_', ' ')
}

export function formatEventTypeLabel(value: string): string {
  return EVENT_TYPE_LABELS[value] ?? value.replaceAll('_', ' ')
}

export function formatToolExecutionStatusLabel(value: string): string {
  return TOOL_EXECUTION_STATUS_LABELS[value] ?? value.replaceAll('_', ' ')
}

const emptyPayloadLabel = EMPTY_PAYLOAD_LABEL

type ParsedPayload =
  | {
      kind: 'empty'
    }
  | {
      kind: 'json'
      value: unknown
    }
  | {
      kind: 'raw'
      value: string
    }

export type PayloadDisplay = {
  preview: string
  content: string
}

function parseJsonPayload(payload: string): ParsedPayload {
  if (payload.trim() === '') {
    return { kind: 'empty' }
  }
  try {
    return { kind: 'json', value: JSON.parse(payload) as unknown }
  } catch {
    return { kind: 'raw', value: payload }
  }
}

function compactPayloadText(payload: string): string {
  const normalized = payload.replace(/\s+/g, ' ').trim()
  if (normalized === '') {
    return emptyPayloadLabel
  }
  return normalized
}

function truncatePayloadText(payload: string, maxLength = 120): string {
  if (payload.length <= maxLength) {
    return payload
  }
  return `${payload.slice(0, Math.max(0, maxLength - 1)).trimEnd()}…`
}

export function formatPayloadDisplay(payload: string, maxLength = 120): PayloadDisplay {
  const parsed = parseJsonPayload(payload)

  if (parsed.kind === 'empty') {
    return {
      preview: emptyPayloadLabel,
      content: emptyPayloadLabel,
    }
  }

  if (parsed.kind === 'json') {
    const preview = JSON.stringify(parsed.value) ?? emptyPayloadLabel
    const content = JSON.stringify(parsed.value, null, 2) ?? emptyPayloadLabel
    return {
      preview: truncatePayloadText(preview, maxLength),
      content,
    }
  }

  const preview = compactPayloadText(parsed.value)
  return {
    preview: truncatePayloadText(preview, maxLength),
    content: parsed.value,
  }
}

export function formatPayloadPreview(payload: string, maxLength = 120): string {
  return formatPayloadDisplay(payload, maxLength).preview
}

export function formatPayloadContent(payload: string): string {
  return formatPayloadDisplay(payload).content
}

export function formatIssueBody(value?: string): string {
  return value && value.trim().length > 0 ? value : EMPTY_ISSUE_BODY_MESSAGE
}

export function formatLogName(name: string): string {
  if (name === 'stdout.log') {
    return '標準出力'
  }
  if (name === 'stderr.log') {
    return '標準エラー'
  }
  if (name === 'git-push.log') {
    return 'git push'
  }
  if (name === 'gh-pr-create.log') {
    return 'gh pr create'
  }
  if (name === 'gh-pr-comment.log') {
    return 'gh pr comment'
  }
  return name
}
