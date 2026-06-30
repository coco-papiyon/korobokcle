import { mount } from '@vue/test-utils'
import { nextTick } from 'vue'
import { vi } from 'vitest'
import JobDetailPanel from './JobDetailPanel.vue'

function jsonResponse(body: Record<string, unknown>, status = 200) {
  return new Response(JSON.stringify(body), {
    status,
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
  it('shows the branch returned by the detail API', async () => {
    const fetchMock = vi.fn().mockResolvedValueOnce(
      jsonResponse({
        id: 'issue-42',
        kind: 'issue_design',
        state: 'detected',
        repository: 'owner/repo',
        number: 42,
        title: 'design the thing',
        branch: 'issue_#42',
      }),
    )
    vi.stubGlobal('fetch', fetchMock)

    const wrapper = mount(JobDetailPanel, {
      props: {
        jobId: 'issue-42',
        refreshKey: 0,
      },
    })

    await flushPromises()

    expect(wrapper.text()).toContain('ブランチ')
    expect(wrapper.text()).toContain('issue_#42')
  })

  it('falls back when the branch is missing', async () => {
    const fetchMock = vi.fn().mockResolvedValueOnce(
      jsonResponse({
        id: 'pr-12',
        kind: 'pr_review',
        state: 'review_running',
        repository: 'owner/repo',
        number: 12,
        title: 'review the thing',
      }),
    )
    vi.stubGlobal('fetch', fetchMock)

    const wrapper = mount(JobDetailPanel, {
      props: {
        jobId: 'pr-12',
        refreshKey: 0,
      },
    })

    await flushPromises()

    expect(wrapper.text()).toContain('ブランチ')
    expect(wrapper.text()).toContain('-')
  })
})
