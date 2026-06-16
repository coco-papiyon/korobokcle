import { flushPromises } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'

const fetchMock = vi.fn(async (input: RequestInfo | URL) => {
  const path = String(input)
  if (path === '/api/app-config') {
    return jsonResponse({ screenRefreshInterval: 0 })
  }
  if (path === '/api/jobs?deleted=exclude') {
    return jsonResponse([])
  }
  throw new Error(`unexpected request: ${path}`)
})

vi.stubGlobal('fetch', fetchMock)

function jsonResponse(body: unknown) {
  return {
    ok: true,
    json: async () => body,
  } as Response
}

describe('main', () => {
  beforeEach(() => {
    vi.resetModules()
    document.body.innerHTML = '<div id="app"></div>'
    fetchMock.mockClear()
  })

  it('mounts the app into the root element', async () => {
    await import('./main')
    await flushPromises()
    await flushPromises()

    expect(fetchMock).toHaveBeenCalledWith('/api/app-config', expect.any(Object))
    expect(fetchMock).toHaveBeenCalledWith('/api/jobs?deleted=exclude', expect.any(Object))
    expect(document.querySelector('#app')).not.toBeNull()
    expect(document.querySelector('#app')?.textContent).toContain('完了ジョブも表示')
  })
})
