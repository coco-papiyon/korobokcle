import type { AppConfig, IssueBodyResponse, Job, JobDetail, NotificationConfig, SkillSet, SkillSetSummary, TestProfile, ToolCommand, WatchRule } from '@/types'
import { requestFailedMessage } from '@/lib/ui-text'

async function request<T>(path: string, init?: RequestInit): Promise<T> {
  const response = await fetch(path, {
    ...init,
    headers: {
      Accept: 'application/json',
      ...(init?.body ? { 'Content-Type': 'application/json' } : {}),
      ...init?.headers,
    },
  })

  if (!response.ok) {
    const payload = (await response.json().catch(() => null)) as { error?: string } | null
    throw new Error(payload?.error ?? requestFailedMessage(response.status))
  }

  return (await response.json()) as T
}

export function fetchJobs(deleted: 'exclude' | 'only' | 'include' = 'exclude'): Promise<Job[]> {
  return request<Job[]>(`/api/jobs?deleted=${encodeURIComponent(deleted)}`)
}

export function fetchJobDetail(jobId: string): Promise<JobDetail> {
  return request<JobDetail>(`/api/jobs/${jobId}`)
}

export function refreshIssueBody(jobId: string): Promise<IssueBodyResponse> {
  return request<IssueBodyResponse>(`/api/jobs/${jobId}/issue-body`)
}

export function deleteJob(jobId: string): Promise<JobDetail> {
  return request<JobDetail>(`/api/jobs/${jobId}/delete`, {
    method: 'POST',
  })
}

export function restoreJob(jobId: string): Promise<JobDetail> {
  return request<JobDetail>(`/api/jobs/${jobId}/restore`, {
    method: 'POST',
  })
}

export function purgeJob(jobId: string): Promise<{ status: string }> {
  return request<{ status: string }>(`/api/jobs/${jobId}/purge`, {
    method: 'POST',
  })
}

export function submitDesignApproval(jobId: string, status: 'approved' | 'rejected', comment: string): Promise<JobDetail> {
  return request<JobDetail>(`/api/jobs/${jobId}/approvals/design`, {
    method: 'POST',
    body: JSON.stringify({ status, comment }),
  })
}

export function submitDesignRerun(jobId: string, comment: string, eventId?: number): Promise<JobDetail> {
  return request<JobDetail>(`/api/jobs/${jobId}/reruns/design`, {
    method: 'POST',
    body: JSON.stringify({ comment, eventId }),
  })
}

export function submitFinalApproval(jobId: string, status: 'approved' | 'rejected', comment: string): Promise<JobDetail> {
  return request<JobDetail>(`/api/jobs/${jobId}/approvals/final`, {
    method: 'POST',
    body: JSON.stringify({ status, comment }),
  })
}

export function submitImplementationRerun(jobId: string, comment: string, eventId?: number): Promise<JobDetail> {
  return request<JobDetail>(`/api/jobs/${jobId}/reruns/implementation`, {
    method: 'POST',
    body: JSON.stringify({ comment, eventId }),
  })
}

export function submitPRRerun(jobId: string, comment: string, eventId?: number): Promise<JobDetail> {
  return request<JobDetail>(`/api/jobs/${jobId}/reruns/pr`, {
    method: 'POST',
    body: JSON.stringify({ comment, eventId }),
  })
}

export function submitReviewRerun(jobId: string, comment: string, eventId?: number): Promise<JobDetail> {
  return request<JobDetail>(`/api/jobs/${jobId}/reruns/review`, {
    method: 'POST',
    body: JSON.stringify({ comment, eventId }),
  })
}

export function submitReviewComment(jobId: string, comment: string): Promise<JobDetail> {
  return request<JobDetail>(`/api/jobs/${jobId}/reviews/submit`, {
    method: 'POST',
    body: JSON.stringify({ comment }),
  })
}

export function submitReviewApproval(jobId: string): Promise<JobDetail> {
  return request<JobDetail>(`/api/jobs/${jobId}/approvals/review`, {
    method: 'POST',
  })
}

export function fetchWatchRules(): Promise<WatchRule[]> {
  return request<WatchRule[]>('/api/watch-rules')
}

export function saveWatchRules(rules: WatchRule[]): Promise<WatchRule[]> {
  return request<WatchRule[]>('/api/watch-rules', {
    method: 'PUT',
    body: JSON.stringify(rules),
  })
}

export function fetchTestProfiles(): Promise<TestProfile[]> {
  return request<TestProfile[]>('/api/test-profiles')
}

export function saveTestProfiles(profiles: TestProfile[]): Promise<TestProfile[]> {
  return request<TestProfile[]>('/api/test-profiles', {
    method: 'PUT',
    body: JSON.stringify(profiles),
  })
}

export function fetchToolCommands(): Promise<ToolCommand[]> {
  return request<ToolCommand[]>('/api/tool-commands')
}

export function saveToolCommands(commands: ToolCommand[]): Promise<ToolCommand[]> {
  return request<ToolCommand[]>('/api/tool-commands', {
    method: 'PUT',
    body: JSON.stringify(commands),
  })
}

export function startToolCommand(jobId: string, toolCommand?: string): Promise<JobDetail> {
  return request<JobDetail>(`/api/jobs/${jobId}/tool/start`, {
    method: 'POST',
    body: JSON.stringify(toolCommand ? { toolCommand } : {}),
  })
}

export function stopToolCommand(jobId: string): Promise<JobDetail> {
  return request<JobDetail>(`/api/jobs/${jobId}/tool/stop`, {
    method: 'POST',
  })
}

export function fetchAppConfig(): Promise<AppConfig> {
  return request<AppConfig>('/api/app-config')
}

export function saveAppConfig(
  config: Partial<
    Pick<
      AppConfig,
      | 'provider'
      | 'model'
      | 'copilotAllowTools'
      | 'pollInterval'
      | 'screenRefreshInterval'
      | 'shutdownTimeout'
      | 'prTitleTemplate'
      | 'branchTemplate'
      | 'monitoredRepositories'
    >
  >,
): Promise<AppConfig> {
  return request<AppConfig>('/api/app-config', {
    method: 'PUT',
    body: JSON.stringify(config),
  })
}

export function fetchNotificationConfig(): Promise<NotificationConfig> {
  return request<NotificationConfig>('/api/notification-config')
}

export function saveNotificationConfig(config: NotificationConfig): Promise<NotificationConfig> {
  return request<NotificationConfig>('/api/notification-config', {
    method: 'PUT',
    body: JSON.stringify(config),
  })
}

export function fetchSkillSets(): Promise<SkillSetSummary[]> {
  return request<SkillSetSummary[]>('/api/skillsets')
}

export function fetchSkillSet(name: string): Promise<SkillSet> {
  return request<SkillSet>(`/api/skillsets/${encodeURIComponent(name)}`)
}

export function createSkillSet(name: string, source: string): Promise<SkillSet> {
  return request<SkillSet>('/api/skillsets', {
    method: 'POST',
    body: JSON.stringify({ name, source }),
  })
}

export function saveSkillSet(skillSet: SkillSet): Promise<SkillSet> {
  return request<SkillSet>(`/api/skillsets/${encodeURIComponent(skillSet.name)}`, {
    method: 'PUT',
    body: JSON.stringify(skillSet),
  })
}

export function deleteSkillSet(name: string): Promise<{ status: string }> {
  return request<{ status: string }>(`/api/skillsets/${encodeURIComponent(name)}`, {
    method: 'DELETE',
  })
}
