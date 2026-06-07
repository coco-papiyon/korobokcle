package web

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/gorilla/mux"
	"gopkg.in/yaml.v3"

	"github.com/coco-papiyon/korobokcle/internal/artifacts"
	"github.com/coco-papiyon/korobokcle/internal/config"
)

const (
	defaultImprovementBranch  = "develop"
	defaultImprovementDir     = ".improvements"
	defaultImprovementWorkDir = ".improvement"
)

type improvementListItemResponse struct {
	Repository     string `json:"repository"`
	IssueNumber    int    `json:"issueNumber"`
	Title          string `json:"title"`
	State          string `json:"state"`
	UpdatedAt      string `json:"updatedAt"`
	DraftPath      string `json:"draftPath,omitempty"`
	RelatedJobID   string `json:"relatedJobId,omitempty"`
	DecisionReason string `json:"decisionReason,omitempty"`
}

type improvementDetailResponse struct {
	Repository         string   `json:"repository"`
	IssueNumber        int      `json:"issueNumber"`
	State              string   `json:"state"`
	Title              string   `json:"title"`
	Phases             []string `json:"phases"`
	Input              string   `json:"input"`
	Draft              string   `json:"draft"`
	Result             string   `json:"result"`
	Decision           string   `json:"decision"`
	ApprovalStatus     string   `json:"approvalStatus"`
	DecisionReason     string   `json:"decisionReason"`
	RelatedJobID       string   `json:"relatedJobId"`
	ImprovementBranch  string   `json:"improvementBranch"`
	ImprovementDir     string   `json:"improvementDir"`
	ImprovementWorkDir string   `json:"improvementWorkDir"`
	DraftPath          string   `json:"draftPath"`
	UpdatedAt          string   `json:"updatedAt"`
}

type improvementApprovalState struct {
	Status     string `json:"status"`
	Comment    string `json:"comment,omitempty"`
	ApprovedAt string `json:"approvedAt,omitempty"`
}

type improvementDecisionState struct {
	Decision  string `json:"decision"`
	Reason    string `json:"reason,omitempty"`
	UpdatedAt string `json:"updatedAt,omitempty"`
}

type updateImprovementDraftRequest struct {
	Draft string `json:"draft"`
}

type approveImprovementRequest struct {
	Status  string `json:"status,omitempty"`
	Comment string `json:"comment"`
}

type generateImprovementRequest struct {
	Comment string `json:"comment"`
}

func (s *Server) handleImprovements(w http.ResponseWriter, r *http.Request) {
	items, err := s.listImprovements()
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, items)
}

func (s *Server) handleImprovementDetail(w http.ResponseWriter, r *http.Request) {
	repository := strings.TrimSpace(r.URL.Query().Get("repository"))
	issueNumber, err := strconv.Atoi(strings.TrimSpace(r.URL.Query().Get("issueNumber")))
	if err != nil || issueNumber < 1 {
		writeJSONError(w, http.StatusBadRequest, fmt.Errorf("issueNumber must be a positive integer"))
		return
	}
	detail, err := s.loadImprovementDetail(repository, issueNumber)
	if err != nil {
		status := http.StatusInternalServerError
		if os.IsNotExist(err) {
			status = http.StatusNotFound
		}
		writeJSONError(w, status, err)
		return
	}
	writeJSON(w, http.StatusOK, detail)
}

func (s *Server) handleSaveImprovementDraft(w http.ResponseWriter, r *http.Request) {
	repository := strings.TrimSpace(r.URL.Query().Get("repository"))
	issueNumber, err := strconv.Atoi(strings.TrimSpace(r.URL.Query().Get("issueNumber")))
	if err != nil || issueNumber < 1 {
		writeJSONError(w, http.StatusBadRequest, fmt.Errorf("issueNumber must be a positive integer"))
		return
	}
	var payload updateImprovementDraftRequest
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		writeJSONError(w, http.StatusBadRequest, fmt.Errorf("decode draft payload: %w", err))
		return
	}
	if err := s.saveImprovementDraft(repository, issueNumber, payload.Draft); err != nil {
		writeJSONError(w, http.StatusInternalServerError, err)
		return
	}
	s.handleImprovementDetail(w, r)
}

