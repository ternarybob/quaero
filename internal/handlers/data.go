package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/ternarybob/arbor"
	"github.com/ternarybob/quaero/internal/common"
	"github.com/ternarybob/quaero/internal/interfaces"
	"github.com/ternarybob/quaero/internal/models"
)

type DataHandler struct {
	jiraScraper       interfaces.JiraScraper
	confluenceScraper interfaces.ConfluenceScraper
	logger            arbor.ILogger
}

func NewDataHandler(jira interfaces.JiraScraper, confluence interfaces.ConfluenceScraper) *DataHandler {
	return &DataHandler{
		jiraScraper:       jira,
		confluenceScraper: confluence,
		logger:            common.GetLogger(),
	}
}

// GetJiraDataHandler returns all Jira data (projects and issues)
func (h *DataHandler) GetJiraDataHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	data, err := h.jiraScraper.GetJiraData()
	if err != nil {
		h.logger.Error().Err(err).Msg("Failed to fetch Jira data")
		http.Error(w, "Failed to fetch Jira data", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(data)
}

// GetJiraIssuesHandler returns issues optionally filtered by project keys
func (h *DataHandler) GetJiraIssuesHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Get optional project keys filter from query params
	projectKeys := r.URL.Query()["projectKey"]

	h.logger.Info().Strs("projectKeys", projectKeys).Msg("GetJiraIssuesHandler called")

	data, err := h.jiraScraper.GetJiraData()
	if err != nil {
		h.logger.Error().Err(err).Msg("Failed to fetch Jira data")
		http.Error(w, "Failed to fetch Jira data", http.StatusInternalServerError)
		return
	}

	issues := data["issues"]
	if issues == nil {
		issues = []interface{}{}
	}

	totalIssuesInDB := 0
	// Handle both []interface{} and []map[string]interface{} types
	if issueList, ok := issues.([]interface{}); ok {
		totalIssuesInDB = len(issueList)
	} else if issueList, ok := issues.([]map[string]interface{}); ok {
		totalIssuesInDB = len(issueList)
	}

	// Filter by project keys if specified
	if len(projectKeys) > 0 {
		filteredIssues := []interface{}{}

		// Handle []map[string]interface{} type from GetJiraData()
		if issueList, ok := issues.([]map[string]interface{}); ok {
			h.logger.Info().Int("issueCount", len(issueList)).Msg("Processing issues as []map[string]interface{}")

			for i, issue := range issueList {
				// Debug first issue structure
				if i == 0 {
					issueJSON, _ := json.Marshal(issue)
					h.logger.Info().Str("firstIssue", string(issueJSON)).Msg("First issue structure")
					h.logger.Info().Strs("keys", getMapKeys(issue)).Msg("First issue top-level keys")
				}

				// Try direct key field first (from database)
				if issueKey, ok := issue["key"].(string); ok {
					// Extract project key from issue key (e.g., "BI9LLQNGKQ-1" -> "BI9LLQNGKQ")
					projectKey := extractProjectKey(issueKey)
					for _, pk := range projectKeys {
						if projectKey == pk {
							filteredIssues = append(filteredIssues, issue)
							break
						}
					}
					continue
				}

				// Fallback to nested fields.project.key (from Jira API)
				if fields, ok := issue["fields"].(map[string]interface{}); ok {
					if project, ok := fields["project"].(map[string]interface{}); ok {
						if key, ok := project["key"].(string); ok {
							for _, pk := range projectKeys {
								if key == pk {
									filteredIssues = append(filteredIssues, issue)
									break
								}
							}
						}
					}
				}
			}
			issues = filteredIssues
		} else if issueList, ok := issues.([]interface{}); ok {
			h.logger.Info().Int("issueCount", len(issueList)).Msg("Processing issues as []interface{}")

			for i, issue := range issueList {
				if i == 0 {
					h.logger.Info().Str("issueType", fmt.Sprintf("%T", issue)).Msg("First issue type")
				}

				// Handle *models.JiraIssue (from database)
				if jiraIssue, ok := issue.(*models.JiraIssue); ok {
					projectKey := extractProjectKey(jiraIssue.Key)
					if i == 0 {
						h.logger.Info().Str("key", jiraIssue.Key).Str("extractedProjectKey", projectKey).Msg("First JiraIssue")
					}
					for _, pk := range projectKeys {
						if projectKey == pk {
							filteredIssues = append(filteredIssues, issue)
							break
						}
					}
					continue
				}

				// Handle map[string]interface{} (from Jira API or other sources)
				if issueMap, ok := issue.(map[string]interface{}); ok {
					// Try direct key field first
					if issueKey, ok := issueMap["key"].(string); ok {
						projectKey := extractProjectKey(issueKey)
						for _, pk := range projectKeys {
							if projectKey == pk {
								filteredIssues = append(filteredIssues, issue)
								break
							}
						}
						continue
					}

					// Fallback to nested fields.project.key
					if fields, ok := issueMap["fields"].(map[string]interface{}); ok {
						if project, ok := fields["project"].(map[string]interface{}); ok {
							if key, ok := project["key"].(string); ok {
								for _, pk := range projectKeys {
									if key == pk {
										filteredIssues = append(filteredIssues, issue)
										break
									}
								}
							}
						}
					}
				}
			}
			issues = filteredIssues
		}
		firstProjectKey := "none"
		if len(filteredIssues) > 0 {
			if issueMap, ok := filteredIssues[0].(map[string]interface{}); ok {
				if fields, ok := issueMap["fields"].(map[string]interface{}); ok {
					if project, ok := fields["project"].(map[string]interface{}); ok {
						if key, ok := project["key"].(string); ok {
							firstProjectKey = key
						}
					}
				}
			}
		}

		h.logger.Info().
			Int("totalInDB", totalIssuesInDB).
			Int("filtered", len(filteredIssues)).
			Strs("projectKeys", projectKeys).
			Str("firstIssueProject", firstProjectKey).
			Msg("Filtered issues by project")
	}

	// Log what we're returning
	returnCount := 0
	if issueList, ok := issues.([]interface{}); ok {
		returnCount = len(issueList)
	}
	h.logger.Info().
		Int("returningIssueCount", returnCount).
		Strs("requestedProjects", projectKeys).
		Msg("Returning issues to client")

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"issues": issues,
	})
}

