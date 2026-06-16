import { flushPromises, mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'

import WatchRulesPage from './WatchRulesPage.vue'

const watchRules = [
  {
    id: 'rule-1',
    name: 'PR review rule',
    repositories: ['missing/repo'],
    target: 'pull_request_review_comment',
    projectName: 'Roadmap',
    labels: ['bug', 'ai'],
    projectFilters: [{ field: 'Status', values: ['Todo', 'In Progress'] }],
    titlePattern: '^feat:',
    authors: ['alice'],
    assignees: ['bob'],
    reviewers: ['carol'],
    excludeDraftPR: true,
    provider: 'mock',
    model: '',
    skillSet: 'default',
    testProfile: 'go-default',
    toolCommand: '',
    enabled: true,
  },
]

const appConfig = {
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
  providers: [{ name: 'mock', models: ['default-model'] }],
}

const skillSets = [{ name: 'default', mutable: false }]
const testProfiles = [{ name: 'go-default', commands: ['go test ./...'] }]
const toolCommands = [{ name: 'lint', command: 'npm run lint', resident: false }]

const fetchMock = vi.fn(async (input: RequestInfo | URL, init?: RequestInit) => {
  const path = String(input)
  if (path === '/api/watch-rules' && !init) {
    return jsonResponse(structuredClone(watchRules))
  }
  if (path === '/api/app-config' && !init) {
    return jsonResponse(structuredClone(appConfig))
  }
  if (path === '/api/skillsets' && !init) {
    return jsonResponse(structuredClone(skillSets))
  }
  if (path === '/api/test-profiles' && !init) {
    return jsonResponse(structuredClone(testProfiles))
  }
  if (path === '/api/tool-commands' && !init) {
    return jsonResponse(structuredClone(toolCommands))
  }
  if (path === '/api/watch-rules' && init?.method === 'PUT') {
    const body = JSON.parse(String(init.body ?? '[]')) as typeof watchRules
    watchRules.splice(0, watchRules.length, ...structuredClone(body))
    return jsonResponse(structuredClone(watchRules))
  }
  throw new Error(`unexpected request: ${path}`)
})

vi.stubGlobal('fetch', fetchMock)

function jsonResponse(body: unknown) {
  return {
    ok: true,
    json: async () => body,
  } as Response
}

describe('WatchRulesPage', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    watchRules.splice(0, watchRules.length, {
      id: 'rule-1',
      name: 'PR review rule',
      repositories: ['missing/repo'],
      target: 'pull_request_review_comment',
      projectName: 'Roadmap',
      labels: ['bug', 'ai'],
      projectFilters: [{ field: 'Status', values: ['Todo', 'In Progress'] }],
      titlePattern: '^feat:',
      authors: ['alice'],
      assignees: ['bob'],
      reviewers: ['carol'],
      excludeDraftPR: true,
      provider: 'mock',
      model: '',
      skillSet: 'default',
      testProfile: 'go-default',
      toolCommand: '',
      enabled: true,
    })
  })

  it('loads rules, shows invalid repositories, and saves normalized values', async () => {
    const wrapper = mount(WatchRulesPage)
    await flushPromises()

    expect(wrapper.text()).toContain('未登録のリポジトリが含まれています: missing/repo')
    expect(wrapper.text()).toContain('PR レビュー')

    await wrapper.find('label.field-checkbox input').setValue(false)
    await wrapper.find('input[placeholder="^feat:"]').setValue('^feature:')
    await wrapper.find('input[placeholder="alice, bob"]').setValue('alice, bob')
    await wrapper.find('input[placeholder="carol, dave"]').setValue('dave')
    await wrapper.find('input[placeholder="erin, frank"]').setValue('eve')
    await wrapper.find('input[placeholder="Roadmap"]').setValue('Roadmap 2')
    await wrapper.find('textarea').setValue('Status: Todo, In Progress\nIteration: Sprint 12')

    await wrapper.findAll('button').find((button) => button.text() === 'ルールを保存')!.trigger('click')
    await flushPromises()

    expect(fetchMock).toHaveBeenCalledWith(
      '/api/watch-rules',
      expect.objectContaining({
        method: 'PUT',
      }),
    )
    const body = JSON.parse(String(fetchMock.mock.calls.find((call) => call[0] === '/api/watch-rules' && call[1] && (call[1] as RequestInit).method === 'PUT')?.[1]?.body ?? '[]')) as typeof watchRules
    expect(body[0].target).toBe('pull_request_review')
    expect(body[0].projectName).toBe('Roadmap 2')
    expect(body[0].labels).toEqual(['bug', 'ai'])
    expect(body[0].projectFilters).toEqual([
      { field: 'Status', values: ['Todo', 'In Progress'] },
      { field: 'Iteration', values: ['Sprint 12'] },
    ])
    expect(body[0].authors).toEqual(['alice', 'bob'])
    expect(body[0].assignees).toEqual(['dave'])
    expect(body[0].reviewers).toEqual(['eve'])
    expect(body[0].enabled).toBe(false)
    expect(wrapper.text()).toContain('watch-rules.yaml を更新しました。')
  })

  it('adds and removes a rule', async () => {
    const wrapper = mount(WatchRulesPage)
    await flushPromises()

    await wrapper.findAll('button').find((button) => button.text() === 'ルールを追加')!.trigger('click')
    await flushPromises()

    expect(wrapper.text()).toContain('新規ルール 2')
    await wrapper.findAll('button').find((button) => button.text() === '削除')!.trigger('click')
    await flushPromises()

    expect(wrapper.text()).not.toContain('新規ルール 2')
  })
})
