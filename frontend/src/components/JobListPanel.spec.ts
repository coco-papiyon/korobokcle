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

async function flushMicrotasks() {
  await Promise.resolve()
  await nextTick()
}

describe('JobListPanel', () => {
  afterEach(() => {
    vi.useRealTimers()
    vi.unstubAllGlobals()
  })

  it('hides completed jobs by default and shows them when toggled on', async () => {
    const fetchMock = vi.fn().mockResolvedValueOnce(
      jsonResponse({
        jobs: [
          {
            id: 'job-3',
            kind: 'issue_implementation',
            state: 'completed',
            repository: 'owner/repo',
            number: 3,
            title: '完了ジョブ',
          },
          {
            id: 'job-1',
            kind: 'issue_design',
            state: 'design_running',
            repository: 'owner/repo',
            number: 1,
            title: '設計中ジョブ',
          },
          {
            id: 'job-2',
            kind: 'pr_review',
            state: 'completed',
            repository: 'owner/repo',
            number: 2,
            title: '別の完了ジョブ',
          },
        ],
      }),
    )
    vi.stubGlobal('fetch', fetchMock)

    const wrapper = mount(JobListPanel, {
      props: {
        selectedJobId: '',
        active: true,
      },
    })
    await flushPromises()

    expect(wrapper.get('input[type="checkbox"]').element).toHaveProperty('checked', false)
    expect(wrapper.findAll('tbody tr')).toHaveLength(1)
    expect(wrapper.get('tbody').text()).not.toContain('完了ジョブ')
    expect(wrapper.get('tbody').text()).toContain('設計中ジョブ')

    await wrapper.get('input[type="checkbox"]').setChecked(true)
    await flushPromises()

    expect(wrapper.get('input[type="checkbox"]').element).toHaveProperty('checked', true)
    expect(wrapper.findAll('tbody tr')).toHaveLength(3)
    expect(wrapper.text()).toContain('完了ジョブ')
  })

  it('refreshes only while active', async () => {
    vi.useFakeTimers()
    const fetchMock = vi.fn().mockResolvedValue(
      jsonResponse({
        jobs: [
          {
            id: 'job-1',
            kind: 'issue_design',
            state: 'design_running',
            repository: 'owner/repo',
            number: 1,
            title: '設計中ジョブ',
          },
        ],
      }),
    )
    vi.stubGlobal('fetch', fetchMock)

    const wrapper = mount(JobListPanel, {
      props: {
        selectedJobId: '',
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
