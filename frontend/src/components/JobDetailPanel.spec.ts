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

describe('JobDetailPanel', () => {
  it('uses running chip colors for running states in detail view', async () => {
    const fetchMock = vi.fn().mockResolvedValueOnce(
      jsonResponse({
        updatedAt: '2026-07-01T00:00:00Z',
        job: {
          id: 'job-1',
          kind: 'issue_implementation',
          state: 'implementation_running',
          repository: 'owner/repo',
          number: 1,
          title: '実装中ジョブ',
        },
      }),
    )
    vi.stubGlobal('fetch', fetchMock)

    const wrapper = mount(JobDetailPanel, {
      props: {
        jobId: 'job-1',
        refreshKey: 0,
      },
    })
    await flushPromises()

    const stateChip = wrapper.get('.detail__header-actions span')
    expect(stateChip.classes()).toContain('chip')
    expect(stateChip.classes()).toContain('chip--running')
    expect(stateChip.text()).toBe('実装中')
  })

  it('emits close and refresh after approving the artifact', async () => {
    const fetchMock = vi.fn()
    fetchMock
      .mockResolvedValueOnce(
        jsonResponse({
          updatedAt: '2026-07-01T00:00:00Z',
          job: {
            id: 'job-2',
            kind: 'issue_implementation',
            state: 'implementation_ready',
            repository: 'owner/repo',
            number: 2,
            title: '承認対象',
          },
        }),
      )
      .mockResolvedValueOnce(
        jsonResponse({
          content: 'artifact content',
          path: 'artifact.md',
        }),
      )
      .mockResolvedValueOnce(new Response(null, { status: 204 }))
    vi.stubGlobal('fetch', fetchMock)

    const wrapper = mount(JobDetailPanel, {
      props: {
        jobId: 'job-2',
        refreshKey: 0,
      },
    })
    await flushPromises()

    await wrapper.get('button.button').trigger('click')
    await flushPromises()

    expect(fetchMock).toHaveBeenCalledWith('/api/jobs/job-2/artifact', expect.objectContaining({ method: 'POST' }))
    expect(wrapper.emitted('refresh')).toHaveLength(1)
    expect(wrapper.emitted('close')).toHaveLength(1)
  })

  it('deletes the current job after confirmation', async () => {
    const fetchMock = vi.fn()
    fetchMock.mockResolvedValueOnce(
      jsonResponse({
        updatedAt: '2026-07-01T00:00:00Z',
        job: {
          id: 'job-3',
          kind: 'issue_implementation',
          state: 'implementation_ready',
          repository: 'owner/repo',
          number: 3,
          title: '削除対象',
        },
      }),
    )
    fetchMock.mockResolvedValueOnce(
      jsonResponse({
        content: 'artifact content',
        path: 'artifact.md',
      }),
    )
    fetchMock.mockResolvedValueOnce(new Response(null, { status: 204 }))
    vi.stubGlobal('fetch', fetchMock)
    vi.stubGlobal('confirm', vi.fn().mockReturnValue(true))

    const wrapper = mount(JobDetailPanel, {
      props: {
        jobId: 'job-3',
        refreshKey: 0,
      },
    })
    await flushPromises()

    await wrapper.get('button.button--danger').trigger('click')
    await flushPromises()

    expect(fetchMock).toHaveBeenCalledWith('/api/jobs/job-3', { method: 'DELETE' })
    expect(wrapper.emitted('deleted')?.[0]).toEqual(['job-3'])
    expect(wrapper.emitted('close')).toHaveLength(1)
  })
})
