// -----------------------------------------------------------------------
// Last Modified: Saturday, 7th December 2025 12:00:00 pm
// Modified By: Claude Code
// -----------------------------------------------------------------------

package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"

	"github.com/ternarybob/arbor"
	"github.com/ternarybob/quaero/internal/interfaces"
)

// DevOpsHandler handles DevOps enrichment API endpoints
type DevOpsHandler struct {
	kvStorage       interfaces.KeyValueStorage
	documentStorage interfaces.DocumentStorage
	searchService   interfaces.SearchService
	jobManager      interfaces.JobManager
	logger          arbor.ILogger
}

// NewDevOpsHandler creates a new DevOps handler
func NewDevOpsHandler(
	kvStorage interfaces.KeyValueStorage,
	documentStorage interfaces.DocumentStorage,
	searchService interfaces.SearchService,
	jobManager interfaces.JobManager,
	logger arbor.ILogger,
) *DevOpsHandler {
	return &DevOpsHandler{
		kvStorage:       kvStorage,
		documentStorage: documentStorage,
		searchService:   searchService,
		jobManager:      jobManager,
		logger:          logger,
	}
}

// SummaryHandler returns the generated DevOps guide (markdown)
// GET /api/devops/summary
func (h *DevOpsHandler) SummaryHandler(w http.ResponseWriter, r *http.Request) {
	if !RequireMethod(w, r, http.MethodGet) {
		return
	}

	ctx := r.Context()

	// Try to get summary from KV storage
	summary, err := h.kvStorage.Get(ctx, "devops:summary")
	if err != nil || summary == "" {
		h.logger.Debug().Msg("DevOps summary not found in KV storage, checking document storage")

		// Fallback: try to get from document storage (document ID: devops-summary)
		doc, err := h.documentStorage.GetDocument("devops-summary")
		if err != nil || doc == nil {
			h.logger.Debug().Msg("DevOps summary not found in document storage")
			http.Error(w, "DevOps summary not generated yet", http.StatusNotFound)
			return
		}

		summary = doc.ContentMarkdown
	}

	WriteJSON(w, http.StatusOK, map[string]interface{}{
		"summary": summary,
	})
}

// ComponentsHandler lists components with stats
// GET /api/devops/components
func (h *DevOpsHandler) ComponentsHandler(w http.ResponseWriter, r *http.Request) {
	if !RequireMethod(w, r, http.MethodGet) {
		return
	}

	ctx := r.Context()

	// Try to parse from dependency graph first
	graphJSON, err := h.kvStorage.Get(ctx, "devops:dependency_graph")
	if err == nil && graphJSON != "" {
		// Parse graph and extract components
		var graph map[string]interface{}
		if err := json.Unmarshal([]byte(graphJSON), &graph); err == nil {
			components := h.extractComponentsFromGraph(graph)
			WriteJSON(w, http.StatusOK, map[string]interface{}{
				"components": components,
				"source":     "graph",
			})
			return
		}
	}

	// Fallback: query documents with devops tags
	components := h.extractComponentsFromDocuments(ctx)
	WriteJSON(w, http.StatusOK, map[string]interface{}{
		"components": components,
		"source":     "documents",
	})
}

// GraphHandler returns the dependency graph as JSON
// GET /api/devops/graph
func (h *DevOpsHandler) GraphHandler(w http.ResponseWriter, r *http.Request) {
	if !RequireMethod(w, r, http.MethodGet) {
		return
	}

	ctx := r.Context()

	graphJSON, err := h.kvStorage.Get(ctx, "devops:dependency_graph")
	if err != nil || graphJSON == "" {
		h.logger.Debug().Msg("DevOps dependency graph not found")
		http.Error(w, "Dependency graph not generated yet", http.StatusNotFound)
		return
	}

	// Parse and return as JSON
	var graph map[string]interface{}
	if err := json.Unmarshal([]byte(graphJSON), &graph); err != nil {
		h.logger.Error().Err(err).Msg("Failed to parse dependency graph")
		http.Error(w, "Failed to parse dependency graph", http.StatusInternalServerError)
		return
	}

	WriteJSON(w, http.StatusOK, graph)
}

// PlatformsHandler returns the platform matrix
// GET /api/devops/platforms
func (h *DevOpsHandler) PlatformsHandler(w http.ResponseWriter, r *http.Request) {
	if !RequireMethod(w, r, http.MethodGet) {
		return
	}

	ctx := r.Context()

	// Try to extract from dependency graph first
	graphJSON, err := h.kvStorage.Get(ctx, "devops:dependency_graph")
	if err == nil && graphJSON != "" {
		var graph map[string]interface{}
		if err := json.Unmarshal([]byte(graphJSON), &graph); err == nil {
			platforms := h.extractPlatformsFromGraph(graph)
			WriteJSON(w, http.StatusOK, map[string]interface{}{
				"platforms": platforms,
				"source":    "graph",
			})
			return
		}
	}

	// Fallback: aggregate from enriched documents
	platforms := h.extractPlatformsFromDocuments(ctx)
	WriteJSON(w, http.StatusOK, map[string]interface{}{
		"platforms": platforms,
		"source":    "documents",
	})
}

