package web

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gorilla/mux"

	"github.com/coco-papiyon/korobokcle/internal/artifacts"
	"github.com/coco-papiyon/korobokcle/internal/config"
	"github.com/coco-papiyon/korobokcle/internal/domain"
	"github.com/coco-papiyon/korobokcle/internal/orchestrator"
	"github.com/coco-papiyon/korobokcle/internal/skill"
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
	SourceEventType  string   `json:"sourceEventType,omitempty"`
	AvailableActions []string `json:"availableActions"`
}

type jobDetailResponse struct {
	Job                    jobResponse       `json:"job"`
	Events                 []eventResponse   `json:"events"`
	IssueBody              string            `json:"issueBody,omitempty"`
	DesignArtifact         *artifactResponse `json:"designArtifact,omitempty"`
	ImplementationArtifact *artifactResponse `json:"implementationArtifact,omitempty"`
	FixArtifact            *artifactResponse `json:"fixArtifact,omitempty"`
	ReviewArtifact         *artifactResponse `json:"reviewArtifact,omitempty"`
	TestReport             *artifactResponse `json:"testReport,omitempty"`
	PRCreateArtifact       *artifactResponse `json:"prCreateArtifact,omitempty"`
	Logs                   []logResponse     `json:"logs,omitempty"`
}

type artifactResponse struct {
	Path    string `json:"path"`
	Content string `json:"content"`
}

type logResponse struct {
	Name    string `json:"name"`
	Phase   string `json:"phase"`
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
	Provider       string   `json:"provider"`
	Model          string   `json:"model"`
	SkillSet       string   `json:"skillSet"`
	TestProfile    string   `json:"testProfile"`
	Enabled        bool     `json:"enabled"`
}

type providerSpecResponse struct {
	Name   string   `json:"name"`
	Models []string `json:"models"`
}

type appConfigResponse struct {
	Provider     string                 `json:"provider"`
	Model        string                 `json:"model"`
	PollInterval int                    `json:"pollInterval"`
	Providers    []providerSpecResponse `json:"providers"`
}

type notificationChannelResponse struct {
	Name    string   `json:"name"`
	Type    string   `json:"type"`
	Events  []string `json:"events"`
	Enabled bool     `json:"enabled"`
}

type notificationConfigResponse struct {
	Channels []notificationChannelResponse `json:"channels"`
}

type skillSetSummaryResponse struct {
	Name    string `json:"name"`
	Mutable bool   `json:"mutable"`
}

type skillFileResponse struct {
	Definition     skill.Definition `json:"definition"`
	PromptTemplate string           `json:"promptTemplate"`
}

type skillSetResponse struct {
	Name    string                       `json:"name"`
	Mutable bool                         `json:"mutable"`
	Skills  map[string]skillFileResponse `json:"skills"`
}

