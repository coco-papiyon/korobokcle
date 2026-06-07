package web

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/gorilla/mux"

	"github.com/coco-papiyon/korobokcle/internal/config"
	"github.com/coco-papiyon/korobokcle/internal/orchestrator"
)

type Server struct {
	httpServer                  *http.Server
	orchestrator                *orchestrator.Orchestrator
	config                      *config.Service
	staticDir                   string
	issueBodyFetcher            IssueBodyFetcher
	prCommentsFetcher           func(context.Context, PRCommentsFetchRequest) (PRCommentsArtifact, error)
	prCommentSubmitter          func(context.Context, PRCommentSubmitRequest) error
	prCommentAnalyzer           func(context.Context, string, PRCommentData) error
	improvementGenerator        func(context.Context, string, string) error
	improvementApprover         func(context.Context, string, int, string, string, string) error
	reviewer                    ReviewSubmitter
	commenter                   IssueCommentSubmitter
	tools                       *toolRuntimeManager
	prepareRepositoryWorkspaces func(context.Context, config.App) error
}

type IssueBodyFetcher interface {
	FetchIssueBody(ctx context.Context, repository string, issueNumber int) (string, error)
}

type ReviewSubmitter interface {
	Submit(ctx context.Context, req ReviewSubmitRequest) error
}

type IssueCommentSubmitter interface {
	Submit(ctx context.Context, req IssueCommentSubmitRequest) error
}

type ReviewSubmitRequest struct {
	Repository  string
	PullNumber  int
	Body        string
	ArtifactDir string
}

type IssueCommentSubmitRequest struct {
	Repository  string
	IssueNumber int
	Body        string
	ArtifactDir string
}

type PRCommentsFetchRequest struct {
	Repository  string
	PullNumber  int
	ArtifactDir string
}

type PRCommentData struct {
	Author    string
	Body      string
	URL       string
	CreatedAt string
	Path      string
	Line      int
}

type PRCommentsArtifact struct {
	PullNumber int
	Comments   []PRCommentData
}

type PRCommentSubmitRequest struct {
	Repository  string
	PullNumber  int
	Body        string
	ArtifactDir string
}

type GHReviewSubmitter struct{}
type GHIssueCommentSubmitter struct{}

