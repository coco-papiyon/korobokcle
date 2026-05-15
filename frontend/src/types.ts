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
}

export type JobDetail = {
  job: Job
  events: JobEvent[]
  designArtifact?: Artifact
  implementationArtifact?: Artifact
  testReport?: Artifact
  prCreateArtifact?: Artifact
}

export type Artifact = {
  path: string
  content: string
}

export type WatchRule = {
  id: string
  name: string
  repositories: string[] | null
  target: string
  labels: string[] | null
  titlePattern: string
  authors: string[] | null
  assignees: string[] | null
  excludeDraftPR: boolean
  skillSet: string
  testProfile: string
  enabled: boolean
}

export type WatchRuleForm = WatchRule & {
  repositoriesText: string
  labelsText: string
  authorsText: string
  assigneesText: string
}

export type AppConfig = {
  provider: string
}