type createSkillSetRequest struct {
	Name   string `json:"name"`
	Source string `json:"source"`
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
		sourceEventType := sourceEventTypeForEvent(events, event)
		if out.IssueBody == "" && event.EventType == string(domain.DomainEventIssueMatched) {
			out.IssueBody = extractIssueBody(event.Payload)
		}
		out.Events = append(out.Events, eventResponse{
			ID:               event.ID,
			JobID:            event.JobID,
			EventType:        event.EventType,
			StateFrom:        event.StateFrom,
			StateTo:          event.StateTo,
			Payload:          event.Payload,
			CreatedAt:        event.CreatedAt.Format(timeFormat),
			SourceEventType:  sourceEventType,
			AvailableActions: availableActionsForEvent(event),
		})
	}
	if artifact, err := s.loadDesignArtifact(job.ID); err == nil {
		out.DesignArtifact = artifact
	}
	if artifact, err := s.loadImplementationArtifact(job.ID); err == nil {
		out.ImplementationArtifact = artifact
	}
	if artifact, err := s.loadFixArtifact(job.ID); err == nil {
		out.FixArtifact = artifact
	}
	if artifact, err := s.loadReviewArtifact(job.ID); err == nil {
		out.ReviewArtifact = artifact
	}
	if artifact, err := s.loadTestReport(job.ID); err == nil {
		out.TestReport = artifact
	}
	if artifact, err := s.loadPRCreateArtifact(job.ID); err == nil {
		out.PRCreateArtifact = artifact
	}
	out.Logs = append(out.Logs, s.loadLogResponses("design", artifacts.WorkerDir(s.config.Root(), s.config.App().ArtifactsDir, job.ID, artifacts.WorkerDesign), []string{"stdout.log", "stderr.log"})...)
	out.Logs = append(out.Logs, s.loadLogResponses("implementation", artifacts.WorkerDir(s.config.Root(), s.config.App().ArtifactsDir, job.ID, artifacts.WorkerImplementation), []string{"stdout.log", "stderr.log"})...)
	out.Logs = append(out.Logs, s.loadLogResponses("fix", artifacts.WorkerDir(s.config.Root(), s.config.App().ArtifactsDir, job.ID, artifacts.WorkerFix), []string{"stdout.log", "stderr.log"})...)
	out.Logs = append(out.Logs, s.loadLogResponses("pr", artifacts.WorkerDir(s.config.Root(), s.config.App().ArtifactsDir, job.ID, artifacts.WorkerPR), []string{"git-push.log", "gh-pr-create.log"})...)
	out.Logs = append(out.Logs, s.loadLogResponses("review", artifacts.WorkerDir(s.config.Root(), s.config.App().ArtifactsDir, job.ID, artifacts.WorkerReview), []string{"stdout.log", "stderr.log"})...)
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
			status := http.StatusInternalServerError
			if errors.Is(err, orchestrator.ErrInvalidStateTransition) {
				status = http.StatusBadRequest
			}
			writeJSONError(w, status, err)
			return
		}
	case "rejected":
		if err := s.orchestrator.RejectFinal(r.Context(), jobID, payload.Comment); err != nil {
			status := http.StatusInternalServerError
			if errors.Is(err, orchestrator.ErrInvalidStateTransition) {
				status = http.StatusBadRequest
			}
			writeJSONError(w, status, err)
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

func (s *Server) handleReviewRerun(w http.ResponseWriter, r *http.Request) {
	jobID := mux.Vars(r)["id"]
	var payload approvalRequest
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		writeJSONError(w, http.StatusBadRequest, fmt.Errorf("decode rerun request: %w", err))
		return
	}

	if err := s.orchestrator.RerunReviewFromEvent(r.Context(), jobID, payload.EventID, payload.Comment); err != nil {
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
			Provider:       rule.Provider,
			Model:          rule.Model,
			SkillSet:       rule.SkillSet,
			TestProfile:    rule.TestProfile,
			Enabled:        rule.Enabled,
		})
	}
	writeJSON(w, http.StatusOK, rules)
}

func (s *Server) handleAppConfig(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, toAppConfigResponse(s.config.App()))
}

func (s *Server) handleSaveAppConfig(w http.ResponseWriter, r *http.Request) {
	var payload appConfigResponse
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		writeJSONError(w, http.StatusBadRequest, fmt.Errorf("decode app config: %w", err))
		return
	}

	appConfig := s.config.App()
	provider := strings.ToLower(strings.TrimSpace(payload.Provider))
	if provider == "" {
		provider = appConfig.Provider
	}
	if provider != appConfig.Provider || strings.TrimSpace(payload.Provider) != "" {
		if _, err := s.providerSpecByName(provider); err != nil {
			writeJSONError(w, http.StatusBadRequest, err)
			return
		}
		appConfig.Provider = provider
	}

	modelInput := strings.TrimSpace(payload.Model)
	if modelInput == "" {
		modelInput = appConfig.Model
	} else {
		model, err := s.validateModelForProvider(provider, modelInput)
		if err != nil {
			writeJSONError(w, http.StatusBadRequest, err)
			return
		}
		modelInput = model
	}
	appConfig.Model = modelInput
	if payload.PollInterval < 1 {
		writeJSONError(w, http.StatusBadRequest, fmt.Errorf("pollInterval must be a positive whole number of seconds"))
		return
	}
	if payload.PollInterval > 24*60*60 {
		writeJSONError(w, http.StatusBadRequest, fmt.Errorf("pollInterval must be no more than 86400 seconds"))
		return
	}
	appConfig.PollInterval = time.Duration(payload.PollInterval) * time.Second
	if err := s.config.UpdateApp(appConfig); err != nil {
		writeJSONError(w, http.StatusInternalServerError, err)
		return
	}

	writeJSON(w, http.StatusOK, toAppConfigResponse(appConfig))
}

func (s *Server) handleNotificationConfig(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, toNotificationConfigResponse(s.config.Notifications()))
}

