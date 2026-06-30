import { mount } from '@vue/test-utils'
import { nextTick } from 'vue'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import JobDetailPanel from './JobDetailPanel.vue'

type JsonBody = Record<string, unknown>

function jsonResponse(body: JsonBody) {
  return new Response(JSON.stringify(body), {
    status: 200,
    headers: {
      'Content-Type': 'application/json',
    },
  })
}

async function flushPromises() {
  await Promise.resolve()
  await Promise.resolve()
  await nextTick()
  await Promise.resolve()
  await nextTick()
}

describe('JobDetailPanel', () => {
  beforeEach(() => {
    vi.useFakeTimers()
  })

  afterEach(() => {
    vi.useRealTimers()
  })

  it('loads only while active and keeps refreshing detail and artifact data', async () => {
    const fetchMock = vi.fn((input: RequestInfo | URL, init?: RequestInit) => {
      const url = String(input)
      const method = init?.method ?? 'GET'

      if (url.endsWith('/artifact') && method === 'PATCH') {
        return Promise.resolve(new Response('', { status: 200 }))
      }

      if (url.endsWith('/artifact')) {
        return Promise.resolve(
          jsonResponse({
            content: 'artifact body',
            path: 'artifact.md',
          }),
        )
      }

      if (url.includes('/api/jobs/')) {
        return Promise.resolve(
          jsonResponse({
            id: 'job-1',
            kind: 'issue_implementation',
            state: 'implementation_ready',
            repository: 'owner/repository',
            number: 12,
            title: 'implement refresh control',
          }),
        )
      }

      return Promise.resolve(jsonResponse({}))
    })
    vi.stubGlobal('fetch', fetchMock)

    const wrapper = mount(JobDetailPanel, {
      props: {
        jobId: 'job-1',
        refreshKey: 0,
        active: false,
      },
    })

    await flushPromises()
    expect(fetchMock).not.toHaveBeenCalled()

    await wrapper.setProps({ active: true })
    await flushPromises()
    await flushPromises()
    expect(fetchMock).toHaveBeenCalledTimes(2)

    await vi.advanceTimersByTimeAsync(5000)
    await flushPromises()
    await flushPromises()
    expect(fetchMock).toHaveBeenCalledTimes(4)

    await wrapper.get('button.button--ghost').trigger('click')
    await flushPromises()
    await flushPromises()
    expect(fetchMock).toHaveBeenCalledTimes(7)

    await wrapper.setProps({ active: false })
    await flushPromises()

    await vi.advanceTimersByTimeAsync(5000)
    await flushPromises()
    expect(fetchMock).toHaveBeenCalledTimes(7)
  })
})
