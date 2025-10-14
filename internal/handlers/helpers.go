package handlers

import (
	"encoding/json"
	"math"
	"net/http"
	"strconv"
	"strings"
)

// RequireMethod validates that the HTTP request uses the specified method.
// Returns true if the method matches, false otherwise (and writes error response).
func RequireMethod(w http.ResponseWriter, r *http.Request, method string) bool {
	if r.Method != method {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return false
	}
	return true
}

// WriteJSON writes a JSON response with the specified status code and data.
func WriteJSON(w http.ResponseWriter, statusCode int, data interface{}) error {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	return json.NewEncoder(w).Encode(data)
}

// WriteSuccess writes a standard success JSON response.
func WriteSuccess(w http.ResponseWriter, message string) error {
	return WriteJSON(w, http.StatusOK, map[string]string{
		"status":  "success",
		"message": message,
	})
}

// WriteError writes a standard error JSON response.
func WriteError(w http.ResponseWriter, statusCode int, message string) error {
	return WriteJSON(w, statusCode, map[string]string{
		"status": "error",
		"error":  message,
	})
}

// WriteStarted writes a standard "started" JSON response for async operations.
func WriteStarted(w http.ResponseWriter, message string) error {
	return WriteJSON(w, http.StatusOK, map[string]string{
		"status":  "started",
		"message": message,
	})
}

// AuthChecker interface for services that can check authentication status.
type AuthChecker interface {
	IsAuthenticated() bool
}

// RequireAuth checks if the user is authenticated.
// Returns true if authenticated, false otherwise (and writes error response).
func RequireAuth(w http.ResponseWriter, authService AuthChecker) bool {
	if !authService.IsAuthenticated() {
		WriteError(w, http.StatusUnauthorized, "Not authenticated. Please capture authentication first using the Chrome extension.")
		return false
	}
	return true
}

// PaginationResponse contains pagination metadata for API responses.
type PaginationResponse struct {
	Page       int `json:"page"`
	PageSize   int `json:"page_size"`
	TotalItems int `json:"total_items"`
	TotalPages int `json:"total_pages"`
}

// GetPaginationParams extracts pagination parameters from query string.
// Returns page (0-indexed) and pageSize (default 10, max 100).
func GetPaginationParams(r *http.Request) (page, pageSize int) {
	page = 0
	pageSize = 10

	if pageStr := r.URL.Query().Get("page"); pageStr != "" {
		if p, err := strconv.Atoi(pageStr); err == nil && p >= 0 {
			page = p
		}
	}

	if pageSizeStr := r.URL.Query().Get("pageSize"); pageSizeStr != "" {
		if ps, err := strconv.Atoi(pageSizeStr); err == nil && ps > 0 && ps <= 100 {
			pageSize = ps
		}
	}

	return page, pageSize
}

// Paginate applies pagination to a slice of data.
func Paginate(data []map[string]interface{}, page, pageSize int) ([]map[string]interface{}, PaginationResponse) {
	totalItems := len(data)
	totalPages := int(math.Ceil(float64(totalItems) / float64(pageSize)))

	start := page * pageSize
	end := start + pageSize

	if start >= totalItems {
		return []map[string]interface{}{}, PaginationResponse{
			Page:       page,
			PageSize:   pageSize,
			TotalItems: totalItems,
			TotalPages: totalPages,
		}
	}

	if end > totalItems {
		end = totalItems
	}

	paginatedData := data[start:end]

	pagination := PaginationResponse{
		Page:       page,
		PageSize:   pageSize,
		TotalItems: totalItems,
		TotalPages: totalPages,
	}

	return paginatedData, pagination
}

// ExtractProjectKey extracts the project key from a Jira issue key.
// Example: "BI9LLQNGKQ-1" -> "BI9LLQNGKQ"
func ExtractProjectKey(issueKey string) string {
	if idx := strings.Index(issueKey, "-"); idx > 0 {
		return issueKey[:idx]
	}
	return ""
}

// GetMapKeys returns all keys from a map as a slice.
func GetMapKeys(m map[string]interface{}) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}
