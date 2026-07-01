import { mount } from '@vue/test-utils'
import { nextTick } from 'vue'
import { afterEach, vi } from 'vitest'
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
  afterEach(() => {
    vi.restoreAllMocks()
  })

  it('only refreshes while active', async () => {
    const fetchMock = vi.fn().mockResolvedValueOnce(
      jsonResponse({
        updatedAt: '2026-07-01T00:00:00Z',
        jobs: [],
      }),
    )
    const setIntervalSpy = vi.spyOn(window, 'setInterval').mockReturnValue(1 as unknown as number)
    const clearIntervalSpy = vi.spyOn(window, 'clearInterval')
    vi.stubGlobal('fetch', fetchMock)

    const wrapper = mount(JobListPanel, {
      props: {
        active: false,
        selectedJobId: '',
      },
    })
    await flushPromises()

    expect(fetchMock).not.toHaveBeenCalled()
    expect(setIntervalSpy).not.toHaveBeenCalled()

    await wrapper.setProps({ active: true })
    await flushPromises()

    expect(fetchMock).toHaveBeenCalledTimes(1)
    expect(setIntervalSpy).toHaveBeenCalledTimes(1)

    await wrapper.setProps({ active: false })
    await nextTick()

    expect(clearIntervalSpy).toHaveBeenCalledWith(1)

    wrapper.unmount()
  })

  it('hides completed jobs by default and shows them when toggled on', async () => {
    const fetchMock = vi.fn().mockResolvedValueOnce(
      jsonResponse({
        updatedAt: '2026-07-01T00:00:00Z',
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
        active: true,
        selectedJobId: '',
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

  it('uses running chip colors only for running states', async () => {
    const fetchMock = vi.fn().mockResolvedValueOnce(
      jsonResponse({
        updatedAt: '2026-07-01T00:00:00Z',
        jobs: [
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
            kind: 'issue_implementation',
            state: 'implementation_ready',
            repository: 'owner/repo',
            number: 2,
            title: '待機中ジョブ',
          },
        ],
      }),
    )
    vi.stubGlobal('fetch', fetchMock)

    const wrapper = mount(JobListPanel, {
      props: {
        active: true,
        selectedJobId: '',
      },
    })
    await flushPromises()

    const rows = wrapper.findAll('tbody tr')
    expect(rows).toHaveLength(2)
    expect(rows[0].get('td:last-child span').classes()).toContain('chip')
    expect(rows[0].get('td:last-child span').classes()).toContain('chip--running')
    expect(rows[1].get('td:last-child span').classes()).toContain('chip')
    expect(rows[1].get('td:last-child span').classes()).not.toContain('chip--running')
  })

  it('uses approved chip colors for review approvals', async () => {
    const fetchMock = vi.fn().mockResolvedValueOnce(
      jsonResponse({
        updatedAt: '2026-07-01T00:00:00Z',
        jobs: [
          {
            id: 'job-1',
            kind: 'pr_review',
            state: 'review_approved',
            repository: 'owner/repo',
            number: 1,
            title: '承認済みPR',
          },
        ],
      }),
    )
    vi.stubGlobal('fetch', fetchMock)

    const wrapper = mount(JobListPanel, {
      props: {
        active: true,
        selectedJobId: '',
      },
    })
    await flushPromises()

    const chip = wrapper.get('tbody tr td:last-child span')
    expect(chip.classes()).toContain('chip')
    expect(chip.classes()).toContain('chip--approved')
    expect(chip.text()).toBe('レビュー承認済み')
  })

  it('shows fetched and updated times beside the title', async () => {
    const fetchMock = vi.fn().mockResolvedValueOnce(
      jsonResponse({
        updatedAt: '2026-07-01T00:00:00Z',
        jobs: [
          {
            id: 'job-1',
            kind: 'issue_design',
            state: 'design_running',
            repository: 'owner/repo',
            number: 1,
            title: '時刻付きジョブ',
            fetchedAt: '2026-07-01T00:00:00Z',
            updatedAt: '2026-07-01T03:04:05Z',
          },
        ],
      }),
    )
    vi.stubGlobal('fetch', fetchMock)

    const wrapper = mount(JobListPanel, {
      props: {
        active: true,
        selectedJobId: '',
      },
    })
    await flushPromises()

    expect(wrapper.get('.job-table__title').text()).toContain('時刻付きジョブ')
    expect(wrapper.get('.job-table__title').text()).toContain('取得時間 2026/07/01 09:00:00 / 更新時間 2026/07/01 12:04:05')
  })

  it('shows placeholders when job times are missing', async () => {
    const fetchMock = vi.fn().mockResolvedValueOnce(
      jsonResponse({
        updatedAt: '2026-07-01T00:00:00Z',
        jobs: [
          {
            id: 'job-1',
            kind: 'issue_design',
            state: 'design_running',
            repository: 'owner/repo',
            number: 1,
            title: '時刻なしジョブ',
          },
        ],
      }),
    )
    vi.stubGlobal('fetch', fetchMock)

    const wrapper = mount(JobListPanel, {
      props: {
        active: true,
        selectedJobId: '',
      },
    })
    await flushPromises()

    expect(wrapper.get('.job-table__title').text()).toContain('取得時間 - / 更新時間 -')
  })

  it('skips updating visible jobs when updatedAt is unchanged', async () => {
    try {
      let intervalHandler: TimerHandler | undefined
      const setIntervalSpy = vi.spyOn(window, 'setInterval').mockImplementation((handler) => {
        intervalHandler = handler
        return 1 as unknown as number
      })
      const fetchMock = vi.fn()
      fetchMock
        .mockResolvedValueOnce(
          jsonResponse({
            updatedAt: '2026-07-01T00:00:00Z',
            jobs: [
              {
                id: 'job-1',
                kind: 'issue_design',
                state: 'design_running',
                repository: 'owner/repo',
                number: 1,
                title: '初回ジョブ',
              },
            ],
          }),
        )
        .mockResolvedValueOnce(
          jsonResponse({
            updatedAt: '2026-07-01T00:00:00Z',
            jobs: [
              {
                id: 'job-2',
                kind: 'issue_design',
                state: 'design_running',
                repository: 'owner/repo',
                number: 2,
                title: '更新されないジョブ',
              },
            ],
          }),
        )
      vi.stubGlobal('fetch', fetchMock)

      const wrapper = mount(JobListPanel, {
        props: {
          active: true,
          selectedJobId: '',
        },
      })
      await flushPromises()
      expect(wrapper.text()).toContain('初回ジョブ')

      intervalHandler?.(0 as never)
      await flushPromises()

      expect(wrapper.text()).toContain('初回ジョブ')
      expect(wrapper.text()).not.toContain('更新されないジョブ')
      setIntervalSpy.mockRestore()
    } finally {
      vi.restoreAllMocks()
    }
  })
})
