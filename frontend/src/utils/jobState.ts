const runningStates = new Set([
  'design_running',
  'implementation_running',
  'review_running',
  'review_fix_design_running',
  'review_fix_implementation_running',
])

export function jobStateChipClass(state: string) {
  return runningStates.has(state) ? 'chip chip--running' : 'chip'
}
