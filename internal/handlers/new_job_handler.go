package handlers

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/gorilla/mux"
	"github.com/ternarybob/quaero/internal/jobs"
)

type NewJobHandler struct {
	jobMgr *jobs.Manager
}

func NewJobHandler(jobMgr *jobs.Manager) *NewJobHandler {
	return &NewJobHandler{jobMgr: jobMgr}
}

// ListJobs handles GET /api/jobs
// Returns parent jobs only (no parent_id)
func (h *NewJobHandler) ListJobs(w http.ResponseWriter, r *http.Request) {
	limit := 100
	offset := 0

	if l := r.URL.Query().Get("limit"); l != "" {
		if val, err := strconv.Atoi(l); err == nil {
			limit = val
		}
	}

	if o := r.URL.Query().Get("offset"); o != "" {
		if val, err := strconv.Atoi(o); err == nil {
			offset = val
		}
	}

	jobs, err := h.jobMgr.ListParentJobs(r.Context(), limit, offset)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(jobs)
}

// GetJobChildren handles GET /api/jobs/{id}/children
func (h *NewJobHandler) GetJobChildren(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	jobID := vars["id"]

	children, err := h.jobMgr.ListChildJobs(r.Context(), jobID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(children)
}

// GetJobLogs handles GET /api/jobs/{id}/logs
func (h *NewJobHandler) GetJobLogs(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	jobID := vars["id"]

	limit := 1000
	if l := r.URL.Query().Get("limit"); l != "" {
		if val, err := strconv.Atoi(l); err == nil {
			limit = val
		}
	}

	logs, err := h.jobMgr.GetJobLogs(r.Context(), jobID, limit)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(logs)
}

// GetJob handles GET /api/jobs/{id}
func (h *NewJobHandler) GetJob(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	jobID := vars["id"]

	job, err := h.jobMgr.GetJob(r.Context(), jobID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(job)
}

// CreateJob handles POST /api/jobs
func (h *NewJobHandler) CreateJob(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Type    string      `json:"type"`
		Payload interface{} `json:"payload"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	jobID, err := h.jobMgr.CreateParentJob(r.Context(), req.Type, req.Payload)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"job_id": jobID})
}
