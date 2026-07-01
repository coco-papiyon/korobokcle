const jobTimeFormatter = new Intl.DateTimeFormat('ja-JP', {
  timeZone: 'Asia/Tokyo',
  year: 'numeric',
  month: '2-digit',
  day: '2-digit',
  hour: '2-digit',
  minute: '2-digit',
  second: '2-digit',
  hour12: false,
})

function formatJobTimestamp(value?: string) {
  if (!value) {
    return '-'
  }
  const date = new Date(value)
  if (Number.isNaN(date.getTime()) || date.getUTCFullYear() <= 1) {
    return '-'
  }
  return jobTimeFormatter.format(date)
}

export function jobTimeSummary(fetchedAt?: string, updatedAt?: string) {
  return `取得時間 ${formatJobTimestamp(fetchedAt)} / 更新時間 ${formatJobTimestamp(updatedAt)}`
}

export function formatJobTimestampValue(value?: string) {
  return formatJobTimestamp(value)
}
