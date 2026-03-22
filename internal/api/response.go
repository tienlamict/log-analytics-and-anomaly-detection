package api

import (
	"encoding/json"
	"net/http"
)

// PagedResponse wraps a slice of results with pagination metadata.
type PagedResponse struct {
	Data  any   `json:"data"`
	Total int64 `json:"total"`
	Page  int   `json:"page"`
	Size  int   `json:"size"`
}

// ErrorResponse is the JSON body returned for all error responses.
type ErrorResponse struct {
	Error  string `json:"error"`
	Status int    `json:"status"`
}

// writeJSON sets Content-Type to application/json, writes the status code,
// and encodes v as JSON into the response body.
func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

// writeError writes a JSON ErrorResponse with the given status and message.
func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, ErrorResponse{Error: msg, Status: status})
}
