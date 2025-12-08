// -----------------------------------------------------------------------
// BuildDependencyGraphAction - Aggregates include/link relationships
// into a queryable dependency graph stored in KV storage
// -----------------------------------------------------------------------

package actions

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/ternarybob/arbor"
	"github.com/ternarybob/quaero/internal/interfaces"
	"github.com/ternarybob/quaero/internal/models"
)

// DependencyGraph represents the complete dependency graph for the codebase
type DependencyGraph struct {
	Nodes      []GraphNode        `json:"nodes"`
	Edges      []GraphEdge        `json:"edges"`
	Components []ComponentSummary `json:"components"`
	Generated  time.Time          `json:"generated"`
}

// GraphNode represents a file in the dependency graph
type GraphNode struct {
	ID        string   `json:"id"`        // file path
	Name      string   `json:"name"`      // basename
	Component string   `json:"component"` // from classification
	FileRole  string   `json:"file_role"`
	Platforms []string `json:"platforms"`
}

// GraphEdge represents a dependency relationship between files
type GraphEdge struct {
	Source string `json:"source"` // source file path
	Target string `json:"target"` // target file path
	Type   string `json:"type"`   // includes, links, tests, builds
}

// ComponentSummary represents aggregate statistics for a component
type ComponentSummary struct {
	Name      string   `json:"name"`
	FileCount int      `json:"file_count"`
	Platforms []string `json:"platforms"`
	HasTests  bool     `json:"has_tests"`
}

// BuildDependencyGraphAction builds a dependency graph from DevOps metadata
type BuildDependencyGraphAction struct {
	documentStorage interfaces.DocumentStorage
	kvStorage       interfaces.KeyValueStorage
	searchService   interfaces.SearchService
	logger          arbor.ILogger
}

const dependencyGraphKey = "devops:dependency_graph"

// NewBuildDependencyGraphAction creates a new BuildDependencyGraphAction
func NewBuildDependencyGraphAction(
	documentStorage interfaces.DocumentStorage,
	kvStorage interfaces.KeyValueStorage,
	searchService interfaces.SearchService,
	logger arbor.ILogger,
) *BuildDependencyGraphAction {
	return &BuildDependencyGraphAction{
		documentStorage: documentStorage,
		kvStorage:       kvStorage,
		searchService:   searchService,
		logger:          logger,
	}
}

// Execute builds the dependency graph and stores it in KV storage
func (a *BuildDependencyGraphAction) Execute(ctx context.Context, documents []*models.Document) error {
	a.logger.Info().
		Int("document_count", len(documents)).
		Msg("Building dependency graph from documents")

	// Build the graph from documents
	graph, err := a.BuildGraph(documents)
	if err != nil {
		a.logger.Error().Err(err).Msg("Failed to build dependency graph")
		return fmt.Errorf("failed to build graph: %w", err)
	}

	a.logger.Info().
		Int("nodes", len(graph.Nodes)).
		Int("edges", len(graph.Edges)).
		Int("components", len(graph.Components)).
		Msg("Dependency graph built successfully")

	// Serialize graph to JSON
	graphJSON, err := json.Marshal(graph)
	if err != nil {
		a.logger.Error().Err(err).Msg("Failed to serialize graph to JSON")
		return fmt.Errorf("failed to serialize graph: %w", err)
	}

	// Store in KV storage
	err = a.kvStorage.Set(ctx, dependencyGraphKey, string(graphJSON), "DevOps dependency graph aggregating file relationships")
	if err != nil {
		a.logger.Error().Err(err).Msg("Failed to store graph in KV storage")
		return fmt.Errorf("failed to store graph: %w", err)
	}

	a.logger.Info().
		Str("key", dependencyGraphKey).
		Int("size_bytes", len(graphJSON)).
		Msg("Dependency graph stored in KV storage")

	return nil
}

