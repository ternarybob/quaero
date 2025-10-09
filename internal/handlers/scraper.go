// -----------------------------------------------------------------------
// Last Modified: Thursday, 9th October 2025 8:52:24 am
// Modified By: Bob McAllan
// -----------------------------------------------------------------------

package handlers

import (
	"encoding/json"
	"net/http"
	"sync"

	"github.com/ternarybob/arbor"
	"github.com/ternarybob/quaero/internal/common"
	"github.com/ternarybob/quaero/internal/interfaces"
)

type ScraperHandler struct {
	authService       interfaces.AuthService
	jiraScraper       interfaces.JiraScraper
	confluenceScraper interfaces.ConfluenceScraper
	logger            arbor.ILogger
	wsHandler         *WebSocketHandler
}

func NewScraperHandler(authService interfaces.AuthService, jira interfaces.JiraScraper, confluence interfaces.ConfluenceScraper, ws *WebSocketHandler) *ScraperHandler {
	return &ScraperHandler{
		authService:       authService,
		jiraScraper:       jira,
		confluenceScraper: confluence,
		logger:            common.GetLogger(),
		wsHandler:         ws,
	}
}

// AuthStatusHandler returns the current authentication status
func (h *ScraperHandler) AuthStatusHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	authData, err := h.authService.LoadAuth()

	w.Header().Set("Content-Type", "application/json")
	if err != nil || authData == nil || !h.authService.IsAuthenticated() {
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"authenticated": false,
			"cookies":       []interface{}{},
		})
		return
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(authData)
}

// AuthUpdateHandler handles authentication updates from Chrome extension
func (h *ScraperHandler) AuthUpdateHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var authData interfaces.AuthData
	if err := json.NewDecoder(r.Body).Decode(&authData); err != nil {
		h.logger.Error().Err(err).Msg("Failed to decode auth data")
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Update auth via centralized AuthService (shared by both scrapers)
	if err := h.authService.UpdateAuth(&authData); err != nil {
		h.logger.Error().Err(err).Msg("Failed to update authentication")
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	h.logger.Info().Str("baseURL", authData.BaseURL).Msg("Authentication updated successfully")

	// Broadcast auth data to WebSocket clients
	if h.wsHandler != nil {
		h.wsHandler.BroadcastAuth(&authData)
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{
		"status":  "authenticated",
		"message": "Authentication captured successfully",
	})
}

// ScrapeHandler manually triggers scraping
func (h *ScraperHandler) ScrapeHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" && r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Trigger scraping on both scrapers
	go func() {
		if err := h.jiraScraper.ScrapeProjects(); err != nil {
			h.logger.Error().Err(err).Msg("Jira scraping error")
		}
		if err := h.confluenceScraper.ScrapeConfluence(); err != nil {
			h.logger.Error().Err(err).Msg("Confluence scraping error")
		}
	}()

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{
		"status":  "started",
		"message": "Scraping triggered",
	})
}

// ScrapeProjectsHandler triggers scraping of Jira projects only
func (h *ScraperHandler) ScrapeProjectsHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if !h.authService.IsAuthenticated() {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(map[string]string{
			"status":  "error",
			"message": "Not authenticated. Please capture authentication first.",
		})
		return
	}

	go func() {
		if err := h.jiraScraper.ScrapeProjects(); err != nil {
			h.logger.Error().Err(err).Msg("Project scrape error")
		}
	}()

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{
		"status":  "started",
		"message": "Jira projects scraping started",
	})
}

// ScrapeSpacesHandler triggers scraping of Confluence spaces only
func (h *ScraperHandler) ScrapeSpacesHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if !h.authService.IsAuthenticated() {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(map[string]string{
			"status":  "error",
			"message": "Not authenticated. Please capture authentication first.",
		})
		return
	}

	go func() {
		if err := h.confluenceScraper.ScrapeConfluence(); err != nil {
			h.logger.Error().Err(err).Msg("Confluence scrape error")
		}
	}()

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{
		"status":  "started",
		"message": "Confluence spaces scraping started",
	})
}

