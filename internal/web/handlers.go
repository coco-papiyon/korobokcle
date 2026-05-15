package web

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/gorilla/mux"

	"github.com/coco-papiyon/korobokcle/internal/config"
	"github.com/coco-papiyon/korobokcle/internal/domain"
	"github.com/coco-papiyon/korobokcle/internal/orchestrator"
)

type jobResponse struct {
	ID           string `json:"id"`
	Type         string `json:"type"`
	Repository   string `json:"repository"`
	GitHubNumber int    `json:"githubNumber"`
	State        string `json:"state"`
	Title        string `json:"title"`
	BranchName   string `json:"branchName"`
	WatchRuleID  string `json:"watchRuleId"`
	CreatedAt    string `json:"createdAt"`
	UpdatedAt    string `json:"updatedAt"`
}

type eventResponse struct {
	ID               int64    `json:"id"`
	JobID            string   `json:"jobId"`
	EventType        string   `json:"eventType"`
	StateFrom        string   `json:"stateFrom"`
	StateTo          string   `json:"stateTo"`
	Payload          string   `json:"payload"`
	CreatedAt        string   `json:"createdAt"`
	AvailableActions []string `json:"availableActions"`
}

type jobDetailResponse struct {
	Job                    jobResponse       `json:"job"`
	Events                 []eventResponse   `json:"events"`
	DesignArtifact         *artifactResponse `json:"designArtifact,omitempty"`
	ImplementationArtifact *artifactResponse `json:"implementationArtifact,omitempty"`
	TestReport             *artifactResponse `json:"testReport,omitempty"`
	PRCreateArtifact       *artifactResponse `json:"prCreateArtifact,omitempty"`
}

type artifactResponse struct {
	Path    string `json:"path"`
	Content string `json:"content"`
}

type watchRuleResponse struct {
	ID             string   `json:"id"`
	Name           string   `json:"name"`
	Repositories   []string `json:"repositories"`
	Target         string   `json:"target"`
	Labels         []string `json:"labels"`
	TitlePattern   string   `json:"titlePattern"`
	Authors        []string `json:"authors"`
	Assignees      []string `json:"assignees"`
	ExcludeDraftPR bool     `json:"excludeDraftPR"`
	SkillSet       string   `json:"skillSet"`
	TestProfile    string   `json:"testProfile"`
	Enabled        bool     `json:"enabled"`
}

type appConfigResponse struct {
	Provider string `json:"provider"`
}

func (s *Server) handleJobs(w http.ResponseWriter, r *http.Request) {
	jobs, err := s.orchestrator.ListJobs(r.Context())
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, err)
		return
	}

	out := make([]jobResponse, 0, len(jobs))
	for _, job := range jobs {
		out = append(out, toJobResponse(job))
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) handleJobDetail(w http.ResponseWriter, r *http.Request) {
	jobID := mux.Vars(r)["id"]
	job, events, err := s.orchestrator.JobDetail(r.Context(), jobID)
	if err != nil {
		writeJSONError(w, http.StatusNotFound, err)
		return
	}

	out := jobDetailResponse{
		Job:    toJobResponse(job),
		Events: make([]eventResponse, 0, len(events)),
	}
	for _, event := range events {
		out.Events = append(out.Events, eventResponse{
			ID:               event.ID,
			JobID:            event.JobID,
			EventType:        event.EventType,
			StateFrom:        event.StateFrom,
			StateTo:          event.StateTo,
			Payload:          event.Payload,
			CreatedAt:        event.CreatedAt.Format(timeFormat),
			AvailableActions: availableActionsForEvent(event),
		})
	}
	if artifact, err := s.loadDesignArtifact(job.ID); err == nil {
		out.DesignArtifact = artifact
	}
	if artifact, err := s.loadImplementationArtifact(job.ID); err == nil {
		out.ImplementationArtifact = artifact
	}
	if artifact, err := s.loadTestReport(job.ID); err == nil {
		out.TestReport = artifact
	}
	if artifact, err := s.loadPRCreateArtifact(job.ID); err == nil {
		out.PRCreateArtifact = artifact
	}
	writeJSON(w, http.StatusOK, out)
}

