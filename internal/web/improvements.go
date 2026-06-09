package web

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"

	"github.com/gorilla/mux"

	"github.com/coco-papiyon/korobokcle/internal/artifacts"
	"github.com/coco-papiyon/korobokcle/internal/config"
	"github.com/coco-papiyon/korobokcle/internal/domain"
)

const (
	improvementInputFileName    = "input.md"
	improvementContextFileName  = "context.json"
	improvementDraftDirName     = "draft"
	improvementDraftFileName    = "draft.md"
	improvementNotesFileName    = "notes.md"
	improvementResultFileName   = "result.md"
	improvementApprovalFileName = "approval.json"
	improvementDecisionFileName = "decision.json"
)

type ImprovementApprovalRequest struct {
	Status     string
	Comment    string
	ResultBody string
}

type improvementSummaryResponse struct {
	JobID            string   `json:"jobId"`
	Repository       string   `json:"repository"`
	IssueNumber      int      `json:"issueNumber"`
	Title            string   `json:"title"`
	Status           string   `json:"status"`
	Decision         string   `json:"decision"`
	Reason           string   `json:"reason,omitempty"`
	UpdatedAt        string   `json:"updatedAt,omitempty"`
	SourceEventType  string   `json:"sourceEventType,omitempty"`
	Phases           []string `json:"phases,omitempty"`
	HasDraft         bool     `json:"hasDraft"`
	ImprovementReady bool     `json:"improvementReady"`
	DeletedAt        string   `json:"deletedAt,omitempty"`
}

type improvementDetailResponse struct {
	Summary  improvementSummaryResponse `json:"summary"`
	Input    *artifactResponse          `json:"input,omitempty"`
	Context  *artifactResponse          `json:"context,omitempty"`
	Draft    *artifactResponse          `json:"draft,omitempty"`
	Notes    *artifactResponse          `json:"notes,omitempty"`
	Result   *artifactResponse          `json:"result,omitempty"`
	Decision *artifactResponse          `json:"decision,omitempty"`
	Approval *artifactResponse          `json:"approval,omitempty"`
}

type improvementSaveDraftRequest struct {
	Draft string `json:"draft"`
	Notes string `json:"notes"`
}

type improvementApprovalPayload struct {
	Comment    string `json:"comment"`
	ResultBody string `json:"resultBody"`
}

type improvementGenerateRequest struct {
	SourceEventType string `json:"sourceEventType"`
}

type improvementDecisionRecord struct {
	Decision    string `json:"decision"`
	Reason      string `json:"reason"`
	UpdatedAt   string `json:"updatedAt"`
	SourceEvent string `json:"sourceEvent"`
}

type improvementContextRecord struct {
	Phases []string `json:"phases"`
	Source struct {
		EventType string `json:"eventType"`
	} `json:"source"`
}

func (s *Server) handleImprovements(w http.ResponseWriter, r *http.Request) {
	jobs, err := s.orchestrator.ListJobsByFilter(r.Context(), parseJobListFilter("include"))
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, err)
		return
	}

	items := make([]improvementSummaryResponse, 0, len(jobs))
	for _, job := range jobs {
		summary, err := s.loadImprovementSummary(job)
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			writeJSONError(w, http.StatusInternalServerError, err)
			return
		}
		items = append(items, summary)
	}

	sort.Slice(items, func(i, j int) bool {
		if items[i].UpdatedAt == items[j].UpdatedAt {
			if items[i].Repository == items[j].Repository {
				return items[i].IssueNumber > items[j].IssueNumber
			}
			return items[i].Repository < items[j].Repository
		}
		return items[i].UpdatedAt > items[j].UpdatedAt
	})

	writeJSON(w, http.StatusOK, items)
}

