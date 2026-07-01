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
        selectedJobId: '',
      },
    })
    await flushPromises()

    const rows = wrapper.findAll('tbody tr')
    expect(rows).toHaveLength(2)
    expect(rows[0].get('span').classes()).toContain('chip')
    expect(rows[0].get('span').classes()).toContain('chip--running')
    expect(rows[1].get('span').classes()).toContain('chip')
    expect(rows[1].get('span').classes()).not.toContain('chip--running')
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

  it('reloads when the refresh key changes', async () => {
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
          updatedAt: '2026-07-01T00:00:01Z',
          jobs: [
            {
              id: 'job-1',
              kind: 'issue_design',
              state: 'design_running',
              repository: 'owner/repo',
              number: 1,
              title: '再読込ジョブ',
            },
          ],
        }),
      )
    vi.stubGlobal('fetch', fetchMock)

    const wrapper = mount(JobListPanel, {
      props: {
        selectedJobId: '',
        refreshKey: 0,
      },
    })
    await flushPromises()

    expect(wrapper.text()).toContain('初回ジョブ')

    await wrapper.setProps({ refreshKey: 1 })
    await flushPromises()

    expect(wrapper.text()).toContain('再読込ジョブ')
  })
})
