package api

import (
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"
	"go.uber.org/zap"

	"github.com/log-analytics/server/internal/domain"
)

// handleListAnomalies handles GET /api/v1/anomalies.
// Supports optional query params: type (maps to rule_id), service, severity,
// from (RFC3339), to (RFC3339), page (default 1, min 1, max 10000),
// size (default 20, min 1, max 1000).
// Returns a JSON PagedResponse envelope with {data, total, page, size}.
func (s *Server) handleListAnomalies(w http.ResponseWriter, r *http.Request) {
	ruleType := r.URL.Query().Get("type")
	service := r.URL.Query().Get("service")
	severity := r.URL.Query().Get("severity")

	page, err := parseIntParam(r, "page", 1, 1, 10000)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	size, err := parseIntParam(r, "size", 20, 1, 1000)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	from, err := parseTimeParam(r, "from")
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	to, err := parseTimeParam(r, "to")
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	q := domain.AnomalyQuery{
		Type:     ruleType,
		Service:  service,
		Severity: severity,
		From:     from,
		To:       to,
		Page:     page,
		Size:     size,
	}

	anomalies, total, err := s.anomStore.SearchAnomalies(r.Context(), q)
	if err != nil {
		s.logger.Error("search anomalies failed", zap.Error(err))
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	writeJSON(w, http.StatusOK, PagedResponse{
		Data:  anomalies,
		Total: total,
		Page:  page,
		Size:  size,
	})
}

// handleGetAnomaly handles GET /api/v1/anomalies/{id}.
// Returns the anomaly as JSON with 200 OK, or a 404 JSON error if not found.
func (s *Server) handleGetAnomaly(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "missing id")
		return
	}

	anomaly, err := s.anomStore.GetAnomaly(r.Context(), id)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			writeError(w, http.StatusNotFound, "anomaly not found")
			return
		}
		s.logger.Error("get anomaly failed", zap.Error(err), zap.String("id", id))
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	writeJSON(w, http.StatusOK, anomaly)
}
