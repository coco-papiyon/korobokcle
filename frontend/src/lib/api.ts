import type { AppConfig, Job, JobDetail, WatchRule } from '@/types'

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
    throw new Error(payload?.error ?? `Request failed: ${response.status}`)
  }

  return (await response.json()) as T
}

export function fetchJobs(): Promise<Job[]> {
  return request<Job[]>('/api/jobs')
}

export function fetchJobDetail(jobId: string): Promise<JobDetail> {
  return request<JobDetail>(`/api/jobs/${jobId}`)
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

export function fetchWatchRules(): Promise<WatchRule[]> {
  return request<WatchRule[]>('/api/watch-rules')
}

export function saveWatchRules(rules: WatchRule[]): Promise<WatchRule[]> {
  return request<WatchRule[]>('/api/watch-rules', {
    method: 'PUT',
    body: JSON.stringify(rules),
  })
}

export function fetchAppConfig(): Promise<AppConfig> {
  return request<AppConfig>('/api/app-config')
}

export function saveAppConfig(config: AppConfig): Promise<AppConfig> {
  return request<AppConfig>('/api/app-config', {
    method: 'PUT',
    body: JSON.stringify(config),
  })
}