func (s *Server) handleImprovementDetail(w http.ResponseWriter, r *http.Request) {
	jobID := mux.Vars(r)["id"]
	job, _, err := s.orchestrator.JobDetail(r.Context(), jobID)
	if err != nil {
		writeJSONError(w, http.StatusNotFound, err)
		return
	}

	detail, err := s.loadImprovementDetail(job)
	if errors.Is(err, os.ErrNotExist) {
		writeJSONError(w, http.StatusNotFound, fmt.Errorf("improvement detail not found for job %q", jobID))
		return
	}
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, detail)
}

func (s *Server) handleSaveImprovementDraft(w http.ResponseWriter, r *http.Request) {
	jobID := mux.Vars(r)["id"]
	job, _, err := s.orchestrator.JobDetail(r.Context(), jobID)
	if err != nil {
		writeJSONError(w, http.StatusNotFound, err)
		return
	}

	repoConfig, ok := s.resolveMonitoredRepository(job.Repository)
	if !ok || !repoConfig.ImprovementEnabled {
		writeJSONError(w, http.StatusBadRequest, fmt.Errorf("improvement feature is disabled for repository %q", job.Repository))
		return
	}

	var payload improvementSaveDraftRequest
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		writeJSONError(w, http.StatusBadRequest, fmt.Errorf("decode improvement draft: %w", err))
		return
	}

	workFiles := s.repositoryImprovementWorkFiles(job, repoConfig)
	artifactDir := s.repositoryImprovementArtifactDir(job)
	if err := writeImprovementTextFile(workFiles.DraftPath, payload.Draft); err != nil {
		writeJSONError(w, http.StatusInternalServerError, err)
		return
	}
	if err := writeImprovementTextFile(workFiles.NotesPath, payload.Notes); err != nil {
		writeJSONError(w, http.StatusInternalServerError, err)
		return
	}
	if err := writeImprovementTextFile(filepath.Join(artifactDir, improvementDraftDirName, improvementDraftFileName), payload.Draft); err != nil {
		writeJSONError(w, http.StatusInternalServerError, err)
		return
	}
	if err := writeImprovementTextFile(filepath.Join(artifactDir, improvementNotesFileName), payload.Notes); err != nil {
		writeJSONError(w, http.StatusInternalServerError, err)
		return
	}

	s.handleImprovementDetail(w, r)
}

func (s *Server) handleApproveImprovement(w http.ResponseWriter, r *http.Request) {
	s.handleImprovementApproval(w, r, "approved")
}

func (s *Server) handleRejectImprovement(w http.ResponseWriter, r *http.Request) {
	s.handleImprovementApproval(w, r, "rejected")
}

func (s *Server) handleImprovementApproval(w http.ResponseWriter, r *http.Request, status string) {
	if s.improvementApprover == nil {
		writeJSONError(w, http.StatusInternalServerError, fmt.Errorf("improvement approver is not configured"))
		return
	}

	jobID := mux.Vars(r)["id"]
	var payload improvementApprovalPayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil && !errors.Is(err, io.EOF) {
		writeJSONError(w, http.StatusBadRequest, fmt.Errorf("decode improvement approval: %w", err))
		return
	}

	if err := s.improvementApprover(r.Context(), jobID, ImprovementApprovalRequest{
		Status:     status,
		Comment:    payload.Comment,
		ResultBody: payload.ResultBody,
	}); err != nil {
		writeJSONError(w, http.StatusInternalServerError, err)
		return
	}

	s.handleImprovementDetail(w, r)
}

func (s *Server) handleRegenerateImprovement(w http.ResponseWriter, r *http.Request) {
	jobID := mux.Vars(r)["id"]
	job, _, err := s.orchestrator.JobDetail(r.Context(), jobID)
	if err != nil {
		writeJSONError(w, http.StatusNotFound, err)
		return
	}
	sourceEventType, err := s.resolveImprovementSourceEventType(job, strings.TrimSpace(readGeneratePayload(r)))
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, err)
		return
	}
	if s.improvementGenerator == nil {
		writeJSONError(w, http.StatusInternalServerError, fmt.Errorf("improvement generator is not configured"))
		return
	}
	if err := s.improvementGenerator(r.Context(), jobID, sourceEventType); err != nil {
		writeJSONError(w, http.StatusInternalServerError, err)
		return
	}
	s.handleImprovementDetail(w, r)
}