func (s *Server) handleApproveImprovement(w http.ResponseWriter, r *http.Request) {
	repository := strings.TrimSpace(r.URL.Query().Get("repository"))
	issueNumber, err := strconv.Atoi(strings.TrimSpace(r.URL.Query().Get("issueNumber")))
	if err != nil || issueNumber < 1 {
		writeJSONError(w, http.StatusBadRequest, fmt.Errorf("issueNumber must be a positive integer"))
		return
	}
	var payload approveImprovementRequest
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		writeJSONError(w, http.StatusBadRequest, fmt.Errorf("decode approval payload: %w", err))
		return
	}
	if err := s.approveImprovement(repository, issueNumber, payload.Status, payload.Comment); err != nil {
		writeJSONError(w, http.StatusInternalServerError, err)
		return
	}
	s.handleImprovementDetail(w, r)
}

func (s *Server) handleGenerateImprovement(w http.ResponseWriter, r *http.Request) {
	jobID := mux.Vars(r)["id"]
	if s.improvementGenerator == nil {
		writeJSONError(w, http.StatusInternalServerError, fmt.Errorf("improvement generator is not configured"))
		return
	}
	var payload generateImprovementRequest
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		writeJSONError(w, http.StatusBadRequest, fmt.Errorf("decode improvement payload: %w", err))
		return
	}
	if err := s.improvementGenerator(r.Context(), jobID, payload.Comment); err != nil {
		writeJSONError(w, http.StatusInternalServerError, err)
		return
	}
	job, _, err := s.orchestrator.JobDetail(r.Context(), jobID)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, err)
		return
	}
	r.URL.RawQuery = (&url.Values{
		"repository":  []string{job.Repository},
		"issueNumber": []string{strconv.Itoa(job.GitHubNumber)},
	}).Encode()
	s.handleImprovementDetail(w, r)
}

func (s *Server) listImprovements() ([]improvementListItemResponse, error) {
	app := s.config.App()
	out := make([]improvementListItemResponse, 0)
	for _, repo := range app.MonitoredRepositories {
		if !repo.ImprovementEnabled {
			continue
		}
		baseDir := filepath.Join(artifacts.WorkersDir(s.config.Root(), app.ArtifactsDir), improvementRepositoryComponent(repo.Repository), "jobs")
		entries, err := os.ReadDir(baseDir)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return nil, err
		}
		for _, entry := range entries {
			if !entry.IsDir() || !strings.HasPrefix(entry.Name(), "issue_") {
				continue
			}
			issueNumber, err := strconv.Atoi(strings.TrimPrefix(entry.Name(), "issue_"))
			if err != nil {
				continue
			}
			item, err := s.loadImprovementDetail(repo.Repository, issueNumber)
			if err != nil {
				continue
			}
			out = append(out, improvementListItemResponse{
				Repository:     repo.Repository,
				IssueNumber:    issueNumber,
				Title:          item.Title,
				State:          item.State,
				UpdatedAt:      item.UpdatedAt,
				DraftPath:      item.DraftPath,
				RelatedJobID:   item.RelatedJobID,
				DecisionReason: item.DecisionReason,
			})
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].UpdatedAt == out[j].UpdatedAt {
			if out[i].Repository == out[j].Repository {
				return out[i].IssueNumber > out[j].IssueNumber
			}
			return out[i].Repository < out[j].Repository
		}
		return out[i].UpdatedAt > out[j].UpdatedAt
	})
	return out, nil
}

