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

function mountPanel() {
  return mount(JobListPanel, {
    props: {
      active: true,
      selectedJobId: '',
      refreshKey: 0,
    },
  })
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
        refreshKey: 0,
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

  it('keeps completed jobs hidden by default and can show them from the status filter', async () => {
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

    const wrapper = mountPanel()
    await flushPromises()

    expect(wrapper.get('input[value="completed"]').element).toHaveProperty('checked', false)
    expect(wrapper.findAll('tbody tr')).toHaveLength(1)
    expect(wrapper.get('tbody').text()).not.toContain('完了ジョブ')
    expect(wrapper.get('tbody').text()).toContain('設計中ジョブ')
    expect(wrapper.text()).toContain('表示 1 / 3 件')

    await wrapper.get('input[value="completed"]').setChecked(true)
    await flushPromises()

    expect(wrapper.get('input[value="completed"]').element).toHaveProperty('checked', true)
    expect(wrapper.findAll('tbody tr')).toHaveLength(3)
    expect(wrapper.text()).toContain('完了ジョブ')
  })

  it('filters jobs by kind and status independently', async () => {
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
            kind: 'pr_review',
            state: 'design_running',
            repository: 'owner/repo',
            number: 2,
            title: '別Kindジョブ',
          },
          {
            id: 'job-3',
            kind: 'issue_design',
            state: 'completed',
            repository: 'owner/repo',
            number: 3,
            title: '完了ジョブ',
          },
        ],
      }),
    )
    vi.stubGlobal('fetch', fetchMock)

    const wrapper = mountPanel()
    await flushPromises()

    expect(wrapper.findAll('tbody tr')).toHaveLength(2)
    expect(wrapper.text()).toContain('設計中ジョブ')
    expect(wrapper.text()).toContain('別Kindジョブ')

    await wrapper.get('input[value="pr_review"]').setChecked(false)
    await flushPromises()

    expect(wrapper.findAll('tbody tr')).toHaveLength(1)
    expect(wrapper.text()).not.toContain('別Kindジョブ')

    await wrapper.get('input[value="design_running"]').setChecked(false)
    await flushPromises()

    expect(wrapper.findAll('tbody tr')).toHaveLength(0)
    expect(wrapper.text()).toContain('条件に一致するジョブがありません。')

    await wrapper.get('input[value="design_running"]').setChecked(true)
    await flushPromises()

    expect(wrapper.findAll('tbody tr')).toHaveLength(1)
    expect(wrapper.text()).toContain('設計中ジョブ')

    await wrapper.get('input[value="completed"]').setChecked(true)
    await flushPromises()

    expect(wrapper.findAll('tbody tr')).toHaveLength(2)
    expect(wrapper.text()).toContain('完了ジョブ')
  })

  it('shows a dedicated empty state when no jobs match filters and keeps the no-data state distinct', async () => {
    const fetchMock = vi.fn().mockResolvedValueOnce(
      jsonResponse({
        updatedAt: '2026-07-01T00:00:00Z',
        jobs: [],
      }),
    )
    vi.stubGlobal('fetch', fetchMock)

    const wrapper = mountPanel()
    await flushPromises()

    expect(wrapper.text()).toContain('まだジョブがありません。')

    const filteredFetchMock = vi.fn().mockResolvedValueOnce(
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
        ],
      }),
    )
    vi.stubGlobal('fetch', filteredFetchMock)

    const filteredWrapper = mountPanel()
    await flushPromises()

    await filteredWrapper.get('input[value="issue_design"]').setChecked(false)
    await flushPromises()

    expect(filteredWrapper.text()).toContain('条件に一致するジョブがありません。')
    expect(filteredWrapper.text()).not.toContain('まだジョブがありません。')
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

    const wrapper = mountPanel()
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

    const wrapper = mountPanel()
    await flushPromises()

    const chip = wrapper.get('tbody tr td:last-child span')
    expect(chip.classes()).toContain('chip')
    expect(chip.classes()).toContain('chip--approved')
    expect(chip.text()).toBe('レビュー承認済み')
  })

  it('shows fetched time to the right of the title', async () => {
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

    const wrapper = mountPanel()
    await flushPromises()

    const row = wrapper.get('tbody tr')
    expect(row.get('td:nth-child(4)').text()).toContain('時刻付きジョブ')
    expect(row.get('td:nth-child(5)').text()).toBe('2026/07/01 09:00:00')
    expect(row.get('td:nth-child(6)').text()).toContain('設計中')
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

      const wrapper = mountPanel()
      await flushPromises()
      expect(wrapper.text()).toContain('初回ジョブ')

      await wrapper.setProps({ refreshKey: 1 })
      await flushPromises()

      expect(fetchMock).toHaveBeenCalledTimes(2)
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
