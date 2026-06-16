import { flushPromises, mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'

import SkillSetsPage from './SkillSetsPage.vue'

type SkillSetRecord = {
  name: string
  mutable: boolean
  skills: Record<string, { definition: { name: string; title: string; role: string; promptTemplates: string[] }; promptTemplate: string }>
}

const skillSets: SkillSetRecord[] = []

const fetchMock = vi.fn(async (input: RequestInfo | URL, init?: RequestInit) => {
  const path = String(input)
  if (path === '/api/skillsets' && !init) {
    return jsonResponse(skillSets.map((set) => ({ name: set.name, mutable: set.mutable })))
  }
  if (path.startsWith('/api/skillsets/') && !init) {
    const name = decodeURIComponent(path.slice('/api/skillsets/'.length))
    const set = skillSets.find((item) => item.name === name)
    if (!set) {
      throw new Error(`missing skillset: ${name}`)
    }
    return jsonResponse(structuredClone(set))
  }
  if (path === '/api/skillsets' && init?.method === 'POST') {
    const body = JSON.parse(String(init.body ?? '{}')) as { name: string; source: string }
    const source = skillSets.find((item) => item.name === body.source)
    const created: SkillSetRecord = source
      ? structuredClone({
          ...source,
          name: body.name,
          mutable: true,
        })
      : {
          name: body.name,
          mutable: true,
          skills: {},
        }
    created.name = body.name
    skillSets.push(structuredClone(created))
    return jsonResponse(created)
  }
  if (path.startsWith('/api/skillsets/') && init?.method === 'PUT') {
    const name = decodeURIComponent(path.slice('/api/skillsets/'.length))
    const body = JSON.parse(String(init.body ?? '{}')) as SkillSetRecord
    const index = skillSets.findIndex((item) => item.name === name)
    if (index >= 0) {
      skillSets[index] = structuredClone(body)
    }
    return jsonResponse(body)
  }
  if (path.startsWith('/api/skillsets/') && init?.method === 'DELETE') {
    const name = decodeURIComponent(path.slice('/api/skillsets/'.length))
    const index = skillSets.findIndex((item) => item.name === name)
    if (index >= 0) {
      skillSets.splice(index, 1)
    }
    return jsonResponse({ status: 'ok' })
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

function createSkillSet(name: string, mutable: boolean): SkillSetRecord {
  const skills: SkillSetRecord['skills'] = {}
  ;[
    'design',
    'implement',
    'implement_fix',
    'review',
    'review_fix',
    'improvement_consideration',
    'improvement_implementation',
  ].forEach((skillName) => {
    skills[skillName] = {
      definition: {
        name: skillName,
        title: `${name}-${skillName}`,
        role: `${name} role ${skillName}`,
        promptTemplates: [],
      },
      promptTemplate: `${name} prompt ${skillName}`,
    }
  })
  return {
    name,
    mutable,
    skills,
  }
}

describe('SkillSetsPage', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    skillSets.splice(0, skillSets.length, createSkillSet('default', false), createSkillSet('team-a', true))
  })

  it('loads, edits, saves, creates, and deletes skill sets', async () => {
    const wrapper = mount(SkillSetsPage)
    await flushPromises()

    expect(wrapper.text()).toContain('default')
    expect(wrapper.text()).toContain('team-a')
    expect(wrapper.text()).toContain('default は編集不可です。複製して変更してください。')

    await wrapper.findAll('button.rule-item').find((button) => button.text().includes('team-a'))!.trigger('click')
    await flushPromises()

    const titleInputs = wrapper.findAll('.skill-panel input[type="text"]')
    await titleInputs[0].setValue('updated title')
    await wrapper.findAll('button').find((button) => button.text() === 'スキルセットを保存')!.trigger('click')
    await flushPromises()

    expect(fetchMock).toHaveBeenCalledWith(
      '/api/skillsets/team-a',
      expect.objectContaining({
        method: 'PUT',
      }),
    )
    expect(wrapper.text()).toContain('スキルセットを保存しました。')

    await wrapper.find('.skillset-create input[type="text"]').setValue('team-b')
    await wrapper.find('.skillset-create button.button-primary').trigger('click')
    await flushPromises()

    expect(fetchMock).toHaveBeenCalledWith(
      '/api/skillsets',
      expect.objectContaining({
        method: 'POST',
        body: JSON.stringify({ name: 'team-b', source: 'default' }),
      }),
    )
    expect(wrapper.text()).toContain('team-b')

    await wrapper.findAll('button.rule-item').find((button) => button.text().includes('team-b'))!.trigger('click')
    await flushPromises()
    await wrapper.findAll('button').find((button) => button.text() === '削除')!.trigger('click')
    await flushPromises()

    expect(fetchMock).toHaveBeenCalledWith(
      '/api/skillsets/team-b',
      expect.objectContaining({
        method: 'DELETE',
      }),
    )
  })
})
