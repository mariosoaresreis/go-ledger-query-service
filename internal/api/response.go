package api

import (
	"encoding/json"
	"net/http"
)

// ErrorResponse is the standard error envelope.
type ErrorResponse struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// PaginatedResponse wraps a paginated list result.
type PaginatedResponse struct {
	Data       interface{} `json:"data"`
	Page       int         `json:"page"`
	Size       int         `json:"size"`
	TotalCount int         `json:"totalCount"`
}

func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, ErrorResponse{Code: status, Message: msg})
}
