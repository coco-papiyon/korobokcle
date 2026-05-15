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
  return value.replaceAll('_', ' ')
}
