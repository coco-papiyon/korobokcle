import { describe, expect, it } from 'vitest'

import {
  DEFAULT_MODEL_LABEL,
  EMPTY_ISSUE_BODY_MESSAGE,
  EMPTY_PAYLOAD_LABEL,
  ERROR_MESSAGE,
  JOB_TYPE_LABELS,
  LOADING_MESSAGE,
  NOTIFICATION_CHANNEL_LABELS,
  PROVIDER_USE_SETTING_LABEL,
  TOOL_EXECUTION_STATUS_LABELS,
  notificationChannelDisplayName,
  requestFailedMessage,
  UNKNOWN_ERROR_MESSAGE,
} from './ui-text'

describe('ui text constants', () => {
  it('exposes message strings and label maps', () => {
    expect(UNKNOWN_ERROR_MESSAGE).toBe('不明なエラーです。')
    expect(requestFailedMessage(500)).toBe('リクエストに失敗しました: 500')
    expect(EMPTY_ISSUE_BODY_MESSAGE).toBe('Issue 本文は空です。')
    expect(EMPTY_PAYLOAD_LABEL).toBe('（空）')
    expect(LOADING_MESSAGE).toBe('読み込み中')
    expect(ERROR_MESSAGE).toBe('エラー')
    expect(PROVIDER_USE_SETTING_LABEL).toBe('設定を使用')
    expect(DEFAULT_MODEL_LABEL).toBe('既定')
    expect(NOTIFICATION_CHANNEL_LABELS.windows_toast).toBe('Windowsデスクトップ通知')
    expect(JOB_TYPE_LABELS.issue).toBe('Issue')
    expect(TOOL_EXECUTION_STATUS_LABELS.running).toBe('実行中')
    expect(notificationChannelDisplayName('windows_toast')).toBe('Windowsデスクトップ通知')
    expect(notificationChannelDisplayName('custom')).toBe('custom')
  })
})