func (s *Server) loadImprovementDetail(repository string, issueNumber int) (improvementDetailResponse, error) {
	repo, ok := s.findMonitoredRepository(repository)
	if !ok || !repo.ImprovementEnabled {
		return improvementDetailResponse{}, os.ErrNotExist
	}
	app := s.config.App()
	jobDir := artifacts.RepositoryWorkerJobDir(s.config.Root(), app.ArtifactsDir, repo.Repository, issueNumber)
	improvementDir := filepath.Join(jobDir, "improvement")
	if info, err := os.Stat(improvementDir); err != nil || !info.IsDir() {
		return improvementDetailResponse{}, os.ErrNotExist
	}
	if entries, err := os.ReadDir(improvementDir); err != nil {
		return improvementDetailResponse{}, err
	} else if len(entries) == 0 {
		return improvementDetailResponse{}, os.ErrNotExist
	}
	workDir := artifacts.RepositoryWorkerWorkDir(s.config.Root(), app.ArtifactsDir, repo.Repository, repo.WorkDir)
	draftPath := artifacts.RepositoryWorkerImprovementDraftPath(workDir, improvementWorkDir(repo), issueNumber, loadImprovementTitle(improvementDir, issueNumber))

	input := readTextIfExists(filepath.Join(improvementDir, "input.md"))
	draft := strings.TrimSpace(readTextIfExists(draftPath))
	if draft == "" {
		draft = readTextIfExists(filepath.Join(improvementDir, "draft.md"))
	}
	result := readTextIfExists(filepath.Join(improvementDir, "result.md"))
	approval := readApprovalState(filepath.Join(improvementDir, "approval.json"))
	decision := readDecisionState(filepath.Join(improvementDir, "decision.json"))
	title := loadImprovementTitle(improvementDir, issueNumber)
	phases := loadImprovementPhases(improvementDir)
	state := deriveImprovementState(decision, approval, draft)
	if state == "" {
		state = "draft_created"
	}
	updatedAt := latestModTime(
		filepath.Join(improvementDir, "decision.json"),
		filepath.Join(improvementDir, "approval.json"),
		filepath.Join(improvementDir, "result.md"),
		filepath.Join(improvementDir, "draft.md"),
		draftPath,
	)
	if updatedAt.IsZero() {
		if stat, err := os.Stat(improvementDir); err == nil {
			updatedAt = stat.ModTime()
		}
	}
	return improvementDetailResponse{
		Repository:         repository,
		IssueNumber:        issueNumber,
		State:              state,
		Title:              title,
		Phases:             phases,
		Input:              input,
		Draft:              draft,
		Result:             result,
		Decision:           decision.Decision,
		ApprovalStatus:     approval.Status,
		DecisionReason:     decision.Reason,
		RelatedJobID:       findImprovementRelatedJobID(improvementDir),
		ImprovementBranch:  improvementBranch(repo),
		ImprovementDir:     improvementDirSetting(repo),
		ImprovementWorkDir: improvementWorkDir(repo),
		DraftPath:          draftPath,
		UpdatedAt:          updatedAt.UTC().Format(time.RFC3339),
	}, nil
}

func (s *Server) saveImprovementDraft(repository string, issueNumber int, draft string) error {
	repo, ok := s.findMonitoredRepository(repository)
	if !ok || !repo.ImprovementEnabled {
		return os.ErrNotExist
	}
	app := s.config.App()
	workDir := artifacts.RepositoryWorkerWorkDir(s.config.Root(), app.ArtifactsDir, repo.Repository, repo.WorkDir)
	jobDir := artifacts.RepositoryWorkerJobDir(s.config.Root(), app.ArtifactsDir, repo.Repository, issueNumber)
	improvementDir := filepath.Join(jobDir, "improvement")
	title := loadImprovementTitle(improvementDir, issueNumber)
	draftPath := artifacts.RepositoryWorkerImprovementDraftPath(workDir, improvementWorkDir(repo), issueNumber, title)
	if err := os.MkdirAll(filepath.Dir(draftPath), 0o755); err != nil {
		return err
	}
	if err := os.MkdirAll(improvementDir, 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(draftPath, []byte(draft), 0o644); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(improvementDir, "draft.md"), []byte(draft), 0o644); err != nil {
		return err
	}
	decision := improvementDecisionState{
		Decision:  "draft_created",
		UpdatedAt: time.Now().UTC().Format(time.RFC3339),
	}
	return writeJSONFile(filepath.Join(improvementDir, "decision.json"), decision)
}

