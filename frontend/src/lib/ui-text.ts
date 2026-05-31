export const UNKNOWN_ERROR_MESSAGE = '不明なエラーです。'

export function requestFailedMessage(status: number): string {
  return `リクエストに失敗しました: ${status}`
}

export const EMPTY_ISSUE_BODY_MESSAGE = 'Issue 本文は空です。'
export const EMPTY_PAYLOAD_LABEL = '（空）'

export const LOADING_MESSAGE = '読み込み中'
export const ERROR_MESSAGE = 'エラー'

export const PROVIDER_USE_SETTING_LABEL = '設定を使用'
export const DEFAULT_MODEL_LABEL = '既定'

export const NOTIFICATION_CHANNEL_LABELS: Record<string, string> = {
  windows_toast: 'Windowsデスクトップ通知',
}

export function notificationChannelDisplayName(type: string): string {
  return NOTIFICATION_CHANNEL_LABELS[type] ?? type
}

export const JOB_TYPE_LABELS: Record<string, string> = {
  issue: 'Issue',
  pr_review: 'PR レビュー',
  pr_feedback: 'PR フィードバック',
}

export const EVENT_TYPE_LABELS: Record<string, string> = {
  issue_matched: 'Issue 検知',
  pull_request_review_matched: 'PR レビュー検知',
  design_started: '設計開始',
  design_ready: '設計完了',
  waiting_design_approval: '設計承認待ち',
  design_approved: '設計承認',
  design_rejected: '設計差し戻し',
  design_failed: '設計失敗',
  design_interrupted: '設計中断',
  design_rerun_requested: '設計再実行',
  implementation_started: '実装開始',
  implementation_ready: '実装完了',
  waiting_final_approval: '最終承認待ち',
  final_approved: '最終承認',
  final_rejected: '最終差し戻し',
  implementation_failed: '実装失敗',
  implementation_interrupted: '実装中断',
  implementation_rerun_requested: '実装再実行',
  test_failed: 'テスト失敗',
  test_interrupted: 'テスト中断',
  review_started: 'レビュー開始',
  review_ready: 'レビュー完了',
  review_completed: 'レビュー完了',
  review_failed: 'レビュー失敗',
  review_interrupted: 'レビュー中断',
  review_rerun_requested: 'レビュー再実行',
  pr_creating_started: 'PR 作成開始',
  pr_creating: 'PR 作成中',
  pr_created: 'PR 作成完了',
  pr_updated: 'PR 更新完了',
  pr_create_failed: 'PR 作成失敗',
  pr_push_failed: 'PR 反映失敗',
  pr_comment_failed: 'PR コメント失敗',
  pr_interrupted: 'PR 中断',
  pr_rerun_requested: 'PR 再実行',
  detected: '検知済み',
  interrupted: '中断',
  completed: '完了',
  failed: '失敗',
}

export const TOOL_EXECUTION_STATUS_LABELS: Record<string, string> = {
  resident: '常駐',
  'one-shot': '単発',
  running: '実行中',
  stopped: '停止中',
}
