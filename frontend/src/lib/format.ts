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

function parseJsonPayload(payload: string): { value: unknown; isJson: boolean } {
  if (payload.trim() === '') {
    return { value: payload, isJson: false }
  }
  try {
    return { value: JSON.parse(payload) as unknown, isJson: true }
  } catch {
    return { value: payload, isJson: false }
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

export function formatPayloadPreview(payload: string, maxLength = 120): string {
  const parsed = parseJsonPayload(payload)
  const text = parsed.isJson ? JSON.stringify(parsed.value) ?? payload : compactPayloadText(payload)
  return truncatePayloadText(text, maxLength)
}

export function formatPayloadContent(payload: string): string {
  const parsed = parseJsonPayload(payload)
  if (!parsed.isJson) {
    return payload.trim() === '' ? emptyPayloadLabel : payload
  }
  try {
    return JSON.stringify(parsed.value, null, 2) ?? (payload.trim() === '' ? emptyPayloadLabel : payload)
  } catch {
    return payload.trim() === '' ? emptyPayloadLabel : payload
  }
}
