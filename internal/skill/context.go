package skill

type DesignContext struct {
	JobID       string   `json:"jobId"`
	Repository  string   `json:"repository"`
	IssueNumber int      `json:"issueNumber"`
	Title       string   `json:"title"`
	Body        string   `json:"body"`
	Author      string   `json:"author"`
	Labels      []string `json:"labels"`
	Assignees   []string `json:"assignees"`
	WatchRuleID string   `json:"watchRuleId"`
	BranchName  string   `json:"branchName"`
	ArtifactDir string   `json:"artifactDir"`
}

type ImplementationContext struct {
	JobID              string   `json:"jobId"`
	Repository         string   `json:"repository"`
	IssueNumber        int      `json:"issueNumber"`
	Title              string   `json:"title"`
	Body               string   `json:"body"`
	Author             string   `json:"author"`
	Labels             []string `json:"labels"`
	Assignees          []string `json:"assignees"`
	WatchRuleID        string   `json:"watchRuleId"`
	BranchName         string   `json:"branchName"`
	DesignArtifact     string   `json:"designArtifact"`
	DesignArtifactDir  string   `json:"designArtifactDir"`
	ArtifactDir        string   `json:"artifactDir"`
	RerunComment       string   `json:"rerunComment,omitempty"`
	PreviousFailure    string   `json:"previousFailure,omitempty"`
	PreviousTestReport string   `json:"previousTestReport,omitempty"`
}

type ReviewContext struct {
	JobID          string   `json:"jobId"`
	Repository     string   `json:"repository"`
	PullNumber     int      `json:"pullNumber"`
	Title          string   `json:"title"`
	Body           string   `json:"body"`
	Author         string   `json:"author"`
	Labels         []string `json:"labels"`
	Assignees      []string `json:"assignees"`
	WatchRuleID    string   `json:"watchRuleId"`
	BranchName     string   `json:"branchName"`
	ArtifactDir    string   `json:"artifactDir"`
	SourceURL      string   `json:"sourceUrl"`
	RepositoryHint string   `json:"repositoryHint"`
}
