package handlers

import (
	"net/http"

	"github.com/ternarybob/arbor"
	"github.com/ternarybob/quaero/internal/common"
)

type APIHandler struct {
	logger arbor.ILogger
}

func NewAPIHandler() *APIHandler {
	return &APIHandler{
		logger: common.GetLogger(),
	}
}

// VersionHandler returns version information
func (h *APIHandler) VersionHandler(w http.ResponseWriter, r *http.Request) {
	if !RequireMethod(w, r, "GET") {
		return
	}

	WriteJSON(w, http.StatusOK, map[string]string{
		"version":    common.GetVersion(),
		"build":      common.GetBuild(),
		"git_commit": common.GetGitCommit(),
	})
}

// HealthHandler returns health check status
func (h *APIHandler) HealthHandler(w http.ResponseWriter, r *http.Request) {
	if !RequireMethod(w, r, "GET") {
		return
	}

	WriteJSON(w, http.StatusOK, map[string]string{
		"status": "ok",
	})
}

// NotFoundHandler handles 404 errors with JSON response
func (h *APIHandler) NotFoundHandler(w http.ResponseWriter, r *http.Request) {
	WriteJSON(w, http.StatusNotFound, map[string]interface{}{
		"error":   "Not Found",
		"path":    r.URL.Path,
		"message": "The requested endpoint does not exist",
	})
}
