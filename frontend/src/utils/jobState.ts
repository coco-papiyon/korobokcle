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