func (s *Server) handleGenerateImprovement(w http.ResponseWriter, r *http.Request) {
	jobID := mux.Vars(r)["id"]
	job, _, err := s.orchestrator.JobDetail(r.Context(), jobID)
	if err != nil {
		writeJSONError(w, http.StatusNotFound, err)
		return
	}
	sourceEventType, err := s.resolveImprovementSourceEventType(job, strings.TrimSpace(readGeneratePayload(r)))
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, err)
		return
	}
	if s.improvementGenerator == nil {
		writeJSONError(w, http.StatusInternalServerError, fmt.Errorf("improvement generator is not configured"))
		return
	}
	if err := s.improvementGenerator(r.Context(), jobID, sourceEventType); err != nil {
		writeJSONError(w, http.StatusInternalServerError, err)
		return
	}
	s.handleImprovementDetail(w, r)
}

func readGeneratePayload(r *http.Request) string {
	var payload improvementGenerateRequest
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		return ""
	}
	return payload.SourceEventType
}

func (s *Server) resolveImprovementSourceEventType(job domain.Job, requested string) (string, error) {
	if requested != "" {
		return requested, nil
	}

	repoConfig, ok := s.resolveMonitoredRepository(job.Repository)
	if ok {
		artifactDir := s.repositoryImprovementArtifactDir(job)
		if raw, err := os.ReadFile(filepath.Join(artifactDir, improvementContextFileName)); err == nil {
			var contextRecord improvementContextRecord
			if json.Unmarshal(raw, &contextRecord) == nil {
				if eventType := strings.TrimSpace(contextRecord.Source.EventType); eventType != "" {
					return eventType, nil
				}
			}
		}
		workFiles := s.repositoryImprovementWorkFiles(job, repoConfig)
		if raw, err := os.ReadFile(workFiles.ContextPath); err == nil {
			var contextRecord improvementContextRecord
			if json.Unmarshal(raw, &contextRecord) == nil {
				if eventType := strings.TrimSpace(contextRecord.Source.EventType); eventType != "" {
					return eventType, nil
				}
			}
		}
	}

	_, events, err := s.orchestrator.JobDetail(context.Background(), job.ID)
	if err != nil {
		return "", err
	}
	for i := len(events) - 1; i >= 0; i-- {
		switch events[i].EventType {
		case "pr_comment_analysis_ready", "design_rejected", "final_rejected", "design_rerun_requested", "implementation_rerun_requested", "pr_rerun_requested", "review_rerun_requested":
			return events[i].EventType, nil
		}
	}
	return "", fmt.Errorf("improvement source event is not available for job %q", job.ID)
}