// RefreshProjectsCacheHandler clears projects cache and re-syncs from Jira
func (h *ScraperHandler) RefreshProjectsCacheHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if !h.authService.IsAuthenticated() {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(map[string]string{
			"status":  "error",
			"message": "Not authenticated. Please capture authentication first.",
		})
		return
	}

	// Type assertion to access ClearProjectsCache method
	type projectCacheClearer interface {
		ClearProjectsCache() error
	}

	// Clear cache synchronously first, so immediate API calls won't see old data
	if clearer, ok := h.jiraScraper.(projectCacheClearer); ok {
		if err := clearer.ClearProjectsCache(); err != nil {
			h.logger.Error().Err(err).Msg("Failed to clear projects cache")
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]string{
				"status":  "error",
				"message": "Failed to clear projects cache",
			})
			return
		}
	}

	// Re-sync projects in background
	go func() {
		if err := h.jiraScraper.ScrapeProjects(); err != nil {
			h.logger.Error().Err(err).Msg("Project scrape error after cache refresh")
		}
	}()

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{
		"status":  "started",
		"message": "Projects cache refresh started",
	})
}

// GetProjectIssuesHandler fetches issues for selected projects
func (h *ScraperHandler) GetProjectIssuesHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if !h.authService.IsAuthenticated() {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(map[string]string{
			"status":  "error",
			"message": "Not authenticated. Please capture authentication first.",
		})
		return
	}

	var request struct {
		ProjectKeys []string `json:"projectKeys"`
	}
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}

	if len(request.ProjectKeys) == 0 {
		http.Error(w, "No projects specified", http.StatusBadRequest)
		return
	}

	// Type assertion to access GetProjectIssues method
	type projectIssueGetter interface {
		GetProjectIssues(projectKey string) error
	}

	// Fetch issues for each project in parallel using goroutines
	go func() {
		if getter, ok := h.jiraScraper.(projectIssueGetter); ok {
			var wg sync.WaitGroup

			for _, projectKey := range request.ProjectKeys {
				wg.Add(1)

				// Launch goroutine for each project
				go func(key string) {
					defer wg.Done()

					h.logger.Info().Str("project", key).Msg("Starting parallel fetch for project")

					if err := getter.GetProjectIssues(key); err != nil {
						h.logger.Error().Err(err).Str("project", key).Msg("Failed to get project issues")
					} else {
						h.logger.Info().Str("project", key).Msg("Completed parallel fetch for project")
					}
				}(projectKey)
			}

			// Wait for all projects to complete
			wg.Wait()
			h.logger.Info().Int("projectCount", len(request.ProjectKeys)).Msg("Completed fetching all projects")
		}
	}()

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{
		"status":  "started",
		"message": "Fetching issues for selected projects",
	})
}

// RefreshSpacesCacheHandler clears spaces cache and re-syncs from Confluence
func (h *ScraperHandler) RefreshSpacesCacheHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if !h.authService.IsAuthenticated() {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(map[string]string{
			"status":  "error",
			"message": "Not authenticated. Please capture authentication first.",
		})
		return
	}

	type spaceCacheClearer interface {
		ClearSpacesCache() error
	}

	if clearer, ok := h.confluenceScraper.(spaceCacheClearer); ok {
		if err := clearer.ClearSpacesCache(); err != nil {
			h.logger.Error().Err(err).Msg("Failed to clear spaces cache")
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]string{
				"status":  "error",
				"message": "Failed to clear spaces cache",
			})
			return
		}
	}

	go func() {
		if err := h.confluenceScraper.ScrapeConfluence(); err != nil {
			h.logger.Error().Err(err).Msg("Confluence scrape error after cache refresh")
		}
	}()

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{
		"status":  "started",
		"message": "Spaces cache refresh started",
	})
}