// EnrichHandler triggers the DevOps enrichment pipeline
// POST /api/devops/enrich
func (h *DevOpsHandler) EnrichHandler(w http.ResponseWriter, r *http.Request) {
	if !RequireMethod(w, r, http.MethodPost) {
		return
	}

	ctx := r.Context()

	// Create a job from the devops_enrich job definition
	// The job definition should exist and be registered in the system
	jobID, err := h.jobManager.CreateJob(ctx, "devops_enrich", nil)
	if err != nil {
		h.logger.Error().Err(err).Msg("Failed to create DevOps enrichment job")
		http.Error(w, "Failed to trigger enrichment pipeline: "+err.Error(), http.StatusInternalServerError)
		return
	}

	h.logger.Info().Str("job_id", jobID).Msg("DevOps enrichment pipeline job created")

	WriteJSON(w, http.StatusCreated, map[string]interface{}{
		"job_id":  jobID,
		"message": "DevOps enrichment pipeline started",
	})
}

// Helper methods

// extractComponentsFromGraph extracts component stats from dependency graph
func (h *DevOpsHandler) extractComponentsFromGraph(graph map[string]interface{}) []map[string]interface{} {
	components := []map[string]interface{}{}

	// Try to extract components from graph structure
	// Graph structure may vary, so we'll try common patterns
	if nodes, ok := graph["nodes"].([]interface{}); ok {
		componentMap := make(map[string]int)
		for _, node := range nodes {
			if nodeMap, ok := node.(map[string]interface{}); ok {
				if component, ok := nodeMap["component"].(string); ok {
					componentMap[component]++
				} else if name, ok := nodeMap["name"].(string); ok {
					// Use first segment of name as component
					parts := strings.Split(name, "/")
					if len(parts) > 0 {
						componentMap[parts[0]]++
					}
				}
			}
		}

		for name, count := range componentMap {
			components = append(components, map[string]interface{}{
				"name":       name,
				"file_count": count,
			})
		}
	}

	return components
}

// extractComponentsFromDocuments extracts component stats from documents
func (h *DevOpsHandler) extractComponentsFromDocuments(ctx context.Context) []map[string]interface{} {
	components := []map[string]interface{}{}

	// Query documents with devops-related tags
	opts := &interfaces.ListOptions{
		Tags:  []string{"devops", "devops-enriched"},
		Limit: 1000,
	}

	docs, err := h.documentStorage.ListDocuments(opts)
	if err != nil {
		h.logger.Error().Err(err).Msg("Failed to list documents")
		return components
	}

	// Group by component (extracted from metadata or title)
	componentMap := make(map[string]int)
	for _, doc := range docs {
		component := "unknown"

		// Try to extract component from metadata
		if doc.Metadata != nil {
			if comp, ok := doc.Metadata["component"].(string); ok {
				component = comp
			} else if comp, ok := doc.Metadata["service"].(string); ok {
				component = comp
			}
		}

		// Fallback: extract from title or URL
		if component == "unknown" {
			if doc.Title != "" {
				parts := strings.Split(doc.Title, "/")
				if len(parts) > 0 {
					component = parts[0]
				}
			}
		}

		componentMap[component]++
	}

	for name, count := range componentMap {
		components = append(components, map[string]interface{}{
			"name":       name,
			"file_count": count,
		})
	}

	return components
}

// extractPlatformsFromGraph extracts platform matrix from dependency graph
func (h *DevOpsHandler) extractPlatformsFromGraph(graph map[string]interface{}) map[string]int {
	platforms := make(map[string]int)

	// Try to extract platforms from graph metadata
	if metadata, ok := graph["metadata"].(map[string]interface{}); ok {
		if platformData, ok := metadata["platforms"].(map[string]interface{}); ok {
			for platform, count := range platformData {
				if countNum, ok := count.(float64); ok {
					platforms[platform] = int(countNum)
				}
			}
		}
	}

	// If not in metadata, scan nodes for platform information
	if len(platforms) == 0 {
		if nodes, ok := graph["nodes"].([]interface{}); ok {
			for _, node := range nodes {
				if nodeMap, ok := node.(map[string]interface{}); ok {
					if platform, ok := nodeMap["platform"].(string); ok {
						platforms[platform]++
					}
				}
			}
		}
	}

	return platforms
}

// extractPlatformsFromDocuments extracts platform matrix from documents
func (h *DevOpsHandler) extractPlatformsFromDocuments(ctx context.Context) map[string]int {
	platforms := make(map[string]int)

	// Query documents with devops-related tags
	opts := &interfaces.ListOptions{
		Tags:  []string{"devops", "devops-enriched"},
		Limit: 1000,
	}

	docs, err := h.documentStorage.ListDocuments(opts)
	if err != nil {
		h.logger.Error().Err(err).Msg("Failed to list documents")
		return platforms
	}

	// Extract platform information from metadata
	for _, doc := range docs {
		if doc.Metadata != nil {
			if platform, ok := doc.Metadata["platform"].(string); ok {
				platforms[platform]++
			} else if platformList, ok := doc.Metadata["platforms"].([]interface{}); ok {
				for _, p := range platformList {
					if pStr, ok := p.(string); ok {
						platforms[pStr]++
					}
				}
			}
		}
	}

	return platforms
}
