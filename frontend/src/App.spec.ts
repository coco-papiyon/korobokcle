import { mount } from '@vue/test-utils'
import { nextTick } from 'vue'
import { vi } from 'vitest'
import App from './App.vue'

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
  it('keeps the tab navigation and opens job details in a modal from the job list', async () => {
    const fetchMock = vi.fn((input: RequestInfo | URL, init?: RequestInit) => {
      const url = String(input)
      if (url === '/api/settings') {
        return Promise.resolve(jsonResponse({ aiProvider: 'codex' }))
      }
      if (url === '/api/skills') {
        return Promise.resolve(jsonResponse({ skills: [] }))
      }
      if (url === '/api/jobs') {
        return Promise.resolve(
          jsonResponse({
            updatedAt: '2026-07-01T00:00:00Z',
            jobs: [
              {
                id: 'job-1',
                kind: 'issue_implementation',
                state: 'implementation_ready',
                repository: 'owner/repo',
                number: 1,
                title: '一覧から開くジョブ',
              },
            ],
          }),
        )
      }
      if (url === '/api/jobs/job-1') {
        return Promise.resolve(
          jsonResponse({
            updatedAt: '2026-07-01T00:00:00Z',
            job: {
              id: 'job-1',
              kind: 'issue_implementation',
              state: 'implementation_ready',
              repository: 'owner/repo',
              number: 1,
              title: '一覧から開くジョブ',
            },
          }),
        )
      }
      if (url === '/api/jobs/job-1/artifact') {
        if (init?.method === 'POST' || init?.method === 'PATCH') {
          return Promise.resolve(new Response(null, { status: 204 }))
        }
        return Promise.resolve(
          jsonResponse({
            content: 'artifact content',
            path: 'artifact.md',
          }),
        )
      }
      return Promise.reject(new Error(`Unexpected fetch: ${url}`))
    })
    vi.stubGlobal('fetch', fetchMock)

    const wrapper = mount(App)
    await flushPromises()

    const tabs = wrapper.findAll('button[role="tab"]')
    expect(tabs).toHaveLength(3)
    expect(wrapper.get('.tab-description').text()).toBe('監視中のジョブ一覧を確認し、処理対象を選択する。')

    await tabs[1].trigger('click')
    await nextTick()
    expect(wrapper.get('.tab-description').text()).toBe('AI プロバイダーと監視条件をまとめて設定する。')

    await tabs[0].trigger('click')
    await nextTick()
    expect(wrapper.get('.tab-description').text()).toBe('Issue駆動開発に必要な Agent Skill を監視対象リポジトリへ生成する。')

    await tabs[2].trigger('click')
    await nextTick()
    expect(wrapper.get('.tab-description').text()).toBe('監視中のジョブ一覧を確認し、処理対象を選択する。')

    await wrapper.get('tbody tr').trigger('click')
    await flushPromises()

    const dialog = wrapper.get('[role="dialog"]')
    expect(dialog.text()).toContain('一覧から開くジョブ')
    expect(dialog.text()).toContain('artifact content')

    await wrapper.get('button[aria-label="閉じる"]').trigger('click')
    await flushPromises()

    expect(wrapper.find('[role="dialog"]').exists()).toBe(false)
  })
})
