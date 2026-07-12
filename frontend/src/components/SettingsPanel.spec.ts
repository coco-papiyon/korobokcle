import { mount } from '@vue/test-utils'
import { nextTick } from 'vue'
import { vi } from 'vitest'
import SettingsPanel from './SettingsPanel.vue'

type JsonBody = Record<string, unknown>

function jsonResponse(body: JsonBody) {
  return new Response(JSON.stringify(body), {
    status: 200,
    headers: {
      'Content-Type': 'application/json',
    },
  })
}

function textResponse(body: string, status = 400) {
  return new Response(body, {
    status,
    headers: {
      'Content-Type': 'text/plain',
    },
  })
}

function deferred<T>() {
  let resolve!: (value: T | PromiseLike<T>) => void
  let reject!: (reason?: unknown) => void
  const promise = new Promise<T>((res, rej) => {
    resolve = res
    reject = rej
  })
  return { promise, resolve, reject }
}

async function flushPromises() {
  await Promise.resolve()
  await Promise.resolve()
  await new Promise((resolve) => setTimeout(resolve, 0))
  await nextTick()
  await new Promise((resolve) => setTimeout(resolve, 0))
  await nextTick()
}

function baseSettings(overrides: JsonBody = {}) {
  return {
    repository: 'owner/repository',
    aiProvider: 'codex',
    startupCommand: 'npm run dev',
    residentMode: true,
    pollIntervalSeconds: 120,
    jobConcurrency: 4,
    implementationLoopCount: 3,
    baseBranch: 'main',
    branchNamePattern: 'issue_#<issue番号>',
    aiAllowedCommands: ['npm ci', 'go test ./...'],
    models: {
      codex: { mode: 'custom', value: 'gpt-5.5' },
      githubCopilot: { mode: 'default', value: '' },
    },
    verificationAiProvider: '',
    verificationAiModel: { mode: 'default', value: '' },
    reviewerAiProvider: '',
    reviewerAiModel: { mode: 'default', value: '' },
    issue: {
      enabled: true,
      labelIncludes: ['bug', 'ai:design'],
      labelExcludes: ['wip'],
      titleContains: ['fix'],
      authors: ['alice'],
      assignees: ['bob'],
    },
    pullRequest: {
      enabled: true,
      labelIncludes: ['ready'],
      labelExcludes: ['draft'],
      titleContains: ['update'],
      authors: ['carol'],
      assignees: ['dave'],
    },
    ...overrides,
  }
}

