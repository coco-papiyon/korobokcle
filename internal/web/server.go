package web

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/coco-papiyon/korobokcle/internal/config"
	"github.com/coco-papiyon/korobokcle/internal/domain"
)

type JobStore interface {
	List(context.Context) ([]domain.Job, error)
	UpdatedAt(context.Context) (time.Time, error)
	Get(context.Context, string) (domain.Job, bool, error)
	Upsert(context.Context, domain.Job) error
	Delete(context.Context, string) error
}

type SettingsStore interface {
	Load(context.Context) (domain.WatchSettings, error)
	Save(context.Context, domain.WatchSettings) error
}

type JobBranchResolver interface {
	ResolveJobBranch(context.Context, domain.Job) (string, error)
}

type JobContextLoader interface {
	Load(context.Context, domain.Job) (string, error)
}

type ArtifactActions interface {
	GetArtifact(context.Context, string) (DesignArtifact, error)
	GetSourceDiff(context.Context, string) (JobSourceDiff, error)
	UpdateArtifact(context.Context, string, string) (DesignArtifact, error)
	ApproveArtifact(context.Context, string, string) (domain.Job, error)
	RequestChanges(context.Context, string, string) (domain.Job, error)
	RerunArtifact(context.Context, string, string) (domain.Job, error)
}

type SkillActions interface {
	SkillStatus(context.Context) ([]domain.SkillStatus, error)
	GenerateSkills(context.Context, domain.SkillGenerationRequest) (domain.SkillGenerationResult, error)
}

type DesignArtifact struct {
	Content string `json:"content"`
	Path    string `json:"path"`
}

type JobSourceDiff struct {
	Content string `json:"content"`
	Path    string `json:"path"`
	BaseRef string `json:"baseRef,omitempty"`
}

type JobLogFile struct {
	Kind    string `json:"kind"`
	Label   string `json:"label"`
	Path    string `json:"path"`
	Content string `json:"content"`
}

type JobLogGroup struct {
	Role      string       `json:"role"`
	RoleLabel string       `json:"roleLabel"`
	Attempt   int          `json:"attempt"`
	Files     []JobLogFile `json:"files"`
}

type JobDetailResponse struct {
	UpdatedAt    string        `json:"updatedAt"`
	Job          domain.Job    `json:"job"`
	Branch       string        `json:"branch"`
	IssueContext string        `json:"issueContext,omitempty"`
	Logs         []JobLogGroup `json:"logs,omitempty"`
}

type Server struct {
	httpServer      *http.Server
	artifactActions ArtifactActions
	skillActions    SkillActions
	detailLoader    JobContextLoader
}

