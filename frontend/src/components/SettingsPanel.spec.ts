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

async function flushPromises() {
  await Promise.resolve()
  await Promise.resolve()
  await new Promise((resolve) => setTimeout(resolve, 0))
  await nextTick()
  await new Promise((resolve) => setTimeout(resolve, 0))
  await nextTick()
}

describe('SettingsPanel', () => {
  function mockInitialFetch() {
    const fetchMock = vi.fn()
    fetchMock.mockResolvedValueOnce(
      jsonResponse({
        repository: 'owner/repository',
        aiProvider: 'codex',
        pollIntervalSeconds: 120,
        jobConcurrency: 4,
        baseBranch: 'main',
        branchNamePattern: 'issue_#<issue番号>',
        aiAllowedCommands: ['npm ci', 'go test ./...'],
        models: {
          codex: { mode: 'custom', value: 'gpt-5.5' },
          githubCopilot: { mode: 'default', value: '' },
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
          labelIncludes: ['ready'],
          labelExcludes: ['draft'],
          titleContains: ['update'],
          authors: ['carol'],
          assignees: ['dave'],
        },
      }),
    )
    vi.stubGlobal('fetch', fetchMock)
    return fetchMock
  }

  it('loads settings into the form and saves normalized payloads', async () => {
    const fetchMock = mockInitialFetch()

    const wrapper = mount(SettingsPanel)
    await flushPromises()

    const repoInput = wrapper.get('input[placeholder="owner/repository"]')
    const numberInputs = wrapper.findAll('input[type="number"]')
    const pollInput = numberInputs[0]
    const concurrencyInput = numberInputs[1]
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

    expect(headings).toContain('プロバイダー設定')
    expect(headings).toContain('監視設定')
    expect(conditionToggles).toHaveLength(2)
    expect((conditionToggles[0].element as HTMLInputElement).checked).toBe(false)
    expect((conditionToggles[1].element as HTMLInputElement).checked).toBe(true)
    expect(repoInput.element).toHaveProperty('value', 'owner/repository')
    expect(pollInput.element).toHaveProperty('value', '120')
    expect(concurrencyInput.element).toHaveProperty('value', '4')
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
    const textareas = wrapper.findAll('textarea')
    expect(textareas[0].element).toHaveProperty('value', 'npm ci\ngo test ./...')

    await repoInput.setValue(' owner/new-repository ')
    await pollInput.setValue('59.7')
    await concurrencyInput.setValue('6')
    await baseBranchInput.setValue(' release ')
    await branchPatternInput.setValue(' issue_#<issue番号> ')
    await prIncludeInput.setValue('ready, review')
    await conditionToggles[0].setChecked(false)
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
      pollIntervalSeconds: 59,
      jobConcurrency: 6,
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
  })
})
