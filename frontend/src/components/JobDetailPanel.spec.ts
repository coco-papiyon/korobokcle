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
        id: 'job-1',
        kind: 'issue_implementation',
        state: 'implementation_running',
        repository: 'owner/repo',
        number: 1,
        title: '実装中ジョブ',
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

  it('keeps ready states on the existing chip style in detail view', async () => {
    const fetchMock = vi.fn()
    fetchMock
      .mockResolvedValueOnce(
        jsonResponse({
          id: 'job-2',
          kind: 'issue_implementation',
          state: 'implementation_ready',
          repository: 'owner/repo',
          number: 2,
          title: '待機中ジョブ',
        }),
      )
      .mockResolvedValueOnce(
        jsonResponse({
          content: 'artifact content',
          path: 'artifact.md',
        }),
      )
    vi.stubGlobal('fetch', fetchMock)

    const wrapper = mount(JobDetailPanel, {
      props: {
        jobId: 'job-2',
        refreshKey: 0,
      },
    })
    await flushPromises()

    const stateChip = wrapper.get('.detail__header-actions span')
    expect(stateChip.classes()).toContain('chip')
    expect(stateChip.classes()).not.toContain('chip--running')
    expect(stateChip.text()).toBe('実装完了')
  })
})