func NewServer(cfg config.Config, store JobStore, settingsStore SettingsStore, artifactActions ArtifactActions, branchResolver JobBranchResolver, detailLoader JobContextLoader, optionalSkillActions ...SkillActions) *Server {
	mux := http.NewServeMux()
	var skillActions SkillActions
	if len(optionalSkillActions) > 0 {
		skillActions = optionalSkillActions[0]
	}

	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	})

	mux.HandleFunc("/api/jobs", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		if store == nil {
			http.Error(w, "job store not configured", http.StatusServiceUnavailable)
			return
		}

		switch r.Method {
		case http.MethodGet:
			jobs, err := store.List(r.Context())
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			updatedAt, err := store.UpdatedAt(r.Context())
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			_ = json.NewEncoder(w).Encode(map[string]any{
				"updatedAt": updatedAt.UTC().Format(time.RFC3339Nano),
				"jobs":      jobs,
			})
		case http.MethodPost:
			var req struct {
				ID         string          `json:"id"`
				Kind       domain.JobKind  `json:"kind"`
				State      domain.JobState `json:"state"`
				Repository string          `json:"repository"`
				Number     int             `json:"number"`
				Title      string          `json:"title"`
			}
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			if req.Kind == "" {
				http.Error(w, "kind is required", http.StatusBadRequest)
				return
			}
			if req.State == "" {
				req.State = domain.InitialStateForKind(req.Kind)
			}
			if req.ID == "" {
				req.ID = buildJobID(req.Kind, req.Repository, req.Number, req.Title)
			}
			job := domain.Job{
				ID:         req.ID,
				Kind:       req.Kind,
				State:      req.State,
				Repository: req.Repository,
				Number:     req.Number,
				Title:      req.Title,
			}
			if branchResolver != nil {
				if branch, err := branchResolver.ResolveJobBranch(r.Context(), job); err == nil {
					job.Branch = strings.TrimSpace(branch)
				}
			}
			if err := store.Upsert(r.Context(), job); err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			w.WriteHeader(http.StatusCreated)
			_ = json.NewEncoder(w).Encode(job)
		default:
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		}
	})

	mux.HandleFunc("/api/jobs/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		if store == nil {
			http.Error(w, "job store not configured", http.StatusServiceUnavailable)
			return
		}

		id := strings.TrimPrefix(r.URL.Path, "/api/jobs/")
		if strings.HasSuffix(id, "/artifact/request-changes") {
			id = strings.TrimSuffix(id, "/artifact/request-changes")
			if id == "" {
				http.NotFound(w, r)
				return
			}
			if artifactActions == nil {
				http.Error(w, "artifact actions not configured", http.StatusServiceUnavailable)
				return
			}
			switch r.Method {
			case http.MethodPost:
				var req struct {
					Comment string `json:"comment"`
				}
				if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
					http.Error(w, err.Error(), http.StatusBadRequest)
					return
				}
				job, err := artifactActions.RequestChanges(r.Context(), id, req.Comment)
				if err != nil {
					http.Error(w, err.Error(), http.StatusBadRequest)
					return
				}
				_ = json.NewEncoder(w).Encode(job)
			default:
				http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			}
			return
		}
		if strings.HasSuffix(id, "/artifact/content") {
			id = strings.TrimSuffix(id, "/artifact/content")
			if id == "" {
				http.NotFound(w, r)
				return
			}
			if artifactActions == nil {
				http.Error(w, "artifact actions not configured", http.StatusServiceUnavailable)
				return
			}
			switch r.Method {
			case http.MethodPut:
				var req struct {
					Content string `json:"content"`
				}
				if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
					http.Error(w, err.Error(), http.StatusBadRequest)
					return
				}
				artifact, err := artifactActions.UpdateArtifact(r.Context(), id, req.Content)
				if err != nil {
					http.Error(w, err.Error(), http.StatusBadRequest)
					return
				}
				_ = json.NewEncoder(w).Encode(artifact)
			default:
				http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			}
			return
		}
		if strings.HasSuffix(id, "/diff") {
			id = strings.TrimSuffix(id, "/diff")
			if id == "" {
				http.NotFound(w, r)
				return
			}
			if artifactActions == nil {
				http.Error(w, "artifact actions not configured", http.StatusServiceUnavailable)
				return
			}
			switch r.Method {
			case http.MethodGet:
				diff, err := artifactActions.GetSourceDiff(r.Context(), id)
				if err != nil {
					http.Error(w, err.Error(), http.StatusNotFound)
					return
				}
				_ = json.NewEncoder(w).Encode(diff)
			default:
				http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			}
			return
		}
		if strings.HasSuffix(id, "/artifact") {
			id = strings.TrimSuffix(id, "/artifact")
			if id == "" {
				http.NotFound(w, r)
				return
			}
			if artifactActions == nil {
				http.Error(w, "artifact actions not configured", http.StatusServiceUnavailable)
				return
			}
			switch r.Method {
			case http.MethodGet:
				artifact, err := artifactActions.GetArtifact(r.Context(), id)
				if err != nil {
					http.Error(w, err.Error(), http.StatusNotFound)
					return
				}
				_ = json.NewEncoder(w).Encode(artifact)
			case http.MethodPost:
				var req struct {
					Comment string `json:"comment"`
				}
				if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
					http.Error(w, err.Error(), http.StatusBadRequest)
					return
				}
				job, err := artifactActions.ApproveArtifact(r.Context(), id, req.Comment)
				if err != nil {
					http.Error(w, err.Error(), http.StatusBadRequest)
					return
				}
				_ = json.NewEncoder(w).Encode(job)
			case http.MethodPatch:
				var req struct {
					Comment string `json:"comment"`
				}
				if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
					http.Error(w, err.Error(), http.StatusBadRequest)
					return
				}
				job, err := artifactActions.RerunArtifact(r.Context(), id, req.Comment)
				if err != nil {
					http.Error(w, err.Error(), http.StatusBadRequest)
					return
				}
				_ = json.NewEncoder(w).Encode(job)
			default:
				http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			}
			return
		}
		if strings.HasSuffix(id, "/state") {
			id = strings.TrimSuffix(id, "/state")
			if id == "" {
				http.NotFound(w, r)
				return
			}
			switch r.Method {
			case http.MethodPatch:
				var req struct {
					State domain.JobState `json:"state"`
				}
				if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
					http.Error(w, err.Error(), http.StatusBadRequest)
					return
				}
				if req.State == "" {
					http.Error(w, "state is required", http.StatusBadRequest)
					return
				}
				job, ok, err := store.Get(r.Context(), id)
				if err != nil {
					http.Error(w, err.Error(), http.StatusInternalServerError)
					return
				}
				if !ok {
					http.NotFound(w, r)
					return
				}
				if !job.State.CanTransitionTo(req.State) && job.State != req.State {
					http.Error(w, "invalid state transition", http.StatusBadRequest)
					return
				}
				if job.State != req.State {
					job.State = req.State
					job.UpdatedAt = time.Now().UTC()
				}
				if err := store.Upsert(r.Context(), job); err != nil {
					http.Error(w, err.Error(), http.StatusInternalServerError)
					return
				}
				_ = json.NewEncoder(w).Encode(job)
			default:
				http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			}
			return
		}
		if r.Method == http.MethodDelete {
			if err := store.Delete(r.Context(), id); err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			w.WriteHeader(http.StatusNoContent)
			return
		}
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if id == "" {
			http.NotFound(w, r)
			return
		}
		job, ok, err := store.Get(r.Context(), id)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		if !ok {
			http.NotFound(w, r)
			return
		}
		updatedAt, err := store.UpdatedAt(r.Context())
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		responseUpdatedAt := updatedAt
		branch := strings.TrimSpace(job.Branch)
		if branch == "" && branchResolver != nil {
			if resolved, err := branchResolver.ResolveJobBranch(r.Context(), job); err == nil {
				branch = strings.TrimSpace(resolved)
			}
		}
		issueContext := strings.TrimSpace(job.IssueContext)
		if issueContext == "" && detailLoader != nil && isIssueJob(job.Kind) {
			if loaded, err := detailLoader.Load(r.Context(), job); err == nil {
				issueContext = strings.TrimSpace(loaded)
				if issueContext != "" {
					job.IssueContext = issueContext
					job.UpdatedAt = time.Now().UTC()
					responseUpdatedAt = job.UpdatedAt
					if err := store.Upsert(r.Context(), job); err != nil {
						http.Error(w, err.Error(), http.StatusInternalServerError)
						return
					}
				}
			}
		}
		_ = json.NewEncoder(w).Encode(JobDetailResponse{
			UpdatedAt:    responseUpdatedAt.UTC().Format(time.RFC3339Nano),
			Job:          job,
			Branch:       branch,
			IssueContext: issueContext,
			Logs:         loadJobLogs(cfg.WorkDir, job),
		})
	})

	mux.HandleFunc("/api/settings", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		if settingsStore == nil {
			http.Error(w, "settings store not configured", http.StatusServiceUnavailable)
			return
		}

		switch r.Method {
		case http.MethodGet:
			settings, err := settingsStore.Load(r.Context())
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			settings = domain.NormalizeWatchSettings(settings)
			_ = json.NewEncoder(w).Encode(settings)
		case http.MethodPut:
			var settings domain.WatchSettings
			if err := json.NewDecoder(r.Body).Decode(&settings); err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			settings = domain.NormalizeWatchSettings(settings)
			if err := settingsStore.Save(r.Context(), settings); err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			_ = json.NewEncoder(w).Encode(settings)
		default:
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		}
	})

	mux.HandleFunc("/api/skills", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		if skillActions == nil {
			http.Error(w, "skill generator not configured", http.StatusServiceUnavailable)
			return
		}
		switch r.Method {
		case http.MethodGet:
			statuses, err := skillActions.SkillStatus(r.Context())
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"skills": statuses})
		case http.MethodPost:
			var req domain.SkillGenerationRequest
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			result, err := skillActions.GenerateSkills(r.Context(), req)
			if err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			_ = json.NewEncoder(w).Encode(result)
		default:
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		}
	})

	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/api/") {
			http.NotFound(w, r)
			return
		}

		distDir := filepath.Join(cfg.ToolDir, "static")
		indexPath := filepath.Join(distDir, "index.html")
		if _, err := os.Stat(indexPath); err != nil {
			http.Error(w, "frontend not built", http.StatusServiceUnavailable)
			return
		}

		requestPath := strings.TrimPrefix(filepath.Clean(r.URL.Path), string(filepath.Separator))
		if requestPath != "." && requestPath != "" {
			filePath := filepath.Join(distDir, requestPath)
			if info, err := os.Stat(filePath); err == nil && !info.IsDir() {
				http.ServeFile(w, r, filePath)
				return
			}
		}

		if ext := filepath.Ext(r.URL.Path); ext != "" && r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}

		http.ServeFile(w, r, indexPath)
	})

	return &Server{
		httpServer: &http.Server{
			Addr:    cfg.Addr,
			Handler: mux,
		},
		artifactActions: artifactActions,
		skillActions:    skillActions,
		detailLoader:    detailLoader,
	}
}

