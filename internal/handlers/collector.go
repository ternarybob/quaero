package handlers

import (
	"net/http"

	"github.com/ternarybob/arbor"
)

// CollectorHandler handles API endpoints for the collector service
type CollectorHandler struct {
	jiraScraper       JiraDataProvider
	confluenceScraper ConfluenceDataProvider
	logger            arbor.ILogger
}

// JiraDataProvider interface for accessing Jira data
type JiraDataProvider interface {
	GetJiraData() (map[string]interface{}, error)
}

// ConfluenceDataProvider interface for accessing Confluence data
type ConfluenceDataProvider interface {
	GetConfluenceData() (map[string]interface{}, error)
}

// PaginationResponse contains pagination metadata
type PaginationResponse struct {
	Page       int `json:"page"`
	PageSize   int `json:"pageSize"`
	TotalItems int `json:"totalItems"`
	TotalPages int `json:"totalPages"`
}

// CollectorResponse is the standard response format
type CollectorResponse struct {
	Data       interface{}        `json:"data"`
	Pagination PaginationResponse `json:"pagination"`
}

// NewCollectorHandler creates a new collector handler
func NewCollectorHandler(jiraScraper JiraDataProvider, confluenceScraper ConfluenceDataProvider, logger arbor.ILogger) *CollectorHandler {
	return &CollectorHandler{
		jiraScraper:       jiraScraper,
		confluenceScraper: confluenceScraper,
		logger:            logger,
	}
}

// GetProjectsHandler returns paginated list of projects with counts
func (h *CollectorHandler) GetProjectsHandler(w http.ResponseWriter, r *http.Request) {
	if !RequireMethod(w, r, "GET") {
		return
	}

	page, pageSize := GetPaginationParams(r)

	data, err := h.jiraScraper.GetJiraData()
	if err != nil {
		h.logger.Error().Err(err).Msg("Failed to get Jira data")
		WriteError(w, http.StatusInternalServerError, "Failed to get projects")
		return
	}

	projects, ok := data["projects"].([]map[string]interface{})
	if !ok {
		projects = []map[string]interface{}{}
	}

	// Add issue count to each project
	issues, _ := data["issues"].([]map[string]interface{})
	projectIssueCounts := make(map[string]int)
	for _, issue := range issues {
		if fields, ok := issue["fields"].(map[string]interface{}); ok {
			if project, ok := fields["project"].(map[string]interface{}); ok {
				if key, ok := project["key"].(string); ok {
					projectIssueCounts[key]++
				}
			}
		}
	}

	// Enrich projects with issue counts
	for i := range projects {
		if key, ok := projects[i]["key"].(string); ok {
			projects[i]["issueCount"] = projectIssueCounts[key]
		}
	}

	paginatedData, pagination := Paginate(projects, page, pageSize)

	response := CollectorResponse{
		Data:       paginatedData,
		Pagination: pagination,
	}

	WriteJSON(w, http.StatusOK, response)
}

// GetSpacesHandler returns paginated list of Confluence spaces with page counts
func (h *CollectorHandler) GetSpacesHandler(w http.ResponseWriter, r *http.Request) {
	if !RequireMethod(w, r, "GET") {
		return
	}

	page, pageSize := GetPaginationParams(r)

	data, err := h.confluenceScraper.GetConfluenceData()
	if err != nil {
		h.logger.Error().Err(err).Msg("Failed to get Confluence data")
		WriteError(w, http.StatusInternalServerError, "Failed to get spaces")
		return
	}

	spaces, ok := data["spaces"].([]map[string]interface{})
	if !ok {
		spaces = []map[string]interface{}{}
	}

	paginatedData, pagination := Paginate(spaces, page, pageSize)

	response := CollectorResponse{
		Data:       paginatedData,
		Pagination: pagination,
	}

	WriteJSON(w, http.StatusOK, response)
}

// GetIssuesHandler returns paginated list of issues for a project
func (h *CollectorHandler) GetIssuesHandler(w http.ResponseWriter, r *http.Request) {
	if !RequireMethod(w, r, "GET") {
		return
	}

	projectKey := r.URL.Query().Get("projectKey")
	if projectKey == "" {
		WriteError(w, http.StatusBadRequest, "projectKey parameter required")
		return
	}

	page, pageSize := GetPaginationParams(r)

	data, err := h.jiraScraper.GetJiraData()
	if err != nil {
		h.logger.Error().Err(err).Msg("Failed to get Jira data")
		WriteError(w, http.StatusInternalServerError, "Failed to get issues")
		return
	}

	allIssues, ok := data["issues"].([]map[string]interface{})
	if !ok {
		allIssues = []map[string]interface{}{}
	}

	// Filter issues by project key
	var filteredIssues []map[string]interface{}
	for _, issue := range allIssues {
		if fields, ok := issue["fields"].(map[string]interface{}); ok {
			if project, ok := fields["project"].(map[string]interface{}); ok {
				if key, ok := project["key"].(string); ok && key == projectKey {
					filteredIssues = append(filteredIssues, issue)
				}
			}
		}
	}

	paginatedData, pagination := Paginate(filteredIssues, page, pageSize)

	response := CollectorResponse{
		Data:       paginatedData,
		Pagination: pagination,
	}

	WriteJSON(w, http.StatusOK, response)
}

// GetPagesHandler returns paginated list of pages for a space
func (h *CollectorHandler) GetPagesHandler(w http.ResponseWriter, r *http.Request) {
	if !RequireMethod(w, r, "GET") {
		return
	}

	spaceKey := r.URL.Query().Get("spaceKey")
	if spaceKey == "" {
		WriteError(w, http.StatusBadRequest, "spaceKey parameter required")
		return
	}

	page, pageSize := GetPaginationParams(r)

	data, err := h.confluenceScraper.GetConfluenceData()
	if err != nil {
		h.logger.Error().Err(err).Msg("Failed to get Confluence data")
		WriteError(w, http.StatusInternalServerError, "Failed to get pages")
		return
	}

	allPages, ok := data["pages"].([]map[string]interface{})
	if !ok {
		allPages = []map[string]interface{}{}
	}

	// Filter pages by space key
	var filteredPages []map[string]interface{}
	for _, page := range allPages {
		if space, ok := page["space"].(map[string]interface{}); ok {
			if key, ok := space["key"].(string); ok && key == spaceKey {
				filteredPages = append(filteredPages, page)
			}
		}
	}

	paginatedData, pagination := Paginate(filteredPages, page, pageSize)

	response := CollectorResponse{
		Data:       paginatedData,
		Pagination: pagination,
	}

	WriteJSON(w, http.StatusOK, response)
}