describe('SettingsPanel', () => {
  function mockInitialFetch(overrides: JsonBody = {}) {
    const fetchMock = vi.fn()
    fetchMock.mockResolvedValueOnce(jsonResponse(baseSettings(overrides)))
    vi.stubGlobal('fetch', fetchMock)
    return fetchMock
  }

  it('loads settings into the form and saves normalized payloads', async () => {
    const fetchMock = mockInitialFetch()
    fetchMock.mockResolvedValueOnce(
      jsonResponse(
        baseSettings({
          repository: 'owner/new-repository',
          aiProvider: 'github_copilot',
          pollIntervalSeconds: 59,
          jobConcurrency: 6,
          implementationLoopCount: 5,
          baseBranch: 'release',
          aiAllowedCommands: ['npm ci', 'npm test'],
          models: {
            codex: { mode: 'custom', value: 'gpt-5.5' },
            githubCopilot: { mode: 'custom', value: 'claude-opus-4.6' },
          },
          verificationAiProvider: 'codex',
          verificationAiModel: { mode: 'custom', value: 'gpt-5.4-mini' },
          reviewerAiProvider: 'github_copilot',
          reviewerAiModel: { mode: 'custom', value: 'claude-opus-4.6' },
          issue: {
            enabled: false,
            labelIncludes: ['bug', 'ai:design'],
            labelExcludes: ['wip'],
            titleContains: ['fix'],
            authors: ['alice'],
            assignees: ['bob'],
          },
          pullRequest: {
            enabled: true,
            labelIncludes: ['ready', 'review'],
            labelExcludes: ['draft'],
            titleContains: ['update'],
            authors: ['carol'],
            assignees: ['dave'],
          },
        }),
      ),
    )

    const wrapper = mount(SettingsPanel)
    await flushPromises()

    const repoInput = wrapper.get('input[placeholder="owner/repository"]')
    const startupCommandInput = wrapper.get('textarea[placeholder^="cd /d"]')
    const numberInputs = wrapper.findAll('input[type="number"]')
    const concurrencyInput = numberInputs[0]
    const implementationLoopInput = numberInputs[1]
    const pollInput = numberInputs[2]
    const baseBranchInput = wrapper.get('input[placeholder="main"]')
    const branchPatternInput = wrapper.get('input[placeholder="issue_#<issue番号>"]')
    const issueIncludeInput = wrapper.get('input[placeholder="bug, ai:design"]')
    const issueExcludeInput = wrapper.get('input[placeholder="wip, draft"]')
    const issueTitleInput = wrapper.get('input[placeholder="fix, refactor"]')
    const issueAuthorsInput = wrapper.get('input[placeholder="alice, bob"]')
    const issueAssigneesInput = wrapper.get('input[placeholder="carol, dave"]')
    const prIncludeInput = wrapper.get('input[placeholder="ready, review"]')
    const prExcludeInput = wrapper.findAll('input[placeholder="wip, draft"]')[1]
    const prTitleInput = wrapper.get('input[placeholder="fix, update"]')
    const prAuthorsInput = wrapper.findAll('input[placeholder="alice, bob"]')[1]
    const prAssigneesInput = wrapper.findAll('input[placeholder="carol, dave"]')[1]
    const conditionToggles = wrapper.findAll('input[type="checkbox"]')
    const selects = wrapper.findAll('select')
    const headings = wrapper.findAll('h2').map((heading) => heading.text())
    const roleHeadings = wrapper.findAll('.model-role-row h4').map((heading) => heading.text())

    expect(headings).toEqual(['プロバイダー設定', '実行設定', '監視設定'])
    expect(roleHeadings).toEqual(['実装者', '検証者', 'レビューア'])
    expect(conditionToggles).toHaveLength(3)
    expect((conditionToggles[0].element as HTMLInputElement).checked).toBe(true)
    expect((conditionToggles[1].element as HTMLInputElement).checked).toBe(true)
    expect((conditionToggles[2].element as HTMLInputElement).checked).toBe(true)
    expect(repoInput.element).toHaveProperty('value', 'owner/repository')
    expect(startupCommandInput.element).toHaveProperty('value', 'cd /d ".\\tests\\mock-app"\nnpm run dev')
    expect(pollInput.element).toHaveProperty('value', '120')
    expect(concurrencyInput.element).toHaveProperty('value', '4')
    expect(implementationLoopInput.element).toHaveProperty('value', '3')
    expect(baseBranchInput.element).toHaveProperty('value', 'main')
    expect(branchPatternInput.element).toHaveProperty('value', 'issue_#<issue番号>')
    expect(issueIncludeInput.element).toHaveProperty('value', 'bug, ai:design')
    expect(issueExcludeInput.element).toHaveProperty('value', 'wip')
    expect(issueTitleInput.element).toHaveProperty('value', 'fix')
    expect(issueAuthorsInput.element).toHaveProperty('value', 'alice')
    expect(issueAssigneesInput.element).toHaveProperty('value', 'bob')
    expect(prIncludeInput.element).toHaveProperty('value', 'ready')
    expect(prExcludeInput.element).toHaveProperty('value', 'draft')
    expect(prTitleInput.element).toHaveProperty('value', 'update')
    expect(prAuthorsInput.element).toHaveProperty('value', 'carol')
    expect(prAssigneesInput.element).toHaveProperty('value', 'dave')
    expect(selects[0].element).toHaveProperty('value', 'codex')
    expect(selects[1].element).toHaveProperty('value', 'gpt-5.5')
    expect(selects[2].element).toHaveProperty('value', '')
    expect(selects[3].element).toHaveProperty('value', 'default')
    expect(selects[4].element).toHaveProperty('value', '')
    expect(selects[5].element).toHaveProperty('value', 'default')
    const textareas = wrapper.findAll('textarea')
    expect(textareas[0].element).toHaveProperty('value', 'cd /d ".\\tests\\mock-app"\nnpm run dev')
    expect(textareas[1].element).toHaveProperty('value', 'npm ci\ngo test ./...')

    await repoInput.setValue(' owner/new-repository ')
    await startupCommandInput.setValue(' cd /d ".\\tests\\mock-app"\nnpm run dev -- --host ')
    await pollInput.setValue('59.7')
    await concurrencyInput.setValue('6')
    await implementationLoopInput.setValue('5')
    await baseBranchInput.setValue(' release ')
    await branchPatternInput.setValue(' issue_#<issue番号> ')
    await prIncludeInput.setValue('ready, review')
    await selects[2].setValue('codex')
    await selects[3].setValue('gpt-5.4-mini')
    await selects[4].setValue('github_copilot')
    await selects[5].setValue('claude-opus-4.6')
    await conditionToggles[1].setChecked(false)
    await textareas[0].setValue('npm ci\nnpm test\n')
    await selects[0].setValue('github_copilot')
    await selects[1].setValue('claude-opus-4.6')

    await (wrapper.vm as unknown as { saveSettings: () => Promise<void> }).saveSettings()
    await flushPromises()

    expect(fetchMock).toHaveBeenNthCalledWith(
      2,
      '/api/settings',
      expect.objectContaining({
        method: 'PUT',
        headers: { 'Content-Type': 'application/json' },
      }),
    )

    const request = fetchMock.mock.calls[1]
    expect(JSON.parse(request[1]?.body as string)).toEqual({
      repository: 'owner/new-repository',
      aiProvider: 'github_copilot',
      startupCommand: 'cd /d ".\\tests\\mock-app"\nnpm run dev -- --host',
      residentMode: true,
      pollIntervalSeconds: 59,
      jobConcurrency: 6,
      implementationLoopCount: 5,
      verificationAiProvider: 'codex',
      verificationAiModel: { mode: 'custom', value: 'gpt-5.4-mini' },
      reviewerAiProvider: 'github_copilot',
      reviewerAiModel: { mode: 'custom', value: 'claude-opus-4.6' },
      baseBranch: 'release',
      branchNamePattern: 'issue_#<issue番号>',
      aiAllowedCommands: ['npm ci', 'npm test'],
      models: {
        codex: { mode: 'custom', value: 'gpt-5.5' },
        githubCopilot: { mode: 'custom', value: 'claude-opus-4.6' },
      },
      issue: {
        enabled: false,
        labelIncludes: ['bug', 'ai:design'],
        labelExcludes: ['wip'],
        titleContains: ['fix'],
        authors: ['alice'],
        assignees: ['bob'],
      },
      pullRequest: {
        enabled: true,
        labelIncludes: ['ready', 'review'],
        labelExcludes: ['draft'],
        titleContains: ['update'],
        authors: ['carol'],
        assignees: ['dave'],
      },
    })
    expect(wrapper.get('p.success').text()).toBe('設定を保存しました')
    expect(wrapper.get('p.success').attributes('role')).toBe('status')
    expect(wrapper.get('p.success').attributes('aria-live')).toBe('polite')
    expect(wrapper.get('input[placeholder="owner/repository"]').element).toHaveProperty('value', 'owner/new-repository')
  })

  it('shows an error without a success message when saving fails', async () => {
    const fetchMock = vi.fn()
    fetchMock.mockResolvedValueOnce(jsonResponse(baseSettings()))
    fetchMock.mockResolvedValueOnce(textResponse('保存に失敗しました', 500))
    vi.stubGlobal('fetch', fetchMock)

    const wrapper = mount(SettingsPanel)
    await flushPromises()

    await (wrapper.vm as unknown as { saveSettings: () => Promise<void> }).saveSettings()
    await flushPromises()

    expect(wrapper.find('p.success').exists()).toBe(false)
    expect(wrapper.get('p.error').text()).toBe('保存に失敗しました')
  })

  it('clears the previous success message before the next save starts', async () => {
    const fetchMock = vi.fn()
    fetchMock.mockResolvedValueOnce(jsonResponse(baseSettings()))
    fetchMock.mockResolvedValueOnce(jsonResponse(baseSettings()))
    const pendingSave = deferred<Response>()
    fetchMock.mockImplementationOnce(() => pendingSave.promise)
    vi.stubGlobal('fetch', fetchMock)

    const wrapper = mount(SettingsPanel)
    await flushPromises()

    await (wrapper.vm as unknown as { saveSettings: () => Promise<void> }).saveSettings()
    await flushPromises()
    expect(wrapper.get('p.success').text()).toBe('設定を保存しました')

    const savePromise = (wrapper.vm as unknown as { saveSettings: () => Promise<void> }).saveSettings()
    await nextTick()

    expect(wrapper.find('p.success').exists()).toBe(false)

    pendingSave.resolve(jsonResponse(baseSettings()))
    await savePromise
    await flushPromises()
    expect(wrapper.get('p.success').text()).toBe('設定を保存しました')
  })

  it('does not send another save request while one is already in flight', async () => {
    const fetchMock = vi.fn()
    fetchMock.mockResolvedValueOnce(jsonResponse(baseSettings()))
    const pendingSave = deferred<Response>()
    fetchMock.mockImplementationOnce(() => pendingSave.promise)
    vi.stubGlobal('fetch', fetchMock)

    const wrapper = mount(SettingsPanel)
    await flushPromises()

    const saveSettings = wrapper.vm as unknown as { saveSettings: () => Promise<void> }
    const firstSave = saveSettings.saveSettings()
    await nextTick()
    const requestCountAfterFirstSave = fetchMock.mock.calls.length

    await saveSettings.saveSettings()
    expect(fetchMock.mock.calls.length).toBe(requestCountAfterFirstSave)

    pendingSave.resolve(jsonResponse(baseSettings()))
    await firstSave
    await flushPromises()
    expect(wrapper.get('p.success').text()).toBe('設定を保存しました')
  })
})
