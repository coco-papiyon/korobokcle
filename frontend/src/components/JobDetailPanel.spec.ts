import { mount } from '@vue/test-utils'
import { nextTick } from 'vue'
import { afterEach, vi } from 'vitest'
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
  afterEach(() => {
    vi.restoreAllMocks()
  })

  it('only refreshes while active', async () => {
    let intervalHandler: TimerHandler | undefined
    const fetchMock = vi.fn().mockImplementation(() =>
      Promise.resolve(
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
      ),
    )
    const setIntervalSpy = vi.spyOn(window, 'setInterval').mockImplementation((handler) => {
      intervalHandler = handler
      return 1 as unknown as number
    })
    const clearIntervalSpy = vi.spyOn(window, 'clearInterval')
    vi.stubGlobal('fetch', fetchMock)

    const wrapper = mount(JobDetailPanel, {
      props: {
        active: false,
        jobId: 'job-1',
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

    intervalHandler?.(0 as never)
    await flushPromises()

    expect(fetchMock).toHaveBeenCalledTimes(2)

    await wrapper.setProps({ active: false })
    await nextTick()

    expect(clearIntervalSpy).toHaveBeenCalledWith(1)
  })

  it('uses running chip colors for running states in detail view', async () => {
    const fetchMock = vi.fn().mockResolvedValueOnce(
      jsonResponse({
        updatedAt: '2026-07-01T00:00:00Z',
        branch: 'issue_#1',
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
        active: true,
        jobId: 'job-1',
        refreshKey: 0,
      },
    })
    await flushPromises()

    const stateChip = wrapper.get('.detail__header-actions span')
    expect(stateChip.classes()).toContain('chip')
    expect(stateChip.classes()).toContain('chip--running')
    expect(stateChip.text()).toBe('実装中')
    expect(wrapper.text()).toContain('ブランチ')
    expect(wrapper.text()).toContain('issue_#1')
  })

  it('keeps ready states on the existing chip style in detail view', async () => {
    const fetchMock = vi.fn()
    fetchMock
      .mockResolvedValueOnce(
      jsonResponse({
        updatedAt: '2026-07-01T00:00:00Z',
        branch: '',
        job: {
          id: 'job-2',
          kind: 'issue_implementation',
          state: 'implementation_ready',
          repository: 'owner/repo',
          number: 2,
          title: '待機中ジョブ',
        },
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
        active: true,
        jobId: 'job-2',
        refreshKey: 0,
      },
    })
    await flushPromises()

    const stateChip = wrapper.get('.detail__header-actions span')
    expect(stateChip.classes()).toContain('chip')
    expect(stateChip.classes()).not.toContain('chip--running')
    expect(stateChip.text()).toBe('実装完了')
    expect(wrapper.text()).toContain('-')
  })

  it('uses approved chip colors for review approvals in detail view', async () => {
    const fetchMock = vi.fn().mockResolvedValueOnce(
      jsonResponse({
        updatedAt: '2026-07-01T00:00:00Z',
        branch: 'pr-1',
        job: {
          id: 'job-4',
          kind: 'pr_review',
          state: 'review_approved',
          repository: 'owner/repo',
          number: 4,
          title: '承認済みPR',
        },
      }),
    )
    vi.stubGlobal('fetch', fetchMock)

    const wrapper = mount(JobDetailPanel, {
      props: {
        active: true,
        jobId: 'job-4',
        refreshKey: 0,
      },
    })
    await flushPromises()

    const stateChip = wrapper.get('.detail__header-actions span')
    expect(stateChip.classes()).toContain('chip')
    expect(stateChip.classes()).toContain('chip--approved')
    expect(stateChip.text()).toBe('レビュー承認済み')
  })

  it('deletes the current job after confirmation', async () => {
    const fetchMock = vi.fn()
    fetchMock.mockResolvedValueOnce(
      jsonResponse({
        updatedAt: '2026-07-01T00:00:00Z',
        branch: 'issue_#3',
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
        active: true,
        jobId: 'job-3',
        refreshKey: 0,
      },
    })
    await flushPromises()

    await wrapper.get('button.button--danger').trigger('click')
    await flushPromises()

    expect(fetchMock).toHaveBeenCalledWith('/api/jobs/job-3', { method: 'DELETE' })
    expect(wrapper.text()).toContain('一覧からジョブを選択してください。')
  })
})
