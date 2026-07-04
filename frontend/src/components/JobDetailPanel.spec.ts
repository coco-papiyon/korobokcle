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

  it('shows a failed job error with the failed chip style', async () => {
	const fetchMock = vi.fn().mockResolvedValueOnce(
	  jsonResponse({
		updatedAt: '2026-07-01T00:00:00Z',
		job: {
		  id: 'job-failed',
		  kind: 'issue_implementation',
		  state: 'failed',
		  repository: 'owner/repo',
		  number: 500,
		  title: '失敗ジョブ',
		  errorMessage: 'copilot permission denied: execute: npm test',
		},
	  }),
	)
	vi.stubGlobal('fetch', fetchMock)

	const wrapper = mount(JobDetailPanel, {
	  props: { active: true, jobId: 'job-failed', refreshKey: 0 },
	})
	await flushPromises()

	expect(wrapper.find('.chip--failed').text()).toBe('失敗')
	expect(wrapper.find('.detail__error').text()).toContain('copilot permission denied: execute: npm test')
	expect(wrapper.find('.detail__retry').text()).toBe('再実行')
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
          subStatus: '実装(1回目)',
          repository: 'owner/repo',
          number: 1,
          title: '実装中ジョブ',
          fetchedAt: '2026-07-01T00:00:00Z',
          updatedAt: '2026-07-01T03:04:05Z',
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
    expect(wrapper.get('.detail__substatus').text()).toBe('実装(1回目)')
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
          fetchedAt: '2026-07-01T00:00:00Z',
          updatedAt: '2026-07-01T03:04:05Z',
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
    const metaItems = wrapper.findAll('.detail__meta-item')
    expect(metaItems).toHaveLength(5)
    expect(metaItems[2].text()).toContain('#2')
    expect(metaItems[3].text()).toContain('ブランチ')
    expect(metaItems[4].text()).toContain('取得時間')
    expect(metaItems[4].text()).toContain('2026/07/01 09:00:00')
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
          fetchedAt: '2026-07-01T00:00:00Z',
          updatedAt: '2026-07-01T03:04:05Z',
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

  it('shows fetched time in the detail summary', async () => {
    const fetchMock = vi.fn().mockResolvedValueOnce(
      jsonResponse({
        updatedAt: '2026-07-01T00:00:00Z',
        branch: 'issue_#9',
        job: {
          id: 'job-9',
          kind: 'issue_implementation',
          state: 'implementation_ready',
          repository: 'owner/repo',
          number: 9,
          title: '時刻付き詳細',
          fetchedAt: '2026-07-01T00:00:00Z',
          updatedAt: '2026-07-01T03:04:05Z',
        },
      }),
    )
    vi.stubGlobal('fetch', fetchMock)

    const wrapper = mount(JobDetailPanel, {
      props: {
        active: true,
        jobId: 'job-9',
        refreshKey: 0,
      },
    })
    await flushPromises()

    const metaItems = wrapper.findAll('.detail__meta-item')
    expect(metaItems).toHaveLength(5)
    expect(metaItems[2].text()).toContain('#9')
    expect(metaItems[3].text()).toContain('issue_#9')
    expect(metaItems[4].text()).toContain('取得時間')
    expect(metaItems[4].text()).toContain('2026/07/01 09:00:00')
  })

  it('shows issue context above the artifact section for issue jobs', async () => {
    const fetchMock = vi.fn().mockResolvedValueOnce(
      jsonResponse({
        updatedAt: '2026-07-01T00:00:00Z',
        branch: 'issue_#12',
        job: {
          id: 'job-12',
          kind: 'issue_design',
          state: 'design_ready',
          repository: 'owner/repo',
          number: 12,
          title: '画面調整',
          issueContext: '#12 画面調整\n\n詳細な要件',
          fetchedAt: '2026-07-01T00:00:00Z',
          updatedAt: '2026-07-01T03:04:05Z',
        },
      }),
    )
    vi.stubGlobal('fetch', fetchMock)

    const wrapper = mount(JobDetailPanel, {
      props: {
        active: true,
        jobId: 'job-12',
        refreshKey: 0,
      },
    })
    await flushPromises()

    const details = wrapper.get('details.detail-context')
    expect(details.get('summary').text()).toBe('Issue の内容')
    expect(wrapper.text()).toContain('#12 画面調整')
    expect(wrapper.text()).toContain('詳細な要件')
    expect(wrapper.text()).toContain('設計結果')
  })

  it('shows logs grouped by role and attempt', async () => {
    const fetchMock = vi.fn()
      .mockResolvedValueOnce(
        jsonResponse({
          updatedAt: '2026-07-01T00:00:00Z',
          branch: 'issue_#30',
          logs: [
            {
              role: 'agent',
              roleLabel: '実装者',
              attempt: 1,
              files: [
                {
                  kind: 'activity',
                  label: '処理ログ',
                  path: 'logs/30/implementation_attempt-1_agent.log',
                  content: 'request\nresponse',
                },
                {
                  kind: 'stdout',
                  label: '標準出力',
                  path: 'logs/30/implementation_attempt-1_agent_stdout.log',
                  content: 'agent stdout',
                },
              ],
            },
            {
              role: 'verifier',
              roleLabel: '検証者',
              attempt: 1,
              files: [
                {
                  kind: 'activity',
                  label: '処理ログ',
                  path: 'logs/30/implementation_attempt-1_verifier.log',
                  content: 'verification summary',
                },
              ],
            },
          ],
          job: {
            id: 'job-30',
            kind: 'issue_implementation',
            state: 'implementation_ready',
            repository: 'owner/repo',
            number: 30,
            title: 'ログ確認',
            fetchedAt: '2026-07-01T00:00:00Z',
            updatedAt: '2026-07-01T03:04:05Z',
          },
        }),
      )
      .mockResolvedValueOnce(
        jsonResponse({
          content: 'implementation artifact content',
          path: 'artifact.md',
        }),
      )
    vi.stubGlobal('fetch', fetchMock)

    const wrapper = mount(JobDetailPanel, {
      props: {
        active: true,
        jobId: 'job-30',
        refreshKey: 0,
      },
    })
    await flushPromises()

    expect(wrapper.text()).toContain('実装者 / 試行 1')
    expect(wrapper.text()).toContain('検証者 / 試行 1')
    expect(wrapper.text()).toContain('処理ログ')
    expect(wrapper.text()).toContain('agent stdout')
    expect(wrapper.text()).toContain('verification summary')
  })

  it('shows design artifacts for completed issue design jobs', async () => {
    const fetchMock = vi.fn()
      .mockResolvedValueOnce(
        jsonResponse({
          updatedAt: '2026-07-01T00:00:00Z',
          branch: 'issue_#16',
          job: {
            id: 'job-16',
            kind: 'issue_design',
            state: 'completed',
            repository: 'owner/repo',
            number: 16,
            title: '完了済み設計ジョブ',
            issueContext: '#16 完了済み設計ジョブ\n\n設計内容',
            fetchedAt: '2026-07-01T00:00:00Z',
            updatedAt: '2026-07-01T03:04:05Z',
          },
        }),
      )
      .mockResolvedValueOnce(
        jsonResponse({
          content: 'design artifact content',
          path: 'artifact.md',
        }),
      )
    vi.stubGlobal('fetch', fetchMock)

    const wrapper = mount(JobDetailPanel, {
      props: {
        active: true,
        jobId: 'job-16',
        refreshKey: 0,
      },
    })
    await flushPromises()

    expect(wrapper.text()).toContain('設計結果')
    expect(wrapper.text()).toContain('design artifact content')
    expect(fetchMock).toHaveBeenNthCalledWith(2, '/api/jobs/job-16/artifact', { cache: 'no-store' })
  })

  it('shows implementation artifacts for completed issue implementation jobs', async () => {
    const fetchMock = vi.fn()
      .mockResolvedValueOnce(
        jsonResponse({
          updatedAt: '2026-07-01T00:00:00Z',
          branch: 'issue_#17',
          job: {
            id: 'job-17',
            kind: 'issue_implementation',
            state: 'completed',
            repository: 'owner/repo',
            number: 17,
            title: '完了済み実装ジョブ',
            fetchedAt: '2026-07-01T00:00:00Z',
            updatedAt: '2026-07-01T03:04:05Z',
          },
        }),
      )
      .mockResolvedValueOnce(
        jsonResponse({
          content: 'implementation artifact content',
          path: 'artifact.md',
        }),
      )
    vi.stubGlobal('fetch', fetchMock)

    const wrapper = mount(JobDetailPanel, {
      props: {
        active: true,
        jobId: 'job-17',
        refreshKey: 0,
      },
    })
    await flushPromises()

    expect(wrapper.text()).toContain('実装結果')
    expect(wrapper.text()).toContain('implementation artifact content')
    expect(fetchMock).toHaveBeenNthCalledWith(2, '/api/jobs/job-17/artifact', { cache: 'no-store' })
  })

  it('shows artifacts for approved and completed jobs', async () => {
    const cases = [
      {
        jobId: 'job-18',
        kind: 'issue_design',
        state: 'design_approved',
        title: '設計承認済み',
        artifactLabel: '設計結果',
        artifactContent: 'design approved artifact content',
      },
      {
        jobId: 'job-19',
        kind: 'issue_implementation',
        state: 'implementation_approved',
        title: '実装承認済み',
        artifactLabel: '実装結果',
        artifactContent: 'implementation approved artifact content',
      },
      {
        jobId: 'job-20',
        kind: 'issue_implementation',
        state: 'pr_created',
        title: 'PR 済み',
        artifactLabel: '実装結果',
        artifactContent: 'pr created artifact content',
      },
      {
        jobId: 'job-21',
        kind: 'pr_conflict',
        state: 'pr_conflict_resolved',
        title: 'コンフリクト解消済み',
        artifactLabel: 'コンフリクト解消結果',
        artifactContent: 'pr conflict resolved artifact content',
      },
      {
        jobId: 'job-22',
        kind: 'pr_review',
        state: 'review_approved',
        title: 'レビュー承認済み',
        artifactLabel: 'レビュー結果',
        artifactContent: 'review approved artifact content',
      },
      {
        jobId: 'job-23',
        kind: 'pr_feedback',
        state: 'review_fixed',
        title: 'レビュー指摘修正済み',
        artifactLabel: 'レビュー指摘修正結果',
        artifactContent: 'review fixed artifact content',
      },
      {
        jobId: 'job-24',
        kind: 'issue_implementation',
        state: 'completed',
        title: '完了済み',
        artifactLabel: '実装結果',
        artifactContent: 'completed artifact content',
      },
    ] as const

    for (const testCase of cases) {
      const fetchMock = vi.fn()
        .mockResolvedValueOnce(
          jsonResponse({
            updatedAt: '2026-07-01T00:00:00Z',
            branch: `branch-${testCase.jobId}`,
            job: {
              id: testCase.jobId,
              kind: testCase.kind,
              state: testCase.state,
              repository: 'owner/repo',
              number: Number(testCase.jobId.split('-')[1]),
              title: testCase.title,
              fetchedAt: '2026-07-01T00:00:00Z',
              updatedAt: '2026-07-01T03:04:05Z',
            },
          }),
        )
        .mockResolvedValueOnce(
          jsonResponse({
            content: testCase.artifactContent,
            path: 'artifact.md',
          }),
        )
      vi.stubGlobal('fetch', fetchMock)

      const wrapper = mount(JobDetailPanel, {
        props: {
          active: true,
          jobId: testCase.jobId,
          refreshKey: 0,
        },
      })
      await flushPromises()

      expect(wrapper.text()).toContain(testCase.artifactLabel)
      expect(wrapper.text()).toContain(testCase.artifactContent)
      expect(fetchMock).toHaveBeenNthCalledWith(2, `/api/jobs/${testCase.jobId}/artifact`, { cache: 'no-store' })
      wrapper.unmount()
    }
  })

  it('reloads artifacts when the same job is refreshed', async () => {
    let resolveSecondArtifact: ((value: Response) => void) | undefined
    const fetchMock = vi.fn()
      .mockResolvedValueOnce(
        jsonResponse({
          updatedAt: '2026-07-01T00:00:00Z',
          branch: 'issue_#25',
          job: {
            id: 'job-25',
            kind: 'issue_implementation',
            state: 'completed',
            repository: 'owner/repo',
            number: 25,
            title: '再読込対象',
            fetchedAt: '2026-07-01T00:00:00Z',
            updatedAt: '2026-07-01T03:04:05Z',
          },
        }),
      )
      .mockResolvedValueOnce(
        jsonResponse({
          content: 'artifact content v1',
          path: 'artifact.md',
        }),
      )
      .mockResolvedValueOnce(
        jsonResponse({
          updatedAt: '2026-07-01T00:00:00Z',
          branch: 'issue_#25',
          job: {
            id: 'job-25',
            kind: 'issue_implementation',
            state: 'completed',
            repository: 'owner/repo',
            number: 25,
            title: '再読込対象',
            fetchedAt: '2026-07-01T00:00:00Z',
            updatedAt: '2026-07-01T03:04:05Z',
          },
        }),
      )
      .mockResolvedValueOnce(
        new Promise<Response>((resolve) => {
          resolveSecondArtifact = resolve
        }),
      )
    vi.stubGlobal('fetch', fetchMock)

    const wrapper = mount(JobDetailPanel, {
      props: {
        active: true,
        jobId: 'job-25',
        refreshKey: 0,
      },
    })
    await flushPromises()

    expect(wrapper.text()).toContain('artifact content v1')

    await wrapper.setProps({ refreshKey: 1 })
    await flushPromises()

    expect(fetchMock).toHaveBeenNthCalledWith(4, '/api/jobs/job-25/artifact', { cache: 'no-store' })
    expect(wrapper.text()).toContain('artifact content v1')
    expect(wrapper.text()).not.toContain('読み込み中...')

    resolveSecondArtifact?.(
      jsonResponse({
        content: 'artifact content v2',
        path: 'artifact.md',
      }),
    )
    await flushPromises()

    expect(wrapper.text()).toContain('artifact content v2')
  })

  it('shows an issue link for issue jobs', async () => {
    const fetchMock = vi.fn().mockResolvedValueOnce(
      jsonResponse({
        updatedAt: '2026-07-01T00:00:00Z',
        branch: 'issue_#13',
        job: {
          id: 'job-13',
          kind: 'issue_implementation',
          state: 'implementation_ready',
          repository: 'owner/repo',
          number: 13,
          title: 'Issue リンク',
          fetchedAt: '2026-07-01T00:00:00Z',
          updatedAt: '2026-07-01T03:04:05Z',
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
        jobId: 'job-13',
        refreshKey: 0,
      },
    })
    await flushPromises()

    const link = wrapper.get('.detail-links__link')
    expect(link.text()).toBe('Issue を開く')
    expect(link.attributes('href')).toBe('https://github.com/owner/repo/issues/13')
    expect(link.attributes('target')).toBe('_blank')
    expect(link.attributes('rel')).toBe('noreferrer')
  })

  it('shows a PR link for PR jobs', async () => {
    const fetchMock = vi.fn().mockResolvedValueOnce(
      jsonResponse({
        updatedAt: '2026-07-01T00:00:00Z',
        branch: 'pr-14',
        job: {
          id: 'job-14',
          kind: 'pr_conflict',
          state: 'pr_conflict_ready',
          repository: 'owner/repo',
          number: 14,
          title: 'PR リンク',
          fetchedAt: '2026-07-01T00:00:00Z',
          updatedAt: '2026-07-01T03:04:05Z',
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
        jobId: 'job-14',
        refreshKey: 0,
      },
    })
    await flushPromises()

    const link = wrapper.get('.detail-links__link')
    expect(link.text()).toBe('PR を開く')
    expect(link.attributes('href')).toBe('https://github.com/owner/repo/pull/14')
    expect(link.attributes('target')).toBe('_blank')
    expect(link.attributes('rel')).toBe('noreferrer')
  })

  it('hides the link section when repository data is missing', async () => {
    const fetchMock = vi.fn().mockResolvedValueOnce(
      jsonResponse({
        updatedAt: '2026-07-01T00:00:00Z',
        branch: 'issue_#15',
        job: {
          id: 'job-15',
          kind: 'issue_implementation',
          state: 'implementation_ready',
          repository: '',
          number: 15,
          title: 'リンク非表示',
          fetchedAt: '2026-07-01T00:00:00Z',
          updatedAt: '2026-07-01T03:04:05Z',
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
        jobId: 'job-15',
        refreshKey: 0,
      },
    })
    await flushPromises()

    expect(wrapper.find('.detail-links').exists()).toBe(false)
  })

  it('keeps the current view when a refreshed job has the same updatedAt', async () => {
    const fetchMock = vi.fn()
      .mockResolvedValueOnce(
        jsonResponse({
          updatedAt: '2026-07-01T00:00:00Z',
          branch: 'issue_#21',
          job: {
            id: 'job-21',
            kind: 'issue_design',
            state: 'design_ready',
            repository: 'owner/repo',
          number: 21,
          title: '最初の表示',
          issueContext: '#21 最初の表示\n\n本文A',
            fetchedAt: '2026-07-01T00:00:00Z',
            updatedAt: '2026-07-01T03:04:05Z',
          },
        }),
      )
      .mockResolvedValueOnce(
        jsonResponse({
          content: 'artifact content',
          path: 'artifact.md',
        }),
      )
      .mockResolvedValueOnce(
        jsonResponse({
          updatedAt: '2026-07-01T00:00:00Z',
          branch: 'issue_#21',
          job: {
            id: 'job-21',
            kind: 'issue_design',
            state: 'design_ready',
            repository: 'owner/repo',
          number: 21,
          title: '更新されない表示',
          issueContext: '#21 更新されない表示\n\n本文B',
            fetchedAt: '2026-07-01T00:00:00Z',
            updatedAt: '2026-07-01T03:04:05Z',
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
        jobId: 'job-21',
        refreshKey: 0,
      },
    })
    await flushPromises()

    expect(wrapper.text()).toContain('最初の表示')
    expect(wrapper.text()).toContain('本文A')

    await wrapper.setProps({ refreshKey: 1 })
    await flushPromises()

    expect(wrapper.text()).toContain('最初の表示')
    expect(wrapper.text()).toContain('本文A')
    expect(wrapper.text()).not.toContain('更新されない表示')
    expect(wrapper.text()).not.toContain('本文B')
  })

  it('shows placeholders for missing job times in detail view', async () => {
    const fetchMock = vi.fn().mockResolvedValueOnce(
      jsonResponse({
        updatedAt: '2026-07-01T00:00:00Z',
        branch: '',
        job: {
          id: 'job-10',
          kind: 'issue_implementation',
          state: 'implementation_ready',
          repository: 'owner/repo',
          number: 10,
          title: '時刻なし詳細',
        },
      }),
    )
    vi.stubGlobal('fetch', fetchMock)

    const wrapper = mount(JobDetailPanel, {
      props: {
        active: true,
        jobId: 'job-10',
        refreshKey: 0,
      },
    })
    await flushPromises()

    const metaItems = wrapper.findAll('.detail__meta-item')
    expect(metaItems).toHaveLength(5)
    expect(metaItems[4].text()).toContain('取得時間')
    expect(metaItems[4].text()).toContain('-')
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
    expect(wrapper.emitted('deleted')).toEqual([['job-3']])
    expect(wrapper.emitted('refresh')).toHaveLength(1)
    expect(wrapper.emitted('close')).toHaveLength(1)
  })

  it('emits close and refresh after approving an artifact', async () => {
    const fetchMock = vi.fn()
    fetchMock
      .mockResolvedValueOnce(
        jsonResponse({
          updatedAt: '2026-07-01T00:00:00Z',
          branch: 'issue_#11',
          job: {
            id: 'job-11',
            kind: 'issue_implementation',
            state: 'implementation_ready',
            repository: 'owner/repo',
            number: 11,
            title: '承認対象',
            fetchedAt: '2026-07-01T00:00:00Z',
            updatedAt: '2026-07-01T03:04:05Z',
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
        active: true,
        jobId: 'job-11',
        refreshKey: 0,
      },
    })
    await flushPromises()

    await wrapper.get('button.button').trigger('click')
    await flushPromises()

    expect(fetchMock).toHaveBeenCalledWith('/api/jobs/job-11/artifact', {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
      },
      body: JSON.stringify({ comment: '' }),
    })
    expect(wrapper.emitted('refresh')).toHaveLength(1)
    expect(wrapper.emitted('close')).toHaveLength(1)
  })
})