func (s *Server) handleSaveNotificationConfig(w http.ResponseWriter, r *http.Request) {
	var payload notificationConfigResponse
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		writeJSONError(w, http.StatusBadRequest, fmt.Errorf("decode notification config: %w", err))
		return
	}

	file := config.Notifications{
		Channels: make([]config.NotificationChannel, 0, len(payload.Channels)),
	}
	for index, channel := range payload.Channels {
		name := strings.TrimSpace(channel.Name)
		if name == "" {
			writeJSONError(w, http.StatusBadRequest, fmt.Errorf("channel[%d].name is required", index))
			return
		}
		channelType := strings.TrimSpace(channel.Type)
		if channelType == "" {
			writeJSONError(w, http.StatusBadRequest, fmt.Errorf("channel[%d].type is required", index))
			return
		}
		file.Channels = append(file.Channels, config.NotificationChannel{
			Name:    name,
			Type:    channelType,
			Events:  compactStrings(channel.Events),
			Enabled: channel.Enabled,
		})
	}

	if err := s.config.UpdateNotifications(file); err != nil {
		writeJSONError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, toNotificationConfigResponse(file))
}

func (s *Server) handleSkillSets(w http.ResponseWriter, _ *http.Request) {
	sets, err := skill.ListSkillSets(s.config.Root())
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, err)
		return
	}

	response := make([]skillSetSummaryResponse, 0, len(sets))
	for _, set := range sets {
		response = append(response, skillSetSummaryResponse{
			Name:    set.Name,
			Mutable: set.Mutable,
		})
	}
	writeJSON(w, http.StatusOK, response)
}

func (s *Server) handleSkillSet(w http.ResponseWriter, r *http.Request) {
	name := mux.Vars(r)["name"]
	set, err := skill.LoadSkillSet(s.config.Root(), name)
	if err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, os.ErrNotExist) {
			status = http.StatusNotFound
		}
		writeJSONError(w, status, err)
		return
	}
	writeJSON(w, http.StatusOK, toSkillSetResponse(set))
}

func (s *Server) handleCreateSkillSet(w http.ResponseWriter, r *http.Request) {
	var payload createSkillSetRequest
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		writeJSONError(w, http.StatusBadRequest, fmt.Errorf("decode skill set: %w", err))
		return
	}

	set, err := skill.CreateSkillSet(s.config.Root(), payload.Name, payload.Source)
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusCreated, toSkillSetResponse(set))
}

func (s *Server) handleSaveSkillSet(w http.ResponseWriter, r *http.Request) {
	name := mux.Vars(r)["name"]
	var payload skillSetResponse
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		writeJSONError(w, http.StatusBadRequest, fmt.Errorf("decode skill set: %w", err))
		return
	}
	if strings.TrimSpace(payload.Name) != "" && strings.TrimSpace(payload.Name) != name {
		writeJSONError(w, http.StatusBadRequest, fmt.Errorf("payload name must match path"))
		return
	}

	set := skill.SkillSet{
		Name:   name,
		Skills: make(map[string]skill.SkillFile, len(payload.Skills)),
	}
	for skillName, file := range payload.Skills {
		set.Skills[skillName] = skill.SkillFile{
			Definition:     file.Definition,
			PromptTemplate: file.PromptTemplate,
		}
	}
	if err := skill.SaveSkillSet(s.config.Root(), set); err != nil {
		writeJSONError(w, http.StatusBadRequest, err)
		return
	}

	saved, err := skill.LoadSkillSet(s.config.Root(), name)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, toSkillSetResponse(saved))
}

