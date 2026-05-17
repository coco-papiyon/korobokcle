export type Job = {
  id: string
  type: string
  repository: string
  githubNumber: number
  state: string
  title: string
  branchName: string
  watchRuleId: string
  createdAt: string
  updatedAt: string
}

export type JobEvent = {
  id: number
  jobId: string
  eventType: string
  stateFrom: string
  stateTo: string
  payload: string
  createdAt: string
  sourceEventType?: string
  availableActions: string[]
}

export type JobDetail = {
  job: Job
  events: JobEvent[]
  issueBody?: string
  designArtifact?: Artifact
  implementationArtifact?: Artifact
  fixArtifact?: Artifact
  reviewArtifact?: Artifact
  testReport?: Artifact
  prCreateArtifact?: Artifact
  logs?: JobLog[]
}

export type Artifact = {
  path: string
  content: string
}

export type JobLog = {
  name: string
  phase: string
  path: string
  content: string
}

export type ProjectFieldFilter = {
  field: string
  values: string[]
}

export type WatchRule = {
  id: string
  name: string
  repositories: string[] | null
  target: string
  branch: string
  projectName: string
  labels: string[] | null
  projectFilters: ProjectFieldFilter[]
  titlePattern: string
  authors: string[] | null
  assignees: string[] | null
  excludeDraftPR: boolean
  provider: string
  model: string
  skillSet: string
  testProfile: string
  enabled: boolean
}

export type WatchRuleForm = WatchRule & {
  repositoriesText: string
  projectFiltersText: string
  labelsText: string
  authorsText: string
  assigneesText: string
}

export type AppConfig = {
  provider: string
  model: string
  pollInterval: number
  prTitleTemplate: string
  branchTemplate: string
  providers: ProviderSpec[]
}

export type ProviderSpec = {
  name: string
  models: string[]
}

export type SkillDefinition = {
  name: string
  provider: string
  inputs: string[]
  outputs: string[]
  artifacts: {
    output_file: string
  }
}

export type SkillFile = {
  definition: SkillDefinition
  promptTemplate: string
}

export type SkillSetSummary = {
  name: string
  mutable: boolean
}

export type SkillSet = {
  name: string
  mutable: boolean
  skills: Record<string, SkillFile>
}

export type NotificationChannel = {
  name: string
  type: string
  events: string[]
  enabled: boolean
}

export type NotificationConfig = {
  channels: NotificationChannel[]
}
