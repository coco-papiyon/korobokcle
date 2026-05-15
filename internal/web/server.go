package web

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/gorilla/mux"

	"github.com/coco-papiyon/korobokcle/internal/config"
	"github.com/coco-papiyon/korobokcle/internal/orchestrator"
)

type Server struct {
	httpServer   *http.Server
	orchestrator *orchestrator.Orchestrator
	config       *config.Service
	staticDir    string
}

func New(cfg *config.Service, orch *orchestrator.Orchestrator) (*Server, error) {
	s := &Server{
		orchestrator: orch,
		config:       cfg,
	}
	s.staticDir = filepath.Join(cfg.App().WorkspaceDir, "frontend", "dist")

	router := mux.NewRouter()
	api := router.PathPrefix("/api").Subrouter()
	api.HandleFunc("/jobs", s.handleJobs).Methods(http.MethodGet)
	api.HandleFunc("/jobs/{id}", s.handleJobDetail).Methods(http.MethodGet)
	api.HandleFunc("/jobs/{id}/approvals/design", s.handleDesignApproval).Methods(http.MethodPost)
	api.HandleFunc("/jobs/{id}/reruns/design", s.handleDesignRerun).Methods(http.MethodPost)
	api.HandleFunc("/jobs/{id}/approvals/final", s.handleFinalApproval).Methods(http.MethodPost)
	api.HandleFunc("/jobs/{id}/reruns/implementation", s.handleImplementationRerun).Methods(http.MethodPost)
	api.HandleFunc("/app-config", s.handleAppConfig).Methods(http.MethodGet)
	api.HandleFunc("/app-config", s.handleSaveAppConfig).Methods(http.MethodPut)
	api.HandleFunc("/watch-rules", s.handleWatchRules).Methods(http.MethodGet)
	api.HandleFunc("/watch-rules", s.handleSaveWatchRules).Methods(http.MethodPut)
	router.HandleFunc("/healthz", s.handleHealth).Methods(http.MethodGet)
	router.PathPrefix("/").HandlerFunc(s.handleSPA).Methods(http.MethodGet)

	s.httpServer = &http.Server{
		Addr:              fmt.Sprintf(":%d", cfg.App().HTTPPort),
		Handler:           router,
		ReadHeaderTimeout: 5 * time.Second,
	}
	return s, nil
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
