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
  it('shows three tabs and opens the detail modal from the job list', async () => {
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
        return Promise.resolve(
          jsonResponse({
            updatedAt: '2026-07-01T00:00:00Z',
            jobs: [
              {
                id: 'job-1',
                kind: 'issue_implementation',
                state: 'implementation_running',
                repository: 'owner/repo',
                number: 1,
                title: '実装中ジョブ',
              },
            ],
          }),
        )
      }
      if (url === '/api/jobs/job-1') {
        return Promise.resolve(
          jsonResponse({
            updatedAt: '2026-07-01T00:00:00Z',
            branch: 'issue_#1',
            job: {
              id: 'job-1',
              kind: 'issue_implementation',
              state: 'implementation_running',
              repository: 'owner/repo',
              number: 1,
              title: '実装中ジョブ',
              fetchedAt: '2026-07-01T00:00:00Z',
              updatedAt: '2026-07-01T03:04:05Z',
            },
          }),
        )
      }
      return Promise.reject(new Error(`Unexpected fetch: ${url}`))
    })
    vi.stubGlobal('fetch', fetchMock)

    const wrapper = mount(App)
    await flushPromises()

    const tabs = wrapper.findAll('button[role="tab"]')
    const description = () => wrapper.get('.tab-description__text').text()

    expect(tabs).toHaveLength(3)
    expect(description()).toBe('監視中のジョブ一覧を確認し、処理対象を選択する。')

    await tabs[1].trigger('click')
    await nextTick()
    expect(description()).toBe('Issue駆動開発に必要な Agent Skill を監視対象リポジトリへ生成する。')

    await tabs[0].trigger('click')
    await nextTick()
    expect(description()).toBe('監視中のジョブ一覧を確認し、処理対象を選択する。')

    await tabs[2].trigger('click')
    await nextTick()
    expect(description()).toBe('AI プロバイダーと監視条件をまとめて設定する。')

    await tabs[0].trigger('click')
    await flushPromises()

    const row = wrapper.get('tbody tr')
    await row.trigger('click')
    await flushPromises()

    expect(wrapper.get('[role="dialog"]').text()).toContain('ジョブ詳細')
    expect(wrapper.get('tbody tr').classes()).toContain('job-table__row--active')

    await wrapper.get('.modal-dialog__close').trigger('click')
    await nextTick()

    expect(wrapper.find('[role="dialog"]').exists()).toBe(false)
  })
})
