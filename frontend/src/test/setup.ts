import { afterEach, vi } from 'vitest'

afterEach(() => {
  vi.unstubAllGlobals()
  document.body.innerHTML = ''
})
