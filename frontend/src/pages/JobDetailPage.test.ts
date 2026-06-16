import { flushPromises, mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { ref } from 'vue'

import JobDetailPage from './JobDetailPage.vue'

const apiMocks = vi.hoisted(() => ({
  fetchAppConfig: vi.fn(),
  fetchToolCommands: vi.fn(),
  fetchWatchRules: vi.fn(),
  generateImprovement: vi.fn(),
}))

vi.mock('@/lib/api', async () => {
  const actual = await vi.importActual<typeof import('@/lib/api')>('@/lib/api')
  return {
    ...actual,
    fetchAppConfig: apiMocks.fetchAppConfig,
    fetchToolCommands: apiMocks.fetchToolCommands,
    fetchWatchRules: apiMocks.fetchWatchRules,
    generateImprovement: apiMocks.generateImprovement,
  }
})

const reloadMock = vi.fn()

const appConfigData = ref({
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
})

const toolCommandsData = ref([])
const watchRulesData = ref([])
const jobDetailData = ref(createJobDetail())

vi.mock('@/composables/useAsyncData', () => ({
  useAsyncData: (loader: unknown) => {
    if (loader === apiMocks.fetchAppConfig) {
      return {
        data: appConfigData,
        isLoading: ref(false),
        isRefreshing: ref(false),
        error: ref(null),
        reload: reloadMock,
      }
    }
    if (loader === apiMocks.fetchToolCommands) {
      return {
        data: toolCommandsData,
        isLoading: ref(false),
        isRefreshing: ref(false),
        error: ref(null),
        reload: reloadMock,
      }
    }
    if (loader === apiMocks.fetchWatchRules) {
      return {
        data: watchRulesData,
        isLoading: ref(false),
        isRefreshing: ref(false),
        error: ref(null),
        reload: reloadMock,
      }
    }
    return {
      data: jobDetailData,
      isLoading: ref(false),
      isRefreshing: ref(false),
      error: ref(null),
      reload: reloadMock,
    }
  },
}))

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

function createJobDetail(overrides: Partial<ReturnType<typeof createJobDetailBase>> = {}) {
  return {
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
      content: [
        '# Design result',
        '',
        '- first',
        '- second',
        '',
        '| Name | Value |',
        '| --- | --- |',
        '| Example | `code` |',
      ].join('\n'),
    },
    implementationArtifact: {
      path: 'implementation.md',
      content: [
        '## Implementation result',
        '',
        '```ts',
        'const value = 1',
        '```',
      ].join('\n'),
    },
    testReport: {
      path: 'test-report.json',
      content: JSON.stringify({
        profile: 'go-default',
        success: false,
        startedAt: '2026-06-08T00:00:00Z',
        finishedAt: '2026-06-08T00:05:00Z',
        results: [
          {
            command: 'go test ./...',
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
    ...overrides,
  }
}

function createJobDetailBase() {
  return {
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
    events: [],
    issueBody: '',
    logs: [],
  }
}

function mountPage() {
  return mount(JobDetailPage, {
    global: {
      stubs: {
        RouterLink: {
          props: ['to'],
          template: '<a :href="typeof to === \'string\' ? to : String(to)"><slot /></a>',
        },
      },
    },
  })
}

describe('JobDetailPage', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    apiMocks.generateImprovement.mockResolvedValue({})
    jobDetailData.value = createJobDetail()
  })

  it('shows the improvement panel and triggers generation', async () => {
    jobDetailData.value = createJobDetail({
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
    })

    const wrapper = mountPage()
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
    const wrapper = mountPage()
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

  it('shows test failure details in the flow area', async () => {
    jobDetailData.value = createJobDetail({
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
        updatedAt: '2026-06-08T00:05:00Z',
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
    })

    const wrapper = mountPage()
    await flushPromises()

    expect(wrapper.text()).toContain('ジョブが失敗しました: tests failed')
    expect(wrapper.text()).toContain('テスト結果')

    const openButton = wrapper.findAll('button').find((candidate) => candidate.text() === 'テスト結果を開く')
    expect(openButton).toBeDefined()
    await openButton!.trigger('click')
    await flushPromises()

    const markdown = wrapper.find('.markdown-content')
    expect(markdown.exists()).toBe(true)
    expect(markdown.text()).toContain('go test ./...')
    expect(markdown.text()).toContain('failed tests')
  })
})
