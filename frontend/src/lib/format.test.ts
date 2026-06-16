import { describe, expect, it } from 'vitest'

import {
  formatDateTime,
  formatEventTypeLabel,
  formatIssueBody,
  formatJobTypeLabel,
  formatLogName,
  formatPayloadContent,
  formatPayloadDisplay,
  formatPayloadPreview,
  formatStateLabel,
  formatToolExecutionStatusLabel,
} from './format'
import { EMPTY_ISSUE_BODY_MESSAGE, EMPTY_PAYLOAD_LABEL } from '@/lib/ui-text'

describe('format helpers', () => {
  it('formats labels and fallback text', () => {
    expect(formatStateLabel('waiting_design_approval')).toBe('設計承認待ち')
    expect(formatStateLabel('rejected')).toBe('却下')
    expect(formatStateLabel('provider:copilot')).toBe('プロバイダー copilot')
    expect(formatStateLabel('model:gpt-4.1')).toBe('モデル gpt-4.1')
    expect(formatStateLabel('custom_state_value')).toBe('custom state value')

    expect(formatJobTypeLabel('pr_review')).toBe('PR レビュー')
    expect(formatJobTypeLabel('custom_job')).toBe('custom job')
    expect(formatEventTypeLabel('pr_comment_failed')).toBe('PR コメント失敗')
    expect(formatEventTypeLabel('custom_event')).toBe('custom event')
    expect(formatToolExecutionStatusLabel('one-shot')).toBe('単発')
    expect(formatToolExecutionStatusLabel('custom_status')).toBe('custom status')
  })

  it('formats issue body and log names', () => {
    expect(formatIssueBody(undefined)).toBe(EMPTY_ISSUE_BODY_MESSAGE)
    expect(formatIssueBody('')).toBe(EMPTY_ISSUE_BODY_MESSAGE)
    expect(formatIssueBody('  body  ')).toBe('  body  ')

    expect(formatLogName('stdout.log')).toBe('標準出力')
    expect(formatLogName('stderr.log')).toBe('標準エラー')
    expect(formatLogName('git-push.log')).toBe('git push')
    expect(formatLogName('gh-pr-create.log')).toBe('gh pr create')
    expect(formatLogName('gh-pr-comments.log')).toBe('gh pr comments')
    expect(formatLogName('gh-pr-comment.log')).toBe('gh pr comment')
    expect(formatLogName('custom.log')).toBe('custom.log')
  })

  it('formats payload previews and content', () => {
    expect(formatPayloadDisplay('')).toEqual({
      preview: EMPTY_PAYLOAD_LABEL,
      content: EMPTY_PAYLOAD_LABEL,
    })

    expect(formatPayloadDisplay('{"foo":"bar"}')).toEqual({
      preview: '{"foo":"bar"}',
      content: '{\n  "foo": "bar"\n}',
    })

    expect(formatPayloadPreview('plain text payload', 8)).toBe('plain te…')
    expect(formatPayloadContent('plain text payload')).toBe('plain text payload')

    const longJson = JSON.stringify({ value: 'x'.repeat(80) })
    expect(formatPayloadDisplay(longJson, 20).preview).toMatch(/…$/)
    expect(formatPayloadDisplay('  mixed\n  whitespace  ').preview).toBe('mixed whitespace')
  })

  it('formats dates in a locale aware way', () => {
    const formatted = formatDateTime('2026-06-08T00:00:00Z')
    expect(formatted).toMatch(/\d{4}\/\d{2}\/\d{2}/)
    expect(formatted).toMatch(/\d{2}:\d{2}:\d{2}/)
  })
})
