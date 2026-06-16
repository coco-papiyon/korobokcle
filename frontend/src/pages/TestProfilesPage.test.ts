import { flushPromises, mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'

import TestProfilesPage from './TestProfilesPage.vue'

const profiles = [
  { name: 'go-default', commands: ['go test ./...'] },
]

const fetchMock = vi.fn(async (input: RequestInfo | URL, init?: RequestInit) => {
  const path = String(input)
  if (path === '/api/test-profiles' && !init) {
    return jsonResponse(structuredClone(profiles))
  }
  if (path === '/api/test-profiles' && init?.method === 'PUT') {
    const body = JSON.parse(String(init.body ?? '[]')) as typeof profiles
    profiles.splice(0, profiles.length, ...structuredClone(body))
    return jsonResponse(structuredClone(profiles))
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

describe('TestProfilesPage', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    profiles.splice(0, profiles.length, { name: 'go-default', commands: ['go test ./...'] })
  })

  it('adds, saves, and removes profiles', async () => {
    const wrapper = mount(TestProfilesPage)
    await flushPromises()

    expect(wrapper.text()).toContain('go-default')
    await wrapper.find('button.button-primary').trigger('click')
    await flushPromises()

    const selectedName = wrapper.find('input[type="text"]')
    await selectedName.setValue('go-unit')
    const commandsArea = wrapper.find('textarea')
    await commandsArea.setValue('go test ./internal/...\n')
    await wrapper.findAll('button').find((button) => button.text() === 'プロファイルを保存')!.trigger('click')
    await flushPromises()

    expect(fetchMock).toHaveBeenCalledWith(
      '/api/test-profiles',
      expect.objectContaining({
        method: 'PUT',
      }),
    )
    expect(wrapper.text()).toContain('test-profiles.yaml を更新しました。')

    await wrapper.findAll('button').find((button) => button.text() === '削除')!.trigger('click')
    await flushPromises()

    expect(wrapper.text()).not.toContain('go-unit')
  })

  it('rejects duplicate profile names before saving', async () => {
    const wrapper = mount(TestProfilesPage)
    await flushPromises()

    await wrapper.find('button.button-primary').trigger('click')
    await flushPromises()

    const selectedName = wrapper.find('input[type="text"]')
    await selectedName.setValue('go-default')
    await wrapper.findAll('button').find((button) => button.text() === 'プロファイルを保存')!.trigger('click')
    await flushPromises()

    expect(wrapper.text()).toContain('profile[1].name は重複できません: go-default')
    expect(fetchMock).not.toHaveBeenCalledWith(
      '/api/test-profiles',
      expect.objectContaining({
        method: 'PUT',
      }),
    )
  })
})
