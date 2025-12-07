// -----------------------------------------------------------------------
// AggregateDevOpsSummaryAction - LLM synthesis of comprehensive DevOps guide
// -----------------------------------------------------------------------

package actions

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/ternarybob/arbor"
	"github.com/ternarybob/quaero/internal/interfaces"
	"github.com/ternarybob/quaero/internal/models"
)

const summaryPromptTemplate = `You are creating a DevOps guide for a C/C++ codebase.
The target audience is a DevOps engineer who needs to create CI/CD pipelines but is NOT a C/C++ programmer.

PROJECT ANALYSIS:
================

Build System:
%s

Platforms Detected: %v

Components:
%s

External Dependencies:
%v

Test Information:
%s

Generate a comprehensive markdown guide covering:

## 1. Build System Overview
- What build system(s) are used?
- Key build targets and what they produce
- Required toolchain and versions

## 2. Toolchain Requirements
- Compilers needed (with versions if detectable)
- Build tools (make, cmake, msbuild)
- Required libraries and how to install them

## 3. Component Architecture
- High-level component breakdown
- Dependencies between components
- Entry points

## 4. Test Strategy
- Types of tests present
- Test frameworks used
- How to run tests
- External requirements for testing

## 5. Platform Matrix
- Which platforms are supported
- Platform-specific considerations
- Cross-compilation notes if applicable

## 6. CI/CD Recommendations
- Suggested pipeline stages
- Build caching opportunities
- Parallelization possibilities
- Artifact management

## 7. Common Issues & Troubleshooting
- Likely build problems and solutions
- Configuration pitfalls
- Platform-specific gotchas

Be specific and actionable. Include example commands where possible.`

const devopsSummaryKey = "devops:summary"
const devopsSummaryDocID = "devops-summary"

// AggregateDevOpsSummaryAction aggregates DevOps metadata and generates a comprehensive guide
type AggregateDevOpsSummaryAction struct {
	documentStorage interfaces.DocumentStorage
	kvStorage       interfaces.KeyValueStorage
	searchService   interfaces.SearchService
	llmService      interfaces.LLMService
	logger          arbor.ILogger
}

// NewAggregateDevOpsSummaryAction creates a new aggregate summary action
func NewAggregateDevOpsSummaryAction(
	documentStorage interfaces.DocumentStorage,
	kvStorage interfaces.KeyValueStorage,
	searchService interfaces.SearchService,
	llmService interfaces.LLMService,
	logger arbor.ILogger,
) *AggregateDevOpsSummaryAction {
	return &AggregateDevOpsSummaryAction{
		documentStorage: documentStorage,
		kvStorage:       kvStorage,
		searchService:   searchService,
		llmService:      llmService,
		logger:          logger,
	}
}

// AggregatedData contains all aggregated DevOps metadata
type AggregatedData struct {
	BuildInfo    BuildInfo       `json:"build_info"`
	Platforms    []string        `json:"platforms"`
	Components   []ComponentInfo `json:"components"`
	Dependencies []string        `json:"dependencies"`
	TestInfo     TestInfo        `json:"test_info"`
}

// BuildInfo contains aggregated build system information
type BuildInfo struct {
	Systems   []string `json:"systems"`   // cmake, make, msbuild
	Targets   []string `json:"targets"`   // build targets
	Toolchain string   `json:"toolchain"` // compiler toolchain info
}

// ComponentInfo represents a component in the codebase
type ComponentInfo struct {
	Name      string `json:"name"`
	FileCount int    `json:"file_count"`
	Role      string `json:"role"`
}

// TestInfo contains aggregated test information
type TestInfo struct {
	Frameworks []string `json:"frameworks"`
	Types      []string `json:"types"`    // unit, integration, etc.
	Requires   []string `json:"requires"` // external requirements
}