type approvalRequest struct {
	Status  string `json:"status"`
	Comment string `json:"comment"`
	EventID *int64 `json:"eventId,omitempty"`
}

func (s *Server) handleDesignApproval(w http.ResponseWriter, r *http.Request) {
	jobID := mux.Vars(r)["id"]
	var payload approvalRequest
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		writeJSONError(w, http.StatusBadRequest, fmt.Errorf("decode approval request: %w", err))
		return
	}

	switch strings.TrimSpace(payload.Status) {
	case "approved":
		if err := s.orchestrator.ApproveDesign(r.Context(), jobID, payload.Comment); err != nil {
			writeJSONError(w, http.StatusInternalServerError, err)
			return
		}
	case "rejected":
		if err := s.orchestrator.RejectDesign(r.Context(), jobID, payload.Comment); err != nil {
			writeJSONError(w, http.StatusInternalServerError, err)
			return
		}
	default:
		writeJSONError(w, http.StatusBadRequest, fmt.Errorf("status must be approved or rejected"))
		return
	}

	s.handleJobDetail(w, r)
}

func (s *Server) handleFinalApproval(w http.ResponseWriter, r *http.Request) {
	jobID := mux.Vars(r)["id"]
	var payload approvalRequest
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		writeJSONError(w, http.StatusBadRequest, fmt.Errorf("decode approval request: %w", err))
		return
	}

	switch strings.TrimSpace(payload.Status) {
	case "approved":
		if err := s.orchestrator.ApproveFinal(r.Context(), jobID, payload.Comment); err != nil {
			writeJSONError(w, http.StatusInternalServerError, err)
			return
		}
	case "rejected":
		if err := s.orchestrator.RejectFinal(r.Context(), jobID, payload.Comment); err != nil {
			writeJSONError(w, http.StatusInternalServerError, err)
			return
		}
	default:
		writeJSONError(w, http.StatusBadRequest, fmt.Errorf("status must be approved or rejected"))
		return
	}

	s.handleJobDetail(w, r)
}

func (s *Server) handleDesignRerun(w http.ResponseWriter, r *http.Request) {
	jobID := mux.Vars(r)["id"]
	var payload approvalRequest
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		writeJSONError(w, http.StatusBadRequest, fmt.Errorf("decode rerun request: %w", err))
		return
	}

	if err := s.orchestrator.RerunDesignFromEvent(r.Context(), jobID, payload.EventID, payload.Comment); err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, orchestrator.ErrInvalidStateTransition) {
			status = http.StatusBadRequest
		}
		writeJSONError(w, status, err)
		return
	}

	s.handleJobDetail(w, r)
}

func (s *Server) handleImplementationRerun(w http.ResponseWriter, r *http.Request) {
	jobID := mux.Vars(r)["id"]
	var payload approvalRequest
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		writeJSONError(w, http.StatusBadRequest, fmt.Errorf("decode rerun request: %w", err))
		return
	}

	if err := s.orchestrator.RerunImplementationFromEvent(r.Context(), jobID, payload.EventID, payload.Comment); err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, orchestrator.ErrInvalidStateTransition) {
			status = http.StatusBadRequest
		}
		writeJSONError(w, status, err)
		return
	}

	s.handleJobDetail(w, r)
}

func (s *Server) handlePRRerun(w http.ResponseWriter, r *http.Request) {
	jobID := mux.Vars(r)["id"]
	var payload approvalRequest
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		writeJSONError(w, http.StatusBadRequest, fmt.Errorf("decode rerun request: %w", err))
		return
	}

	if err := s.orchestrator.RerunPRCreationFromEvent(r.Context(), jobID, payload.EventID, payload.Comment); err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, orchestrator.ErrInvalidStateTransition) {
			status = http.StatusBadRequest
		}
		writeJSONError(w, status, err)
		return
	}

	s.handleJobDetail(w, r)
}

