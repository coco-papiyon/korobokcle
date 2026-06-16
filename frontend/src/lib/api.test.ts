import { afterEach, describe, expect, it, vi } from 'vitest'

import {
  analyzePRComment,
  createSkillSet,
  deleteJob,
  deleteSkillSet,
  fetchAppConfig,
  fetchJobDetail,
  fetchJobs,
  fetchNotificationConfig,
  fetchPRComments,
  fetchSkillSet,
  fetchSkillSets,
  fetchTestProfiles,
  fetchToolCommands,
  fetchWatchRules,
  approveImprovement,
  fetchImprovementDetail,
  fetchImprovements,
  generateImprovement,
  purgeJob,
  refreshIssueBody,
  regenerateImprovement,
  rejectImprovement,
  restoreJob,
  saveAppConfig,
  saveImprovementDraft,
  saveNotificationConfig,
  saveSkillSet,
  saveTestProfiles,
  saveToolCommands,
  saveWatchRules,
  startToolCommand,
  stopToolCommand,
  submitDesignApproval,
  submitDesignRerun,
  submitFinalApproval,
  submitImplementationRerun,
  submitPRRerun,
  submitReviewApproval,
  submitReviewComment,
  submitReviewRerun,
  saveImprovementWorkspace,
} from './api'

const fetchMock = vi.fn()

vi.stubGlobal('fetch', fetchMock)

