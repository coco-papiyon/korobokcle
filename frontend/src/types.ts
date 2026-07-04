export type JobKind = 'issue_design' | 'issue_implementation' | 'pr_review' | 'pr_feedback' | 'pr_conflict'

export type Job = {
  id: string
  kind: JobKind
  state: string
  repository: string
  number: number
  title: string
  branch?: string
  issueContext?: string
  errorMessage?: string
  failedFromState?: string
  fetchedAt?: string
  updatedAt?: string
}

export type JobArtifact = {
  content: string
  path: string
}

export type JobListResponse = {
  updatedAt: string
  jobs?: Job[]
}

export type JobDetailResponse = {
  updatedAt: string
  job: Job
  branch: string
  issueContext?: string
  logs?: JobLogGroup[]
}

export type JobLogFile = {
  kind: string
  label: string
  path: string
  content: string
}

export type JobLogGroup = {
  role: string
  roleLabel: string
  attempt: number
  files: JobLogFile[]
}

export type SearchCondition = {
  enabled?: boolean
  aiProvider?: AIProvider
  aiModel?: ModelSelection
  labelIncludes: string[]
  labelExcludes: string[]
  titleContains: string[]
  authors: string[]
  assignees: string[]
}

export type AIProvider = 'codex' | 'github_copilot'

export type ModelSelection = {
  mode: 'default' | 'custom'
  value: string
}

export type AIModels = {
  codex: ModelSelection
  githubCopilot: ModelSelection
}

export type WatchSettings = {
  repository: string
  aiProvider: AIProvider
  pollIntervalSeconds: number
  jobConcurrency: number
  implementationLoopCount: number
  verificationAiProvider?: AIProvider
  verificationAiModel?: ModelSelection
  baseBranch: string
  branchNamePattern: string
  aiAllowedCommands: string[]
  codexAllowedCommands?: string[]
  models: AIModels
  issue: SearchCondition
  pullRequest: SearchCondition
}

export type SkillStatus = {
  purpose: string
  name: string
  displayName: string
  exists: boolean
  aiExists: boolean
  generated: boolean
  path?: string
}

export type SkillGenerationResult = {
  provider: AIProvider
  skills: SkillStatus[]
  message: string
}