func (s *Server) handleWatchRules(w http.ResponseWriter, r *http.Request) {
	watchRules := s.config.WatchRules()
	rules := make([]watchRuleResponse, 0, len(watchRules.Rules))
	for _, rule := range watchRules.Rules {
		rules = append(rules, watchRuleResponse{
			ID:             rule.ID,
			Name:           rule.Name,
			Repositories:   sliceOrEmpty(rule.Repositories),
			Target:         rule.Target,
			Labels:         sliceOrEmpty(rule.Labels),
			TitlePattern:   rule.TitlePattern,
			Authors:        sliceOrEmpty(rule.Authors),
			Assignees:      sliceOrEmpty(rule.Assignees),
			ExcludeDraftPR: rule.ExcludeDraftPR,
			SkillSet:       rule.SkillSet,
			TestProfile:    rule.TestProfile,
			Enabled:        rule.Enabled,
		})
	}
	writeJSON(w, http.StatusOK, rules)
}

func (s *Server) handleAppConfig(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, appConfigResponse{
		Provider: s.config.App().Provider,
	})
}

func (s *Server) handleSaveAppConfig(w http.ResponseWriter, r *http.Request) {
	var payload appConfigResponse
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		writeJSONError(w, http.StatusBadRequest, fmt.Errorf("decode app config: %w", err))
		return
	}

	provider := strings.ToLower(strings.TrimSpace(payload.Provider))
	switch provider {
	case "mock", "copilot", "codex":
	default:
		writeJSONError(w, http.StatusBadRequest, fmt.Errorf("provider must be mock, copilot, or codex"))
		return
	}

	appConfig := s.config.App()
	appConfig.Provider = provider
	if err := s.config.UpdateApp(appConfig); err != nil {
		writeJSONError(w, http.StatusInternalServerError, err)
		return
	}

	writeJSON(w, http.StatusOK, appConfigResponse{Provider: provider})
}

const (
	actionRetryDesign         = "retry_design"
	actionRetryImplementation = "retry_implementation"
	actionRetryPR             = "retry_pr"
)

func availableActionsForEvent(event domain.Event) []string {
	actions := make([]string, 0, 1)

	switch {
	case event.EventType == "design_ready" && event.StateFrom == string(domain.StateDesignRunning):
		actions = append(actions, actionRetryDesign)
	case event.EventType == "design_failed" || event.EventType == "design_rejected":
		actions = append(actions, actionRetryDesign)
	}

	switch {
	case event.EventType == "waiting_final_approval" && event.StateFrom == string(domain.StateImplementationReady):
		actions = append(actions, actionRetryImplementation)
	case event.EventType == "implementation_failed" || event.EventType == "test_failed" || event.EventType == "final_rejected":
		actions = append(actions, actionRetryImplementation)
	}

	switch {
	case event.EventType == "pr_created" && event.StateFrom == string(domain.StatePRCreating):
		actions = append(actions, actionRetryPR)
	case event.EventType == "pr_push_failed" || event.EventType == "pr_create_failed":
		actions = append(actions, actionRetryPR)
	}

	return actions
}

func (s *Server) handleSaveWatchRules(w http.ResponseWriter, r *http.Request) {
	var payload []watchRuleResponse
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		writeJSONError(w, http.StatusBadRequest, fmt.Errorf("decode watch rules: %w", err))
		return
	}

	file := config.WatchRulesFile{
		Rules: make([]config.WatchRule, 0, len(payload)),
	}
	for index, rule := range payload {
		if strings.TrimSpace(rule.ID) == "" {
			writeJSONError(w, http.StatusBadRequest, fmt.Errorf("rule[%d].id is required", index))
			return
		}
		if strings.TrimSpace(rule.Name) == "" {
			writeJSONError(w, http.StatusBadRequest, fmt.Errorf("rule[%d].name is required", index))
			return
		}
		if rule.Target != string(domain.TargetIssue) && rule.Target != string(domain.TargetPullRequest) {
			writeJSONError(w, http.StatusBadRequest, fmt.Errorf("rule[%d].target must be issue or pull_request", index))
			return
		}
		file.Rules = append(file.Rules, config.WatchRule{
			ID:             strings.TrimSpace(rule.ID),
			Name:           strings.TrimSpace(rule.Name),
			Repositories:   compactStrings(rule.Repositories),
			Target:         rule.Target,
			Labels:         compactStrings(rule.Labels),
			TitlePattern:   strings.TrimSpace(rule.TitlePattern),
			Authors:        compactStrings(rule.Authors),
			Assignees:      compactStrings(rule.Assignees),
			ExcludeDraftPR: rule.ExcludeDraftPR,
			SkillSet:       strings.TrimSpace(rule.SkillSet),
			TestProfile:    strings.TrimSpace(rule.TestProfile),
			Enabled:        rule.Enabled,
		})
	}

	if err := s.config.UpdateWatchRules(file); err != nil {
		writeJSONError(w, http.StatusInternalServerError, err)
		return
	}
	s.handleWatchRules(w, r)
}

