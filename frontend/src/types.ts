export type JobKind = 'issue_design' | 'issue_implementation' | 'pr_review' | 'pr_feedback'

export type Job = {
  id: string
  kind: JobKind
  state: string
  repository: string
  number: number
  title: string
}

export type JobArtifact = {
  content: string
  path: string
}

export type SearchCondition = {
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
