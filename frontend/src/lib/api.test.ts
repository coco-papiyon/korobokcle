import { afterEach, describe, expect, it, vi } from 'vitest'

import {
  approveImprovement,
  fetchImprovementDetail,
  fetchImprovements,
  generateImprovement,
  regenerateImprovement,
  rejectImprovement,
  saveImprovementDraft,
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
})
