import { mount } from '@vue/test-utils'
import { nextTick } from 'vue'
import { vi } from 'vitest'
import JobListPanel from './JobListPanel.vue'

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

describe('JobListPanel', () => {
  function mockInitialFetch() {
    const fetchMock = vi.fn()
    fetchMock.mockResolvedValueOnce(
      jsonResponse({
        jobs: [
          {
            id: 'job-001',
            kind: 'issue_design',
            state: 'design_ready',
            repository: 'owner/repository',
            number: 12,
            title: '設計タスク',
          },
          {
            id: 'job-002',
            kind: 'issue_implementation',
            state: 'completed',
            repository: 'owner/repository',
            number: 13,
            title: '完了タスク',
          },
          {
            id: 'job-003',
            kind: 'pr_review',
            state: 'review_ready',
            repository: 'owner/repository',
            number: 14,
            title: 'レビュータスク',
          },
        ],
      }),
    )
    vi.stubGlobal('fetch', fetchMock)
    return fetchMock
  }

  it('shows completed jobs by default and filters them when toggled off', async () => {
    mockInitialFetch()

    const wrapper = mount(JobListPanel, {
      props: {
        selectedJobId: '',
      },
    })
    await flushPromises()

    expect(wrapper.findAll('tbody tr')).toHaveLength(3)
    expect(wrapper.text()).toContain('job-002')

    const checkbox = wrapper.get('input[type="checkbox"]')
    expect((checkbox.element as HTMLInputElement).checked).toBe(true)

    await checkbox.setChecked(false)
    await nextTick()

    expect(wrapper.findAll('tbody tr')).toHaveLength(2)
    expect(wrapper.text()).not.toContain('job-002')

    await checkbox.setChecked(true)
    await nextTick()

    expect(wrapper.findAll('tbody tr')).toHaveLength(3)
    expect(wrapper.text()).toContain('job-002')
  })
})