// BuildGraph constructs the dependency graph from documents
func (a *BuildDependencyGraphAction) BuildGraph(docs []*models.Document) (*DependencyGraph, error) {
	graph := &DependencyGraph{
		Nodes:     make([]GraphNode, 0),
		Edges:     make([]GraphEdge, 0),
		Generated: time.Now(),
	}

	// Map for quick node lookup and deduplication
	nodeMap := make(map[string]*GraphNode)

	for _, doc := range docs {
		// Extract devops metadata
		devops := a.GetDevOpsMetadata(doc)
		if devops == nil {
			a.logger.Debug().
				Str("doc_id", doc.ID).
				Str("title", doc.Title).
				Msg("Skipping document without DevOps metadata")
			continue
		}

		// Use URL or source path as node ID
		nodeID := doc.URL
		if nodeID == "" {
			nodeID = doc.ID
		}

		// Create node
		node := GraphNode{
			ID:        nodeID,
			Name:      filepath.Base(nodeID),
			Component: devops.Component,
			FileRole:  devops.FileRole,
			Platforms: devops.Platforms,
		}

		normalizedID := a.NormalizePath(nodeID)
		nodeMap[normalizedID] = &node
		graph.Nodes = append(graph.Nodes, node)

		// Create edges from local includes
		for _, inc := range devops.LocalIncludes {
			resolvedPath := a.ResolvePath(nodeID, inc)
			edge := GraphEdge{
				Source: nodeID,
				Target: resolvedPath,
				Type:   "includes",
			}
			graph.Edges = append(graph.Edges, edge)
		}

		// Create edges from build dependencies
		for _, dep := range devops.BuildDeps {
			edge := GraphEdge{
				Source: nodeID,
				Target: dep,
				Type:   "builds",
			}
			graph.Edges = append(graph.Edges, edge)
		}

		// Create edges from linked libraries
		for _, lib := range devops.LinkedLibraries {
			edge := GraphEdge{
				Source: nodeID,
				Target: lib,
				Type:   "links",
			}
			graph.Edges = append(graph.Edges, edge)
		}

		// Create edges from test requirements
		for _, req := range devops.TestRequires {
			edge := GraphEdge{
				Source: nodeID,
				Target: req,
				Type:   "tests",
			}
			graph.Edges = append(graph.Edges, edge)
		}
	}

	// Compute component summaries
	graph.Components = a.ComputeComponentSummaries(graph.Nodes)

	a.logger.Info().
		Int("total_nodes", len(graph.Nodes)).
		Int("total_edges", len(graph.Edges)).
		Int("components", len(graph.Components)).
		Msg("Graph construction complete")

	return graph, nil
}

// GetDevOpsMetadata extracts DevOpsMetadata from a document
func (a *BuildDependencyGraphAction) GetDevOpsMetadata(doc *models.Document) *models.DevOpsMetadata {
	if doc.Metadata == nil {
		return nil
	}

	devopsData, ok := doc.Metadata["devops"]
	if !ok {
		return nil
	}

	// Marshal to JSON and unmarshal to DevOpsMetadata
	jsonData, err := json.Marshal(devopsData)
	if err != nil {
		a.logger.Warn().
			Err(err).
			Str("doc_id", doc.ID).
			Msg("Failed to marshal devops metadata")
		return nil
	}

	var devops models.DevOpsMetadata
	err = json.Unmarshal(jsonData, &devops)
	if err != nil {
		a.logger.Warn().
			Err(err).
			Str("doc_id", doc.ID).
			Msg("Failed to unmarshal devops metadata")
		return nil
	}

	return &devops
}

// NormalizePath normalizes a file path for consistent matching
func (a *BuildDependencyGraphAction) NormalizePath(path string) string {
	// Clean the path to resolve . and .. elements
	cleaned := filepath.Clean(path)
	// Convert to lowercase for case-insensitive matching
	normalized := strings.ToLower(cleaned)
	// Convert backslashes to forward slashes for cross-platform consistency
	normalized = strings.ReplaceAll(normalized, "\\", "/")
	return normalized
}

// ResolvePath resolves an include path relative to a base file
func (a *BuildDependencyGraphAction) ResolvePath(basePath, includePath string) string {
	// If include path is absolute, return it as-is
	if filepath.IsAbs(includePath) {
		return includePath
	}

	// Get directory of base file
	dir := filepath.Dir(basePath)

	// Join paths and clean
	resolved := filepath.Join(dir, includePath)
	return filepath.Clean(resolved)
}

// ComputeComponentSummaries aggregates statistics by component
func (a *BuildDependencyGraphAction) ComputeComponentSummaries(nodes []GraphNode) []ComponentSummary {
	// Group nodes by component
	componentMap := make(map[string]*ComponentSummary)

	for _, node := range nodes {
		component := node.Component
		if component == "" {
			component = "unknown"
		}

		summary, exists := componentMap[component]
		if !exists {
			summary = &ComponentSummary{
				Name:      component,
				FileCount: 0,
				Platforms: make([]string, 0),
				HasTests:  false,
			}
			componentMap[component] = summary
		}

		// Increment file count
		summary.FileCount++

		// Aggregate platforms (unique)
		platformSet := make(map[string]bool)
		for _, p := range summary.Platforms {
			platformSet[p] = true
		}
		for _, p := range node.Platforms {
			if !platformSet[p] {
				platformSet[p] = true
				summary.Platforms = append(summary.Platforms, p)
			}
		}

		// Check for test files
		if node.FileRole == "test" || strings.Contains(strings.ToLower(node.FileRole), "test") {
			summary.HasTests = true
		}
	}

	// Convert map to slice
	summaries := make([]ComponentSummary, 0, len(componentMap))
	for _, summary := range componentMap {
		summaries = append(summaries, *summary)
	}

	return summaries
}
