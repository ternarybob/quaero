// -----------------------------------------------------------------------
// ClassifyDevOps Action - LLM-based classification for DevOps enrichment
// Uses LLM to classify file roles, components, and external dependencies
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

// ClassifyDevOpsAction performs LLM-based classification of C/C++ files
// for DevOps pipeline understanding
type ClassifyDevOpsAction struct {
	documentStorage interfaces.DocumentStorage
	llmService      interfaces.LLMService
	logger          arbor.ILogger
}

// NewClassifyDevOpsAction creates a new ClassifyDevOps action
func NewClassifyDevOpsAction(
	documentStorage interfaces.DocumentStorage,
	llmService interfaces.LLMService,
	logger arbor.ILogger,
) *ClassifyDevOpsAction {
	return &ClassifyDevOpsAction{
		documentStorage: documentStorage,
		llmService:      llmService,
		logger:          logger,
	}
}

// ClassificationResult represents the LLM's classification response
type ClassificationResult struct {
	FileRole      string   `json:"file_role"`
	Component     string   `json:"component"`
	TestType      string   `json:"test_type"`
	TestFramework string   `json:"test_framework,omitempty"`
	TestRequires  []string `json:"test_requires,omitempty"`
	ExternalDeps  []string `json:"external_deps,omitempty"`
	ConfigSources []string `json:"config_sources,omitempty"`
}

const classifyPromptTemplate = `You are helping a DevOps engineer understand a C/C++ codebase for CI/CD pipeline creation.
The engineer is NOT a C/C++ programmer and needs high-level understanding.

File: %s
Language: %s

Extracted Structure (from automated analysis):
- Includes: %v
- Defines: %v
- Platforms: %v

File Content (truncated):
%s

Classify this file and return ONLY valid JSON (no markdown, no explanation):
{
  "file_role": "header|source|build|test|config|resource",
  "component": "logical component/module name",
  "test_type": "unit|integration|hardware|manual|none",
  "test_framework": "gtest|catch|cunit|custom|none",
  "test_requires": ["external requirement 1", "requirement 2"],
  "external_deps": ["hardware/service/database/SDK dependencies"],
  "config_sources": ["env|file|registry|hardcoded"]
}

Guidelines:
- file_role: What role does this file play in the build?
- component: What logical part of the system is this? (e.g., "networking", "database", "ui")
- test_type: If this is a test file, what kind of testing?
- test_requires: What external resources are needed to run tests?
- external_deps: What external systems does this code depend on?
- config_sources: Where does configuration come from?`

const (
	maxContentLength = 6000
	maxRetries       = 3
)

// Execute performs LLM-based classification on a document
func (a *ClassifyDevOpsAction) Execute(ctx context.Context, doc *models.Document, force bool) error {
	a.logger.Debug().
		Str("doc_id", doc.ID).
		Str("title", doc.Title).
		Bool("force", force).
		Msg("Starting classify_devops action")

	// Check if LLM service is configured
	if a.llmService == nil {
		a.logger.Warn().
			Str("doc_id", doc.ID).
			Msg("LLM service not configured, skipping classification")
		return nil
	}

	// 1. Check if already classified (unless force)
	if !force && a.isAlreadyClassified(doc) {
		a.logger.Debug().
			Str("doc_id", doc.ID).
			Msg("Document already classified, skipping")
		return nil
	}

	// 2. Get or initialize DevOps metadata
	devopsMetadata := a.getDevOpsMetadata(doc)

	// 3. Truncate content to ~6000 chars
	truncatedContent := a.TruncateContent(doc.ContentMarkdown, maxContentLength)

	// 4. Build prompt with file info and extracted data
	prompt := a.buildPrompt(doc, devopsMetadata, truncatedContent)

	// 5. Call LLM with retry on error
	response, err := a.CallLLMWithRetry(ctx, prompt, maxRetries)
	if err != nil {
		a.logger.Error().
			Err(err).
			Str("doc_id", doc.ID).
			Msg("LLM classification failed after retries")

		// Mark document as enrichment_failed
		a.markEnrichmentFailed(doc)
		return fmt.Errorf("LLM classification failed: %w", err)
	}

	// 6. Parse JSON response
	classification, err := a.ParseClassification(response)
	if err != nil {
		a.logger.Error().
			Err(err).
			Str("doc_id", doc.ID).
			Str("response", response).
			Msg("Failed to parse classification response")

		// Mark document as enrichment_failed
		a.markEnrichmentFailed(doc)
		return fmt.Errorf("failed to parse classification: %w", err)
	}

	// 7. Add "classify_devops" to enrichment_passes
	a.addEnrichmentPass(devopsMetadata, "classify_devops")

	// 8. Update doc.Metadata with classification fields (merge with existing)
	a.updateDocumentMetadata(doc, devopsMetadata, classification)

	// Update document in storage
	if err := a.documentStorage.UpdateDocument(doc); err != nil {
		return fmt.Errorf("failed to update document: %w", err)
	}

	a.logger.Info().
		Str("doc_id", doc.ID).
		Str("file_role", classification.FileRole).
		Str("component", classification.Component).
		Msg("Document classified successfully")

	return nil
}