// Execute performs the aggregation and LLM synthesis
func (a *AggregateDevOpsSummaryAction) Execute(ctx context.Context) error {
	a.logger.Info().Msg("Starting DevOps summary aggregation")

	// 1. Query all enriched documents
	docs, err := a.queryEnrichedDocuments(ctx)
	if err != nil {
		return fmt.Errorf("failed to query enriched documents: %w", err)
	}

	a.logger.Info().
		Int("document_count", len(docs)).
		Msg("Queried enriched documents")

	// 2. Aggregate data from documents
	aggregated, err := a.AggregateFromDocuments(docs)
	if err != nil {
		return fmt.Errorf("failed to aggregate data: %w", err)
	}

	// 3. Format data for LLM prompt
	formattedData := a.FormatForPrompt(aggregated)

	// 4. Call LLM to generate markdown guide
	a.logger.Info().Msg("Calling LLM to generate DevOps guide")
	markdown, err := a.generateSummaryWithLLM(ctx, formattedData)
	if err != nil {
		return fmt.Errorf("failed to generate summary with LLM: %w", err)
	}

	a.logger.Info().
		Int("markdown_length", len(markdown)).
		Msg("Generated DevOps guide")

	// 5. Store summary in KV under "devops:summary"
	err = a.kvStorage.Set(ctx, devopsSummaryKey, markdown, "DevOps summary guide for C/C++ codebase")
	if err != nil {
		return fmt.Errorf("failed to store summary in KV: %w", err)
	}

	a.logger.Info().
		Str("key", devopsSummaryKey).
		Msg("Stored summary in KV storage")

	// 6. Create searchable document with ID "devops-summary"
	err = a.CreateSummaryDocument(ctx, markdown)
	if err != nil {
		return fmt.Errorf("failed to create summary document: %w", err)
	}

	a.logger.Info().
		Str("document_id", devopsSummaryDocID).
		Msg("Created searchable summary document")

	// 7. Track that aggregation was completed
	err = a.kvStorage.Set(ctx, "devops:enrichment:aggregate_completed", time.Now().Format(time.RFC3339), "Timestamp when aggregate_devops_summary completed")
	if err != nil {
		a.logger.Warn().
			Err(err).
			Msg("Failed to store aggregation completion timestamp")
		// Don't fail on tracking error
	}

	return nil
}

// queryEnrichedDocuments retrieves all documents with DevOps enrichment
func (a *AggregateDevOpsSummaryAction) queryEnrichedDocuments(ctx context.Context) ([]*models.Document, error) {
	// Query documents with devops-enriched tag
	opts := interfaces.SearchOptions{
		Tags:  []string{"devops-enriched"},
		Limit: 10000,
	}

	docs, err := a.searchService.Search(ctx, "", opts)
	if err != nil {
		return nil, fmt.Errorf("failed to search documents: %w", err)
	}

	return docs, nil
}