func isIssueJob(kind domain.JobKind) bool {
	switch kind {
	case domain.JobKindIssueDesign, domain.JobKindIssueImplementation:
		return true
	default:
		return false
	}
}

func buildJobID(kind domain.JobKind, repository string, number int, title string) string {
	repo := sanitizePart(repository)
	if repo == "" {
		repo = "repo"
	}
	titlePart := sanitizePart(title)
	if titlePart == "" {
		return fmt.Sprintf("%s-%s-%d", kind, repo, number)
	}
	return fmt.Sprintf("%s-%s-%d-%s", kind, repo, number, titlePart)
}

func sanitizePart(value string) string {
	value = strings.TrimSpace(strings.ToLower(value))
	replacer := strings.NewReplacer(" ", "-", "/", "-", "\\", "-", ":", "-", "#", "-", ".", "-", ",", "-", "(", "-", ")", "-")
	value = replacer.Replace(value)
	value = strings.Trim(value, "-")
	return value
}

func jobWorkspaceDir(workDir string, job domain.Job) string {
	repoDir := sanitizePart(strings.ReplaceAll(job.Repository, "/", "_"))
	return filepath.Join(workDir, "workspace", repoDir, job.ID)
}

func jobWorkspaceLogDir(workDir string, job domain.Job) string {
	return filepath.Join(jobWorkspaceDir(workDir, job), "logs")
}

