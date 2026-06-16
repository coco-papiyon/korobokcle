import { flushPromises, mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { ref } from 'vue'

import JobDetailPage from './JobDetailPage.vue'

const apiMocks = vi.hoisted(() => ({
  fetchAppConfig: vi.fn(),
  fetchToolCommands: vi.fn(),
  fetchWatchRules: vi.fn(),
  fetchJobDetail: vi.fn(),
  generateImprovement: vi.fn(),
  submitImplementationRerun: vi.fn(),
}))

vi.mock('@/lib/api', async () => {
  const actual = await vi.importActual<typeof import('@/lib/api')>('@/lib/api')
  return {
    ...actual,
    fetchAppConfig: apiMocks.fetchAppConfig,
    fetchToolCommands: apiMocks.fetchToolCommands,
    fetchWatchRules: apiMocks.fetchWatchRules,
    fetchJobDetail: apiMocks.fetchJobDetail,
    generateImprovement: apiMocks.generateImprovement,
    submitImplementationRerun: apiMocks.submitImplementationRerun,
  }
})

const reloadMock = vi.fn()

vi.mock('vue-router', () => ({
  useRoute: () => ({
    params: {
      id: 'job-1',
    },
  }),
  useRouter: () => ({
    push: vi.fn(),
  }),
  RouterLink: {
    props: ['to'],
    template: '<a :href="typeof to === \'string\' ? to : String(to)"><slot /></a>',
  },
}))

vi.mock('@/composables/useAsyncData', () => ({
  useAsyncData: (loader: unknown) => {
    if (loader === apiMocks.fetchAppConfig) {
      return {
        data: ref({
          screenRefreshInterval: 0,
          monitoredRepositories: [
            {
              repository: 'owner/repository',
              branch: 'main',
              workDir: '',
              implementationWorkers: 1,
              reviewWorkers: 1,
              improvementEnabled: true,
              improvementBranch: 'develop',
              improvementDir: '.improvement',
              workerDirs: [],
            },
          ],
        }),
        isLoading: ref(false),
        isRefreshing: ref(false),
        error: ref(null),
        reload: reloadMock,
      }
    }
    if (loader === apiMocks.fetchToolCommands || loader === apiMocks.fetchWatchRules) {
      return {
        data: ref([]),
        isLoading: ref(false),
        isRefreshing: ref(false),
        error: ref(null),
        reload: reloadMock,
      }
    }
    const data = ref<unknown>(null)
    void Promise.resolve().then(async () => {
      data.value = await apiMocks.fetchJobDetail()
    })
    return {
      data: ref({
        job: {
          id: 'job-1',
          type: 'issue',
          repository: 'owner/repository',
          githubNumber: 42,
          state: 'failed',
          title: 'Improve prompts',
          branchName: 'issue_42',
          watchRuleId: 'rule-1',
          createdAt: '2026-06-08T00:00:00Z',
          updatedAt: '2026-06-08T00:00:00Z',
        },
        events: [
          {
            id: 1,
            jobId: 'job-1',
            eventType: 'test_failed',
            stateFrom: 'test_running',
            stateTo: 'failed',
            payload: '{"error":"tests failed"}',
            createdAt: '2026-06-08T00:00:00Z',
            availableActions: [],
          },
        ],
        issueBody: [
          '# Issue title',
          '',
          '| Name | Value |',
          '| --- | --- |',
          '| Example | `code` |',
          '',
          '```ts',
          'const value = 1',
          '```',
        ].join('\n'),
        designArtifact: {
          path: 'result.md',
          content: 'design',
        },
        testReport: {
          path: 'test-report.json',
          content: JSON.stringify({
            profile: 'default',
            success: false,
            startedAt: '2026-06-08T00:00:00Z',
            finishedAt: '2026-06-08T00:05:00Z',
            results: [
              {
                command: 'npm test',
                exitCode: 1,
                durationMs: 1000,
                stdout: 'running tests',
                stderr: 'failed tests',
                success: false,
              },
            ],
          }),
        },
        logs: [],
      }),
      data,
      isLoading: ref(false),
      isRefreshing: ref(false),
      error: ref(null),
      reload: reloadMock,
    }
  },
}))

describe('JobDetailPage', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    apiMocks.generateImprovement.mockResolvedValue({})
    apiMocks.submitImplementationRerun.mockResolvedValue({
    apiMocks.fetchJobDetail.mockResolvedValue({
      job: {
        id: 'job-1',
        type: 'issue',
        repository: 'owner/repository',
        githubNumber: 42,
        state: 'implementation_running',
        state: 'waiting_design_approval',
        title: 'Improve prompts',
        branchName: 'issue_42',
        watchRuleId: 'rule-1',
        createdAt: '2026-06-08T00:00:00Z',
        updatedAt: '2026-06-08T00:06:00Z',
        updatedAt: '2026-06-08T00:00:00Z',
      },
      events: [
        {
          id: 1,
          jobId: 'job-1',
          eventType: 'test_failed',
          stateFrom: 'test_running',
          stateTo: 'failed',
          payload: '{"error":"tests failed"}',
          createdAt: '2026-06-08T00:00:00Z',
          availableActions: [],
        },
        {
          id: 2,
          jobId: 'job-1',
          eventType: 'implementation_rerun_requested',
          stateFrom: 'failed',
          stateTo: 'implementation_running',
          payload: '{"comment":"tests failed"}',
          createdAt: '2026-06-08T00:06:00Z',
          availableActions: [],
        },
      ],
      testReport: {
        path: 'test-report.json',
        content: JSON.stringify({
          profile: 'default',
          success: true,
          startedAt: '2026-06-08T00:06:00Z',
          finishedAt: '2026-06-08T00:10:00Z',
          results: [
            {
              command: 'npm test',
              exitCode: 0,
              durationMs: 1000,
              stdout: 'running tests',
              stderr: '',
              success: true,
            },
          ],
        }),
      },
          eventType: 'design_rejected',
          stateFrom: 'design_ready',
          stateTo: 'waiting_design_approval',
          payload: '{}',
          createdAt: '2026-06-08T00:00:00Z',
          availableActions: [],
        },
      ],
      issueBody: [
        '# Issue title',
        '',
        '| Name | Value |',
        '| --- | --- |',
        '| Example | `code` |',
        '',
        '```ts',
        'const value = 1',
        '```',
      ].join('\n'),
      designArtifact: {
        path: 'result.md',
        content: 'design',
      },
      logs: [],
    })
  })

  it('shows improvement panel and starts generation from job detail', async () => {
    const wrapper = mount(JobDetailPage, {
      global: {
        stubs: {
          RouterLink: {
            props: ['to'],
            template: '<a :href="typeof to === \'string\' ? to : String(to)"><slot /></a>',
          },
        },
      },
    })

    await flushPromises()

    expect(wrapper.text()).toContain('改善機能')
    expect(wrapper.text()).toContain('改善案を生成')

    const button = wrapper.findAll('button').find((candidate) => candidate.text() === '改善案を生成')
    expect(button).toBeDefined()
    await button!.trigger('click')
    await flushPromises()

    expect(apiMocks.generateImprovement).toHaveBeenCalledWith('job-1', 'design_rejected')
    expect(wrapper.text()).toContain('改善案を生成しました。改善一覧から確認できます。')
  })

  it('renders issue body as markdown inside the modal', async () => {
    const wrapper = mount(JobDetailPage, {
      global: {
        stubs: {
          RouterLink: {
            props: ['to'],
            template: '<a :href="typeof to === \'string\' ? to : String(to)"><slot /></a>',
          },
        },
      },
    })

    await flushPromises()

    const openButton = wrapper.findAll('button').find((candidate) => candidate.text() === 'Issue 本文を開く')
    expect(openButton).toBeDefined()
    await openButton!.trigger('click')
    await flushPromises()

    const markdown = wrapper.find('.markdown-content')
    expect(markdown.exists()).toBe(true)
    expect(markdown.find('h1').text()).toBe('Issue title')
    expect(markdown.find('table').exists()).toBe(true)
    expect(markdown.find('pre code').text()).toContain('const value = 1')
  })

  it('does not show PR comment analysis results without the analysis event', async () => {
    apiMocks.fetchJobDetail.mockResolvedValueOnce({
      job: {
        id: 'job-1',
        type: 'issue',
        repository: 'owner/repository',
        githubNumber: 42,
        state: 'waiting_design_approval',
        title: 'Improve prompts',
        branchName: 'issue_42',
        watchRuleId: 'rule-1',
        createdAt: '2026-06-08T00:00:00Z',
        updatedAt: '2026-06-08T00:00:00Z',
      },
      events: [
        {
          id: 1,
          jobId: 'job-1',
          eventType: 'design_rejected',
          stateFrom: 'design_ready',
          stateTo: 'waiting_design_approval',
          payload: '{}',
          createdAt: '2026-06-08T00:00:00Z',
          availableActions: [],
        },
      ],
      reviewArtifact: {
        path: 'review/result.md',
        content: 'review result body',
      },
      prCommentAnalysisArtifact: {
        path: 'review/result.md',
        content: 'review result body',
      },
      logs: [],
    })

  it('reloads before opening the test report modal', async () => {
    const wrapper = mount(JobDetailPage, {
      global: {
        stubs: {
          RouterLink: {
            props: ['to'],
            template: '<a :href="typeof to === \'string\' ? to : String(to)"><slot /></a>',
          },
        },
      },
    })

    await flushPromises()

    expect(wrapper.text()).not.toContain('PR コメント分析結果')
    expect(wrapper.findAll('button').find((candidate) => candidate.text() === '分析結果を開く')).toBeUndefined()
  })

  it('shows PR comment analysis results when the analysis event exists', async () => {
    apiMocks.fetchJobDetail.mockResolvedValueOnce({
      job: {
        id: 'job-1',
        type: 'issue',
        repository: 'owner/repository',
        githubNumber: 42,
        state: 'waiting_design_approval',
        title: 'Improve prompts',
        branchName: 'issue_42',
        watchRuleId: 'rule-1',
        createdAt: '2026-06-08T00:00:00Z',
        updatedAt: '2026-06-08T00:00:00Z',
      },
      events: [
        {
          id: 1,
          jobId: 'job-1',
          eventType: 'pr_comment_analysis_ready',
          stateFrom: 'design_running',
          stateTo: 'waiting_design_approval',
          payload: '{"artifactDir":"review"}',
          createdAt: '2026-06-08T00:00:00Z',
          availableActions: [],
        },
      ],
      reviewArtifact: {
        path: 'review/result.md',
        content: 'review result body',
      },
      prCommentAnalysisArtifact: {
        path: 'review/result.md',
        content: 'analysis result body',
      },
      logs: [],
    })

    const openButton = wrapper.findAll('button').find((candidate) => candidate.text() === 'テスト結果を開く')
    expect(openButton).toBeDefined()
    await openButton!.trigger('click')
    await flushPromises()

    expect(reloadMock).toHaveBeenCalledWith({ silent: true })
    expect(wrapper.text()).toContain('テスト結果')
    expect(wrapper.text()).toContain('テストを再実行')
    expect(wrapper.text()).toContain('npm test')
  })

  it('reruns the test report and refreshes the detail view', async () => {
    const wrapper = mount(JobDetailPage, {
      global: {
        stubs: {
          RouterLink: {
            props: ['to'],
            template: '<a :href="typeof to === \'string\' ? to : String(to)"><slot /></a>',
          },
        },
      },
    })

    await flushPromises()

    const openButton = wrapper.findAll('button').find((candidate) => candidate.text() === '分析結果を開く')
    const openButton = wrapper.findAll('button').find((candidate) => candidate.text() === 'テスト結果を開く')
    expect(openButton).toBeDefined()
    await openButton!.trigger('click')
    await flushPromises()

    expect(wrapper.text()).toContain('PR コメント分析結果')
    expect(wrapper.text()).toContain('analysis result body')
    expect(wrapper.text()).not.toContain('review result body')
  })

    const rerunButton = wrapper.findAll('button').find((candidate) => candidate.text() === 'テストを再実行')
    expect(rerunButton).toBeDefined()
    await rerunButton!.trigger('click')
    await flushPromises()

    expect(apiMocks.submitImplementationRerun).toHaveBeenCalledWith('job-1', 'tests failed')
    expect(reloadMock).toHaveBeenCalledWith({ silent: true })
    expect(reloadMock).toHaveBeenCalled()
    expect(wrapper.text()).not.toContain('テストの再実行:')

    await wrapper.findAll('button').find((candidate) => candidate.text() === 'テスト結果を開く')!.trigger('click')
    await flushPromises()

    expect(reloadMock).toHaveBeenCalledWith({ silent: true })
    expect(wrapper.text()).toContain('成功: はい')
  })

  it('shows rerun errors in the test report modal', async () => {
    apiMocks.submitImplementationRerun.mockRejectedValueOnce(new Error('rerun failed'))

  it('shows interrupted job errors prominently', async () => {
    apiMocks.fetchJobDetail.mockResolvedValueOnce({
      job: {
        id: 'job-1',
        type: 'issue',
        repository: 'owner/repository',
        githubNumber: 42,
        state: 'interrupted',
        title: 'Improve prompts',
        branchName: 'issue_42',
        watchRuleId: 'rule-1',
        createdAt: '2026-06-08T00:00:00Z',
        updatedAt: '2026-06-08T00:00:00Z',
      },
      events: [
        {
          id: 1,
          jobId: 'job-1',
          eventType: 'design_interrupted',
          stateFrom: 'detected',
          stateTo: 'interrupted',
          payload: JSON.stringify({ error: "fatal: 'issue_42' is already used by worktree" }),
          createdAt: '2026-06-08T00:00:00Z',
          availableActions: [],
        },
      ],
      logs: [],
    })

    const wrapper = mount(JobDetailPage, {
      global: {
        stubs: {
          RouterLink: {
            props: ['to'],
            template: '<a :href="typeof to === \'string\' ? to : String(to)"><slot /></a>',
          },
        },
      },
    })

    await flushPromises()

    const openButton = wrapper.findAll('button').find((candidate) => candidate.text() === 'テスト結果を開く')
    expect(openButton).toBeDefined()
    await openButton!.trigger('click')
    await flushPromises()

    const rerunButton = wrapper.findAll('button').find((candidate) => candidate.text() === 'テストを再実行')
    expect(rerunButton).toBeDefined()
    await rerunButton!.trigger('click')
    await flushPromises()

    expect(wrapper.text()).toContain('テストの再実行: rerun failed')
    expect(wrapper.text()).toContain('ジョブが中断されました')
    expect(wrapper.text()).toContain("fatal: 'issue_42' is already used by worktree")
  })
})
