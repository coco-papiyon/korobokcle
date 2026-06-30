import { mount } from '@vue/test-utils'
import { nextTick } from 'vue'
import { vi } from 'vitest'
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
  await new Promise((resolve) => setTimeout(resolve, 0))
  await nextTick()
  await new Promise((resolve) => setTimeout(resolve, 0))
  await nextTick()
}

async function flushMicrotasks() {
  await Promise.resolve()
  await nextTick()
}

describe('JobDetailPanel', () => {
  afterEach(() => {
    vi.useRealTimers()
    vi.unstubAllGlobals()
  })

  it('loads artifact when active and inspectable', async () => {
    const fetchMock = vi.fn()
      .mockResolvedValueOnce(
        jsonResponse({
          id: 'job-1',
          kind: 'issue_design',
          state: 'design_ready',
          repository: 'owner/repo',
          number: 1,
          title: '設計中ジョブ',
        }),
      )
      .mockResolvedValueOnce(
        jsonResponse({
          content: 'artifact body',
        }),
      )
    vi.stubGlobal('fetch', fetchMock)

    mount(JobDetailPanel, {
      props: {
        jobId: 'job-1',
        refreshKey: 0,
        active: true,
      },
    })

    await flushPromises()

    expect(fetchMock).toHaveBeenCalledTimes(2)
    expect(fetchMock).toHaveBeenNthCalledWith(
      1,
      '/api/jobs/job-1',
      expect.objectContaining({
        cache: 'no-store',
      }),
    )
    expect(fetchMock).toHaveBeenNthCalledWith(
      2,
      '/api/jobs/job-1/artifact',
      expect.objectContaining({
        cache: 'no-store',
      }),
    )
  })

  it('does not start artifact loading after the panel becomes inactive', async () => {
    let resolveDetail: ((value: Response) => void) | undefined
    const detailPromise = new Promise<Response>((resolve) => {
      resolveDetail = resolve
    })
    const fetchMock = vi.fn().mockReturnValue(detailPromise)
    vi.stubGlobal('fetch', fetchMock)

    const wrapper = mount(JobDetailPanel, {
      props: {
        jobId: 'job-1',
        refreshKey: 0,
        active: true,
      },
    })

    await flushMicrotasks()
    await wrapper.setProps({ active: false })
    resolveDetail?.(
      jsonResponse({
        id: 'job-1',
        kind: 'issue_design',
        state: 'design_ready',
        repository: 'owner/repo',
        number: 1,
        title: '設計中ジョブ',
      }),
    )

    await flushMicrotasks()

    expect(fetchMock).toHaveBeenCalledTimes(1)
    expect(fetchMock).toHaveBeenCalledWith(
      '/api/jobs/job-1',
      expect.objectContaining({
        cache: 'no-store',
      }),
    )
  })

  it('refreshes only while active', async () => {
    vi.useFakeTimers()
    const fetchMock = vi.fn().mockResolvedValue(
      jsonResponse({
        id: 'job-1',
        kind: 'issue_design',
        state: 'design_running',
        repository: 'owner/repo',
        number: 1,
        title: '設計中ジョブ',
      }),
    )
    vi.stubGlobal('fetch', fetchMock)

    const wrapper = mount(JobDetailPanel, {
      props: {
        jobId: 'job-1',
        refreshKey: 0,
        active: false,
      },
    })

    await flushMicrotasks()
    expect(fetchMock).not.toHaveBeenCalled()

    await wrapper.setProps({ active: true })
    await flushMicrotasks()
    expect(fetchMock).toHaveBeenCalledTimes(1)

    await vi.advanceTimersByTimeAsync(5000)
    await flushMicrotasks()
    expect(fetchMock).toHaveBeenCalledTimes(2)

    await wrapper.setProps({ active: false })
    await vi.advanceTimersByTimeAsync(5000)
    await flushMicrotasks()
    expect(fetchMock).toHaveBeenCalledTimes(2)
  })
})
