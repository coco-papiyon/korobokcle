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
              workers: 1,
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
    return {
      data: ref({
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
        issueBody: 'body',
        designArtifact: {
          path: 'result.md',
          content: 'design',
        },
        logs: [],
      }),
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
})
