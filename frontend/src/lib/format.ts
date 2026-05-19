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
  if (value.startsWith('provider:')) {
    return `provider ${value.slice('provider:'.length)}`
  }
  if (value.startsWith('model:')) {
    return `model ${value.slice('model:'.length)}`
  }
  if (value === 'interrupted') {
    return 'interrupted'
  }
  return value.replaceAll('_', ' ')
}

const emptyPayloadLabel = '(empty)'

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
