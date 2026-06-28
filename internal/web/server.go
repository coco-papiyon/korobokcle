package web

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/coco-papiyon/korobokcle/internal/config"
	"github.com/coco-papiyon/korobokcle/internal/domain"
)

type JobStore interface {
	List(context.Context) ([]domain.Job, error)
	Get(context.Context, string) (domain.Job, bool, error)
	Upsert(context.Context, domain.Job) error
}

type SettingsStore interface {
	Load(context.Context) (domain.WatchSettings, error)
	Save(context.Context, domain.WatchSettings) error
}

type Server struct {
	httpServer *http.Server
}

func NewServer(cfg config.Config, store JobStore, settingsStore SettingsStore) *Server {
	mux := http.NewServeMux()

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
			_ = json.NewEncoder(w).Encode(map[string]any{"jobs": jobs})
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
		_ = json.NewEncoder(w).Encode(job)
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

	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/api/") {
			http.NotFound(w, r)
			return
		}

		distDir := filepath.Join(cfg.ToolDir, "frontend", "dist")
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
