package api

import (
	"context"
	"fmt"
	"net/http"
	"sync/atomic"
	"time"

	elasticsearch "github.com/elastic/go-elasticsearch/v9"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.uber.org/zap"

	"github.com/log-analytics/server/internal/domain"
)

// Server is the HTTP API server. It wraps a chi router with middleware,
// exposes health/ready/metrics endpoints, and accepts log and anomaly stores
// for use by request handlers registered in subsequent plans.
type Server struct {
	httpServer *http.Server
	router     chi.Router
	logStore   domain.LogStore
	anomStore  domain.AnomalyStore
	esClient   *elasticsearch.TypedClient
	logger     *zap.Logger
	ready      atomic.Bool
}

// NewServer constructs a Server with the chi router, standard middleware stack,
// and the fixed infrastructure routes (/health, /ready, /metrics).
// Log and anomaly routes under /api/v1 will be registered by plans 04-02 and 04-03.
func NewServer(
	logStore domain.LogStore,
	anomStore domain.AnomalyStore,
	esClient *elasticsearch.TypedClient,
	logger *zap.Logger,
	port int,
) *Server {
	s := &Server{
		logStore:  logStore,
		anomStore: anomStore,
		esClient:  esClient,
		logger:    logger,
	}

	r := chi.NewRouter()

	// Middleware stack (applied in declaration order)
	r.Use(middleware.Recoverer)
	r.Use(middleware.RequestID)
	r.Use(middleware.Timeout(30 * time.Second))
	r.Use(zapRequestLogger(logger))

	// Infrastructure routes
	r.Get("/health", s.handleHealth)
	r.Get("/ready", s.handleReady)
	r.Get("/metrics", promhttp.Handler().ServeHTTP)

	// API v1 sub-router — log and anomaly handlers
	r.Route("/api/v1", func(r chi.Router) {
		r.Get("/logs", s.handleListLogs)
		r.Get("/logs/{id}", s.handleGetLog)
		r.Get("/anomalies", s.handleListAnomalies)
		r.Get("/anomalies/{id}", s.handleGetAnomaly)
	})

	s.router = r
	s.httpServer = &http.Server{
		Addr:         fmt.Sprintf(":%d", port),
		Handler:      r,
		ReadTimeout:  60 * time.Second,
		WriteTimeout: 60 * time.Second,
	}

	return s
}

// ListenAndServe starts the HTTP server and blocks until it returns an error.
func (s *Server) ListenAndServe() error {
	return s.httpServer.ListenAndServe()
}

// Shutdown gracefully shuts down the HTTP server without interrupting active connections.
func (s *Server) Shutdown(ctx context.Context) error {
	return s.httpServer.Shutdown(ctx)
}

// SetReady marks the server as ready to accept traffic.
// Call this after all dependencies (ES, Kafka) have been confirmed healthy.
func (s *Server) SetReady() {
	s.ready.Store(true)
}

// Router returns the underlying chi.Router for test access and handler registration.
func (s *Server) Router() chi.Router {
	return s.router
}