// GetSpacePagesHandler fetches pages for selected spaces
func (h *ScraperHandler) GetSpacePagesHandler(w http.ResponseWriter, r *http.Request) {
	h.logger.Info().Msg("GetSpacePagesHandler called")

	if r.Method != "POST" {
		h.logger.Warn().Str("method", r.Method).Msg("Invalid method for GetSpacePagesHandler")
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if !h.authService.IsAuthenticated() {
		h.logger.Warn().Msg("GetSpacePagesHandler called but not authenticated")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(map[string]string{
			"status":  "error",
			"message": "Not authenticated. Please capture authentication first.",
		})
		return
	}

	var request struct {
		SpaceKeys []string `json:"spaceKeys"`
	}
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		h.logger.Error().Err(err).Msg("Failed to decode request body")
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}

	h.logger.Info().Strs("spaceKeys", request.SpaceKeys).Msg("Received request to fetch pages")

	if len(request.SpaceKeys) == 0 {
		h.logger.Warn().Msg("No spaces specified in request")
		http.Error(w, "No spaces specified", http.StatusBadRequest)
		return
	}

	type spacePageGetter interface {
		GetSpacePages(spaceKey string) error
	}

	go func() {
		if getter, ok := h.confluenceScraper.(spacePageGetter); ok {
			var wg sync.WaitGroup

			for _, spaceKey := range request.SpaceKeys {
				wg.Add(1)

				go func(key string) {
					defer wg.Done()

					h.logger.Info().Str("space", key).Msg("Starting parallel fetch for space")

					if err := getter.GetSpacePages(key); err != nil {
						h.logger.Error().Err(err).Str("space", key).Msg("Failed to get space pages")
					} else {
						h.logger.Info().Str("space", key).Msg("Completed parallel fetch for space")
					}
				}(spaceKey)
			}

			wg.Wait()
			h.logger.Info().Int("spaceCount", len(request.SpaceKeys)).Msg("Completed fetching all spaces")
		}
	}()

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{
		"status":  "started",
		"message": "Fetching pages for selected spaces",
	})
}

// ClearAllDataHandler clears all cached data from the database
func (h *ScraperHandler) ClearAllDataHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	h.logger.Info().Msg("Clearing all data from database")

	type dataClearer interface {
		ClearAllData() error
	}

	// Clear data from both scrapers
	jiraClearer, jiraOk := h.jiraScraper.(dataClearer)
	confluenceClearer, confluenceOk := h.confluenceScraper.(dataClearer)

	if !jiraOk && !confluenceOk {
		http.Error(w, "Clear data not supported", http.StatusNotImplemented)
		return
	}

	// Clear Jira data
	if jiraOk {
		if err := jiraClearer.ClearAllData(); err != nil {
			h.logger.Error().Err(err).Msg("Failed to clear Jira data")
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]string{
				"status":  "error",
				"message": "Failed to clear Jira data",
			})
			return
		}
	}

	// Clear Confluence data
	if confluenceOk {
		if err := confluenceClearer.ClearAllData(); err != nil {
			h.logger.Error().Err(err).Msg("Failed to clear Confluence data")
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]string{
				"status":  "error",
				"message": "Failed to clear Confluence data",
			})
			return
		}
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{
		"status":  "success",
		"message": "All data cleared successfully",
	})
}

// ClearJiraDataHandler clears only Jira data from the database
func (h *ScraperHandler) ClearJiraDataHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	h.logger.Info().Msg("Clearing Jira data from database")

	type dataClearer interface {
		ClearAllData() error
	}

	jiraClearer, ok := h.jiraScraper.(dataClearer)
	if !ok {
		http.Error(w, "Clear Jira data not supported", http.StatusNotImplemented)
		return
	}

	if err := jiraClearer.ClearAllData(); err != nil {
		h.logger.Error().Err(err).Msg("Failed to clear Jira data")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{
			"status":  "error",
			"message": "Failed to clear Jira data",
		})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{
		"status":  "success",
		"message": "Jira data cleared successfully",
	})
}