func New(cfg *config.Service, orch *orchestrator.Orchestrator, issueBodyFetcher IssueBodyFetcher) (*Server, error) {
	staticDir, err := resolveStaticDir()
	if err != nil {
		return nil, err
	}
	s := &Server{
		orchestrator:     orch,
		config:           cfg,
		staticDir:        staticDir,
		issueBodyFetcher: issueBodyFetcher,
		reviewer:         GHReviewSubmitter{},
		commenter:        GHIssueCommentSubmitter{},
		tools:            newToolRuntimeManager(),
	}

	router := mux.NewRouter()
	api := router.PathPrefix("/api").Subrouter()
	api.HandleFunc("/jobs", s.handleJobs).Methods(http.MethodGet)
	api.HandleFunc("/jobs/{id}", s.handleJobDetail).Methods(http.MethodGet)
	api.HandleFunc("/jobs/{id}/issue-body", s.handleJobIssueBody).Methods(http.MethodGet)
	api.HandleFunc("/jobs/{id}/pr-comments", s.handlePRComments).Methods(http.MethodGet)
	api.HandleFunc("/jobs/{id}/pr-comments/analyze", s.handleAnalyzePRComment).Methods(http.MethodPost)
	api.HandleFunc("/jobs/{id}/improvements", s.handleGenerateImprovement).Methods(http.MethodPost)
	api.HandleFunc("/jobs/{id}/delete", s.handleDeleteJob).Methods(http.MethodPost)
	api.HandleFunc("/jobs/{id}/restore", s.handleRestoreJob).Methods(http.MethodPost)
	api.HandleFunc("/jobs/{id}/purge", s.handlePurgeJob).Methods(http.MethodPost)
	api.HandleFunc("/jobs/{id}/approvals/design", s.handleDesignApproval).Methods(http.MethodPost)
	api.HandleFunc("/jobs/{id}/reruns/design", s.handleDesignRerun).Methods(http.MethodPost)
	api.HandleFunc("/jobs/{id}/approvals/final", s.handleFinalApproval).Methods(http.MethodPost)
	api.HandleFunc("/jobs/{id}/reruns/implementation", s.handleImplementationRerun).Methods(http.MethodPost)
	api.HandleFunc("/jobs/{id}/reruns/pr", s.handlePRRerun).Methods(http.MethodPost)
	api.HandleFunc("/jobs/{id}/reruns/review", s.handleReviewRerun).Methods(http.MethodPost)
	api.HandleFunc("/jobs/{id}/approvals/review", s.handleReviewApproval).Methods(http.MethodPost)
	api.HandleFunc("/jobs/{id}/reviews/submit", s.handleSubmitReviewComment).Methods(http.MethodPost)
	api.HandleFunc("/jobs/{id}/tool/start", s.handleStartToolCommand).Methods(http.MethodPost)
	api.HandleFunc("/jobs/{id}/tool/stop", s.handleStopToolCommand).Methods(http.MethodPost)
	api.HandleFunc("/app-config", s.handleAppConfig).Methods(http.MethodGet)
	api.HandleFunc("/app-config", s.handleSaveAppConfig).Methods(http.MethodPut)
	api.HandleFunc("/improvements", s.handleImprovements).Methods(http.MethodGet)
	api.HandleFunc("/improvement", s.handleImprovementDetail).Methods(http.MethodGet)
	api.HandleFunc("/improvement/draft", s.handleSaveImprovementDraft).Methods(http.MethodPut)
	api.HandleFunc("/improvement/approve", s.handleApproveImprovement).Methods(http.MethodPost)
	api.HandleFunc("/notification-config", s.handleNotificationConfig).Methods(http.MethodGet)
	api.HandleFunc("/notification-config", s.handleSaveNotificationConfig).Methods(http.MethodPut)
	api.HandleFunc("/watch-rules", s.handleWatchRules).Methods(http.MethodGet)
	api.HandleFunc("/watch-rules", s.handleSaveWatchRules).Methods(http.MethodPut)
	api.HandleFunc("/test-profiles", s.handleTestProfiles).Methods(http.MethodGet)
	api.HandleFunc("/test-profiles", s.handleSaveTestProfiles).Methods(http.MethodPut)
	api.HandleFunc("/tool-commands", s.handleToolCommands).Methods(http.MethodGet)
	api.HandleFunc("/tool-commands", s.handleSaveToolCommands).Methods(http.MethodPut)
	api.HandleFunc("/skillsets", s.handleSkillSets).Methods(http.MethodGet)
	api.HandleFunc("/skillsets", s.handleCreateSkillSet).Methods(http.MethodPost)
	api.HandleFunc("/skillsets/{name}", s.handleSkillSet).Methods(http.MethodGet)
	api.HandleFunc("/skillsets/{name}", s.handleSaveSkillSet).Methods(http.MethodPut)
	api.HandleFunc("/skillsets/{name}", s.handleDeleteSkillSet).Methods(http.MethodDelete)
	router.HandleFunc("/healthz", s.handleHealth).Methods(http.MethodGet)
	router.PathPrefix("/").HandlerFunc(s.handleSPA).Methods(http.MethodGet)

	s.httpServer = &http.Server{
		Addr:              fmt.Sprintf(":%d", cfg.App().HTTPPort),
		Handler:           router,
		ReadHeaderTimeout: 5 * time.Second,
	}
	return s, nil
}

func (s *Server) SetRepositoryWorkspacePreparer(fn func(context.Context, config.App) error) {
	s.prepareRepositoryWorkspaces = fn
}

func (s *Server) SetPRCommentsFetcher(fn func(context.Context, PRCommentsFetchRequest) (PRCommentsArtifact, error)) {
	s.prCommentsFetcher = fn
}

func (s *Server) SetPRCommentSubmitter(fn func(context.Context, PRCommentSubmitRequest) error) {
	s.prCommentSubmitter = fn
}