func (s *Server) approveImprovement(repository string, issueNumber int, status string, comment string) error {
	repo, ok := s.findMonitoredRepository(repository)
	if !ok || !repo.ImprovementEnabled {
		return os.ErrNotExist
	}
	app := s.config.App()
	jobDir := artifacts.RepositoryWorkerJobDir(s.config.Root(), app.ArtifactsDir, repo.Repository, issueNumber)
	improvementDir := filepath.Join(jobDir, "improvement")
	detail, err := s.loadImprovementDetail(repository, issueNumber)
	if err != nil {
		return err
	}
	if strings.TrimSpace(detail.Draft) == "" {
		if !strings.EqualFold(strings.TrimSpace(status), "no_improvement_needed") {
			return fmt.Errorf("draft is empty")
		}
	}
	if err := os.MkdirAll(improvementDir, 0o755); err != nil {
		return err
	}
	normalizedStatus := strings.TrimSpace(status)
	if normalizedStatus == "" {
		normalizedStatus = "approved"
	}
	if strings.EqualFold(normalizedStatus, "no_improvement_needed") {
		return writeNoImprovementNeeded(improvementDir, comment)
	}
	if s.improvementApprover != nil {
		if err := s.improvementApprover(context.Background(), repo.Repository, issueNumber, detail.Title, detail.Draft, detail.RelatedJobID); err != nil {
			return err
		}
	} else {
		approvedDir := artifacts.RepositoryWorkerImprovementApprovedDir(
			artifacts.RepositoryWorkerWorkDir(s.config.Root(), app.ArtifactsDir, repo.Repository, repo.WorkDir),
			improvementDirSetting(repo),
		)
		if err := os.MkdirAll(approvedDir, 0o755); err != nil {
			return err
		}
		fileName := sanitizedImprovementFileName(detail.Title, issueNumber)
		if err := os.WriteFile(filepath.Join(approvedDir, fileName), []byte(detail.Draft), 0o644); err != nil {
			return err
		}
	}
	if err := os.WriteFile(filepath.Join(improvementDir, "result.md"), []byte(detail.Draft), 0o644); err != nil {
		return err
	}
	if err := writeJSONFile(filepath.Join(improvementDir, "approval.json"), improvementApprovalState{
		Status:     "approved",
		Comment:    strings.TrimSpace(comment),
		ApprovedAt: time.Now().UTC().Format(time.RFC3339),
	}); err != nil {
		return err
	}
	if err := writeJSONFile(filepath.Join(improvementDir, "decision.json"), improvementDecisionState{
		Decision:  "approved",
		UpdatedAt: time.Now().UTC().Format(time.RFC3339),
	}); err != nil {
		return err
	}
	return nil
}

func writeNoImprovementNeeded(improvementDir string, comment string) error {
	reason := strings.TrimSpace(comment)
	if reason == "" {
		reason = "ユーザが恒久改善不要と判断しました。"
	}
	if err := writeJSONFile(filepath.Join(improvementDir, "decision.json"), improvementDecisionState{
		Decision:  "no_improvement_needed",
		Reason:    reason,
		UpdatedAt: time.Now().UTC().Format(time.RFC3339),
	}); err != nil {
		return err
	}
	return writeJSONFile(filepath.Join(improvementDir, "approval.json"), improvementApprovalState{
		Status:     "no_improvement_needed",
		Comment:    reason,
		ApprovedAt: time.Now().UTC().Format(time.RFC3339),
	})
}

func (s *Server) findMonitoredRepository(repository string) (config.MonitoredRepository, bool) {
	for _, repo := range s.config.App().MonitoredRepositories {
		if strings.TrimSpace(repo.Repository) == strings.TrimSpace(repository) {
			return repo, true
		}
	}
	return config.MonitoredRepository{}, false
}

func improvementBranch(repo config.MonitoredRepository) string {
	if trimmed := strings.TrimSpace(repo.ImprovementBranch); trimmed != "" {
		return trimmed
	}
	return defaultImprovementBranch
}

func improvementDirSetting(repo config.MonitoredRepository) string {
	if trimmed := strings.TrimSpace(repo.ImprovementDir); trimmed != "" {
		return trimmed
	}
	return defaultImprovementDir
}

func improvementWorkDir(repo config.MonitoredRepository) string {
	if trimmed := strings.TrimSpace(repo.ImprovementWorkDir); trimmed != "" {
		return trimmed
	}
	return defaultImprovementWorkDir
}

func improvementRepositoryComponent(repository string) string {
	return artifacts.RepositoryWorkerComponent(repository)
}

func readTextIfExists(path string) string {
	raw, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return string(raw)
}

func readApprovalState(path string) improvementApprovalState {
	var out improvementApprovalState
	raw, err := os.ReadFile(path)
	if err != nil {
		return out
	}
	_ = json.Unmarshal(raw, &out)
	return out
}

func readDecisionState(path string) improvementDecisionState {
	var out improvementDecisionState
	raw, err := os.ReadFile(path)
	if err != nil {
		return out
	}
	_ = json.Unmarshal(raw, &out)
	return out
}

func latestModTime(paths ...string) time.Time {
	var latest time.Time
	for _, path := range paths {
		if stat, err := os.Stat(path); err == nil && stat.ModTime().After(latest) {
			latest = stat.ModTime()
		}
	}
	return latest
}

func deriveImprovementState(decision improvementDecisionState, approval improvementApprovalState, draft string) string {
	if strings.TrimSpace(decision.Decision) != "" {
		return decision.Decision
	}
	if strings.TrimSpace(approval.Status) != "" {
		return approval.Status
	}
	if strings.TrimSpace(draft) != "" {
		return "draft_created"
	}
	return ""
}

