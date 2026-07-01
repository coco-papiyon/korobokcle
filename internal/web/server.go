package web

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
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

type ArtifactActions interface {
	GetArtifact(context.Context, string) (DesignArtifact, error)
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

type Server struct {
	httpServer      *http.Server
	artifactActions ArtifactActions
	skillActions    SkillActions
}

func NewServer(cfg config.Config, store JobStore, settingsStore SettingsStore, artifactActions ArtifactActions, optionalSkillActions ...SkillActions) *Server {
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
				job.State = req.State
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
		_ = json.NewEncoder(w).Encode(map[string]any{
			"updatedAt": updatedAt.UTC().Format(time.RFC3339Nano),
			"job":       job,
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

func (s *Server) ListenAndServe() error {
	return s.httpServer.ListenAndServe()
}

func (s *Server) Shutdown(ctx context.Context) error {
	return s.httpServer.Shutdown(ctx)
}