// GetConfluenceDataHandler returns all Confluence data (spaces and pages)
func (h *DataHandler) GetConfluenceDataHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	data, err := h.confluenceScraper.GetConfluenceData()
	if err != nil {
		h.logger.Error().Err(err).Msg("Failed to fetch Confluence data")
		http.Error(w, "Failed to fetch Confluence data", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(data)
}

// GetConfluencePagesHandler returns pages optionally filtered by space keys
func (h *DataHandler) GetConfluencePagesHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	spaceKeys := r.URL.Query()["spaceKey"]

	h.logger.Info().Strs("spaceKeys", spaceKeys).Msg("GetConfluencePagesHandler called")

	data, err := h.confluenceScraper.GetConfluenceData()
	if err != nil {
		h.logger.Error().Err(err).Msg("Failed to fetch Confluence data")
		http.Error(w, "Failed to fetch Confluence data", http.StatusInternalServerError)
		return
	}

	pages := data["pages"]
	if pages == nil {
		pages = []interface{}{}
	}

	totalPagesInDB := 0
	if pageList, ok := pages.([]interface{}); ok {
		totalPagesInDB = len(pageList)
	} else if pageList, ok := pages.([]map[string]interface{}); ok {
		totalPagesInDB = len(pageList)
	}

	if len(spaceKeys) > 0 {
		filteredPages := []interface{}{}

		if pageList, ok := pages.([]map[string]interface{}); ok {
			for _, page := range pageList {
				if space, ok := page["space"].(map[string]interface{}); ok {
					if key, ok := space["key"].(string); ok {
						for _, sk := range spaceKeys {
							if key == sk {
								filteredPages = append(filteredPages, page)
								break
							}
						}
					}
				}
			}
			pages = filteredPages
		} else if pageList, ok := pages.([]interface{}); ok {
			for i, page := range pageList {
				// Handle *models.ConfluencePage (from database)
				if confluencePage, ok := page.(*models.ConfluencePage); ok {
					if i == 0 {
						h.logger.Info().Str("spaceId", confluencePage.SpaceID).Msg("First ConfluencePage from DB")
					}
					for _, sk := range spaceKeys {
						if confluencePage.SpaceID == sk {
							filteredPages = append(filteredPages, page)
							break
						}
					}
					continue
				}

				// Handle map[string]interface{} (from Confluence API or other sources)
				if pageMap, ok := page.(map[string]interface{}); ok {
					if space, ok := pageMap["space"].(map[string]interface{}); ok {
						if key, ok := space["key"].(string); ok {
							for _, sk := range spaceKeys {
								if key == sk {
									filteredPages = append(filteredPages, page)
									break
								}
							}
						}
					}
				}
			}
			pages = filteredPages
		}

		h.logger.Info().
			Int("totalInDB", totalPagesInDB).
			Int("filtered", len(filteredPages)).
			Strs("spaceKeys", spaceKeys).
			Msg("Filtered pages by space")
	}

	returnCount := 0
	if pageList, ok := pages.([]interface{}); ok {
		returnCount = len(pageList)
	}
	h.logger.Info().
		Int("returningPageCount", returnCount).
		Strs("requestedSpaces", spaceKeys).
		Msg("Returning pages to client")

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"pages": pages,
	})
}

// extractProjectKey extracts the project key from an issue key (e.g., "BI9LLQNGKQ-1" -> "BI9LLQNGKQ")
func extractProjectKey(issueKey string) string {
	if idx := strings.Index(issueKey, "-"); idx > 0 {
		return issueKey[:idx]
	}
	return ""
}

// getMapKeys returns all keys from a map
func getMapKeys(m map[string]interface{}) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}
