package skill

type DesignContext struct {
	JobID                string   `json:"jobId"`
	Repository           string   `json:"repository"`
	IssueNumber          int      `json:"issueNumber"`
	Title                string   `json:"title"`
	Body                 string   `json:"body"`
	Author               string   `json:"author"`
	Labels               []string `json:"labels"`
	Assignees            []string `json:"assignees"`
	RerunComment         string   `json:"rerunComment,omitempty"`
	ExistingDesign       string   `json:"existingDesign,omitempty"`
	ExistingImprovements string   `json:"existingImprovements,omitempty"`
	WatchRuleID          string   `json:"watchRuleId"`
	BranchName           string   `json:"branchName"`
	ArtifactDir          string   `json:"artifactDir"`
}

type ImplementationContext struct {
	JobID                  string          `json:"jobId"`
	Repository             string          `json:"repository"`
	IssueNumber            int             `json:"issueNumber"`
	Title                  string          `json:"title"`
	Body                   string          `json:"body"`
	Author                 string          `json:"author"`
	Labels                 []string        `json:"labels"`
	Assignees              []string        `json:"assignees"`
	WatchRuleID            string          `json:"watchRuleId"`
	BranchName             string          `json:"branchName"`
	DesignArtifact         string          `json:"designArtifact"`
	DesignArtifactDir      string          `json:"designArtifactDir"`
	DesignApprovalComment  string          `json:"designApprovalComment,omitempty"`
	ImplementationArtifact string          `json:"implementationArtifact,omitempty"`
	ExistingImprovements   string          `json:"existingImprovements,omitempty"`
	ArtifactDir            string          `json:"artifactDir"`
	RerunComment           string          `json:"rerunComment,omitempty"`
	PreviousFailure        string          `json:"previousFailure,omitempty"`
	PreviousTestReport     string          `json:"previousTestReport,omitempty"`
	SourceURL              string          `json:"sourceUrl,omitempty"`
	ReviewComments         []ReviewComment `json:"reviewComments,omitempty"`
}

type ReviewComment struct {
	Author string `json:"author"`
	Body   string `json:"body"`
	Path   string `json:"path"`
	Line   int    `json:"line"`
	URL    string `json:"url"`
}

type ReviewContext struct {
	JobID                string   `json:"jobId"`
	Repository           string   `json:"repository"`
	PullNumber           int      `json:"pullNumber"`
	Title                string   `json:"title"`
	Body                 string   `json:"body"`
	Author               string   `json:"author"`
	Labels               []string `json:"labels"`
	Assignees            []string `json:"assignees"`
	WatchRuleID          string   `json:"watchRuleId"`
	BranchName           string   `json:"branchName"`
	ArtifactDir          string   `json:"artifactDir"`
	SourceURL            string   `json:"sourceUrl"`
	RepositoryHint       string   `json:"repositoryHint"`
	ExistingImprovements string   `json:"existingImprovements,omitempty"`
}

type ImprovementContext struct {
	JobID                   string `json:"jobId"`
	Repository              string `json:"repository"`
	IssueNumber             int    `json:"issueNumber"`
	Title                   string `json:"title"`
	JobType                 string `json:"jobType"`
	Comment                 string `json:"comment"`
	InputArtifactPath       string `json:"inputArtifactPath"`
	ExistingImprovements    string `json:"existingImprovements,omitempty"`
	ExistingDesign          string `json:"existingDesign,omitempty"`
	ExistingImplementation  string `json:"existingImplementation,omitempty"`
	ExistingReview          string `json:"existingReview,omitempty"`
	ExistingPRCommentResult string `json:"existingPrCommentResult,omitempty"`
	ArtifactDir             string `json:"artifactDir"`
}
