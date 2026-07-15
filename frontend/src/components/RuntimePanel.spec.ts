import { mount } from '@vue/test-utils'
import { nextTick } from 'vue'
import { vi } from 'vitest'
import RuntimePanel from './RuntimePanel.vue'

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

describe('RuntimePanel', () => {
  it('loads runtime status and toggles start and stop actions', async () => {
    const fetchMock = vi.fn()
    fetchMock.mockResolvedValueOnce(
      jsonResponse({
        running: false,
        command: 'npm run dev',
        startupMode: 'background',
        hasStopCommand: true,
        workingDir: 'workspace/owner_repo/issue-impl/worktree',
        logPath: 'workspace/owner_repo/issue-impl/logs/startup.log',
      }),
    )
    fetchMock.mockResolvedValueOnce(
      jsonResponse({
        content: 'startup ready',
        path: 'workspace/owner_repo/issue-impl/logs/startup.log',
        updatedAt: '2026-07-11T00:00:00Z',
      }),
    )
    fetchMock.mockResolvedValueOnce(
      jsonResponse({
        running: true,
        pid: 1234,
        command: 'npm run dev',
        startupMode: 'background',
        hasStopCommand: true,
        workingDir: 'workspace/owner_repo/issue-impl/worktree',
        startedAt: '2026-07-11T00:00:01Z',
        logPath: 'workspace/owner_repo/issue-impl/logs/startup.log',
      }),
    )
    fetchMock.mockResolvedValueOnce(
      jsonResponse({
        running: true,
        pid: 1234,
        command: 'npm run dev',
        startupMode: 'background',
        hasStopCommand: true,
        workingDir: 'workspace/owner_repo/issue-impl/worktree',
        startedAt: '2026-07-11T00:00:01Z',
        logPath: 'workspace/owner_repo/issue-impl/logs/startup.log',
      }),
    )
    fetchMock.mockResolvedValueOnce(
      jsonResponse({
        content: 'startup ready\nrunning',
        path: 'workspace/owner_repo/issue-impl/logs/startup.log',
        updatedAt: '2026-07-11T00:00:02Z',
      }),
    )
    vi.stubGlobal('fetch', fetchMock)

    const wrapper = mount(RuntimePanel, {
      props: {
        active: true,
        jobId: 'issue-impl',
      },
    })
    await flushPromises()

    expect(wrapper.text()).toContain('停止中')
    expect(wrapper.text()).toContain('npm run dev')
    expect(wrapper.text()).toContain('バックグラウンド起動')
    expect(wrapper.text()).toContain('startup ready')

    await wrapper.get('button').trigger('click')
    await flushPromises()

    expect(fetchMock).toHaveBeenCalledWith(
      '/api/jobs/issue-impl/runtime',
      expect.objectContaining({
        method: 'POST',
      }),
    )
    expect(wrapper.text()).toContain('停止')
    expect(wrapper.text()).toContain('1234')
    expect(wrapper.text()).toContain('running')
    expect(wrapper.text()).toContain('workspace/owner_repo/issue-impl/worktree')
  })
})