// AggregateFromDocuments aggregates DevOps metadata from documents
func (a *AggregateDevOpsSummaryAction) AggregateFromDocuments(docs []*models.Document) (*AggregatedData, error) {
	data := &AggregatedData{
		Platforms:    make([]string, 0),
		Components:   make([]ComponentInfo, 0),
		Dependencies: make([]string, 0),
	}

	// Use sets to deduplicate
	platformSet := make(map[string]bool)
	componentMap := make(map[string]int) // name -> file count
	componentRoles := make(map[string]string)
	frameworkSet := make(map[string]bool)
	typeSet := make(map[string]bool)
	requiresSet := make(map[string]bool)
	depsSet := make(map[string]bool)
	targetSet := make(map[string]bool)
	systemSet := make(map[string]bool)

	for _, doc := range docs {
		devops := a.GetDevOpsMetadata(doc)
		if devops == nil {
			continue
		}

		// Aggregate platforms
		for _, p := range devops.Platforms {
			if p != "" {
				platformSet[p] = true
			}
		}

		// Aggregate components
		if devops.Component != "" {
			componentMap[devops.Component]++
			if devops.FileRole != "" {
				// Keep the first role we see for each component
				if _, exists := componentRoles[devops.Component]; !exists {
					componentRoles[devops.Component] = devops.FileRole
				}
			}
		}

		// Aggregate build targets
		for _, t := range devops.BuildTargets {
			if t != "" {
				targetSet[t] = true
			}
		}

		// Infer build systems from file paths or metadata
		if doc.SourceType == "local_dir" {
			if localMeta, ok := doc.Metadata["local_dir"].(map[string]interface{}); ok {
				if filePath, ok := localMeta["file_path"].(string); ok {
					if strings.Contains(filePath, "CMakeLists.txt") {
						systemSet["cmake"] = true
					} else if strings.Contains(filePath, "Makefile") || strings.Contains(filePath, "makefile") {
						systemSet["make"] = true
					} else if strings.Contains(filePath, ".vcxproj") || strings.Contains(filePath, ".sln") {
						systemSet["msbuild"] = true
					}
				}
			}
		}

		// Aggregate test info
		if devops.TestFramework != "" && devops.TestFramework != "none" {
			frameworkSet[devops.TestFramework] = true
		}
		if devops.TestType != "" && devops.TestType != "none" {
			typeSet[devops.TestType] = true
		}
		for _, r := range devops.TestRequires {
			if r != "" {
				requiresSet[r] = true
			}
		}

		// Aggregate external deps
		for _, d := range devops.ExternalDeps {
			if d != "" {
				depsSet[d] = true
			}
		}
	}

	// Convert sets to slices
	for p := range platformSet {
		data.Platforms = append(data.Platforms, p)
	}

	for name, count := range componentMap {
		role := componentRoles[name]
		if role == "" {
			role = "unknown"
		}
		data.Components = append(data.Components, ComponentInfo{
			Name:      name,
			FileCount: count,
			Role:      role,
		})
	}

	for d := range depsSet {
		data.Dependencies = append(data.Dependencies, d)
	}

	// Build info
	systems := make([]string, 0, len(systemSet))
	for s := range systemSet {
		systems = append(systems, s)
	}

	targets := make([]string, 0, len(targetSet))
	for t := range targetSet {
		targets = append(targets, t)
	}

	data.BuildInfo = BuildInfo{
		Systems:   systems,
		Targets:   targets,
		Toolchain: "C/C++ compiler (gcc/clang/msvc)",
	}

	// Test info
	frameworks := make([]string, 0, len(frameworkSet))
	for f := range frameworkSet {
		frameworks = append(frameworks, f)
	}

	types := make([]string, 0, len(typeSet))
	for t := range typeSet {
		types = append(types, t)
	}

	requires := make([]string, 0, len(requiresSet))
	for r := range requiresSet {
		requires = append(requires, r)
	}

	data.TestInfo = TestInfo{
		Frameworks: frameworks,
		Types:      types,
		Requires:   requires,
	}

	return data, nil
}

// GetDevOpsMetadata extracts DevOpsMetadata from a document
func (a *AggregateDevOpsSummaryAction) GetDevOpsMetadata(doc *models.Document) *models.DevOpsMetadata {
	if doc.Metadata == nil {
		return nil
	}

	devopsData, ok := doc.Metadata["devops"]
	if !ok {
		return nil
	}

	// Convert to JSON and back to properly unmarshal
	jsonData, err := json.Marshal(devopsData)
	if err != nil {
		a.logger.Warn().
			Err(err).
			Str("document_id", doc.ID).
			Msg("Failed to marshal devops metadata")
		return nil
	}

	var devops models.DevOpsMetadata
	if err := json.Unmarshal(jsonData, &devops); err != nil {
		a.logger.Warn().
			Err(err).
			Str("document_id", doc.ID).
			Msg("Failed to unmarshal devops metadata")
		return nil
	}

	return &devops
}

