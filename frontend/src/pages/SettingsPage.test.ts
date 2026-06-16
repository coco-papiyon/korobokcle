import { flushPromises, mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'

import SettingsPage from './SettingsPage.vue'

const state = {
  appConfig: {
    provider: 'copilot',
    model: 'gpt-4.1',
    copilotAllowTools: ['write', 'shell'],
    pollInterval: 120,
    screenRefreshInterval: 5,
    shutdownTimeout: 10,
    prTitleTemplate: 'PR {{issue_number}}',
    branchTemplate: 'issue/{{issue_number}}',
    monitoredRepositories: [],
    providers: [
      { name: 'mock', models: ['default'] },
      { name: 'copilot', models: ['gpt-4.1', 'o4-mini'] },
    ],
  },
  notificationConfig: {
    channels: [
      {
        name: 'Slack',
        type: 'windows_toast',
        events: ['waiting_design_approval'],
        enabled: true,
      },
    ],
  },
}

const fetchMock = vi.fn(async (input: RequestInfo | URL, init?: RequestInit) => {
  const path = String(input)
  if (path === '/api/app-config' && !init) {
    return jsonResponse(state.appConfig)
  }
  if (path === '/api/notification-config' && !init) {
    return jsonResponse(state.notificationConfig)
  }
  if (path === '/api/app-config' && init?.method === 'PUT') {
    const body = JSON.parse(String(init.body ?? '{}')) as Partial<typeof state.appConfig>
    state.appConfig = { ...state.appConfig, ...body }
    return jsonResponse(state.appConfig)
  }
  if (path === '/api/notification-config' && init?.method === 'PUT') {
    const body = JSON.parse(String(init.body ?? '{}')) as typeof state.notificationConfig
    state.notificationConfig = {
      channels: body.channels.map((channel) => ({
        ...channel,
        events: [...channel.events],
      })),
    }
    return jsonResponse(state.notificationConfig)
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

describe('SettingsPage', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    state.appConfig = {
      provider: 'copilot',
      model: 'gpt-4.1',
      copilotAllowTools: ['write', 'shell'],
      pollInterval: 120,
      screenRefreshInterval: 5,
      shutdownTimeout: 10,
      prTitleTemplate: 'PR {{issue_number}}',
      branchTemplate: 'issue/{{issue_number}}',
      monitoredRepositories: [],
      providers: [
        { name: 'mock', models: ['default'] },
        { name: 'copilot', models: ['gpt-4.1', 'o4-mini'] },
      ],
    }
    state.notificationConfig = {
      channels: [
        {
          name: 'Slack',
          type: 'windows_toast',
          events: ['waiting_design_approval'],
          enabled: true,
        },
      ],
    }
  })

  it('shows provider specific controls and saves both configurations', async () => {
    const wrapper = mount(SettingsPage)
    await flushPromises()

    expect(wrapper.text()).toContain('アプリケーション設定')
    expect(wrapper.text()).toContain('通知設定')
    expect(wrapper.text()).toContain('Copilot 許可ツール')
    expect(wrapper.get('textarea').element).toBeInstanceOf(HTMLTextAreaElement)

    await wrapper.findAll('button').find((button) => button.text() === '設定を保存')!.trigger('click')
    await flushPromises()

    expect(fetchMock).toHaveBeenCalledWith(
      '/api/app-config',
      expect.objectContaining({
        method: 'PUT',
      }),
    )
    expect(wrapper.text()).toContain('app.yaml を更新しました。')

    const failedCheckbox = wrapper.findAll('input[type="checkbox"]').find((input) => input.element.parentElement?.textContent?.includes('失敗時'))
    expect(failedCheckbox).toBeDefined()
    await failedCheckbox!.setValue(true)

    await wrapper.findAll('button').find((button) => button.text() === '通知を保存')!.trigger('click')
    await flushPromises()

    expect(fetchMock).toHaveBeenCalledWith(
      '/api/notification-config',
      expect.objectContaining({
        method: 'PUT',
      }),
    )
    expect(wrapper.text()).toContain('notifications.yaml を更新しました。')
    expect(state.notificationConfig.channels[0].events).toContain('failed')
  })

  it('rejects invalid numeric values before saving', async () => {
    const wrapper = mount(SettingsPage)
    await flushPromises()

    const inputs = wrapper.findAll('input[type="number"]')
    await inputs[0].setValue('')

    await wrapper.findAll('button').find((button) => button.text() === '設定を保存')!.trigger('click')
    await flushPromises()

    expect(wrapper.text()).toContain('Git ポーリング間隔 は必須です。')
    expect(fetchMock).not.toHaveBeenCalledWith(
      '/api/app-config',
      expect.objectContaining({
        method: 'PUT',
      }),
    )
  })
})
