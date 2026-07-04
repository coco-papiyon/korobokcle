export const jobStateDefinitions = [
  { state: 'detected', label: '検知済み' },
  { state: 'design_running', label: '設計中' },
  { state: 'design_ready', label: '設計完了' },
  { state: 'design_approved', label: '設計承認済み' },
  { state: 'completed', label: '完了' },
  { state: 'implementation_running', label: '実装中' },
  { state: 'implementation_ready', label: '実装完了' },
  { state: 'implementation_approved', label: '実装承認済み' },
  { state: 'pr_created', label: 'PR済み' },
  { state: 'pr_review_comment', label: 'レビュー指摘あり' },
  { state: 'pr_conflict', label: 'コンフリクト検知済み' },
  { state: 'pr_conflict_running', label: 'コンフリクト解消中' },
  { state: 'pr_conflict_ready', label: 'コンフリクト解消完了' },
  { state: 'pr_conflict_resolved', label: 'コンフリクト解消済み' },
  { state: 'review_fix_design_running', label: 'レビュー指摘検討中' },
  { state: 'review_fix_design_ready', label: 'レビュー指摘検討済み' },
  { state: 'review_fix_design_approved', label: 'レビュー検討承認済み' },
  { state: 'review_fix_implementation_running', label: 'レビュー指摘修正中' },
  { state: 'review_fix_implementation_ready', label: 'レビュー指摘修正完了' },
  { state: 'review_fix_implementation_approved', label: 'レビュー指摘修正承認済み' },
  { state: 'review_fixed', label: 'レビュー指摘修正済み' },
  { state: 'review_running', label: 'レビュー中' },
  { state: 'review_ready', label: 'レビュー完了' },
  { state: 'review_approved', label: 'レビュー承認済み' },
  { state: 'failed', label: '失敗' },
] as const

export const jobStateLabels: Record<string, string> = Object.fromEntries(
  jobStateDefinitions.map(({ state, label }) => [state, label]),
)

const runningStates = new Set([
  'design_running',
  'implementation_running',
  'review_running',
  'review_fix_design_running',
  'review_fix_implementation_running',
  'pr_conflict_running',
])

const waitingStates = new Set([
  'design_ready',
  'implementation_ready',
  'review_ready',
  'review_fix_design_ready',
  'review_fix_implementation_ready',
  'pr_conflict_ready',
])

const approvedStates = new Set([
  'design_approved',
  'implementation_approved',
  'review_approved',
  'review_fix_design_approved',
  'review_fix_implementation_approved',
  'pr_conflict_resolved',
])

export type JobStateFilterValue = 'all' | 'unfinished' | 'running' | 'waiting' | 'approved' | 'completed' | 'failed' | 'other'

export const jobStateFilterDefinitions = [
  { value: 'all', label: 'すべて' },
  { value: 'unfinished', label: '未完了' },
  { value: 'running', label: '実行中' },
  { value: 'waiting', label: '承認待ち' },
  { value: 'approved', label: '承認済み' },
  { value: 'completed', label: '完了' },
  { value: 'failed', label: '失敗' },
  { value: 'other', label: 'その他' },
] as const

export function jobStateChipClass(state: string) {
  if (state === 'failed') {
    return 'chip chip--failed'
  }
  if (state === 'review_approved') {
    return 'chip chip--approved'
  }
  return runningStates.has(state) ? 'chip chip--running' : 'chip'
}

export function jobStateLabel(state: string) {
  return jobStateLabels[state] ?? state
}

export function jobStateMatchesFilter(state: string, filter: JobStateFilterValue) {
  switch (filter) {
    case 'all':
      return true
    case 'unfinished':
      return state !== 'completed'
    case 'running':
      return runningStates.has(state)
    case 'waiting':
      return waitingStates.has(state)
    case 'approved':
      return approvedStates.has(state)
    case 'completed':
      return state === 'completed'
    case 'failed':
      return state === 'failed'
    case 'other':
      return (
        state !== 'completed' &&
        state !== 'failed' &&
        !runningStates.has(state) &&
        !waitingStates.has(state) &&
        !approvedStates.has(state)
      )
  }

  return false
}
