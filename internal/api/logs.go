package api

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"go.uber.org/zap"

	"github.com/log-analytics/server/internal/domain"
)

// parseIntParam reads a query param by name, returns def if absent, and returns
// an error if the value is not a valid integer. The result is clamped to [min, max].
func parseIntParam(r *http.Request, name string, def, min, max int) (int, error) {
	raw := r.URL.Query().Get(name)
	if raw == "" {
		return def, nil
	}
	v, err := strconv.Atoi(raw)
	if err != nil {
		return 0, fmt.Errorf("%s: not an integer", name)
	}
	if v < min {
		v = min
	}
	if v > max {
		v = max
	}
	return v, nil
}

// parseTimeParam reads a query param by name and parses it as RFC 3339.
// Returns the zero time and nil if the param is absent.
func parseTimeParam(r *http.Request, name string) (time.Time, error) {
	raw := r.URL.Query().Get(name)
	if raw == "" {
		return time.Time{}, nil
	}
	t, err := time.Parse(time.RFC3339, raw)
	if err != nil {
		return time.Time{}, fmt.Errorf("%s: must be RFC 3339 format", name)
	}
	return t, nil
}

// handleListLogs handles GET /api/v1/logs.
// It supports optional query params: service, level, from (RFC3339), to (RFC3339),
// page (default 1, min 1, max 10000), size (default 20, min 1, max 1000).
// Returns a JSON PagedResponse envelope with {data, total, page, size}.
func (s *Server) handleListLogs(w http.ResponseWriter, r *http.Request) {
	service := r.URL.Query().Get("service")
	level := r.URL.Query().Get("level")

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

	q := domain.LogQuery{
		Service: service,
		Level:   level,
		From:    from,
		To:      to,
		Page:    page,
		Size:    size,
	}

	entries, total, err := s.logStore.SearchLogs(r.Context(), q)
	if err != nil {
		s.logger.Error("search logs failed", zap.Error(err))
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	writeJSON(w, http.StatusOK, PagedResponse{
		Data:  entries,
		Total: total,
		Page:  page,
		Size:  size,
	})
}

// handleGetLog handles GET /api/v1/logs/{id}.
// Returns the log entry as JSON with 200 OK, or a 404 JSON error if not found.
func (s *Server) handleGetLog(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "missing id")
		return
	}

	entry, err := s.logStore.GetLog(r.Context(), id)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			writeError(w, http.StatusNotFound, "log entry not found")
			return
		}
		s.logger.Error("get log failed", zap.Error(err), zap.String("id", id))
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	writeJSON(w, http.StatusOK, entry)
}