func (s *Server) handleHealth(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	_, _ = w.Write([]byte("ok"))
}

func (s *Server) handleSPA(w http.ResponseWriter, r *http.Request) {
	if !s.hasStaticDist() {
		http.Error(w, "frontend dist is missing; run npm install && npm run build in frontend", http.StatusServiceUnavailable)
		return
	}

	requestPath := filepath.Clean(r.URL.Path)
	if requestPath == "." || requestPath == "/" {
		http.ServeFile(w, r, filepath.Join(s.staticDir, "index.html"))
		return
	}

	target := filepath.Join(s.staticDir, requestPath)
	_, err := os.Stat(target)
	if err == nil {
		http.ServeFile(w, r, target)
		return
	}
	if !errors.Is(err, os.ErrNotExist) {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	http.ServeFile(w, r, filepath.Join(s.staticDir, "index.html"))
}

const timeFormat = "2006-01-02T15:04:05Z07:00"

func toJobResponse(job domain.Job) jobResponse {
	return jobResponse{
		ID:           job.ID,
		Type:         string(job.Type),
		Repository:   job.Repository,
		GitHubNumber: job.GitHubNumber,
		State:        string(job.State),
		Title:        job.Title,
		BranchName:   job.BranchName,
		WatchRuleID:  job.WatchRuleID,
		CreatedAt:    job.CreatedAt.Format(timeFormat),
		UpdatedAt:    job.UpdatedAt.Format(timeFormat),
	}
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func writeJSONError(w http.ResponseWriter, status int, err error) {
	writeJSON(w, status, map[string]string{"error": err.Error()})
}

func compactStrings(values []string) []string {
	out := make([]string, 0, len(values))
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed != "" {
			out = append(out, trimmed)
		}
	}
	return out
}

func sliceOrEmpty(values []string) []string {
	if values == nil {
		return []string{}
	}
	return values
}

func (s *Server) loadDesignArtifact(jobID string) (*artifactResponse, error) {
	path := filepath.Join(s.config.Root(), s.config.App().ArtifactsDir, "designs", jobID, "design.md")
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return &artifactResponse{
		Path:    path,
		Content: string(raw),
	}, nil
}

func (s *Server) loadImplementationArtifact(jobID string) (*artifactResponse, error) {
	path := filepath.Join(s.config.Root(), s.config.App().ArtifactsDir, "changes", jobID, "summary.md")
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return &artifactResponse{
		Path:    path,
		Content: string(raw),
	}, nil
}

func (s *Server) loadTestReport(jobID string) (*artifactResponse, error) {
	path := filepath.Join(s.config.Root(), s.config.App().ArtifactsDir, "changes", jobID, "test-report.json")
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return &artifactResponse{
		Path:    path,
		Content: string(raw),
	}, nil
}

func (s *Server) loadPRCreateArtifact(jobID string) (*artifactResponse, error) {
	path := filepath.Join(s.config.Root(), s.config.App().ArtifactsDir, "changes", jobID, "pr-create.json")
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return &artifactResponse{
		Path:    path,
		Content: string(raw),
	}, nil
}
