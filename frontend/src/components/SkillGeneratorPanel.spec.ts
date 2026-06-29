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
      purpose: 'review-pull-request',
      name: 'review-pull-request',
      displayName: 'Review Pull Request',
      exists: false,
      aiExists: false,
      generated: true,
      path: '.agents/skills/review-pull-request/SKILL.md',
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

  it('loads skills with all items selected by default', async () => {
    mockInitialFetch()

    const wrapper = mount(SkillGeneratorPanel)
    await flushPromises()

    expect(wrapper.text()).toContain('ローカル存在')
    expect(wrapper.text()).toContain('AI確認済み')
    expect(wrapper.text()).toContain('AI生成済み')

    const checkboxes = wrapper.findAll('input[type="checkbox"]')
    expect(checkboxes).toHaveLength(4)
    expect(checkboxes.every((checkbox) => (checkbox.element as HTMLInputElement).checked)).toBe(true)
    expect(wrapper.get('button:not(.button--ghost)').text()).toContain('選択スキルを生成 (3)')
    expect(wrapper.findAll('button.button--ghost').at(-1)?.text()).toContain('選択スキルを再生成 (3)')
  })

  it('sends only checked purposes when generating and regenerating skills', async () => {
    const fetchMock = mockInitialFetch()

    const wrapper = mount(SkillGeneratorPanel)
    await flushPromises()

    const checkboxes = wrapper.findAll('input[type="checkbox"]')
    await checkboxes[1].setChecked(false)
    await wrapper.get('textarea[placeholder^="go test ./..."]').setValue('go test ./...\ngo test ./internal/app')

    expect(wrapper.get('button:not(.button--ghost)').text()).toContain('選択スキルを生成 (2)')

    await wrapper.get('button:not(.button--ghost)').trigger('click')
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
      forcePurposes: ['implement-from-design', 'review-pull-request'],
      overwriteExisting: false,
    })

    await wrapper.findAll('button.button--ghost').at(-1)!.trigger('click')
    await flushPromises()

    const regenerateRequest = fetchMock.mock.calls[3]
    expect(JSON.parse(regenerateRequest[1]?.body as string)).toEqual({
      projectContext: '',
      testCommand: 'go test ./...\ngo test ./internal/app',
      maxFixLoops: 3,
      forcePurposes: ['implement-from-design', 'review-pull-request'],
      overwriteExisting: true,
    })
  })
})
