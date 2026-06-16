import { flushPromises, mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { ref } from 'vue'

import WorkerSettingsPage from './WorkerSettingsPage.vue'

const apiMocks = vi.hoisted(() => ({
  fetchAppConfig: vi.fn(),
  saveAppConfig: vi.fn(),
}))

vi.mock('@/lib/api', async () => {
  const actual = await vi.importActual<typeof import('@/lib/api')>('@/lib/api')
  return {
    ...actual,
    fetchAppConfig: apiMocks.fetchAppConfig,
    saveAppConfig: apiMocks.saveAppConfig,
  }
})

const reloadMock = vi.fn()

vi.mock('@/composables/useAsyncData', () => ({
  useAsyncData: (loader: unknown) => {
    if (loader === apiMocks.fetchAppConfig) {
      return {
        data: ref({
          monitoredRepositories: [
            {
              repository: 'owner/repository',
              branch: 'main',
              workDir: '',
              implementationWorkers: 1,
              reviewWorkers: 1,
              improvementEnabled: false,
              improvementBranch: '',
              improvementDir: '',
              workerDirs: [],
            },
          ],
        }),
        isLoading: ref(false),
        error: ref(null),
        reload: reloadMock,
      }
    }
    throw new Error('unexpected loader')
  },
}))

vi.mock('vue-router', () => ({
  RouterLink: {
    props: ['to'],
    template: '<a :href="typeof to === \'string\' ? to : String(to)"><slot /></a>',
  },
}))

describe('WorkerSettingsPage', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    apiMocks.saveAppConfig.mockResolvedValue({
      monitoredRepositories: [
        {
          repository: 'owner/repository',
          branch: 'main',
          workDir: '',
          implementationWorkers: 1,
          reviewWorkers: 1,
          improvementEnabled: false,
          improvementBranch: '',
          improvementDir: '',
          workerDirs: [],
        },
      ],
    })
  })

  it('shows repository setting labels and success message', async () => {
    const wrapper = mount(WorkerSettingsPage)

    await flushPromises()

    expect(wrapper.text()).toContain('リポジトリ設定')
    expect(wrapper.text()).toContain('リポジトリ設定を保存')
    expect(wrapper.text()).toContain('リポジトリ設定')

    const saveButton = wrapper.findAll('button').find((candidate) => candidate.text() === 'リポジトリ設定を保存')
    expect(saveButton).toBeDefined()
    await saveButton!.trigger('click')
    await flushPromises()

    expect(apiMocks.saveAppConfig).toHaveBeenCalled()
    expect(wrapper.text()).toContain('リポジトリ設定を更新しました。')
  })
})