func (s *Server) SetPRCommentAnalyzer(fn func(context.Context, string, PRCommentData) error) {
	s.prCommentAnalyzer = fn
}

func (s *Server) SetImprovementGenerator(fn func(context.Context, string, string) error) {
	s.improvementGenerator = fn
}

func (s *Server) SetImprovementApprover(fn func(context.Context, string, int, string, string, string) error) {
	s.improvementApprover = fn
}

func resolveStaticDir() (string, error) {
	exe, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("resolve executable path: %w", err)
	}
	return filepath.Join(filepath.Dir(exe), "frontend", "dist"), nil
}

func (GHReviewSubmitter) Submit(ctx context.Context, req ReviewSubmitRequest) error {
	if _, err := exec.LookPath("gh"); err != nil {
		return fmt.Errorf("gh command is not available: %w", err)
	}
	if strings.TrimSpace(req.Repository) == "" {
		return fmt.Errorf("repository is required")
	}
	if req.PullNumber < 1 {
		return fmt.Errorf("pull number must be positive")
	}
	if strings.TrimSpace(req.Body) == "" {
		return fmt.Errorf("review body is empty")
	}
	if err := os.MkdirAll(req.ArtifactDir, 0o755); err != nil {
		return err
	}

	bodyPath := filepath.Join(req.ArtifactDir, "gh-pr-comment-body.md")
	if err := os.WriteFile(bodyPath, []byte(req.Body), 0o644); err != nil {
		return err
	}

	cmd := exec.CommandContext(ctx, "gh", "pr", "comment",
		fmt.Sprintf("%d", req.PullNumber),
		"--repo", req.Repository,
		"--body-file", bodyPath,
	)
	raw, err := cmd.CombinedOutput()
	output := strings.TrimSpace(string(raw))
	if writeErr := os.WriteFile(filepath.Join(req.ArtifactDir, "gh-pr-comment.log"), []byte(output), 0o644); writeErr != nil {
		return writeErr
	}
	if err != nil {
		return fmt.Errorf("gh pr comment failed: %w: %s", err, output)
	}
	return nil
}

func (GHIssueCommentSubmitter) Submit(ctx context.Context, req IssueCommentSubmitRequest) error {
	if _, err := exec.LookPath("gh"); err != nil {
		return fmt.Errorf("gh command is not available: %w", err)
	}
	if strings.TrimSpace(req.Repository) == "" {
		return fmt.Errorf("repository is required")
	}
	if req.IssueNumber < 1 {
		return fmt.Errorf("issue number must be positive")
	}
	if strings.TrimSpace(req.Body) == "" {
		return fmt.Errorf("comment body is empty")
	}
	if err := os.MkdirAll(req.ArtifactDir, 0o755); err != nil {
		return err
	}

	bodyPath := filepath.Join(req.ArtifactDir, "gh-issue-comment-body.md")
	if err := os.WriteFile(bodyPath, []byte(req.Body), 0o644); err != nil {
		return err
	}

	cmd := exec.CommandContext(ctx, "gh", "issue", "comment",
		fmt.Sprintf("%d", req.IssueNumber),
		"--repo", req.Repository,
		"--body-file", bodyPath,
	)
	raw, err := cmd.CombinedOutput()
	output := strings.TrimSpace(string(raw))
	if writeErr := os.WriteFile(filepath.Join(req.ArtifactDir, "gh-issue-comment.log"), []byte(output), 0o644); writeErr != nil {
		return writeErr
	}
	if err != nil {
		return fmt.Errorf("gh issue comment failed: %w: %s", err, output)
	}
	return nil
}

func (s *Server) Start() error {
	return s.httpServer.ListenAndServe()
}

func (s *Server) Shutdown(ctx context.Context) error {
	return s.httpServer.Shutdown(ctx)
}

func (s *Server) hasStaticDist() bool {
	info, err := os.Stat(filepath.Join(s.staticDir, "index.html"))
	return err == nil && !info.IsDir()
}