func (s *Server) loadImprovementDetail(job domain.Job) (improvementDetailResponse, error) {
	summary, err := s.loadImprovementSummary(job)
	if err != nil {
		return improvementDetailResponse{}, err
	}

	detail := improvementDetailResponse{Summary: summary}
	artifactDir := s.repositoryImprovementArtifactDir(job)
	if detail.Input, err = loadArtifactIfExists(filepath.Join(artifactDir, improvementInputFileName)); err != nil {
		return improvementDetailResponse{}, err
	}
	if detail.Context, err = loadArtifactIfExists(filepath.Join(artifactDir, improvementContextFileName)); err != nil {
		return improvementDetailResponse{}, err
	}
	if detail.Draft, err = loadArtifactIfExists(filepath.Join(artifactDir, improvementDraftDirName, improvementDraftFileName)); err != nil {
		return improvementDetailResponse{}, err
	}
	if detail.Notes, err = loadArtifactIfExists(filepath.Join(artifactDir, improvementNotesFileName)); err != nil {
		return improvementDetailResponse{}, err
	}
	repoConfig, ok := s.resolveMonitoredRepository(job.Repository)
	if ok {
		workFiles := s.repositoryImprovementWorkFiles(job, repoConfig)
		if detail.Draft == nil {
			if detail.Draft, err = loadArtifactIfExists(workFiles.DraftPath); err != nil {
				return improvementDetailResponse{}, err
			}
		}
		if detail.Notes == nil {
			if detail.Notes, err = loadArtifactIfExists(workFiles.NotesPath); err != nil {
				return improvementDetailResponse{}, err
			}
		}
	}
	if detail.Result, err = loadArtifactIfExists(filepath.Join(artifactDir, improvementResultFileName)); err != nil {
		return improvementDetailResponse{}, err
	}
	if detail.Decision, err = loadArtifactIfExists(filepath.Join(artifactDir, improvementDecisionFileName)); err != nil {
		return improvementDetailResponse{}, err
	}
	if detail.Approval, err = loadArtifactIfExists(filepath.Join(artifactDir, improvementApprovalFileName)); err != nil {
		return improvementDetailResponse{}, err
	}

	return detail, nil
}

func (s *Server) loadImprovementSummary(job domain.Job) (improvementSummaryResponse, error) {
	summary := improvementSummaryResponse{
		JobID:       job.ID,
		Repository:  job.Repository,
		IssueNumber: job.GitHubNumber,
		Title:       job.Title,
	}
	if job.DeletedAt != nil && !job.DeletedAt.IsZero() {
		summary.DeletedAt = job.DeletedAt.Format(timeFormat)
	}

	repoConfig, ok := s.resolveMonitoredRepository(job.Repository)
	artifactDir := s.repositoryImprovementArtifactDir(job)
	artifactDraftExists, err := webFileExists(filepath.Join(artifactDir, improvementDraftDirName, improvementDraftFileName))
	if err != nil {
		return improvementSummaryResponse{}, err
	}
	if artifactDraftExists {
		summary.HasDraft = true
	}
	artifactInputExists, err := webFileExists(filepath.Join(artifactDir, improvementInputFileName))
	if err != nil {
		return improvementSummaryResponse{}, err
	}
	artifactContextExists, err := webFileExists(filepath.Join(artifactDir, improvementContextFileName))
	if err != nil {
		return improvementSummaryResponse{}, err
	}
	if ok {
		workFiles := s.repositoryImprovementWorkFiles(job, repoConfig)
		if raw, err := os.ReadFile(filepath.Join(artifactDir, improvementContextFileName)); err == nil {
			var record improvementContextRecord
			if json.Unmarshal(raw, &record) == nil {
				summary.Phases = append([]string(nil), record.Phases...)
				summary.SourceEventType = strings.TrimSpace(record.Source.EventType)
			}
		} else if err != nil && !errors.Is(err, os.ErrNotExist) {
			return improvementSummaryResponse{}, err
		}
		if len(summary.Phases) == 0 {
			if raw, err := os.ReadFile(workFiles.ContextPath); err == nil {
				var record improvementContextRecord
				if json.Unmarshal(raw, &record) == nil {
					summary.Phases = append([]string(nil), record.Phases...)
					summary.SourceEventType = strings.TrimSpace(record.Source.EventType)
				}
			} else if err != nil && !errors.Is(err, os.ErrNotExist) {
				return improvementSummaryResponse{}, err
			}
		}
	}

	decisionPath := filepath.Join(artifactDir, improvementDecisionFileName)
	raw, err := os.ReadFile(decisionPath)
	if err != nil {
		hasInputContext := artifactInputExists || artifactContextExists
		switch {
		case summary.HasDraft:
			summary.Status = "draft_created"
			summary.Decision = "draft_created"
			summary.ImprovementReady = true
			return summary, nil
		case hasInputContext:
			summary.Status = "generating"
			return summary, nil
		default:
			return improvementSummaryResponse{}, err
		}
	}
	var decision improvementDecisionRecord
	if err := json.Unmarshal(raw, &decision); err != nil {
		return improvementSummaryResponse{}, err
	}
	summary.Status = strings.TrimSpace(decision.Decision)
	summary.Decision = strings.TrimSpace(decision.Decision)
	summary.Reason = strings.TrimSpace(decision.Reason)
	summary.UpdatedAt = strings.TrimSpace(decision.UpdatedAt)
	if summary.SourceEventType == "" {
		summary.SourceEventType = strings.TrimSpace(decision.SourceEvent)
	}
	summary.ImprovementReady = true
	return summary, nil
}

