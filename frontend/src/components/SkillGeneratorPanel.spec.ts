import { mount } from '@vue/test-utils'
import { nextTick } from 'vue'
import { vi } from 'vitest'
import SkillGeneratorPanel from './SkillGeneratorPanel.vue'

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

describe('SkillGeneratorPanel', () => {
  const skills = [
    {
      purpose: 'design-from-issue',
      name: 'design-from-issue',
      displayName: 'Design From Issue',
      exists: true,
      aiExists: false,
      generated: false,
      path: '.agents/skills/design-from-issue/SKILL.md',
    },
    {
      purpose: 'implement-from-design',
      name: 'implement-from-design',
      displayName: 'Implement From Design',
      exists: true,
      aiExists: true,
      generated: false,
      path: '.agents/skills/implement-from-design/SKILL.md',
    },
    {
      purpose: 'issue_verification',
      name: 'verifier-from-design',
      displayName: 'Verifier From Design',
      exists: true,
      aiExists: false,
      generated: false,
      path: '.agents/skills/verifier-from-design/SKILL.md',
    },
    {
      purpose: 'review-pull-request',
      name: 'review-pull-request',
      displayName: 'Review Pull Request',
      exists: false,
      aiExists: false,
      generated: true,
      path: '.agents/skills/review-pull-request/SKILL.md',
    },
    {
      purpose: 'pr_conflict_resolution',
      name: 'resolve-pr-conflicts',
      displayName: 'PRのコンフリクト解消',
      exists: true,
      aiExists: true,
      generated: true,
      path: '.agents/skills/resolve-pr-conflicts/SKILL.md',
    },
  ]

  function mockInitialFetch() {
    const fetchMock = vi.fn()
    fetchMock
      .mockResolvedValueOnce(jsonResponse({ skills }))
      .mockResolvedValueOnce(
        jsonResponse({
          aiProvider: 'codex',
        }),
      )
    vi.stubGlobal('fetch', fetchMock)
    return fetchMock
  }

  beforeEach(() => {
    window.localStorage.clear()
  })

  it('loads skills with all items selected by default', async () => {
    mockInitialFetch()

    const wrapper = mount(SkillGeneratorPanel)
    await flushPromises()

    expect(wrapper.text()).toContain('ローカル存在')
    expect(wrapper.text()).toContain('AI確認済み')
    expect(wrapper.text()).toContain('AI生成済み')

    const checkboxes = wrapper.findAll('input[type="checkbox"]')
    expect(checkboxes).toHaveLength(6)
    expect(checkboxes.every((checkbox) => (checkbox.element as HTMLInputElement).checked)).toBe(true)
  })

  it('sends only checked purposes when generating and regenerating skills', async () => {
    const fetchMock = mockInitialFetch()

    const wrapper = mount(SkillGeneratorPanel)
    await flushPromises()

    const checkboxes = wrapper.findAll('input[type="checkbox"]')
    await checkboxes[1].setChecked(false)
    await wrapper.get('textarea[placeholder^="go test ./..."]').setValue('go test ./...\ngo test ./internal/app')

    await (wrapper.vm as unknown as { generateSelectedSkills: () => Promise<void> }).generateSelectedSkills()
    await flushPromises()

    expect(fetchMock).toHaveBeenNthCalledWith(
      3,
      '/api/skills',
      expect.objectContaining({
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
      }),
    )

    const generateRequest = fetchMock.mock.calls[2]
    expect(JSON.parse(generateRequest[1]?.body as string)).toEqual({
      projectContext: '',
      testCommand: 'go test ./...\ngo test ./internal/app',
      maxFixLoops: 3,
      forcePurposes: ['implement-from-design', 'issue_verification', 'review-pull-request', 'pr_conflict_resolution'],
      overwriteExisting: false,
    })

    await (wrapper.vm as unknown as { regenerateSelectedSkills: () => Promise<void> }).regenerateSelectedSkills()
    await flushPromises()

    const regenerateRequest = fetchMock.mock.calls[3]
    expect(JSON.parse(regenerateRequest[1]?.body as string)).toEqual({
      projectContext: '',
      testCommand: 'go test ./...\ngo test ./internal/app',
      maxFixLoops: 3,
      forcePurposes: ['implement-from-design', 'issue_verification', 'review-pull-request', 'pr_conflict_resolution'],
      overwriteExisting: true,
    })
  })

  it('restores saved generation form from localStorage', async () => {
    window.localStorage.setItem(
      'korobokcle.skillGenerationForm.v1',
      JSON.stringify({
        projectContext: 'バックエンドはGo、フロントはVue',
        testCommand: 'go test ./...\nnpm test',
        maxFixLoops: 5,
      }),
    )
    mockInitialFetch()

    const wrapper = mount(SkillGeneratorPanel)
    await flushPromises()

    expect(wrapper.get('textarea[placeholder="使用言語、フレームワーク、設計規約、確認必須事項など"]').element).toHaveProperty(
      'value',
      'バックエンドはGo、フロントはVue',
    )
    expect(wrapper.get('textarea[placeholder^="go test ./..."]').element).toHaveProperty(
      'value',
      'go test ./...\nnpm test',
    )
    expect(wrapper.get('input[type="number"]').element).toHaveProperty('value', '5')
  })
})
