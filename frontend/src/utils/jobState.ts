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
])

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