// isAlreadyClassified checks if document has already been classified
func (a *ClassifyDevOpsAction) isAlreadyClassified(doc *models.Document) bool {
	if doc.Metadata == nil {
		return false
	}

	devopsData, ok := doc.Metadata["devops"]
	if !ok {
		return false
	}

	devopsMap, ok := devopsData.(map[string]interface{})
	if !ok {
		return false
	}

	passes, ok := devopsMap["enrichment_passes"].([]interface{})
	if !ok {
		return false
	}

	for _, pass := range passes {
		if passStr, ok := pass.(string); ok && passStr == "classify_devops" {
			return true
		}
	}

	return false
}

// getDevOpsMetadata extracts or initializes DevOpsMetadata from document
func (a *ClassifyDevOpsAction) getDevOpsMetadata(doc *models.Document) *models.DevOpsMetadata {
	if doc.Metadata == nil {
		doc.Metadata = make(map[string]interface{})
	}

	devopsData, ok := doc.Metadata["devops"]
	if !ok {
		return &models.DevOpsMetadata{}
	}

	// Try to convert to DevOpsMetadata struct
	devopsMap, ok := devopsData.(map[string]interface{})
	if !ok {
		return &models.DevOpsMetadata{}
	}

	// Marshal and unmarshal to convert map to struct
	data, err := json.Marshal(devopsMap)
	if err != nil {
		a.logger.Warn().Err(err).Msg("Failed to marshal devops metadata")
		return &models.DevOpsMetadata{}
	}

	var metadata models.DevOpsMetadata
	if err := json.Unmarshal(data, &metadata); err != nil {
		a.logger.Warn().Err(err).Msg("Failed to unmarshal devops metadata")
		return &models.DevOpsMetadata{}
	}

	return &metadata
}

// buildPrompt constructs the LLM prompt from document and metadata
func (a *ClassifyDevOpsAction) buildPrompt(doc *models.Document, metadata *models.DevOpsMetadata, content string) string {
	// Determine language from tags or metadata
	language := "C/C++"
	if doc.Tags != nil {
		for _, tag := range doc.Tags {
			if strings.Contains(tag, "lang:") {
				language = strings.TrimPrefix(tag, "lang:")
				break
			}
		}
	}

	// Format metadata fields for prompt
	includesStr := formatStringSlice(metadata.Includes)
	definesStr := formatStringSlice(metadata.Defines)
	platformsStr := formatStringSlice(metadata.Platforms)

	return fmt.Sprintf(classifyPromptTemplate,
		doc.Title,
		language,
		includesStr,
		definesStr,
		platformsStr,
		content,
	)
}

// formatStringSlice formats a string slice for display in prompt
func formatStringSlice(slice []string) string {
	if len(slice) == 0 {
		return "(none)"
	}
	if len(slice) > 10 {
		return fmt.Sprintf("%v ... (%d total)", slice[:10], len(slice))
	}
	return fmt.Sprintf("%v", slice)
}

// TruncateContent truncates content to maxLen characters
func (a *ClassifyDevOpsAction) TruncateContent(content string, maxLen int) string {
	if len(content) <= maxLen {
		return content
	}
	return content[:maxLen] + "\n... [truncated]"
}

// CallLLMWithRetry calls the LLM service with exponential backoff retry logic
func (a *ClassifyDevOpsAction) CallLLMWithRetry(ctx context.Context, prompt string, maxRetries int) (string, error) {
	var lastErr error

	for i := 0; i < maxRetries; i++ {
		if i > 0 {
			a.logger.Debug().
				Int("attempt", i+1).
				Int("max_retries", maxRetries).
				Msg("Retrying LLM call")
		}

		response, err := a.llmService.Chat(ctx, []interfaces.Message{
			{Role: "user", Content: prompt},
		})

		if err == nil {
			return response, nil
		}

		lastErr = err

		// Check if context is cancelled
		if ctx.Err() != nil {
			return "", fmt.Errorf("context cancelled: %w", ctx.Err())
		}

		// Exponential backoff: 1s, 2s, 4s
		if i < maxRetries-1 {
			backoffDuration := time.Duration(1<<uint(i)) * time.Second
			a.logger.Warn().
				Err(err).
				Dur("backoff", backoffDuration).
				Int("attempt", i+1).
				Msg("LLM call failed, backing off")

			select {
			case <-time.After(backoffDuration):
				// Continue to next retry
			case <-ctx.Done():
				return "", fmt.Errorf("context cancelled during backoff: %w", ctx.Err())
			}
		}
	}

	return "", fmt.Errorf("LLM call failed after %d retries: %w", maxRetries, lastErr)
}