func loadJobLogs(workDir string, job domain.Job) []JobLogGroup {
	logDir := jobWorkspaceLogDir(workDir, job)
	entries, err := os.ReadDir(logDir)
	if err != nil {
		return nil
	}

	groups := map[string]*JobLogGroup{}
	keys := make([]string, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".log") {
			continue
		}
		displayPath := filepath.Join("workspace", sanitizePart(strings.ReplaceAll(job.Repository, "/", "_")), job.ID, "logs", entry.Name())
		parsed := parseJobLogFile(job, displayPath, filepath.Join(logDir, entry.Name()))
		if parsed == nil {
			continue
		}
		key := fmt.Sprintf("%04d|%s", parsed.Attempt, parsed.Role)
		group, ok := groups[key]
		if !ok {
			group = &JobLogGroup{
				Role:      parsed.Role,
				RoleLabel: jobLogRoleLabel(job, parsed.Role),
				Attempt:   parsed.Attempt,
			}
			groups[key] = group
			keys = append(keys, key)
		}
		group.Files = append(group.Files, parsed.File)
	}

	sort.Strings(keys)

	result := make([]JobLogGroup, 0, len(keys))
	for _, key := range keys {
		group := groups[key]
		sort.Slice(group.Files, func(i, j int) bool {
			return jobLogFileSortOrder(group.Files[i].Kind) < jobLogFileSortOrder(group.Files[j].Kind)
		})
		result = append(result, *group)
	}
	return result
}

