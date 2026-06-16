import { flushPromises, mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'

import DashboardPage from './DashboardPage.vue'

type JobRecord = {
  id: string
  type: string
  repository: string
  githubNumber: number
  state: string
  title: string
  branchName: string
  watchRuleId: string
  deletedAt?: string
  createdAt: string
  updatedAt: string
}

const jobs: JobRecord[] = []
const fetchMock = vi.fn(async (input: RequestInfo | URL) => {
  const path = String(input)
  if (path === '/api/app-config') {
    return jsonResponse({ screenRefreshInterval: 0 })
  }
  if (path.startsWith('/api/jobs?deleted=')) {
    const deleted = new URL(path, 'http://localhost').searchParams.get('deleted')
    const filtered = deleted === 'only' ? jobs.filter((job) => job.deletedAt) : jobs.filter((job) => !job.deletedAt)
    return jsonResponse(filtered)
  }
  if (path.endsWith('/delete') && path.includes('/api/jobs/')) {
    const jobId = path.split('/')[3]
    const job = jobs.find((item) => item.id === jobId)
    if (job) {
      job.deletedAt = '2026-06-08T01:00:00Z'
      job.updatedAt = '2026-06-08T01:00:00Z'
    }
    return jsonResponse(createJobDetail(jobId))
  }
  if (path.endsWith('/restore') && path.includes('/api/jobs/')) {
    const jobId = path.split('/')[3]
    const job = jobs.find((item) => item.id === jobId)
    if (job) {
      delete job.deletedAt
      job.updatedAt = '2026-06-08T02:00:00Z'
    }
    return jsonResponse(createJobDetail(jobId))
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

function createJobDetail(jobId: string) {
  const job = jobs.find((item) => item.id === jobId) ?? jobs[0]
  return {
    job,
    events: [],
    logs: [],
  }
}

describe('DashboardPage', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    jobs.splice(0, jobs.length, ...[
      {
        id: 'job-1',
        type: 'issue',
        repository: 'owner/repository',
        githubNumber: 42,
        state: 'waiting_design_approval',
        title: 'First job',
        branchName: 'issue_42',
        watchRuleId: 'rule-1',
        createdAt: '2026-06-08T00:00:00Z',
        updatedAt: '2026-06-08T00:00:00Z',
      },
      {
        id: 'job-2',
        type: 'issue',
        repository: 'owner/repository',
        githubNumber: 43,
        state: 'completed',
        title: 'Completed job',
        branchName: 'issue_43',
        watchRuleId: 'rule-1',
        createdAt: '2026-06-08T00:00:00Z',
        updatedAt: '2026-06-08T00:00:00Z',
      },
      {
        id: 'job-3',
        type: 'issue',
        repository: 'owner/repository',
        githubNumber: 44,
        state: 'failed',
        title: 'Deleted job',
        branchName: 'issue_44',
        watchRuleId: 'rule-1',
        deletedAt: '2026-06-08T00:00:00Z',
        createdAt: '2026-06-08T00:00:00Z',
        updatedAt: '2026-06-08T00:00:00Z',
      },
    ])
  })

  it('shows jobs, toggles completed jobs, and deletes the selected job', async () => {
    const confirmSpy = vi.spyOn(window, 'confirm').mockReturnValue(true)
    const wrapper = mount(DashboardPage, {
      global: {
        stubs: {
          RouterLink: {
            props: ['to'],
            template: '<a :href="typeof to === \'string\' ? to : String(to)"><slot /></a>',
          },
        },
      },
    })

    await flushPromises()

    expect(wrapper.text()).toContain('First job')
    expect(wrapper.text()).not.toContain('Completed job')
    expect(wrapper.text()).not.toContain('Deleted job')

    await wrapper.findAll('button').find((button) => button.text() === '完了ジョブも表示')!.trigger('click')
    await flushPromises()

    expect(wrapper.text()).toContain('Completed job')

    await wrapper.get('input[aria-label="First job を選択"]').setValue(true)
    await wrapper.findAll('button').find((button) => button.text().includes('削除'))!.trigger('click')
    await flushPromises()

    expect(confirmSpy).toHaveBeenCalled()
    expect(fetchMock).toHaveBeenCalledWith('/api/jobs/job-1/delete', expect.any(Object))
    expect(wrapper.text()).not.toContain('First job')
    confirmSpy.mockRestore()
  })

  it('shows deleted jobs and restores a selected job', async () => {
    const confirmSpy = vi.spyOn(window, 'confirm').mockReturnValue(true)
    const wrapper = mount(DashboardPage, {
      global: {
        stubs: {
          RouterLink: {
            props: ['to'],
            template: '<a :href="typeof to === \'string\' ? to : String(to)"><slot /></a>',
          },
        },
      },
    })

    await flushPromises()

    await wrapper.findAll('button').find((button) => button.text() === '削除済みジョブを表示')!.trigger('click')
    await flushPromises()

    expect(wrapper.text()).toContain('Deleted job')

    await wrapper.get('input[aria-label="Deleted job を選択"]').setValue(true)
    await wrapper.findAll('button').find((button) => button.text().includes('復元'))!.trigger('click')
    await flushPromises()

    expect(confirmSpy).toHaveBeenCalled()
    expect(fetchMock).toHaveBeenCalledWith('/api/jobs/job-3/restore', expect.any(Object))
    expect(wrapper.text()).not.toContain('Deleted job')
    confirmSpy.mockRestore()
  })
})
