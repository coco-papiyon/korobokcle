import { flushPromises, mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'

import ToolCommandsPage from './ToolCommandsPage.vue'

const commands = [
  { name: 'default-tool', command: 'npm run dev', resident: true },
]

const fetchMock = vi.fn(async (input: RequestInfo | URL, init?: RequestInit) => {
  const path = String(input)
  if (path === '/api/tool-commands' && !init) {
    return jsonResponse(structuredClone(commands))
  }
  if (path === '/api/tool-commands' && init?.method === 'PUT') {
    const body = JSON.parse(String(init.body ?? '[]')) as typeof commands
    commands.splice(0, commands.length, ...structuredClone(body))
    return jsonResponse(structuredClone(commands))
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

describe('ToolCommandsPage', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    commands.splice(0, commands.length, { name: 'default-tool', command: 'npm run dev', resident: true })
  })

  it('adds, saves, and removes commands', async () => {
    const wrapper = mount(ToolCommandsPage)
    await flushPromises()

    expect(wrapper.text()).toContain('default-tool')
    await wrapper.find('button.button-primary').trigger('click')
    await flushPromises()

    await wrapper.find('input[type="text"]').setValue('lint-tool')
    await wrapper.find('textarea').setValue('npm run lint')
    await wrapper.find('input[type="checkbox"]').setValue(false)
    await wrapper.findAll('button').find((button) => button.text() === 'コマンドを保存')!.trigger('click')
    await flushPromises()

    expect(fetchMock).toHaveBeenCalledWith(
      '/api/tool-commands',
      expect.objectContaining({
        method: 'PUT',
      }),
    )
    expect(wrapper.text()).toContain('tool-commands.yaml を更新しました。')

    await wrapper.findAll('button').find((button) => button.text() === '削除')!.trigger('click')
    await flushPromises()

    expect(wrapper.text()).not.toContain('lint-tool')
  })

  it('rejects empty command names before saving', async () => {
    const wrapper = mount(ToolCommandsPage)
    await flushPromises()

    await wrapper.find('button.button-primary').trigger('click')
    await flushPromises()

    await wrapper.find('input[type="text"]').setValue('')
    await wrapper.findAll('button').find((button) => button.text() === 'コマンドを保存')!.trigger('click')
    await flushPromises()

    expect(wrapper.text()).toContain('toolCommand[1].name は必須です。')
    expect(fetchMock).not.toHaveBeenCalledWith(
      '/api/tool-commands',
      expect.objectContaining({
        method: 'PUT',
      }),
    )
  })
})