type parsedJobLogFile struct {
	Role    string
	Attempt int
	File    JobLogFile
}

func parseJobLogFile(job domain.Job, displayPath string, path string) *parsedJobLogFile {
	base := filepath.Base(displayPath)
	name := strings.TrimSuffix(base, ".log")
	kind := "activity"
	label := "処理ログ"

	switch {
	case strings.HasSuffix(name, "_stdout"):
		name = strings.TrimSuffix(name, "_stdout")
		kind = "stdout"
		label = "標準出力"
	case strings.HasSuffix(name, "_stderr"):
		name = strings.TrimSuffix(name, "_stderr")
		kind = "stderr"
		label = "標準エラー"
	}

	attempt := 1
	role := "agent"
	if idx := strings.LastIndex(name, "_attempt-"); idx >= 0 {
		tail := name[idx+len("_attempt-"):]
		name = name[:idx]
		digitEnd := 0
		for digitEnd < len(tail) && tail[digitEnd] >= '0' && tail[digitEnd] <= '9' {
			digitEnd++
		}
		if digitEnd > 0 {
			if parsedAttempt, err := strconv.Atoi(tail[:digitEnd]); err == nil && parsedAttempt > 0 {
				attempt = parsedAttempt
			}
			tail = tail[digitEnd:]
		}
		role = strings.TrimPrefix(tail, "_")
		if role == "" {
			role = "agent"
		}
	} else {
		switch {
		case strings.HasSuffix(name, "_verifier"):
			role = "verifier"
		case strings.HasSuffix(name, "_agent"):
			role = "agent"
		default:
			role = "agent"
		}
	}

	raw, err := os.ReadFile(path)
	if err != nil {
		return nil
	}

	_ = job
	return &parsedJobLogFile{
		Role:    role,
		Attempt: attempt,
		File: JobLogFile{
			Kind:    kind,
			Label:   label,
			Path:    filepath.ToSlash(displayPath),
			Content: strings.TrimRight(string(raw), "\r\n"),
		},
	}
}

func jobLogRoleLabel(job domain.Job, role string) string {
	switch strings.TrimSpace(role) {
	case "verifier":
		return "検証者"
	case "agent":
		if isImplementationJob(job.Kind) {
			return "実装者"
		}
		return "エージェント"
	default:
		return strings.TrimSpace(role)
	}
}

func jobLogFileSortOrder(kind string) int {
	switch kind {
	case "activity":
		return 0
	case "stdout":
		return 1
	case "stderr":
		return 2
	default:
		return 3
	}
}

func isImplementationJob(kind domain.JobKind) bool {
	return kind == domain.JobKindIssueImplementation || kind == domain.JobKindPRConflict || kind == domain.JobKindPRFeedback
}

func (s *Server) ListenAndServe() error {
	return s.httpServer.ListenAndServe()
}

func (s *Server) Shutdown(ctx context.Context) error {
	return s.httpServer.Shutdown(ctx)
}
