import { flushPromises, mount } from '@vue/test-utils'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

import ImprovementsPage from './ImprovementsPage.vue'

const apiMocks = vi.hoisted(() => ({
  fetchAppConfig: vi.fn(),
  fetchImprovements: vi.fn(),
  fetchImprovementDetail: vi.fn(),
  saveImprovementDraft: vi.fn(),
  approveImprovement: vi.fn(),
  rejectImprovement: vi.fn(),
  regenerateImprovement: vi.fn(),
  saveImprovementWorkspace: vi.fn(),
  pushImprovement: vi.fn(),
}))

vi.mock('@/lib/api', () => ({
  fetchAppConfig: apiMocks.fetchAppConfig,
  fetchImprovements: apiMocks.fetchImprovements,
  fetchImprovementDetail: apiMocks.fetchImprovementDetail,
  saveImprovementDraft: apiMocks.saveImprovementDraft,
  approveImprovement: apiMocks.approveImprovement,
  rejectImprovement: apiMocks.rejectImprovement,
  regenerateImprovement: apiMocks.regenerateImprovement,
  saveImprovementWorkspace: apiMocks.saveImprovementWorkspace,
  pushImprovement: apiMocks.pushImprovement,
}))

describe('ImprovementsPage', () => {
  beforeEach(() => {
    apiMocks.fetchAppConfig.mockResolvedValue({
      screenRefreshInterval: 1,
    })
    const listState = {
      job1: {
        jobId: 'job-1',
        repository: 'owner/repository',
        issueNumber: 42,
        title: 'Improve prompts',
        status: 'draft_created',
        decision: 'draft_created',
        updatedAt: '2026-06-08T00:00:00Z',
        sourceEventType: 'design_rejected',
        phases: ['design'],
        hasDraft: true,
        improvementReady: true,
      },
      job2: {
        jobId: 'job-2',
        repository: 'owner/repository',
        issueNumber: 43,
        title: 'Ignore empty comment',
        status: 'draft_created',
        decision: 'draft_created',
        reason: 'comment was empty',
        updatedAt: '2026-06-07T00:00:00Z',
        sourceEventType: 'final_rejected',
        phases: ['implementation'],
        hasDraft: true,
        improvementReady: true,
      },
    }
    const detailState = {
      summary: { ...listState.job2 },
      draft: { path: 'draft.md', content: 'draft body' },
      notes: { path: 'notes.md', content: 'note body' },
      workspace: [] as { path: string; content: string }[],
    }

    apiMocks.fetchImprovements.mockImplementation(async () => [
      { ...listState.job1 },
      { ...listState.job2 },
    ])
    apiMocks.fetchImprovementDetail.mockImplementation(async () => ({
      summary: { ...detailState.summary },
      draft: detailState.draft ? { ...detailState.draft } : undefined,
      notes: detailState.notes ? { ...detailState.notes } : undefined,
      workspace: detailState.workspace.map((file) => ({ ...file })),
    }))
    apiMocks.saveImprovementDraft.mockImplementation(async (_jobId, draft, notes) => {
      detailState.summary = {
        ...detailState.summary,
        status: 'draft_created',
        decision: 'draft_created',
        hasDraft: true,
      }
      detailState.draft = { path: 'draft.md', content: draft }
      detailState.notes = { path: 'notes.md', content: notes }
      listState.job2 = { ...detailState.summary, reason: 'comment was empty' }
      return { summary: { ...detailState.summary } }
    })
    apiMocks.approveImprovement.mockImplementation(async (_jobId, _comment, _resultBody) => {
      detailState.summary = {
        ...detailState.summary,
        status: 'approved',
        decision: 'approved',
        hasDraft: true,
      }
      detailState.workspace = [
        { path: '.improvement/design.md', content: '# approved design\n' },
      ]
      listState.job2 = { ...detailState.summary, reason: 'comment was empty' }
      return {
        summary: { ...detailState.summary },
        draft: detailState.draft ? { ...detailState.draft } : undefined,
        notes: detailState.notes ? { ...detailState.notes } : undefined,
        workspace: detailState.workspace.map((file) => ({ ...file })),
      }
    })
    apiMocks.rejectImprovement.mockImplementation(async (_jobId, _comment, _resultBody) => {
      detailState.summary = {
        ...detailState.summary,
        status: 'rejected',
        decision: 'rejected',
        hasDraft: true,
      }
      listState.job2 = { ...detailState.summary, reason: 'comment was empty' }
      return { summary: { ...detailState.summary } }
    })
    apiMocks.regenerateImprovement.mockImplementation(async (_jobId, sourceEventType) => {
      detailState.summary = {
        ...detailState.summary,
        status: 'generating',
        decision: 'draft_created',
        sourceEventType,
        hasDraft: true,
      }
      detailState.draft = { path: 'draft.md', content: 'regenerated draft' }
      detailState.notes = { path: 'notes.md', content: 'regenerated notes' }
      listState.job2 = { ...detailState.summary, reason: 'comment was empty' }
      return {
        summary: { ...detailState.summary },
        draft: { ...detailState.draft },
        notes: { ...detailState.notes },
      }
    })
    apiMocks.saveImprovementWorkspace.mockImplementation(async (_jobId, files) => {
      detailState.workspace = files.map((file) => ({
        path: file.path,
        content: file.content.endsWith('\n') ? file.content : `${file.content}\n`,
      }))
      return {
        summary: { ...detailState.summary },
        draft: detailState.draft ? { ...detailState.draft } : undefined,
        notes: detailState.notes ? { ...detailState.notes } : undefined,
        workspace: detailState.workspace ? detailState.workspace.map((file) => ({ ...file })) : undefined,
      }
    })
    apiMocks.pushImprovement.mockImplementation(async () => ({
      summary: { ...detailState.summary },
      draft: detailState.draft ? { ...detailState.draft } : undefined,
      notes: detailState.notes ? { ...detailState.notes } : undefined,
      workspace: detailState.workspace ? detailState.workspace.map((file) => ({ ...file })) : undefined,
    }))
  })

  afterEach(() => {
    vi.clearAllMocks()
    vi.useRealTimers()
  })

  it('renders improvements and shows no-improvement-needed reason in detail', async () => {
    vi.useFakeTimers()
    const wrapper = mount(ImprovementsPage, {
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
    await vi.advanceTimersByTimeAsync(1000)
    await flushPromises()

    expect(wrapper.text()).toContain('改善一覧')
    expect(wrapper.text()).toContain('下書き作成済み')
    expect(wrapper.text()).toContain('下書き確認待ち')
    expect(apiMocks.fetchImprovements).toHaveBeenCalledTimes(2)

    const openButtons = wrapper.findAll('button.artifact-link')
    await openButtons[1].trigger('click')
    await flushPromises()

    expect(apiMocks.fetchImprovementDetail).toHaveBeenCalledWith('job-2')
    expect(wrapper.text()).toContain('概要')
    expect(wrapper.text()).toContain('下書き確認待ち')
    expect(wrapper.text()).toContain('comment was empty')
    expect(wrapper.text()).not.toContain('改善機能ステータス:')
    expect(wrapper.text()).not.toContain('decision.json')
    expect(wrapper.text()).not.toContain('result.md')
    expect(wrapper.text()).not.toContain('approval.json')
    expect(wrapper.find('.improvement-modal-panel').exists()).toBe(true)
    expect(wrapper.findAll('textarea.improvement-editor')).toHaveLength(1)
    expect(wrapper.text()).not.toContain('notes.md')
    expect(wrapper.text()).toContain('改善案')
  })

  it('saves, regenerates, approves, and rejects improvement draft', async () => {
    vi.useFakeTimers()
    const wrapper = mount(ImprovementsPage, {
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
    await vi.advanceTimersByTimeAsync(1000)
    await flushPromises()
    const openDetail = async () => {
      await wrapper.findAll('button.artifact-link')[1].trigger('click')
      await flushPromises()
      expect(wrapper.find('.improvement-modal-panel').exists()).toBe(true)
    }

    await openDetail()
    const textareas = wrapper.findAll('textarea')
    await textareas[0].setValue('edited draft')
    await wrapper.find('#improvement-approval-comment').setValue('approval note')

    const saveButtons = wrapper.findAll('button.button-secondary')
    await saveButtons.find((button) => button.text() === '保存')!.trigger('click')
    await flushPromises()
    expect(apiMocks.saveImprovementDraft).toHaveBeenCalledWith('job-2', 'edited draft', 'note body')
    expect(wrapper.find('.improvement-modal-panel').exists()).toBe(false)

    await openDetail()
    const regenerateButtons = wrapper.findAll('button.button-secondary')
    await regenerateButtons.find((button) => button.text() === '再生成')!.trigger('click')
    await flushPromises()
    expect(apiMocks.regenerateImprovement).toHaveBeenCalledWith('job-2', 'final_rejected')
    expect(wrapper.find('.improvement-modal-panel').exists()).toBe(false)
    expect(wrapper.text()).toContain('AIによる改善案作成中')

    await openDetail()
    const approvalButtons = wrapper.findAll('button.button')
    await wrapper.find('#improvement-approval-comment').setValue('approval note')
    await approvalButtons.find((button) => button.text() === '承認')!.trigger('click')
    await flushPromises()
    expect(apiMocks.approveImprovement).toHaveBeenCalledWith('job-2', 'approval note', 'regenerated draft')
    expect(wrapper.find('.improvement-modal-panel').exists()).toBe(true)
    expect(wrapper.text()).toContain('承認済み')
    expect(wrapper.text()).toContain('承認後の修正')
    expect(wrapper.text()).toContain('.improvement/design.md')
    expect(wrapper.findAll('textarea.improvement-editor')).toHaveLength(2)

    const workspaceEditors = wrapper.findAll('textarea.improvement-editor')
    await workspaceEditors[1].setValue('# approved design\n\nreviewed before push\n')

    const pushButton = wrapper.findAll('button.button-primary').find((button) => button.text() === 'push')!
    await pushButton.trigger('click')
    await flushPromises()
    expect(apiMocks.saveImprovementWorkspace).toHaveBeenCalledWith('job-2', [
      { path: '.improvement/design.md', content: '# approved design\n\nreviewed before push\n' },
    ])
    expect(apiMocks.pushImprovement).toHaveBeenCalledWith('job-2')
    expect(wrapper.find('.improvement-modal-panel').exists()).toBe(false)

    await openDetail()
    await wrapper.find('#improvement-approval-comment').setValue('approval note')
    const rejectButtons = wrapper.findAll('button.button')
    await rejectButtons.find((button) => button.text() === '却下')!.trigger('click')
    await flushPromises()
    expect(apiMocks.rejectImprovement).toHaveBeenCalledWith('job-2', 'approval note', 'regenerated draft')
    expect(wrapper.find('.improvement-modal-panel').exists()).toBe(false)
    expect(wrapper.text()).toContain('却下済み')
  })
})
