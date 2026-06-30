import { mount } from '@vue/test-utils'
import { nextTick } from 'vue'
import { vi } from 'vitest'
import App from './App.vue'
import JobDetailPanel from './components/JobDetailPanel.vue'
import JobListPanel from './components/JobListPanel.vue'

function jsonResponse(body: unknown) {
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

describe('App', () => {
  it('shows shared tab descriptions and switches them by tab', async () => {
    const fetchMock = vi.fn((input: RequestInfo | URL) => {
      const url = String(input)
      if (url === '/api/settings') {
        return Promise.resolve(
          jsonResponse({
            aiProvider: 'codex',
          }),
        )
      }
      if (url === '/api/skills') {
        return Promise.resolve(jsonResponse({ skills: [] }))
      }
      if (url === '/api/jobs') {
        return Promise.resolve(jsonResponse({ jobs: [] }))
      }
      return Promise.reject(new Error(`Unexpected fetch: ${url}`))
    })
    vi.stubGlobal('fetch', fetchMock)

    const wrapper = mount(App)
    await flushPromises()

    const tabs = wrapper.findAll('button[role="tab"]')
    const description = () => wrapper.get('.tab-description').text()

    expect(description()).toBe('監視中のジョブ一覧を確認し、処理対象を選択する。')

    await tabs[1].trigger('click')
    await nextTick()
    expect(description()).toBe('AI プロバイダーと監視条件をまとめて設定する。')

    await tabs[0].trigger('click')
    await nextTick()
    expect(description()).toBe('Issue駆動開発に必要な Agent Skill を監視対象リポジトリへ生成する。')

    await tabs[3].trigger('click')
    await nextTick()
    expect(description()).toBe('選択したジョブの詳細と生成物を確認し、必要なら再実行や承認を行う。')
  })

  it('passes active only to the visible job panels', async () => {
    const fetchMock = vi.fn((input: RequestInfo | URL) => {
      const url = String(input)
      if (url === '/api/settings') {
        return Promise.resolve(
          jsonResponse({
            aiProvider: 'codex',
          }),
        )
      }
      if (url === '/api/skills') {
        return Promise.resolve(jsonResponse({ skills: [] }))
      }
      if (url === '/api/jobs') {
        return Promise.resolve(jsonResponse({ jobs: [] }))
      }
      return Promise.reject(new Error(`Unexpected fetch: ${url}`))
    })
    vi.stubGlobal('fetch', fetchMock)

    const wrapper = mount(App)
    await flushPromises()

    expect(wrapper.getComponent(JobListPanel).props('active')).toBe(true)
    expect(wrapper.getComponent(JobDetailPanel).props('active')).toBe(false)

    const tabs = wrapper.findAll('button[role="tab"]')
    await tabs[3].trigger('click')
    await nextTick()

    expect(wrapper.getComponent(JobListPanel).props('active')).toBe(false)
    expect(wrapper.getComponent(JobDetailPanel).props('active')).toBe(true)
  })
})
