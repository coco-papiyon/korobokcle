import { describe, expect, it } from 'vitest'

import { rerunActionFromAvailableActions, rerunActionFromEvent, rerunButtonLabel } from './rerun-actions'

describe('rerun actions helpers', () => {
  it('picks the highest priority rerun action', () => {
    expect(rerunActionFromAvailableActions(null)).toBeNull()
    expect(rerunActionFromAvailableActions([])).toBeNull()
    expect(rerunActionFromAvailableActions(['retry_review', 'retry_pr'])).toBe('retry_pr')
    expect(rerunActionFromAvailableActions(['retry_design'])).toBe('retry_design')
    expect(rerunActionFromEvent({ availableActions: ['retry_implementation'] })).toBe('retry_implementation')
  })

  it('formats rerun button labels', () => {
    expect(rerunButtonLabel('retry_design')).toBe('設計を再実行')
    expect(rerunButtonLabel('retry_implementation')).toBe('実装を再実行')
    expect(rerunButtonLabel('retry_review')).toBe('レビューを再実行')
    expect(rerunButtonLabel('retry_pr')).toBe('PR作成を再実行')
    expect(rerunButtonLabel('retry_implementation', 'test_failed')).toBe('実装を修正')
    expect(rerunButtonLabel('retry_implementation', undefined, 'test_failed')).toBe('実装を修正')
  })
})
