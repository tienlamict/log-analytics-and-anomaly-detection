package api

import (
	"context"
	"net/http"
	"time"
)

// handleHealth pings Elasticsearch with a 3-second timeout.
// Returns 200 {"status":"ok"} if ES responds, 503 {"error":"unhealthy"} otherwise.
func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
	defer cancel()

	_, err := s.esClient.Ping().Do(ctx)
	if err != nil {
		writeError(w, http.StatusServiceUnavailable, "unhealthy")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// handleReady checks the atomic ready flag set by SetReady().
// Returns 200 {"status":"ready"} once the server is ready, 503 {"error":"not ready"} otherwise.
func (s *Server) handleReady(w http.ResponseWriter, r *http.Request) {
	if !s.ready.Load() {
		writeError(w, http.StatusServiceUnavailable, "not ready")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ready"})
}
