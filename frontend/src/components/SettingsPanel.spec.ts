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
          labelIncludes: ['bug', 'ai:design'],
          labelExcludes: ['wip'],
          titleContains: ['fix'],
          authors: ['alice'],
          assignees: ['bob'],
        },
        pullRequest: {
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

    const inputs = wrapper.findAll('input')
    const selects = wrapper.findAll('select')
    const headings = wrapper.findAll('h2').map((heading) => heading.text())

    expect(headings).toContain('プロバイダー設定')
    expect(headings).toContain('監視設定')
    expect(inputs[0].element).toHaveProperty('value', 'owner/repository')
    expect(inputs[1].element).toHaveProperty('value', '120')
    expect(inputs[2].element).toHaveProperty('value', '4')
    expect(inputs[3].element).toHaveProperty('value', 'main')
    expect(inputs[4].element).toHaveProperty('value', 'issue_#<issue番号>')
    expect(inputs[5].element).toHaveProperty('value', 'bug, ai:design')
    expect(inputs[6].element).toHaveProperty('value', 'wip')
    expect(inputs[7].element).toHaveProperty('value', 'fix')
    expect(inputs[8].element).toHaveProperty('value', 'alice')
    expect(inputs[9].element).toHaveProperty('value', 'bob')
    expect(inputs[10].element).toHaveProperty('value', 'ready')
    expect(inputs[11].element).toHaveProperty('value', 'draft')
    expect(inputs[12].element).toHaveProperty('value', 'update')
    expect(inputs[13].element).toHaveProperty('value', 'carol')
    expect(inputs[14].element).toHaveProperty('value', 'dave')
    expect(selects[0].element).toHaveProperty('value', 'codex')
    expect(selects[1].element).toHaveProperty('value', 'gpt-5.5')
    const textareas = wrapper.findAll('textarea')
    expect(textareas[0].element).toHaveProperty('value', 'npm ci\ngo test ./...')

    await inputs[0].setValue(' owner/new-repository ')
    await inputs[1].setValue('59.7')
    await inputs[2].setValue('6')
    await inputs[3].setValue(' release ')
    await inputs[4].setValue(' issue_#<issue番号> ')
    await inputs[5].setValue('bug, ai:design, docs')
    await inputs[10].setValue('ready, review')
    await textareas[0].setValue('npm ci\nnpm test\n')
    await selects[0].setValue('github_copilot')
    await selects[1].setValue('claude-opus-4.6')

    await wrapper.get('button:not(.button--ghost)').trigger('click')
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
        labelIncludes: ['bug', 'ai:design', 'docs'],
        labelExcludes: ['wip'],
        titleContains: ['fix'],
        authors: ['alice'],
        assignees: ['bob'],
      },
      pullRequest: {
        labelIncludes: ['ready', 'review'],
        labelExcludes: ['draft'],
        titleContains: ['update'],
        authors: ['carol'],
        assignees: ['dave'],
      },
    })
  })
})
