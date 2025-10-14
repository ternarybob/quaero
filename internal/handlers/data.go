package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/ternarybob/arbor"
	"github.com/ternarybob/quaero/internal/common"
	"github.com/ternarybob/quaero/internal/interfaces"
	"github.com/ternarybob/quaero/internal/models"
)

type DataHandler struct {
	jiraScraper       interfaces.JiraScraper
	confluenceScraper interfaces.ConfluenceScraper
	documentStorage   interfaces.DocumentStorage
	logger            arbor.ILogger
}

func NewDataHandler(jira interfaces.JiraScraper, confluence interfaces.ConfluenceScraper, documentStorage interfaces.DocumentStorage) *DataHandler {
	return &DataHandler{
		jiraScraper:       jira,
		confluenceScraper: confluence,
		documentStorage:   documentStorage,
		logger:            common.GetLogger(),
	}
}

// GetJiraDataHandler returns all Jira data (projects and issues)
func (h *DataHandler) GetJiraDataHandler(w http.ResponseWriter, r *http.Request) {
	if !RequireMethod(w, r, "GET") {
		return
	}

	data, err := h.jiraScraper.GetJiraData()
	if err != nil {
		h.logger.Error().Err(err).Msg("Failed to fetch Jira data")
		WriteError(w, http.StatusInternalServerError, "Failed to fetch Jira data")
		return
	}

	WriteJSON(w, http.StatusOK, data)
}