func loadImprovementTitle(improvementDir string, issueNumber int) string {
	for _, candidate := range []string{
		filepath.Join(improvementDir, "result.md"),
		filepath.Join(improvementDir, "draft.md"),
	} {
		if title := readImprovementFrontMatterTitle(candidate); title != "" {
			return title
		}
	}
	for _, candidate := range []string{
		filepath.Join(improvementDir, "result.md"),
		filepath.Join(improvementDir, "draft.md"),
		filepath.Join(improvementDir, "input.md"),
	} {
		if title := firstMeaningfulLine(readTextIfExists(candidate)); title != "" {
			return title
		}
	}
	return fmt.Sprintf("改善案 #%d", issueNumber)
}

func firstMeaningfulLine(text string) string {
	for _, line := range strings.Split(text, "\n") {
		trimmed := strings.TrimSpace(strings.TrimLeft(line, "#- "))
		if trimmed != "" {
			return trimmed
		}
	}
	return ""
}

func findImprovementRelatedJobID(improvementDir string) string {
	type improvementContext struct {
		JobID string `json:"jobId"`
	}
	var ctx improvementContext
	if raw, err := os.ReadFile(filepath.Join(improvementDir, "context.json")); err == nil {
		if err := json.Unmarshal(raw, &ctx); err == nil && strings.TrimSpace(ctx.JobID) != "" {
			return strings.TrimSpace(ctx.JobID)
		}
	}
	for _, candidate := range []string{
		filepath.Join(improvementDir, "result.md"),
		filepath.Join(improvementDir, "draft.md"),
	} {
		if jobID := readImprovementFrontMatterJobID(candidate); jobID != "" {
			return jobID
		}
	}
	return ""
}

func sanitizedImprovementFileName(title string, issueNumber int) string {
	trimmed := strings.TrimSpace(title)
	if trimmed == "" {
		return fmt.Sprintf("issue_%d.md", issueNumber)
	}
	replacer := strings.NewReplacer("/", "-", "\\", "-", ":", "-", "@", "-", "?", "-", "#", "-", " ", "-")
	return fmt.Sprintf("issue_%d_%s.md", issueNumber, replacer.Replace(trimmed))
}

func writeJSONFile(path string, value any) error {
	raw, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, raw, 0o644)
}

type improvementDocumentFrontMatter struct {
	Title  string   `yaml:"title"`
	Phases []string `yaml:"phases"`
	Source struct {
		JobID string `yaml:"jobId"`
	} `yaml:"source"`
}

func readImprovementFrontMatterTitle(path string) string {
	meta, ok := readImprovementFrontMatter(path)
	if !ok {
		return ""
	}
	return strings.TrimSpace(meta.Title)
}

func readImprovementFrontMatterJobID(path string) string {
	meta, ok := readImprovementFrontMatter(path)
	if !ok {
		return ""
	}
	return strings.TrimSpace(meta.Source.JobID)
}

func readImprovementFrontMatterPhases(path string) []string {
	meta, ok := readImprovementFrontMatter(path)
	if !ok || len(meta.Phases) == 0 {
		return nil
	}
	phases := make([]string, 0, len(meta.Phases))
	for _, phase := range meta.Phases {
		if trimmed := strings.TrimSpace(phase); trimmed != "" {
			phases = append(phases, trimmed)
		}
	}
	return phases
}

func loadImprovementPhases(improvementDir string) []string {
	for _, candidate := range []string{
		filepath.Join(improvementDir, "result.md"),
		filepath.Join(improvementDir, "draft.md"),
	} {
		if phases := readImprovementFrontMatterPhases(candidate); len(phases) > 0 {
			return phases
		}
	}
	return []string{}
}

func readImprovementFrontMatter(path string) (improvementDocumentFrontMatter, bool) {
	var meta improvementDocumentFrontMatter
	raw, err := os.ReadFile(path)
	if err != nil {
		return meta, false
	}
	text := string(raw)
	if !strings.HasPrefix(text, "---\n") {
		return meta, false
	}
	rest := strings.TrimPrefix(text, "---\n")
	idx := strings.Index(rest, "\n---\n")
	if idx < 0 {
		return meta, false
	}
	if err := yaml.Unmarshal([]byte(rest[:idx]), &meta); err != nil {
		return meta, false
	}
	return meta, true
}