// FormatForPrompt formats aggregated data for the LLM prompt
func (a *AggregateDevOpsSummaryAction) FormatForPrompt(data *AggregatedData) string {
	var sb strings.Builder

	// Build system
	buildInfo := "Build Systems: "
	if len(data.BuildInfo.Systems) > 0 {
		buildInfo += strings.Join(data.BuildInfo.Systems, ", ")
	} else {
		buildInfo += "Not detected (may require manual inspection)"
	}
	buildInfo += "\n"

	if len(data.BuildInfo.Targets) > 0 {
		buildInfo += fmt.Sprintf("Build Targets: %s\n", strings.Join(data.BuildInfo.Targets, ", "))
	}

	buildInfo += fmt.Sprintf("Toolchain: %s\n", data.BuildInfo.Toolchain)

	// Platforms
	platforms := data.Platforms
	if len(platforms) == 0 {
		platforms = []string{"Platform information not available - please check source files"}
	}

	// Components
	components := ""
	if len(data.Components) > 0 {
		for _, c := range data.Components {
			components += fmt.Sprintf("- %s (%d files, role: %s)\n", c.Name, c.FileCount, c.Role)
		}
	} else {
		components = "Component information not available - files may need classification\n"
	}

	// Dependencies
	deps := data.Dependencies
	if len(deps) == 0 {
		deps = []string{"No external dependencies detected - may require source inspection"}
	}

	// Test info
	testInfo := ""
	if len(data.TestInfo.Frameworks) > 0 {
		testInfo += fmt.Sprintf("Test Frameworks: %s\n", strings.Join(data.TestInfo.Frameworks, ", "))
	} else {
		testInfo += "Test Frameworks: Not detected\n"
	}

	if len(data.TestInfo.Types) > 0 {
		testInfo += fmt.Sprintf("Test Types: %s\n", strings.Join(data.TestInfo.Types, ", "))
	}

	if len(data.TestInfo.Requires) > 0 {
		testInfo += fmt.Sprintf("Test Requirements: %s\n", strings.Join(data.TestInfo.Requires, ", "))
	}

	sb.WriteString(fmt.Sprintf(summaryPromptTemplate, buildInfo, platforms, components, deps, testInfo))
	return sb.String()
}

// generateSummaryWithLLM calls the LLM service to generate the summary
func (a *AggregateDevOpsSummaryAction) generateSummaryWithLLM(ctx context.Context, prompt string) (string, error) {
	messages := []interfaces.Message{
		{
			Role:    "system",
			Content: "You are a DevOps expert specializing in C/C++ build systems and CI/CD pipelines. Generate clear, actionable documentation.",
		},
		{
			Role:    "user",
			Content: prompt,
		},
	}

	response, err := a.llmService.Chat(ctx, messages)
	if err != nil {
		return "", fmt.Errorf("LLM chat failed: %w", err)
	}

	if strings.TrimSpace(response) == "" {
		return a.generateMinimalSummary(), nil
	}

	return response, nil
}

// generateMinimalSummary creates a basic summary when data is sparse
func (a *AggregateDevOpsSummaryAction) generateMinimalSummary() string {
	return `# DevOps Guide - C/C++ Codebase

## Overview

This is an automatically generated DevOps guide based on codebase analysis.

## Build System

The build system configuration was not fully detected. Please review the codebase for:
- CMakeLists.txt files (CMake)
- Makefiles (Make)
- .sln or .vcxproj files (Visual Studio/MSBuild)

## Recommended Actions

1. Identify the primary build system
2. Document build targets and dependencies
3. Set up CI/CD pipeline for automated builds
4. Configure test execution
5. Establish artifact management

## Next Steps

- Run a deeper analysis with additional enrichment passes
- Manually inspect build configuration files
- Consult with development team for build requirements
`
}

// CreateSummaryDocument creates a searchable document for the summary
func (a *AggregateDevOpsSummaryAction) CreateSummaryDocument(ctx context.Context, markdown string) error {
	doc := &models.Document{
		ID:              devopsSummaryDocID,
		SourceType:      "system",
		SourceID:        devopsSummaryDocID,
		Title:           "DevOps Summary - C/C++ Codebase Analysis",
		ContentMarkdown: markdown,
		Tags:            []string{"devops", "summary", "generated"},
		Metadata: map[string]interface{}{
			"generated_at": time.Now().Format(time.RFC3339),
			"type":         "devops_summary",
			"devops": map[string]interface{}{
				"enrichment_passes": []string{"aggregate_devops_summary"},
			},
		},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	return a.documentStorage.SaveDocument(doc)
}