func (s *Server) handleDeleteSkillSet(w http.ResponseWriter, r *http.Request) {
	name := mux.Vars(r)["name"]
	if err := skill.DeleteSkillSet(s.config.Root(), name); err != nil {
		status := http.StatusBadRequest
		if errors.Is(err, os.ErrNotExist) {
			status = http.StatusNotFound
		}
		writeJSONError(w, status, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

const (
	actionRetryDesign         = "retry_design"
	actionRetryImplementation = "retry_implementation"
	actionRetryPR             = "retry_pr"
	actionRetryReview         = "retry_review"
)

func availableActionsForEvent(event domain.Event) []string {
	actions := make([]string, 0, 1)

	switch event.StateTo {
	case string(domain.StateDesignReady):
		switch event.EventType {
		case "design_ready":
			actions = append(actions, actionRetryDesign)
		}
	case string(domain.StateImplementationReady):
		switch event.EventType {
		case "implementation_ready":
			actions = append(actions, actionRetryImplementation)
		}
	case string(domain.StateReviewReady), string(domain.StateCompleted):
		switch event.EventType {
		case "review_ready", "review_completed":
			actions = append(actions, actionRetryReview)
		case "pr_created":
			actions = append(actions, actionRetryPR)
		}
	}

	switch event.EventType {
	case "design_failed", "design_rejected":
		actions = append(actions, actionRetryDesign)
	case "implementation_failed", "test_failed", "final_rejected":
		actions = append(actions, actionRetryImplementation)
	case "pr_push_failed", "pr_create_failed":
		actions = append(actions, actionRetryPR)
	case "review_failed":
		actions = append(actions, actionRetryReview)
	}

	return actions
}

func sourceEventTypeForEvent(events []domain.Event, event domain.Event) string {
	if event.EventType != "implementation_failed" && event.EventType != "test_failed" && event.EventType != "pr_create_failed" && event.EventType != "review_failed" {
		return ""
	}

	for i := len(events) - 1; i >= 0; i-- {
		candidate := events[i]
		if candidate.ID >= event.ID {
			continue
		}
		if candidate.EventType == "implementation_rerun_requested" || candidate.EventType == "design_rerun_requested" || candidate.EventType == "pr_rerun_requested" || candidate.EventType == "review_rerun_requested" {
			var payload struct {
				EventID *int64 `json:"eventId"`
			}
			if err := json.Unmarshal([]byte(candidate.Payload), &payload); err != nil {
				return ""
			}
			if payload.EventID == nil {
				return ""
			}
			for j := i - 1; j >= 0; j-- {
				source := events[j]
				if source.ID == *payload.EventID {
					return source.EventType
				}
			}
		}
	}

	return ""
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
		provider, err := s.parseOptionalProvider(rule.Provider)
		if err != nil {
			writeJSONError(w, http.StatusBadRequest, fmt.Errorf("rule[%d].provider: %w", index, err))
			return
		}
		model, err := s.validateRuleModel(provider, rule.Model)
		if err != nil {
			writeJSONError(w, http.StatusBadRequest, fmt.Errorf("rule[%d].model: %w", index, err))
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
			Provider:       provider,
			Model:          model,
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

func toAppConfigResponse(app config.App) appConfigResponse {
	return appConfigResponse{
		Provider:     app.Provider,
		Model:        app.Model,
		PollInterval: int(effectivePollInterval(app.PollInterval) / time.Second),
		Providers:    toProviderSpecResponses(app.Providers),
	}
}

func toNotificationConfigResponse(notifications config.Notifications) notificationConfigResponse {
	channels := make([]notificationChannelResponse, 0, len(notifications.Channels))
	for _, channel := range notifications.Channels {
		channels = append(channels, notificationChannelResponse{
			Name:    channel.Name,
			Type:    channel.Type,
			Events:  sliceOrEmpty(channel.Events),
			Enabled: channel.Enabled,
		})
	}
	return notificationConfigResponse{Channels: channels}
}

func toSkillSetResponse(set skill.SkillSet) skillSetResponse {
	files := make(map[string]skillFileResponse, len(set.Skills))
	for name, file := range set.Skills {
		files[name] = skillFileResponse{
			Definition:     file.Definition,
			PromptTemplate: file.PromptTemplate,
		}
	}
	return skillSetResponse{
		Name:    set.Name,
		Mutable: set.Mutable,
		Skills:  files,
	}
}

func effectivePollInterval(value time.Duration) time.Duration {
	if value <= 0 {
		return config.DefaultPollInterval
	}
	return value
}

func toProviderSpecResponses(providers []config.ProviderSpec) []providerSpecResponse {
	out := make([]providerSpecResponse, 0, len(providers))
	for _, provider := range providers {
		out = append(out, providerSpecResponse{
			Name:   provider.Name,
			Models: sliceOrEmpty(provider.Models),
		})
	}
	return out
}

func (s *Server) providerSpecByName(name string) (config.ProviderSpec, error) {
	trimmed := strings.ToLower(strings.TrimSpace(name))
	if trimmed == "" {
		return config.ProviderSpec{}, fmt.Errorf("provider is required")
	}
	if spec, ok := s.config.ProviderByName(trimmed); ok {
		return spec, nil
	}
	return config.ProviderSpec{}, fmt.Errorf("provider must be one of %s", strings.Join(providerNames(s.config.Providers()), ", "))
}

func (s *Server) parseOptionalProvider(provider string) (string, error) {
	trimmed := strings.ToLower(strings.TrimSpace(provider))
	if trimmed == "" {
		return "", nil
	}
	if _, err := s.providerSpecByName(trimmed); err != nil {
		return "", err
	}
	return trimmed, nil
}

func (s *Server) validateModelForProvider(provider string, model string) (string, error) {
	trimmedModel := strings.TrimSpace(model)
	if trimmedModel == "" {
		return "", nil
	}
	spec, err := s.providerSpecByName(provider)
	if err != nil {
		return "", err
	}
	for _, candidate := range spec.Models {
		if candidate == trimmedModel {
			return trimmedModel, nil
		}
	}
	return "", fmt.Errorf("model must be one of %s", strings.Join(modelNames(spec), ", "))
}

func (s *Server) validateRuleModel(provider string, model string) (string, error) {
	effectiveProvider := strings.TrimSpace(provider)
	if effectiveProvider == "" {
		effectiveProvider = s.config.App().Provider
	}
	return s.validateModelForProvider(effectiveProvider, model)
}

func providerNames(providers []config.ProviderSpec) []string {
	names := make([]string, 0, len(providers))
	for _, provider := range providers {
		if trimmed := strings.TrimSpace(provider.Name); trimmed != "" {
			names = append(names, trimmed)
		}
	}
	return names
}

func modelNames(provider config.ProviderSpec) []string {
	names := []string{}
	for _, model := range provider.Models {
		trimmed := strings.TrimSpace(model)
		if trimmed == "" || containsString(names, trimmed) {
			continue
		}
		names = append(names, trimmed)
	}
	return names
}

func containsString(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

func (s *Server) loadDesignArtifact(jobID string) (*artifactResponse, error) {
	dir := artifacts.WorkerDir(s.config.Root(), s.config.App().ArtifactsDir, jobID, artifacts.WorkerDesign)
	return s.loadFirstArtifact(dir, "result.md", "design.md")
}

func (s *Server) loadImplementationArtifact(jobID string) (*artifactResponse, error) {
	dir := artifacts.WorkerDir(s.config.Root(), s.config.App().ArtifactsDir, jobID, artifacts.WorkerImplementation)
	return s.loadFirstArtifact(dir, "result.md", "summary.md")
}

func (s *Server) loadFixArtifact(jobID string) (*artifactResponse, error) {
	dir := artifacts.WorkerDir(s.config.Root(), s.config.App().ArtifactsDir, jobID, artifacts.WorkerFix)
	return s.loadFirstArtifact(dir, "result.md", "fix-summary.md")
}

func (s *Server) loadArtifact(path string) (*artifactResponse, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return &artifactResponse{
		Path:    path,
		Content: string(raw),
	}, nil
}

func (s *Server) loadReviewArtifact(jobID string) (*artifactResponse, error) {
	dir := artifacts.WorkerDir(s.config.Root(), s.config.App().ArtifactsDir, jobID, artifacts.WorkerReview)
	return s.loadFirstArtifact(dir, "result.md", "review.md")
}

func (s *Server) loadTestReport(jobID string) (*artifactResponse, error) {
	paths := []string{
		filepath.Join(artifacts.WorkerDir(s.config.Root(), s.config.App().ArtifactsDir, jobID, artifacts.WorkerFix), "test-report.json"),
		filepath.Join(artifacts.WorkerDir(s.config.Root(), s.config.App().ArtifactsDir, jobID, artifacts.WorkerImplementation), "test-report.json"),
	}
	for _, path := range paths {
		raw, err := os.ReadFile(path)
		if err == nil {
			return &artifactResponse{
				Path:    path,
				Content: string(raw),
			}, nil
		}
		if !errors.Is(err, os.ErrNotExist) {
			return nil, err
		}
	}
	return nil, os.ErrNotExist
}

func (s *Server) loadPRCreateArtifact(jobID string) (*artifactResponse, error) {
	dir := artifacts.WorkerDir(s.config.Root(), s.config.App().ArtifactsDir, jobID, artifacts.WorkerPR)
	return s.loadFirstArtifact(dir, "result.json", "pr-create.json")
}

func extractIssueBody(payload string) string {
	var eventPayload struct {
		Body string `json:"body"`
	}
	if err := json.Unmarshal([]byte(payload), &eventPayload); err != nil {
		return ""
	}
	return eventPayload.Body
}

func (s *Server) loadLogResponses(phase string, dir string, names []string) []logResponse {
	logs := make([]logResponse, 0, len(names))
	for _, name := range names {
		path := filepath.Join(dir, name)
		raw, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		logs = append(logs, logResponse{
			Name:    name,
			Phase:   phase,
			Path:    path,
			Content: string(raw),
		})
	}
	return logs
}

func (s *Server) loadFirstArtifact(dir string, names ...string) (*artifactResponse, error) {
	for _, name := range names {
		artifact, err := s.loadArtifact(filepath.Join(dir, name))
		if err == nil {
			return artifact, nil
		}
		if !errors.Is(err, os.ErrNotExist) {
			return nil, err
		}
	}
	return nil, os.ErrNotExist
}
