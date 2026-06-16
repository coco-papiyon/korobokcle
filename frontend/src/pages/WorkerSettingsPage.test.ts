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
const appConfigData = ref({
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

vi.mock('@/composables/useAsyncData', () => ({
  useAsyncData: (loader: unknown) => {
    if (loader === apiMocks.fetchAppConfig) {
      return {
        data: appConfigData,
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
    appConfigData.value = {
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
    }
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
    const wrapper = mount(WorkerSettingsPage, {
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

  it('normalizes repository settings before saving', async () => {
    const wrapper = mount(WorkerSettingsPage, {
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

    const textInputs = wrapper.findAll('input[type="text"]')
    await textInputs[0].setValue('git@github.com:owner/repository.git')
    await textInputs[1].setValue('  release  ')
    await textInputs[2].setValue('  ')
    await textInputs[3].setValue(' improvement ')
    await textInputs[4].setValue(' .improvement ')

    const numberInputs = wrapper.findAll('input[type="number"]')
    await numberInputs[0].setValue('0')
    await numberInputs[1].setValue('2.7')

    await wrapper.find('input[type="checkbox"]').setValue(true)
    await wrapper.findAll('button').find((candidate) => candidate.text() === 'リポジトリ設定を保存')!.trigger('click')
    await flushPromises()

    expect(apiMocks.saveAppConfig).toHaveBeenCalledWith({
      monitoredRepositories: [
        {
          repository: 'git@github.com:owner/repository.git',
          branch: 'release',
          workDir: '',
          implementationWorkers: 1,
          reviewWorkers: 2,
          improvementEnabled: true,
          improvementBranch: 'improvement',
          improvementDir: '.improvement',
        },
      ],
    })
    expect(wrapper.find('input[placeholder="source/owner-repository"]').attributes('placeholder')).toBe('source/owner-repository')
  })

  it('adds and removes monitored repositories locally', async () => {
    const wrapper = mount(WorkerSettingsPage, {
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

    expect(wrapper.findAll('input[type="text"]').length).toBeGreaterThan(0)
    await wrapper.findAll('button').find((candidate) => candidate.text() === 'リポジトリを追加')!.trigger('click')
    await flushPromises()

    expect(wrapper.findAll('input[type="text"]').length).toBeGreaterThan(5)
    await wrapper.findAll('button').find((candidate) => candidate.text() === '削除')!.trigger('click')
    await flushPromises()

    expect(wrapper.findAll('input[type="text"]').length).toBeGreaterThan(0)
  })

  it('deduplicates monitored repositories before saving', async () => {
    appConfigData.value = {
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
        {
          repository: 'owner/repository',
          branch: 'develop',
          workDir: 'source/owner-repository',
          implementationWorkers: 2,
          reviewWorkers: 2,
          improvementEnabled: true,
          improvementBranch: 'improvement',
          improvementDir: '.improvement',
          workerDirs: [],
        },
      ],
    }

    const wrapper = mount(WorkerSettingsPage, {
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
    await wrapper.findAll('button').find((candidate) => candidate.text() === 'リポジトリ設定を保存')!.trigger('click')
    await flushPromises()

    expect(apiMocks.saveAppConfig).toHaveBeenCalledWith({
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
        },
      ],
    })
  })
})
