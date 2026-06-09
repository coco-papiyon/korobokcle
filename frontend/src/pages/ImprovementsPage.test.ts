import { flushPromises, mount } from '@vue/test-utils'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

import ImprovementsPage from './ImprovementsPage.vue'

const apiMocks = vi.hoisted(() => ({
  fetchImprovements: vi.fn(),
  fetchImprovementDetail: vi.fn(),
  saveImprovementDraft: vi.fn(),
  approveImprovement: vi.fn(),
  rejectImprovement: vi.fn(),
  regenerateImprovement: vi.fn(),
}))

vi.mock('@/lib/api', () => ({
  fetchImprovements: apiMocks.fetchImprovements,
  fetchImprovementDetail: apiMocks.fetchImprovementDetail,
  saveImprovementDraft: apiMocks.saveImprovementDraft,
  approveImprovement: apiMocks.approveImprovement,
  rejectImprovement: apiMocks.rejectImprovement,
  regenerateImprovement: apiMocks.regenerateImprovement,
}))

describe('ImprovementsPage', () => {
  beforeEach(() => {
    apiMocks.fetchImprovements.mockResolvedValue([
      {
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
      {
        jobId: 'job-2',
        repository: 'owner/repository',
        issueNumber: 43,
        title: 'Ignore empty comment',
        status: 'generating',
        decision: '',
        reason: 'comment was empty',
        updatedAt: '2026-06-07T00:00:00Z',
        sourceEventType: 'final_rejected',
        phases: ['implementation'],
        hasDraft: false,
        improvementReady: true,
      },
    ])
    apiMocks.fetchImprovementDetail.mockResolvedValue({
      summary: {
        jobId: 'job-2',
        repository: 'owner/repository',
        issueNumber: 43,
        title: 'Ignore empty comment',
        status: 'generating',
        decision: '',
        reason: 'comment was empty',
        updatedAt: '2026-06-07T00:00:00Z',
        sourceEventType: 'final_rejected',
        phases: ['implementation'],
        hasDraft: false,
        improvementReady: true,
      },
      draft: { path: 'draft.md', content: 'draft body' },
      notes: { path: 'notes.md', content: 'note body' },
    })
    apiMocks.saveImprovementDraft.mockResolvedValue({
      summary: {
        jobId: 'job-2',
        repository: 'owner/repository',
        issueNumber: 43,
        title: 'Ignore empty comment',
        status: 'draft_created',
        decision: 'draft_created',
        sourceEventType: 'final_rejected',
        phases: ['implementation'],
        hasDraft: true,
        improvementReady: true,
      },
    })
    apiMocks.approveImprovement.mockResolvedValue({
      summary: {
        jobId: 'job-2',
        repository: 'owner/repository',
        issueNumber: 43,
        title: 'Ignore empty comment',
        status: 'approved',
        decision: 'approved',
        sourceEventType: 'final_rejected',
        phases: ['implementation'],
        hasDraft: true,
        improvementReady: true,
      },
    })
    apiMocks.rejectImprovement.mockResolvedValue({
      summary: {
        jobId: 'job-2',
        repository: 'owner/repository',
        issueNumber: 43,
        title: 'Ignore empty comment',
        status: 'rejected',
        decision: 'rejected',
        sourceEventType: 'final_rejected',
        phases: ['implementation'],
        hasDraft: true,
        improvementReady: true,
      },
    })
    apiMocks.regenerateImprovement.mockResolvedValue({
      summary: {
        jobId: 'job-2',
        repository: 'owner/repository',
        issueNumber: 43,
        title: 'Ignore empty comment',
        status: 'draft_created',
        decision: 'draft_created',
        sourceEventType: 'final_rejected',
        phases: ['implementation'],
        hasDraft: true,
        improvementReady: true,
      },
      draft: { path: 'draft.md', content: 'regenerated draft' },
      notes: { path: 'notes.md', content: 'regenerated notes' },
    })
  })

  afterEach(() => {
    vi.clearAllMocks()
  })

  it('renders improvements and shows no-improvement-needed reason in detail', async () => {
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

    expect(wrapper.text()).toContain('改善一覧')
    expect(wrapper.text()).toContain('下書き作成済み')
    expect(wrapper.text()).toContain('AIによる改善案作成中')

    const openButtons = wrapper.findAll('button.artifact-link')
    await openButtons[1].trigger('click')
    await flushPromises()

    expect(apiMocks.fetchImprovementDetail).toHaveBeenCalledWith('job-2')
    expect(wrapper.text()).toContain('概要')
    expect(wrapper.text()).toContain('AIによる改善案作成中')
    expect(wrapper.text()).toContain('comment was empty')
    expect(wrapper.text()).not.toContain('改善機能ステータス:')
    expect(wrapper.text()).not.toContain('decision.json')
    expect(wrapper.text()).not.toContain('result.md')
    expect(wrapper.find('.improvement-modal-panel').exists()).toBe(true)
    expect(wrapper.findAll('textarea.improvement-editor')).toHaveLength(1)
    expect(wrapper.text()).not.toContain('notes.md')
    expect(wrapper.text()).toContain('改善案')
  })

  it('saves, regenerates, approves, and rejects improvement draft', async () => {
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
    await wrapper.findAll('button.artifact-link')[1].trigger('click')
    await flushPromises()

    const textareas = wrapper.findAll('textarea')
    await textareas[0].setValue('edited draft')
    await wrapper.find('#improvement-approval-comment').setValue('approval note')

    const buttons = wrapper.findAll('button.button')
    await buttons.find((button) => button.text() === '保存')!.trigger('click')
    await flushPromises()
    expect(apiMocks.saveImprovementDraft).toHaveBeenCalledWith('job-2', 'edited draft', 'note body')

    await buttons.find((button) => button.text() === '再生成')!.trigger('click')
    await flushPromises()
    expect(apiMocks.regenerateImprovement).toHaveBeenCalledWith('job-2', 'final_rejected')

    await buttons.find((button) => button.text() === '承認')!.trigger('click')
    await flushPromises()
    expect(apiMocks.approveImprovement).toHaveBeenCalledWith('job-2', 'approval note', 'regenerated draft')

    await buttons.find((button) => button.text() === '却下')!.trigger('click')
    await flushPromises()
    expect(apiMocks.rejectImprovement).toHaveBeenCalledWith('job-2', 'approval note', 'regenerated draft')
  })
})
