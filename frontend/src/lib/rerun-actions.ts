import type { JobEvent } from '@/types'

export type RerunAction = 'retry_design' | 'retry_implementation' | 'retry_pr' | 'retry_review'

const actionOrder: RerunAction[] = ['retry_design', 'retry_implementation', 'retry_pr', 'retry_review']

export function rerunActionFromAvailableActions(availableActions?: string[] | null): RerunAction | null {
  const actions = availableActions ?? []
  for (const action of actionOrder) {
    if (actions.includes(action)) {
      return action
    }
  }
  return null
}

export function rerunActionFromEvent(event?: Pick<JobEvent, 'availableActions'> | null): RerunAction | null {
  return rerunActionFromAvailableActions(event?.availableActions)
}

export function rerunButtonLabel(action: RerunAction, eventType?: string, sourceEventType?: string) {
  if (action === 'retry_implementation' && (eventType === 'test_failed' || sourceEventType === 'test_failed')) {
    return '実装を修正'
  }
  if (action === 'retry_design') {
    return '設計を再実行'
  }
  if (action === 'retry_implementation') {
    return '実装を再実行'
  }
  if (action === 'retry_review') {
    return 'レビューを再実行'
  }
  return 'PR作成を再実行'
}