describe('improvement api', () => {
  afterEach(() => {
    fetchMock.mockReset()
  })

  it('fetches improvements list', async () => {
    fetchMock.mockResolvedValue({
      ok: true,
      json: async () => ([]),
    })

    await fetchImprovements()

    expect(fetchMock).toHaveBeenCalledWith('/api/improvements', expect.any(Object))
  })

  it('fetches improvement detail', async () => {
    fetchMock.mockResolvedValue({
      ok: true,
      json: async () => ({ summary: { jobId: 'job-1' } }),
    })

    await fetchImprovementDetail('job-1')

    expect(fetchMock).toHaveBeenCalledWith('/api/improvements/job-1', expect.any(Object))
  })

  it('saves improvement draft', async () => {
    fetchMock.mockResolvedValue({
      ok: true,
      json: async () => ({ summary: { jobId: 'job-1' } }),
    })

    await saveImprovementDraft('job-1', 'draft', 'notes')

    expect(fetchMock).toHaveBeenCalledWith(
      '/api/improvements/job-1/draft',
      expect.objectContaining({
        method: 'PUT',
        body: JSON.stringify({ draft: 'draft', notes: 'notes' }),
      }),
    )
  })

  it('submits improvement approval and rejection', async () => {
    fetchMock.mockResolvedValue({
      ok: true,
      json: async () => ({ summary: { jobId: 'job-1' } }),
    })

    await approveImprovement('job-1', 'ok', 'body')
    await rejectImprovement('job-1', 'no', 'body')

    expect(fetchMock).toHaveBeenNthCalledWith(
      1,
      '/api/improvements/job-1/approve',
      expect.objectContaining({
        method: 'POST',
        body: JSON.stringify({ comment: 'ok', resultBody: 'body' }),
      }),
    )
    expect(fetchMock).toHaveBeenNthCalledWith(
      2,
      '/api/improvements/job-1/reject',
      expect.objectContaining({
        method: 'POST',
        body: JSON.stringify({ comment: 'no', resultBody: 'body' }),
      }),
    )
  })

  it('starts generation and regeneration', async () => {
    fetchMock.mockResolvedValue({
      ok: true,
      json: async () => ({ summary: { jobId: 'job-1' } }),
    })

    await generateImprovement('job-1', 'design_rejected')
    await regenerateImprovement('job-1', 'pr_comment_analysis_ready')

    expect(fetchMock).toHaveBeenNthCalledWith(
      1,
      '/api/jobs/job-1/improvements/generate',
      expect.objectContaining({
        method: 'POST',
        body: JSON.stringify({ sourceEventType: 'design_rejected' }),
      }),
    )
    expect(fetchMock).toHaveBeenNthCalledWith(
      2,
      '/api/improvements/job-1/regenerate',
      expect.objectContaining({
        method: 'POST',
        body: JSON.stringify({ sourceEventType: 'pr_comment_analysis_ready' }),
      }),
    )
  })

  it('calls the remaining endpoints with the expected payloads', async () => {
    fetchMock.mockResolvedValue({
      ok: true,
      json: async () => ({}),
    })

    await fetchJobs()
    await fetchJobs('include')
    await fetchJobDetail('job-1')
    await refreshIssueBody('job-1')
    await fetchPRComments('job-1')
    await analyzePRComment('job-1', { author: 'alice', body: 'review comment' })
    await fetchWatchRules()
    await saveWatchRules([{ id: 'rule-1', name: 'rule', repositories: [], target: 'issue', projectName: '', labels: [], projectFilters: [], titlePattern: '', authors: [], assignees: [], reviewers: [], excludeDraftPR: true, provider: '', model: '', skillSet: '', testProfile: '', toolCommand: '', enabled: true }])
    await fetchTestProfiles()
    await saveTestProfiles([{ name: 'go-default', commands: ['go test ./...'] }])
    await fetchToolCommands()
    await saveToolCommands([{ name: 'tool', command: 'npm run dev', resident: true }])
    await fetchAppConfig()
    await saveAppConfig({ provider: 'mock', model: 'default' })
    await fetchNotificationConfig()
    await saveNotificationConfig({ channels: [] })
    await fetchSkillSets()
    await fetchSkillSet('team a')
    await createSkillSet('team a', 'default')
    await saveSkillSet({ name: 'team a', mutable: true, skills: {} })
    await deleteSkillSet('team a')
    await deleteJob('job-1')
    await restoreJob('job-1')
    await purgeJob('job-1')
    await submitDesignApproval('job-1', 'approved', 'ok')
    await submitDesignRerun('job-1', 'rerun', 1)
    await submitFinalApproval('job-1', 'rejected', 'no')
    await submitImplementationRerun('job-1', 'fix', 2)
    await submitPRRerun('job-1', 'rerun', 3)
    await submitReviewRerun('job-1', 'rerun', 4)
    await submitReviewComment('job-1', 'comment')
    await submitReviewApproval('job-1')
    await saveImprovementWorkspace('job-1', [{ path: 'a.txt', content: 'content' }])
    await startToolCommand('job-1')
    await startToolCommand('job-1', 'npm run dev')
    await stopToolCommand('job-1')

    expect(fetchMock).toHaveBeenCalledWith('/api/jobs?deleted=exclude', expect.any(Object))
    expect(fetchMock).toHaveBeenCalledWith('/api/jobs?deleted=include', expect.any(Object))
    expect(fetchMock).toHaveBeenCalledWith('/api/jobs/job-1', expect.any(Object))
    expect(fetchMock).toHaveBeenCalledWith('/api/jobs/job-1/issue-body', expect.any(Object))
    expect(fetchMock).toHaveBeenCalledWith('/api/jobs/job-1/pr-comments', expect.any(Object))
    expect(fetchMock).toHaveBeenCalledWith(
      '/api/jobs/job-1/pr-comments/analyze',
      expect.objectContaining({
        method: 'POST',
        body: JSON.stringify({ comment: { author: 'alice', body: 'review comment' } }),
      }),
    )
    expect(fetchMock).toHaveBeenCalledWith('/api/watch-rules', expect.any(Object))
    expect(fetchMock).toHaveBeenCalledWith('/api/test-profiles', expect.any(Object))
    expect(fetchMock).toHaveBeenCalledWith('/api/tool-commands', expect.any(Object))
    expect(fetchMock).toHaveBeenCalledWith('/api/app-config', expect.any(Object))
    expect(fetchMock).toHaveBeenCalledWith('/api/notification-config', expect.any(Object))
    expect(fetchMock).toHaveBeenCalledWith('/api/skillsets', expect.any(Object))
    expect(fetchMock).toHaveBeenCalledWith('/api/skillsets/team%20a', expect.any(Object))
    expect(fetchMock).toHaveBeenCalledWith('/api/jobs/job-1/delete', expect.any(Object))
    expect(fetchMock).toHaveBeenCalledWith('/api/jobs/job-1/restore', expect.any(Object))
    expect(fetchMock).toHaveBeenCalledWith('/api/jobs/job-1/purge', expect.any(Object))
    expect(fetchMock).toHaveBeenCalledWith('/api/jobs/job-1/approvals/design', expect.any(Object))
    expect(fetchMock).toHaveBeenCalledWith('/api/jobs/job-1/reruns/design', expect.any(Object))
    expect(fetchMock).toHaveBeenCalledWith('/api/jobs/job-1/approvals/final', expect.any(Object))
    expect(fetchMock).toHaveBeenCalledWith('/api/jobs/job-1/reruns/implementation', expect.any(Object))
    expect(fetchMock).toHaveBeenCalledWith('/api/jobs/job-1/reruns/pr', expect.any(Object))
    expect(fetchMock).toHaveBeenCalledWith('/api/jobs/job-1/reruns/review', expect.any(Object))
    expect(fetchMock).toHaveBeenCalledWith('/api/jobs/job-1/reviews/submit', expect.any(Object))
    expect(fetchMock).toHaveBeenCalledWith('/api/jobs/job-1/approvals/review', expect.any(Object))
    expect(fetchMock).toHaveBeenCalledWith('/api/improvements/job-1/workspace', expect.any(Object))
    expect(fetchMock).toHaveBeenCalledWith(
      '/api/jobs/job-1/tool/start',
      expect.objectContaining({
        method: 'POST',
        body: '{}',
      }),
    )
    expect(fetchMock).toHaveBeenCalledWith(
      '/api/jobs/job-1/tool/start',
      expect.objectContaining({
        method: 'POST',
        body: JSON.stringify({ toolCommand: 'npm run dev' }),
      }),
    )
    expect(fetchMock).toHaveBeenCalledWith('/api/jobs/job-1/tool/stop', expect.any(Object))
  })

  it('surfaces API failures with the server message or fallback status', async () => {
    fetchMock
      .mockResolvedValueOnce({
        ok: false,
        status: 503,
        json: async () => ({ error: 'unavailable' }),
      })
      .mockResolvedValueOnce({
        ok: false,
        status: 418,
        json: async () => {
          throw new Error('broken json')
        },
      })

    await expect(fetchJobs()).rejects.toThrow('unavailable')
    await expect(fetchJobDetail('job-1')).rejects.toThrow('リクエストに失敗しました: 418')
  })
})
