package web

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gorilla/mux"

	"github.com/coco-papiyon/korobokcle/internal/artifacts"
	"github.com/coco-papiyon/korobokcle/internal/config"
	"github.com/coco-papiyon/korobokcle/internal/domain"
	"github.com/coco-papiyon/korobokcle/internal/naming"
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
	DeletedAt    string `json:"deletedAt,omitempty"`
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
	Job                    jobResponse             `json:"job"`
	Events                 []eventResponse         `json:"events"`
	IssueBody              string                  `json:"issueBody,omitempty"`
	ReviewComments         []reviewCommentResponse `json:"reviewComments,omitempty"`
	DesignArtifact         *artifactResponse       `json:"designArtifact,omitempty"`
	ImplementationArtifact *artifactResponse       `json:"implementationArtifact,omitempty"`
	FixArtifact            *artifactResponse       `json:"fixArtifact,omitempty"`
	ReviewArtifact         *artifactResponse       `json:"reviewArtifact,omitempty"`
	TestReport             *artifactResponse       `json:"testReport,omitempty"`
	PRCreateArtifact       *artifactResponse       `json:"prCreateArtifact,omitempty"`
	Logs                   []logResponse           `json:"logs,omitempty"`
}

type reviewSubmitRequest struct {
	Comment string `json:"comment"`
}

type artifactResponse struct {
	Path    string `json:"path"`
	Content string `json:"content"`
}

type reviewCommentResponse struct {
	Author string `json:"author"`
	Body   string `json:"body"`
	Path   string `json:"path,omitempty"`
	Line   int    `json:"line,omitempty"`
	URL    string `json:"url,omitempty"`
}

type logResponse struct {
	Name    string `json:"name"`
	Phase   string `json:"phase"`
	Path    string `json:"path"`
	Content string `json:"content"`
}

type watchRuleResponse struct {
	ID             string                      `json:"id"`
	Name           string                      `json:"name"`
	Repositories   []string                    `json:"repositories"`
	Target         string                      `json:"target"`
	Branch         string                      `json:"branch"`
	ProjectName    string                      `json:"projectName"`
	Labels         []string                    `json:"labels"`
	ProjectFilters []config.ProjectFieldFilter `json:"projectFilters"`
	TitlePattern   string                      `json:"titlePattern"`
	Authors        []string                    `json:"authors"`
	Assignees      []string                    `json:"assignees"`
	Reviewers      []string                    `json:"reviewers"`
	ExcludeDraftPR bool                        `json:"excludeDraftPR"`
	Provider       string                      `json:"provider"`
	Model          string                      `json:"model"`
	SkillSet       string                      `json:"skillSet"`
	TestProfile    string                      `json:"testProfile"`
	Enabled        bool                        `json:"enabled"`
}

type providerSpecResponse struct {
	Name   string   `json:"name"`
	Models []string `json:"models"`
}

type monitoredRepositoryResponse struct {
	Repository string `json:"repository"`
	Workers    int    `json:"workers"`
}

type appConfigResponse struct {
	Provider              string                        `json:"provider"`
	Model                 string                        `json:"model"`
	CopilotAllowTools     []string                      `json:"copilotAllowTools"`
	PollInterval          int                           `json:"pollInterval"`
	ScreenRefreshInterval int                           `json:"screenRefreshInterval"`
	PRTitleTemplate       string                        `json:"prTitleTemplate"`
	BranchTemplate        string                        `json:"branchTemplate"`
	MonitoredRepositories []monitoredRepositoryResponse `json:"monitoredRepositories"`
	Providers             []providerSpecResponse        `json:"providers"`
}