// ClearConfluenceDataHandler clears only Confluence data from the database
func (h *ScraperHandler) ClearConfluenceDataHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	h.logger.Info().Msg("Clearing Confluence data from database")

	type dataClearer interface {
		ClearAllData() error
	}

	confluenceClearer, ok := h.confluenceScraper.(dataClearer)
	if !ok {
		http.Error(w, "Clear Confluence data not supported", http.StatusNotImplemented)
		return
	}

	if err := confluenceClearer.ClearAllData(); err != nil {
		h.logger.Error().Err(err).Msg("Failed to clear Confluence data")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{
			"status":  "error",
			"message": "Failed to clear Confluence data",
		})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{
		"status":  "success",
		"message": "Confluence data cleared successfully",
	})
}

// ParserStatusHandler returns the status of parser/scraper services
func (h *ScraperHandler) ParserStatusHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Get Jira project status
	jiraProjectLastUpdated, jiraProjectDetails, err := h.jiraScraper.GetProjectStatus()
	if err != nil {
		h.logger.Error().Err(err).Msg("Failed to get Jira project status")
		jiraProjectLastUpdated = 0
		jiraProjectDetails = "Error fetching status"
	}

	// Get Jira issue status
	jiraIssueLastUpdated, jiraIssueDetails, err := h.jiraScraper.GetIssueStatus()
	if err != nil {
		h.logger.Error().Err(err).Msg("Failed to get Jira issue status")
		jiraIssueLastUpdated = 0
		jiraIssueDetails = "Error fetching status"
	}

	// Get Confluence space status
	confluenceSpaceLastUpdated, confluenceSpaceDetails, err := h.confluenceScraper.GetSpaceStatus()
	if err != nil {
		h.logger.Error().Err(err).Msg("Failed to get Confluence space status")
		confluenceSpaceLastUpdated = 0
		confluenceSpaceDetails = "Error fetching status"
	}

	// Get Confluence page status
	confluencePageLastUpdated, confluencePageDetails, err := h.confluenceScraper.GetPageStatus()
	if err != nil {
		h.logger.Error().Err(err).Msg("Failed to get Confluence page status")
		confluencePageLastUpdated = 0
		confluencePageDetails = "Error fetching status"
	}

	// Get counts
	jiraProjectCount := h.jiraScraper.GetProjectCount()
	jiraIssueCount := h.jiraScraper.GetIssueCount()
	confluenceSpaceCount := h.confluenceScraper.GetSpaceCount()
	confluencePageCount := h.confluenceScraper.GetPageCount()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"jiraProjects": map[string]interface{}{
			"count":       jiraProjectCount,
			"lastUpdated": jiraProjectLastUpdated,
			"details":     jiraProjectDetails,
		},
		"jiraIssues": map[string]interface{}{
			"count":       jiraIssueCount,
			"lastUpdated": jiraIssueLastUpdated,
			"details":     jiraIssueDetails,
		},
		"confluenceSpaces": map[string]interface{}{
			"count":       confluenceSpaceCount,
			"lastUpdated": confluenceSpaceLastUpdated,
			"details":     confluenceSpaceDetails,
		},
		"confluencePages": map[string]interface{}{
			"count":       confluencePageCount,
			"lastUpdated": confluencePageLastUpdated,
			"details":     confluencePageDetails,
		},
	})
}

// AuthDetailsHandler returns authentication details for services
func (h *ScraperHandler) AuthDetailsHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	authData, err := h.authService.LoadAuth()

	services := []map[string]interface{}{}

	if err == nil && authData != nil && h.authService.IsAuthenticated() {
		// Both services use same Atlassian auth
		services = append(services,
			map[string]interface{}{
				"name":   "Jira",
				"status": "authenticated",
				"user":   authData.BaseURL,
			},
			map[string]interface{}{
				"name":   "Confluence",
				"status": "authenticated",
				"user":   authData.BaseURL,
			},
		)
	} else {
		services = append(services,
			map[string]interface{}{
				"name":   "Jira",
				"status": "not authenticated",
				"user":   "-",
			},
			map[string]interface{}{
				"name":   "Confluence",
				"status": "not authenticated",
				"user":   "-",
			},
		)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"services": services,
	})
}
