package skill

type DesignContext struct {
	JobID               string               `json:"jobId"`
	Repository          string               `json:"repository"`
	IssueNumber         int                  `json:"issueNumber"`
	Title               string               `json:"title"`
	SessionID           string               `json:"sessionId,omitempty"`
	Body                string               `json:"body"`
	Author              string               `json:"author"`
	Labels              []string             `json:"labels"`
	Assignees           []string             `json:"assignees"`
	RerunComment        string               `json:"rerunComment,omitempty"`
	ExistingDesign      string               `json:"existingDesign,omitempty"`
	WatchRuleID         string               `json:"watchRuleId"`
	BranchName          string               `json:"branchName"`
	ArtifactDir         string               `json:"artifactDir"`
	ManagedInstructions []ManagedInstruction `json:"managedInstructions,omitempty"`
}

type ImplementationContext struct {
	JobID                  string               `json:"jobId"`
	Repository             string               `json:"repository"`
	IssueNumber            int                  `json:"issueNumber"`
	Title                  string               `json:"title"`
	SessionID              string               `json:"sessionId,omitempty"`
	Body                   string               `json:"body"`
	Author                 string               `json:"author"`
	Labels                 []string             `json:"labels"`
	Assignees              []string             `json:"assignees"`
	WatchRuleID            string               `json:"watchRuleId"`
	BranchName             string               `json:"branchName"`
	DesignArtifact         string               `json:"designArtifact"`
	DesignArtifactDir      string               `json:"designArtifactDir"`
	DesignApprovalComment  string               `json:"designApprovalComment,omitempty"`
	ImplementationArtifact string               `json:"implementationArtifact,omitempty"`
	ArtifactDir            string               `json:"artifactDir"`
	RerunComment           string               `json:"rerunComment,omitempty"`
	PreviousFailure        string               `json:"previousFailure,omitempty"`
	PreviousTestReport     string               `json:"previousTestReport,omitempty"`
	SourceURL              string               `json:"sourceUrl,omitempty"`
	ReviewComments         []ReviewComment      `json:"reviewComments,omitempty"`
	TestProfile            TestProfileContext   `json:"testProfile"`
	ManagedInstructions    []ManagedInstruction `json:"managedInstructions,omitempty"`
}

type TestProfileContext struct {
	Commands []string `json:"commands,omitempty"`
}

type ReviewComment struct {
	Author string `json:"author"`
	Body   string `json:"body"`
	Path   string `json:"path"`
	Line   int    `json:"line"`
	URL    string `json:"url"`
}

type ReviewContext struct {
	JobID               string               `json:"jobId"`
	Repository          string               `json:"repository"`
	PullNumber          int                  `json:"pullNumber"`
	Title               string               `json:"title"`
	SessionID           string               `json:"sessionId,omitempty"`
	Body                string               `json:"body"`
	Author              string               `json:"author"`
	Labels              []string             `json:"labels"`
	Assignees           []string             `json:"assignees"`
	WatchRuleID         string               `json:"watchRuleId"`
	BranchName          string               `json:"branchName"`
	ArtifactDir         string               `json:"artifactDir"`
	SourceURL           string               `json:"sourceUrl"`
	RepositoryHint      string               `json:"repositoryHint"`
	ManagedInstructions []ManagedInstruction `json:"managedInstructions,omitempty"`
}

type ManagedInstruction struct {
	ID         string   `json:"id"`
	Title      string   `json:"title"`
	Scope      string   `json:"scope"`
	Phases     []string `json:"phases"`
	Status     string   `json:"status"`
	UpdatedAt  string   `json:"updatedAt"`
	SourcePath string   `json:"sourcePath"`
	Body       string   `json:"body"`
}