// ParseClassification parses the LLM response into a ClassificationResult
func (a *ClassifyDevOpsAction) ParseClassification(response string) (*ClassificationResult, error) {
	// Extract JSON from response (handle markdown code blocks)
	jsonStr := a.extractJSON(response)

	var result ClassificationResult
	if err := json.Unmarshal([]byte(jsonStr), &result); err != nil {
		return nil, fmt.Errorf("failed to unmarshal JSON: %w", err)
	}

	// Validate required fields
	if result.FileRole == "" {
		return nil, fmt.Errorf("file_role is required but was empty")
	}

	return &result, nil
}

// extractJSON extracts JSON from response, handling markdown code blocks
func (a *ClassifyDevOpsAction) extractJSON(response string) string {
	// Trim whitespace
	response = strings.TrimSpace(response)

	// Check for markdown code blocks
	if strings.HasPrefix(response, "```") {
		// Find the actual JSON content between code fences
		lines := strings.Split(response, "\n")
		var jsonLines []string
		inCodeBlock := false

		for _, line := range lines {
			if strings.HasPrefix(line, "```") {
				if inCodeBlock {
					// End of code block
					break
				} else {
					// Start of code block
					inCodeBlock = true
					continue
				}
			}
			if inCodeBlock {
				jsonLines = append(jsonLines, line)
			}
		}

		if len(jsonLines) > 0 {
			return strings.Join(jsonLines, "\n")
		}
	}

	// If no code blocks, try to extract JSON object
	startIdx := strings.Index(response, "{")
	endIdx := strings.LastIndex(response, "}")

	if startIdx >= 0 && endIdx > startIdx {
		return response[startIdx : endIdx+1]
	}

	// Return original response if no special handling needed
	return response
}

// updateDocumentMetadata updates the document metadata with classification results
func (a *ClassifyDevOpsAction) updateDocumentMetadata(doc *models.Document, metadata *models.DevOpsMetadata, classification *ClassificationResult) {
	// Update classification fields
	metadata.FileRole = classification.FileRole
	metadata.Component = classification.Component
	metadata.TestType = classification.TestType
	metadata.TestFramework = classification.TestFramework
	metadata.TestRequires = classification.TestRequires
	metadata.ExternalDeps = classification.ExternalDeps
	metadata.ConfigSources = classification.ConfigSources

	// Convert to map and update document
	metadataMap := a.devOpsMetadataToMap(metadata)
	doc.Metadata["devops"] = metadataMap
}

// devOpsMetadataToMap converts DevOpsMetadata struct to map
func (a *ClassifyDevOpsAction) devOpsMetadataToMap(metadata *models.DevOpsMetadata) map[string]interface{} {
	data, err := json.Marshal(metadata)
	if err != nil {
		a.logger.Error().Err(err).Msg("Failed to marshal DevOpsMetadata")
		return make(map[string]interface{})
	}

	var result map[string]interface{}
	if err := json.Unmarshal(data, &result); err != nil {
		a.logger.Error().Err(err).Msg("Failed to unmarshal DevOpsMetadata to map")
		return make(map[string]interface{})
	}

	return result
}

// addEnrichmentPass adds a pass name to the enrichment_passes list
func (a *ClassifyDevOpsAction) addEnrichmentPass(metadata *models.DevOpsMetadata, passName string) {
	// Check if already exists
	for _, pass := range metadata.EnrichmentPasses {
		if pass == passName {
			return
		}
	}

	metadata.EnrichmentPasses = append(metadata.EnrichmentPasses, passName)
}

// markEnrichmentFailed marks the document as having failed enrichment
func (a *ClassifyDevOpsAction) markEnrichmentFailed(doc *models.Document) {
	if doc.Metadata == nil {
		doc.Metadata = make(map[string]interface{})
	}

	doc.Metadata["enrichment_failed"] = true
	doc.Metadata["enrichment_error"] = "classify_devops"

	// Still try to save the failure state
	if err := a.documentStorage.UpdateDocument(doc); err != nil {
		a.logger.Error().
			Err(err).
			Str("doc_id", doc.ID).
			Msg("Failed to mark document as enrichment_failed")
	}
}