func (s *Server) repositoryImprovementArtifactDir(job domain.Job) string {
	return artifacts.RepositoryWorkerImprovementArtifactDir(s.config.Root(), s.config.App().ArtifactsDir, job.Repository, job.GitHubNumber)
}

func (s *Server) repositoryImprovementWorkFiles(job domain.Job, repoConfig config.MonitoredRepository) improvementWorkFiles {
	workDir := artifacts.RepositoryWorkerImprovementWorkspaceDir(s.config.Root(), s.config.App().ArtifactsDir, job.Repository)
	improvementWorkDir := artifacts.RepositoryWorkerImprovementWorkDir(workDir, repoConfig.ImprovementWorkDir)
	return improvementWorkFiles{
		InputPath:    filepath.Join(improvementWorkDir, improvementInputFileName),
		ContextPath:  filepath.Join(improvementWorkDir, improvementContextFileName),
		DraftPath:    filepath.Join(improvementWorkDir, improvementDraftDirName, improvementDraftFileName),
		NotesPath:    filepath.Join(improvementWorkDir, improvementNotesFileName),
		DecisionPath: filepath.Join(improvementWorkDir, improvementDecisionFileName),
	}
}

type improvementWorkFiles struct {
	InputPath    string
	ContextPath  string
	DraftPath    string
	NotesPath    string
	DecisionPath string
}

func webFileExists(path string) (bool, error) {
	info, err := os.Stat(path)
	if err == nil {
		return !info.IsDir(), nil
	}
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	return false, err
}

func (s *Server) resolveMonitoredRepository(repository string) (config.MonitoredRepository, bool) {
	for _, monitored := range s.config.App().MonitoredRepositories {
		if canonicalWebRepositoryID(monitored.Repository) != canonicalWebRepositoryID(repository) {
			continue
		}
		return monitored, true
	}
	return config.MonitoredRepository{}, false
}

func canonicalWebRepositoryID(repository string) string {
	trimmed := strings.TrimSpace(repository)
	if trimmed == "" {
		return ""
	}

	candidate := strings.TrimSuffix(trimmed, ".git")
	if strings.HasPrefix(candidate, "git@") {
		if idx := strings.LastIndex(candidate, ":"); idx >= 0 && idx+1 < len(candidate) {
			candidate = candidate[idx+1:]
		}
	}
	if strings.Contains(candidate, "://") {
		if parsed, err := url.Parse(candidate); err == nil {
			candidate = strings.Trim(parsed.Path, "/")
		}
	}

	candidate = strings.Trim(path.Clean(strings.ReplaceAll(candidate, "\\", "/")), "/")
	parts := strings.Split(candidate, "/")
	if len(parts) >= 2 {
		candidate = strings.Join(parts[len(parts)-2:], "/")
	}
	return strings.ToLower(candidate)
}

func writeImprovementTextFile(path string, content string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	if strings.TrimSpace(content) == "" {
		if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
		return nil
	}
	return os.WriteFile(path, []byte(strings.TrimRight(content, "\n")+"\n"), 0o644)
}

func loadArtifactIfExists(path string) (*artifactResponse, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, err
	}
	return &artifactResponse{
		Path:    filepath.ToSlash(path),
		Content: string(raw),
	}, nil
}