// GetJiraIssuesHandler returns issues optionally filtered by project keys
func (h *DataHandler) GetJiraIssuesHandler(w http.ResponseWriter, r *http.Request) {
	if !RequireMethod(w, r, "GET") {
		return
	}

	// Get optional project keys filter from query params
	projectKeys := r.URL.Query()["projectKey"]

	h.logger.Info().Strs("projectKeys", projectKeys).Msg("GetJiraIssuesHandler called")

	data, err := h.jiraScraper.GetJiraData()
	if err != nil {
		h.logger.Error().Err(err).Msg("Failed to fetch Jira data")
		WriteError(w, http.StatusInternalServerError, "Failed to fetch Jira data")
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
					projectKey := ExtractProjectKey(issueKey)
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
					projectKey := ExtractProjectKey(jiraIssue.Key)
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
						projectKey := ExtractProjectKey(issueKey)
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

	WriteJSON(w, http.StatusOK, map[string]interface{}{
		"issues": issues,
	})
}

// GetConfluenceDataHandler returns all Confluence data (spaces and pages)
func (h *DataHandler) GetConfluenceDataHandler(w http.ResponseWriter, r *http.Request) {
	if !RequireMethod(w, r, "GET") {
		return
	}

	data, err := h.confluenceScraper.GetConfluenceData()
	if err != nil {
		h.logger.Error().Err(err).Msg("Failed to fetch Confluence data")
		WriteError(w, http.StatusInternalServerError, "Failed to fetch Confluence data")
		return
	}

	WriteJSON(w, http.StatusOK, data)
}

// GetConfluencePagesHandler returns pages optionally filtered by space keys
func (h *DataHandler) GetConfluencePagesHandler(w http.ResponseWriter, r *http.Request) {
	if !RequireMethod(w, r, "GET") {
		return
	}

	spaceKeys := r.URL.Query()["spaceKey"]

	h.logger.Info().Strs("spaceKeys", spaceKeys).Msg("GetConfluencePagesHandler called")

	data, err := h.confluenceScraper.GetConfluenceData()
	if err != nil {
		h.logger.Error().Err(err).Msg("Failed to fetch Confluence data")
		WriteError(w, http.StatusInternalServerError, "Failed to fetch Confluence data")
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

	WriteJSON(w, http.StatusOK, map[string]interface{}{
		"pages": pages,
	})
}

// getMapKeys returns all keys from a map
func getMapKeys(m map[string]interface{}) []string {
	return GetMapKeys(m)
}

// ClearAllDataHandler handles DELETE /api/data
func (h *DataHandler) ClearAllDataHandler(w http.ResponseWriter, r *http.Request) {
	if !RequireMethod(w, r, "DELETE") {
		return
	}

	ctx := r.Context()

	// Clear documents
	docs, _ := h.documentStorage.ListDocuments(nil)
	docCount := len(docs)
	for _, doc := range docs {
		h.documentStorage.DeleteDocument(doc.ID)
	}

	// Clear Jira data
	jiraData, _ := h.jiraScraper.GetJiraData()
	jiraProjectCount := 0
	jiraIssueCount := 0
	if projects, ok := jiraData["projects"].([]interface{}); ok {
		jiraProjectCount = len(projects)
	}
	if issues, ok := jiraData["issues"].([]interface{}); ok {
		jiraIssueCount = len(issues)
	}

	// Clear Confluence data
	confluenceData, _ := h.confluenceScraper.GetConfluenceData()
	confluenceSpaceCount := 0
	confluencePageCount := 0
	if spaces, ok := confluenceData["spaces"].([]interface{}); ok {
		confluenceSpaceCount = len(spaces)
	}
	if pages, ok := confluenceData["pages"].([]interface{}); ok {
		confluencePageCount = len(pages)
	}

	h.logger.Info().
		Int("documents", docCount).
		Int("jira_projects", jiraProjectCount).
		Int("jira_issues", jiraIssueCount).
		Int("confluence_spaces", confluenceSpaceCount).
		Int("confluence_pages", confluencePageCount).
		Msg("Cleared all data")

	WriteJSON(w, http.StatusOK, map[string]interface{}{
		"deleted_documents":         docCount,
		"deleted_jira_projects":     jiraProjectCount,
		"deleted_jira_issues":       jiraIssueCount,
		"deleted_confluence_spaces": confluenceSpaceCount,
		"deleted_confluence_pages":  confluencePageCount,
	})
	_ = ctx
}

// ClearDataBySourceHandler handles DELETE /api/data/{sourceType}
func (h *DataHandler) ClearDataBySourceHandler(w http.ResponseWriter, r *http.Request) {
	if !RequireMethod(w, r, "DELETE") {
		return
	}

	// Extract source type from URL path
	sourceType := extractIDFromPath(r.URL.Path, "/api/data/")
	if sourceType == "" {
		WriteError(w, http.StatusBadRequest, "Source type is required")
		return
	}

	ctx := r.Context()

	// Validate source type
	validTypes := map[string]bool{
		"jira":       true,
		"confluence": true,
		"github":     true,
	}
	if !validTypes[sourceType] {
		WriteError(w, http.StatusBadRequest, "Invalid source type")
		return
	}

	// Clear documents by source type
	docs, _ := h.documentStorage.GetDocumentsBySource(sourceType)
	docCount := len(docs)
	for _, doc := range docs {
		h.documentStorage.DeleteDocument(doc.ID)
	}

	result := map[string]interface{}{
		"deleted_documents": docCount,
		"source_type":       sourceType,
	}

	// Clear source-specific data
	switch sourceType {
	case "jira":
		jiraData, _ := h.jiraScraper.GetJiraData()
		projectCount := 0
		issueCount := 0
		if projects, ok := jiraData["projects"].([]interface{}); ok {
			projectCount = len(projects)
		}
		if issues, ok := jiraData["issues"].([]interface{}); ok {
			issueCount = len(issues)
		}
		result["deleted_projects"] = projectCount
		result["deleted_issues"] = issueCount

	case "confluence":
		confluenceData, _ := h.confluenceScraper.GetConfluenceData()
		spaceCount := 0
		pageCount := 0
		if spaces, ok := confluenceData["spaces"].([]interface{}); ok {
			spaceCount = len(spaces)
		}
		if pages, ok := confluenceData["pages"].([]interface{}); ok {
			pageCount = len(pages)
		}
		result["deleted_spaces"] = spaceCount
		result["deleted_pages"] = pageCount
	}

	h.logger.Info().
		Str("source_type", sourceType).
		Int("documents", docCount).
		Msg("Cleared data by source type")

	WriteJSON(w, http.StatusOK, result)
	_ = ctx
}