type saveAppConfigRequest struct {
	Provider              *string                        `json:"provider"`
	Model                 *string                        `json:"model"`
	CopilotAllowTools     []string                       `json:"copilotAllowTools"`
	PollInterval          *int                           `json:"pollInterval"`
	ScreenRefreshInterval *int                           `json:"screenRefreshInterval"`
	PRTitleTemplate       string                         `json:"prTitleTemplate"`
	BranchTemplate        string                         `json:"branchTemplate"`
	MonitoredRepositories *[]monitoredRepositoryResponse `json:"monitoredRepositories"`
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
	jobs, err := s.orchestrator.ListJobsByFilter(r.Context(), parseJobListFilter(r.URL.Query().Get("deleted")))
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

func (s *Server) handleDeleteJob(w http.ResponseWriter, r *http.Request) {
	jobID := mux.Vars(r)["id"]
	if err := s.orchestrator.DeleteJob(r.Context(), jobID); err != nil {
		writeJSONError(w, http.StatusInternalServerError, err)
		return
	}
	s.handleJobDetail(w, r)
}

func (s *Server) handleRestoreJob(w http.ResponseWriter, r *http.Request) {
	jobID := mux.Vars(r)["id"]
	if err := s.orchestrator.RestoreJob(r.Context(), jobID); err != nil {
		writeJSONError(w, http.StatusInternalServerError, err)
		return
	}
	s.handleJobDetail(w, r)
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
		if len(out.ReviewComments) == 0 && event.EventType == string(domain.DomainEventPRReviewMatched) {
			out.ReviewComments = extractReviewComments(event.Payload)
		}
		out.Events = append(out.Events, eventResponse{
			ID:               event.ID,
			JobID:            event.JobID,
			EventType:        event.EventType,
			StateFrom:        event.StateFrom,
			StateTo:          event.StateTo,
			Payload:          sanitizeEventPayloadForResponse(event.Payload),
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
	if artifact, err := s.loadTestReport(job.ID, events); err == nil {
		out.TestReport = artifact
	}
	if artifact, err := s.loadPRCreateArtifact(job.ID); err == nil {
		out.PRCreateArtifact = artifact
	}
	out.Logs = append(out.Logs, s.loadLogResponses("design", artifacts.WorkerDir(s.config.Root(), s.config.App().ArtifactsDir, job.ID, artifacts.WorkerDesign), []string{"stdout.log", "stderr.log"})...)
	out.Logs = append(out.Logs, s.loadLogResponses("implementation", artifacts.WorkerDir(s.config.Root(), s.config.App().ArtifactsDir, job.ID, artifacts.WorkerImplementation), []string{"stdout.log", "stderr.log"})...)
	out.Logs = append(out.Logs, s.loadLogResponses("fix", artifacts.WorkerDir(s.config.Root(), s.config.App().ArtifactsDir, job.ID, artifacts.WorkerFix), []string{"stdout.log", "stderr.log"})...)
	out.Logs = append(out.Logs, s.loadLogResponses("pr", artifacts.WorkerDir(s.config.Root(), s.config.App().ArtifactsDir, job.ID, artifacts.WorkerPR), []string{"git-push.log", "gh-pr-create.log"})...)
	out.Logs = append(out.Logs, s.loadLogResponses("review", artifacts.WorkerDir(s.config.Root(), s.config.App().ArtifactsDir, job.ID, artifacts.WorkerReview), []string{"stdout.log", "stderr.log", "gh-pr-review.log"})...)
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

func (s *Server) handleSubmitReviewComment(w http.ResponseWriter, r *http.Request) {
	jobID := mux.Vars(r)["id"]
	job, _, err := s.orchestrator.JobDetail(r.Context(), jobID)
	if err != nil {
		writeJSONError(w, http.StatusNotFound, err)
		return
	}
	if job.Type != domain.JobTypePRReview {
		writeJSONError(w, http.StatusBadRequest, fmt.Errorf("job %q is not a pr review job", jobID))
		return
	}

	artifact, err := s.loadReviewArtifact(jobID)
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, fmt.Errorf("load review artifact: %w", err))
		return
	}

	var payload reviewSubmitRequest
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil && !errors.Is(err, io.EOF) {
		writeJSONError(w, http.StatusBadRequest, fmt.Errorf("decode review submit request: %w", err))
		return
	}

	comment := strings.TrimSpace(payload.Comment)
	if comment == "" {
		comment = strings.TrimSpace(artifact.Content)
	}
	if comment == "" {
		writeJSONError(w, http.StatusBadRequest, fmt.Errorf("review comment is empty"))
		return
	}

	if err := s.reviewer.Submit(r.Context(), ReviewSubmitRequest{
		Repository:  job.Repository,
		PullNumber:  job.GitHubNumber,
		Body:        comment,
		ArtifactDir: filepath.Dir(artifact.Path),
	}); err != nil {
		writeJSONError(w, http.StatusInternalServerError, err)
		return
	}

	s.handleJobDetail(w, r)
}

func (s *Server) handleWatchRules(w http.ResponseWriter, r *http.Request) {
	watchRules := s.config.WatchRules()
	rules := make([]watchRuleResponse, 0, len(watchRules.Rules))
	for _, rule := range watchRules.Rules {
		target := rule.Target
		if target == "pull_request_review_comment" {
			target = string(domain.TargetPullRequestReview)
		}
		rules = append(rules, watchRuleResponse{
			ID:             rule.ID,
			Name:           rule.Name,
			Repositories:   sliceOrEmpty(rule.Repositories),
			Target:         target,
			Branch:         rule.Branch,
			ProjectName:    rule.ProjectName,
			Labels:         sliceOrEmpty(rule.Labels),
			ProjectFilters: append([]config.ProjectFieldFilter(nil), rule.ProjectFilters...),
			TitlePattern:   rule.TitlePattern,
			Authors:        sliceOrEmpty(rule.Authors),
			Assignees:      sliceOrEmpty(rule.Assignees),
			Reviewers:      sliceOrEmpty(rule.Reviewers),
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
	var payload saveAppConfigRequest
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		writeJSONError(w, http.StatusBadRequest, fmt.Errorf("decode app config: %w", err))
		return
	}

	appConfig := s.config.App()
	provider := appConfig.Provider
	providerChanged := false
	if payload.Provider != nil {
		provider = strings.ToLower(strings.TrimSpace(*payload.Provider))
		if provider == "" {
			provider = appConfig.Provider
		}
		if _, err := s.providerSpecByName(provider); err != nil {
			writeJSONError(w, http.StatusBadRequest, err)
			return
		}
		providerChanged = provider != appConfig.Provider
		appConfig.Provider = provider
	}

	modelInput := appConfig.Model
	modelChanged := payload.Model != nil
	if payload.Model != nil {
		modelInput = normalizeDefaultModelValue(*payload.Model)
	}
	if modelInput != "" {
		model, err := s.validateModelForProvider(provider, modelInput)
		if err != nil {
			if providerChanged && !modelChanged {
				modelInput = ""
			} else {
				writeJSONError(w, http.StatusBadRequest, err)
				return
			}
		} else {
			modelInput = model
		}
	} else if modelChanged {
		modelInput = ""
	}
	appConfig.Model = modelInput
	appConfig.CopilotAllowTools = normalizeStringSlice(payload.CopilotAllowTools)
	if payload.MonitoredRepositories != nil {
		repos, err := normalizeMonitoredRepositoryResponses(*payload.MonitoredRepositories)
		if err != nil {
			writeJSONError(w, http.StatusBadRequest, fmt.Errorf("monitoredRepositories: %w", err))
			return
		}
		appConfig.MonitoredRepositories = repos
	}
	prTitleTemplate := strings.TrimSpace(payload.PRTitleTemplate)
	if prTitleTemplate == "" {
		prTitleTemplate = naming.DefaultPRTitleTemplate
	}
	appConfig.PRTitleTemplate = prTitleTemplate

	branchTemplate := strings.TrimSpace(payload.BranchTemplate)
	if branchTemplate == "" {
		branchTemplate = naming.DefaultBranchTemplate
	}
	appConfig.BranchTemplate = branchTemplate
	if payload.PollInterval != nil {
		if *payload.PollInterval < 1 {
			writeJSONError(w, http.StatusBadRequest, fmt.Errorf("pollInterval must be a positive whole number of seconds"))
			return
		}
		if *payload.PollInterval > 24*60*60 {
			writeJSONError(w, http.StatusBadRequest, fmt.Errorf("pollInterval must be no more than 86400 seconds"))
			return
		}
		appConfig.PollInterval = time.Duration(*payload.PollInterval) * time.Second
	}
	if payload.ScreenRefreshInterval != nil {
		if !allowedScreenRefreshInterval(*payload.ScreenRefreshInterval) {
			writeJSONError(w, http.StatusBadRequest, fmt.Errorf("screenRefreshInterval must be one of 0, 5, 10, 20, or 30 seconds"))
			return
		}
		appConfig.ScreenRefreshInterval = time.Duration(*payload.ScreenRefreshInterval) * time.Second
	}
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
			Events:  normalizeNotificationEvents(compactStrings(channel.Events)),
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
		case "pr_created", "pr_updated":
			actions = append(actions, actionRetryPR)
		}
	}

	switch event.EventType {
	case "design_failed", "design_rejected":
		actions = append(actions, actionRetryDesign)
	case "design_interrupted":
		actions = append(actions, actionRetryDesign)
	case "implementation_failed", "test_failed", "final_rejected", "implementation_interrupted", "test_interrupted":
		actions = append(actions, actionRetryImplementation)
	case "pr_push_failed", "pr_create_failed", "pr_interrupted":
		actions = append(actions, actionRetryPR)
	case "review_failed", "review_interrupted":
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

	allowedRepositories := make(map[string]struct{})
	for _, repository := range s.config.App().MonitoredRepositories {
		trimmed := strings.TrimSpace(repository.Repository)
		if trimmed == "" {
			continue
		}
		allowedRepositories[trimmed] = struct{}{}
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
		if rule.Target != string(domain.TargetIssue) &&
			rule.Target != string(domain.TargetIssueProject) &&
			rule.Target != string(domain.TargetPullRequest) &&
			rule.Target != string(domain.TargetPullRequestReview) &&
			rule.Target != "pull_request_review_comment" {
			writeJSONError(w, http.StatusBadRequest, fmt.Errorf("rule[%d].target must be issue, issue_project, pull_request, or pull_request_review", index))
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
		repositories := compactStrings(rule.Repositories)
		if len(repositories) == 0 {
			writeJSONError(w, http.StatusBadRequest, fmt.Errorf("rule[%d].repositories must include at least one monitored repository", index))
			return
		}
		for _, repository := range repositories {
			if _, ok := allowedRepositories[repository]; !ok {
				writeJSONError(w, http.StatusBadRequest, fmt.Errorf("rule[%d].repositories includes unregistered repository %q", index, repository))
				return
			}
		}
		target := rule.Target
		if target == "pull_request_review_comment" {
			target = string(domain.TargetPullRequestReview)
		}
		file.Rules = append(file.Rules, config.WatchRule{
			ID:             strings.TrimSpace(rule.ID),
			Name:           strings.TrimSpace(rule.Name),
			Repositories:   repositories,
			Target:         target,
			Branch:         strings.TrimSpace(rule.Branch),
			ProjectName:    strings.TrimSpace(rule.ProjectName),
			Labels:         compactStrings(rule.Labels),
			ProjectFilters: compactProjectFilters(rule.ProjectFilters),
			TitlePattern:   strings.TrimSpace(rule.TitlePattern),
			Authors:        compactStrings(rule.Authors),
			Assignees:      compactStrings(rule.Assignees),
			Reviewers:      compactStrings(rule.Reviewers),
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
	deletedAt := ""
	if job.DeletedAt != nil {
		deletedAt = job.DeletedAt.Format(timeFormat)
	}
	return jobResponse{
		ID:           job.ID,
		Type:         string(job.Type),
		Repository:   job.Repository,
		GitHubNumber: job.GitHubNumber,
		State:        string(job.State),
		Title:        job.Title,
		BranchName:   job.BranchName,
		WatchRuleID:  job.WatchRuleID,
		DeletedAt:    deletedAt,
		CreatedAt:    job.CreatedAt.Format(timeFormat),
		UpdatedAt:    job.UpdatedAt.Format(timeFormat),
	}
}

func parseJobListFilter(raw string) orchestrator.JobListFilter {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "only", "deleted":
		return orchestrator.JobListDeletedOnly
	case "all", "include":
		return orchestrator.JobListAll
	default:
		return orchestrator.JobListActiveOnly
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

func compactProjectFilters(values []config.ProjectFieldFilter) []config.ProjectFieldFilter {
	out := make([]config.ProjectFieldFilter, 0, len(values))
	for _, value := range values {
		field := strings.TrimSpace(value.Field)
		if field == "" {
			continue
		}
		out = append(out, config.ProjectFieldFilter{
			Field:  field,
			Values: compactStrings(value.Values),
		})
	}
	return out
}

func sliceOrEmpty(values []string) []string {
	if values == nil {
		return []string{}
	}
	return values
}

func normalizeStringSlice(values []string) []string {
	normalized := make([]string, 0, len(values))
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			continue
		}
		if _, ok := seen[trimmed]; ok {
			continue
		}
		seen[trimmed] = struct{}{}
		normalized = append(normalized, trimmed)
	}
	return normalized
}

func normalizeDefaultModelValue(value string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" || strings.EqualFold(trimmed, "default") {
		return ""
	}
	return trimmed
}

func toAppConfigResponse(app config.App) appConfigResponse {
	prTitleTemplate := strings.TrimSpace(app.PRTitleTemplate)
	if prTitleTemplate == "" {
		prTitleTemplate = naming.DefaultPRTitleTemplate
	}
	branchTemplate := strings.TrimSpace(app.BranchTemplate)
	if branchTemplate == "" {
		branchTemplate = naming.DefaultBranchTemplate
	}
	return appConfigResponse{
		Provider:              app.Provider,
		Model:                 normalizeDefaultModelValue(app.Model),
		CopilotAllowTools:     sliceOrEmpty(app.CopilotAllowTools),
		PollInterval:          int(effectivePollInterval(app.PollInterval) / time.Second),
		ScreenRefreshInterval: durationSeconds(app.ScreenRefreshInterval),
		PRTitleTemplate:       prTitleTemplate,
		BranchTemplate:        branchTemplate,
		MonitoredRepositories: toMonitoredRepositoryResponses(app.MonitoredRepositories),
		Providers:             toProviderSpecResponses(config.ProviderCatalog()),
	}
}

func toMonitoredRepositoryResponses(values []config.MonitoredRepository) []monitoredRepositoryResponse {
	out := make([]monitoredRepositoryResponse, 0, len(values))
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		repository := strings.TrimSpace(value.Repository)
		if repository == "" {
			continue
		}
		if _, ok := seen[repository]; ok {
			continue
		}
		seen[repository] = struct{}{}
		workers := value.Workers
		if workers < 1 {
			workers = 1
		}
		out = append(out, monitoredRepositoryResponse{
			Repository: repository,
			Workers:    workers,
		})
	}
	return out
}

func normalizeMonitoredRepositoryResponses(values []monitoredRepositoryResponse) ([]config.MonitoredRepository, error) {
	out := make([]config.MonitoredRepository, 0, len(values))
	seen := make(map[string]struct{}, len(values))
	for index, value := range values {
		repository := strings.TrimSpace(value.Repository)
		if repository == "" {
			return nil, fmt.Errorf("item[%d].repository is required", index)
		}
		if _, ok := seen[repository]; ok {
			continue
		}
		workers := value.Workers
		if workers < 1 {
			return nil, fmt.Errorf("item[%d].workers must be at least 1", index)
		}
		seen[repository] = struct{}{}
		out = append(out, config.MonitoredRepository{
			Repository: repository,
			Workers:    workers,
		})
	}
	return out, nil
}

func toNotificationConfigResponse(notifications config.Notifications) notificationConfigResponse {
	channels := make([]notificationChannelResponse, 0, len(notifications.Channels))
	for _, channel := range notifications.Channels {
		channels = append(channels, notificationChannelResponse{
			Name:    channel.Name,
			Type:    channel.Type,
			Events:  sliceOrEmpty(normalizeNotificationEvents(channel.Events)),
			Enabled: channel.Enabled,
		})
	}
	return notificationConfigResponse{Channels: channels}
}

func normalizeNotificationEvents(events []string) []string {
	normalized := make([]string, 0, len(events))
	seen := make(map[string]struct{}, len(events))
	for _, candidate := range events {
		switch name := strings.ToLower(strings.TrimSpace(candidate)); name {
		case "waiting_design_approval", "waiting_final_approval", "review_completed", "pr_created", "failed":
			if _, ok := seen[name]; ok {
				continue
			}
			seen[name] = struct{}{}
			normalized = append(normalized, name)
		}
	}
	return normalized
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

func durationSeconds(value time.Duration) int {
	if value <= 0 {
		return 0
	}
	return int(value / time.Second)
}

func allowedScreenRefreshInterval(seconds int) bool {
	switch seconds {
	case 0, 5, 10, 20, 30:
		return true
	default:
		return false
	}
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
	return config.ProviderSpec{}, fmt.Errorf("provider must be one of %s", strings.Join(config.ProviderNames(), ", "))
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
	spec, err := s.providerSpecByName(provider)
	if err != nil {
		return "", err
	}
	return config.ValidateModelForProvider(spec, model)
}

func (s *Server) validateRuleModel(provider string, model string) (string, error) {
	effectiveProvider := strings.TrimSpace(provider)
	if effectiveProvider == "" {
		effectiveProvider = s.config.App().Provider
	}
	return s.validateModelForProvider(effectiveProvider, model)
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
	return s.loadFirstArtifact(dir, "result.md", "review_fix.md", "implement.md", "summary.md")
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

func (s *Server) loadTestReport(jobID string, events []domain.Event) (*artifactResponse, error) {
	paths := []string{
		filepath.Join(resolveTestReportArtifactDir(s.config, jobID, events), "test-report.json"),
	}
	fallbackPath := filepath.Join(artifacts.WorkerDir(s.config.Root(), s.config.App().ArtifactsDir, jobID, artifacts.WorkerImplementation), "test-report.json")
	if fallbackPath != paths[0] {
		paths = append(paths, fallbackPath)
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

func resolveTestReportArtifactDir(cfg *config.Service, jobID string, events []domain.Event) string {
	sourceEventType, err := latestImplementationRerunSourceEventType(events)
	if err == nil && sourceEventType == "test_failed" {
		return artifacts.WorkerDir(cfg.Root(), cfg.App().ArtifactsDir, jobID, artifacts.WorkerFix)
	}
	return artifacts.WorkerDir(cfg.Root(), cfg.App().ArtifactsDir, jobID, artifacts.WorkerImplementation)
}

func latestImplementationRerunSourceEventType(events []domain.Event) (string, error) {
	for i := len(events) - 1; i >= 0; i-- {
		event := events[i]
		if event.EventType != "implementation_rerun_requested" {
			continue
		}

		var payload struct {
			EventID *int64 `json:"eventId"`
		}
		if err := json.Unmarshal([]byte(event.Payload), &payload); err != nil {
			return "", err
		}
		if payload.EventID == nil {
			return "", nil
		}
		for j := i - 1; j >= 0; j-- {
			if events[j].ID == *payload.EventID {
				return events[j].EventType, nil
			}
		}
		return "", nil
	}
	return "", nil
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

func extractReviewComments(payload string) []reviewCommentResponse {
	var eventPayload struct {
		ReviewComments []domain.ReviewComment `json:"reviewComments"`
	}
	if err := json.Unmarshal([]byte(payload), &eventPayload); err != nil {
		return nil
	}
	comments := make([]reviewCommentResponse, 0, len(eventPayload.ReviewComments))
	for _, comment := range eventPayload.ReviewComments {
		comments = append(comments, reviewCommentResponse{
			Author: comment.Author,
			Body:   comment.Body,
			Path:   comment.Path,
			Line:   comment.Line,
			URL:    comment.URL,
		})
	}
	return comments
}

func sanitizeEventPayloadForResponse(payload string) string {
	trimmed := strings.TrimSpace(payload)
	if trimmed == "" {
		return payload
	}
	var parsed any
	if err := json.Unmarshal([]byte(payload), &parsed); err != nil {
		return payload
	}
	sanitized := stripBodyFields(parsed)
	raw, err := json.Marshal(sanitized)
	if err != nil {
		return payload
	}
	return string(raw)
}

func stripBodyFields(value any) any {
	switch v := value.(type) {
	case map[string]any:
		out := make(map[string]any, len(v))
		for key, item := range v {
			if key == "body" {
				continue
			}
			out[key] = stripBodyFields(item)
		}
		return out
	case []any:
		out := make([]any, 0, len(v))
		for _, item := range v {
			out = append(out, stripBodyFields(item))
		}
		return out
	default:
		return value
	}
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
