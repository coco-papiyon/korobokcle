export type Job = {
  id: string
  type: string
  repository: string
  githubNumber: number
  state: string
  title: string
  branchName: string
  watchRuleId: string
  deletedAt?: string
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
  reviewComments?: ReviewComment[]
  prComments?: ReviewComment[]
  designArtifact?: Artifact
  implementationArtifact?: Artifact
  fixArtifact?: Artifact
  reviewArtifact?: Artifact
  prCommentAnalysisArtifact?: Artifact
  testReport?: Artifact
  toolCommand?: ToolCommand
  toolExecution?: ToolExecution
  prCreateArtifact?: Artifact
  logs?: JobLog[]
}

export type PRCommentsResponse = {
  pullNumber: number
  comments: ReviewComment[]
}

export type IssueBodyResponse = {
  issueBody: string
}

export type Artifact = {
  path: string
  content: string
}

export type ReviewComment = {
  author: string
  body: string
  path?: string
  line?: number
  url?: string
  createdAt?: string
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
  projectName: string
  labels: string[] | null
  projectFilters: ProjectFieldFilter[]
  titlePattern: string
  authors: string[] | null
  assignees: string[] | null
  reviewers: string[] | null
  excludeDraftPR: boolean
  provider: string
  model: string
  skillSet: string
  testProfile: string
  toolCommand: string
  enabled: boolean
}

export type WatchRuleForm = WatchRule & {
  localID: string
  selectedRepository: string
  repositoriesText: string
  projectFiltersText: string
  labelsText: string
  authorsText: string
  assigneesText: string
  reviewersText: string
}

export type TestProfile = {
  name: string
  commands: string[]
}

export type ToolCommand = {
  name: string
  command: string
  resident: boolean
}

export type ToolExecution = {
  name: string
  resident: boolean
  running: boolean
  startedAt?: string
  finishedAt?: string
  exitCode?: number
  stdout?: Artifact
  stderr?: Artifact
}

export type AppConfig = {
  provider: string
  model: string
  copilotAllowTools: string[]
  pollInterval: number
  screenRefreshInterval: number
  shutdownTimeout: number
  prTitleTemplate: string
  branchTemplate: string
  monitoredRepositories: MonitoredRepository[]
  providers: ProviderSpec[]
}

export type MonitoredRepository = {
  repository: string
  branch: string
  workDir: string
  workers: number
  improvementEnabled: boolean
  improvementBranch: string
  improvementDir: string
  improvementWorkDir: string
  workerDir?: string
  workerDirs: string[]
}

export type ImprovementItem = {
  repository: string
  issueNumber: number
  title: string
  state: string
  updatedAt: string
  draftPath?: string
  relatedJobId?: string
  decisionReason?: string
}

export type ImprovementDetail = {
  repository: string
  issueNumber: number
  state: string
  title: string
  phases: string[]
  input: string
  draft: string
  result: string
  decision: string
  approvalStatus: string
  decisionReason: string
  relatedJobId: string
  improvementBranch: string
  improvementDir: string
  improvementWorkDir: string
  draftPath: string
  updatedAt: string
}

export type ProviderSpec = {
  name: string
  models: string[]
}

export type SkillDefinition = {
  name: string
  title: string
  role: string
  promptTemplates: string[]
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
