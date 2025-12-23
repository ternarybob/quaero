package agents

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"google.golang.org/genai"
)

// KeywordExtractor implements the AgentExecutor interface for extracting keywords from documents.
// It uses Google Gemini API to analyze document content and identify the most relevant keywords.
//
// Input Format:
//
//	{
//	    "document_id": "doc_123",           // Document identifier
//	    "content": "Document text...",      // Full document content to analyze
//	    "max_keywords": 10                  // Maximum number of keywords to extract
//	}
//
// Output Format:
//
//	{
//	    "keywords": ["keyword1", "keyword2", ...],  // Extracted keywords
//	    "confidence": {                              // Confidence scores (0-1)
//	        "keyword1": 0.95,
//	        "keyword2": 0.87,
//	        ...
//	    }
//	}
type KeywordExtractor struct{}

// validateInput validates and extracts parameters from the input map.
// Returns document_id, content, max_keywords (clamped to [5, 15]), and error if validation fails.
func validateInput(input map[string]interface{}) (string, string, int, error) {
	// Validate document_id
	documentID, ok := input["document_id"].(string)
	if !ok || documentID == "" {
		return "", "", 0, fmt.Errorf("document_id is required and must be a non-empty string")
	}

	// Validate content
	content, ok := input["content"].(string)
	if !ok || content == "" {
		return "", "", 0, fmt.Errorf("content is required and must be a non-empty string")
	}

	// Parse and clamp max_keywords to [5, 15] range
	maxKeywords := 10 // Default
	if mkVal, exists := input["max_keywords"]; exists {
		switch v := mkVal.(type) {
		case int:
			maxKeywords = v
		case float64:
			maxKeywords = int(v)
		case string:
			if parsed, err := strconv.Atoi(v); err == nil {
				maxKeywords = parsed
			}
		}
	}
	// Clamp to [5, 15] range per requirements
	if maxKeywords < 5 {
		maxKeywords = 5
	} else if maxKeywords > 15 {
		maxKeywords = 15
	}

	return documentID, content, maxKeywords, nil
}

// Execute runs the keyword extraction agent.
//
// The agent analyzes document content and extracts the most semantically relevant keywords.
// Results include confidence scores for each keyword indicating extraction reliability.
//
// Parameters:
//   - ctx: Context for cancellation control
//   - client: genai client for API calls
//   - modelName: Model name to use (e.g., "gemini-3-pro-preview")
//   - input: Map containing document_id, content, and max_keywords
//
// Returns:
//   - map[string]interface{}: Keywords and confidence scores
//   - error: nil on success, error with details on failure
//
// Errors:
//   - Missing required input fields (document_id, content)
//   - Invalid input types
//   - API execution failure
//   - Malformed API response
func (k *KeywordExtractor) Execute(ctx context.Context, client *genai.Client, modelName string, input map[string]interface{}) (map[string]interface{}, error) {
	// Validate input and extract parameters
	documentID, content, maxKeywords, err := validateInput(input)
	if err != nil {
		return nil, err
	}

	// Build prompt requesting JSON response with keywords
	instruction := fmt.Sprintf(`You are a keyword extraction specialist.

Task: Extract exactly %d of the most semantically relevant keywords from the document.

Rules:
- Single words or short phrases (2-3 words max)
- Domain-specific terminology and technical concepts
- No stop words (the, is, and, etc.)
- Extract exactly %d keywords (no more, no less)

Output Format (JSON only, no markdown fences):
{"keywords": ["keyword1", "keyword2"], "confidence": {"keyword1": 0.95, "keyword2": 0.87}}

If you cannot assign meaningful confidence scores, use simple array: ["keyword1", "keyword2"]

Document:
%s`, maxKeywords, maxKeywords, content)

	// Generate content with retry logic for rate limiting
	config := &genai.GenerateContentConfig{
		Temperature: genai.Ptr(float32(0.3)),
	}

	var genaiResponse *genai.GenerateContentResponse
	var apiErr error

	// Retry configuration
	const maxRetries = 5
	const initialBackoff = 1 * time.Second
	const maxBackoff = 10 * time.Second
	const backoffMultiplier = 2.0

	for attempt := 0; attempt <= maxRetries; attempt++ {
		genaiResponse, apiErr = client.Models.GenerateContent(ctx, modelName, []*genai.Content{
			{
				Role: "user",
				Parts: []*genai.Part{
					genai.NewPartFromText(instruction),
				},
			},
		}, config)

		if apiErr == nil {
			// Success - break out of retry loop
			break
		}

		// If this was the last attempt, don't wait
		if attempt == maxRetries {
			break
		}

		// Calculate backoff duration with exponential increase
		backoff := time.Duration(float64(initialBackoff) * float64(uint(1)<<uint(attempt)) * backoffMultiplier)
		if backoff > maxBackoff {
			backoff = maxBackoff
		}

		// Wait before retrying
		select {
		case <-ctx.Done():
			return nil, fmt.Errorf("context cancelled during retry for document %s: %w", documentID, ctx.Err())
		case <-time.After(backoff):
			// Continue to next retry attempt
		}
	}

	if apiErr != nil {
		return nil, fmt.Errorf("failed to generate content for document %s (model: %s) after %d retries: %w", documentID, modelName, maxRetries, apiErr)
	}

	// Extract text from response using convenience method
	response := genaiResponse.Text()
	if response == "" {
		return nil, fmt.Errorf("no response from API for document %s", documentID)
	}

	// Robust markdown fence removal
	response = cleanMarkdownFences(response)

	// Flexible parsing - try array first, then object
	keywords, confidence, err := parseKeywordResponse(response, maxKeywords)
	if err != nil {
		return nil, fmt.Errorf("failed to parse agent response for document %s: %w (response: %s)", documentID, err, response)
	}

	// Validate we got keywords
	if len(keywords) == 0 {
		return nil, fmt.Errorf("agent returned no keywords for document %s", documentID)
	}

	// Return structured output
	result := map[string]interface{}{
		"keywords": keywords,
	}
	if confidence != nil && len(confidence) > 0 {
		result["confidence"] = confidence
	}

	return result, nil
}

// cleanMarkdownFences robustly removes markdown code fences from response
func cleanMarkdownFences(s string) string {
	// Trim leading/trailing whitespace
	s = strings.TrimSpace(s)

	// Remove markdown code fences with language hints
	// Match: ```json\n or ```\n at start, and ``` at end
	fencePattern := regexp.MustCompile(`(?s)^\s*` + "```" + `(?:json|JSON)?\s*\n?(.*?)\n?\s*` + "```" + `\s*$`)
	if matches := fencePattern.FindStringSubmatch(s); len(matches) > 1 {
		s = matches[1]
	}

	// Fallback: simple prefix/suffix trimming
	s = strings.TrimPrefix(s, "```json")
	s = strings.TrimPrefix(s, "```JSON")
	s = strings.TrimPrefix(s, "```")
	s = strings.TrimSuffix(s, "```")

	return strings.TrimSpace(s)
}

// parseKeywordResponse flexibly parses JSON response
// Supports both array-only and object formats
func parseKeywordResponse(response string, maxKeywords int) ([]string, map[string]float64, error) {
	// Try parsing as simple array first
	var keywords []string
	if err := json.Unmarshal([]byte(response), &keywords); err == nil {
		// Enforce max_keywords upper bound by truncating
		if len(keywords) > maxKeywords {
			keywords = keywords[:maxKeywords]
		}
		return keywords, nil, nil
	}

	// Try parsing as object with keywords and optional confidence
	var result struct {
		Keywords   []string           `json:"keywords"`
		Confidence map[string]float64 `json:"confidence,omitempty"`
	}
	if err := json.Unmarshal([]byte(response), &result); err != nil {
		return nil, nil, fmt.Errorf("failed to parse as array or object: %w", err)
	}

	// Enforce max_keywords upper bound
	if len(result.Keywords) > maxKeywords {
		result.Keywords = result.Keywords[:maxKeywords]
		// Also trim confidence map to match truncated keywords
		if result.Confidence != nil {
			truncatedConfidence := make(map[string]float64)
			for _, kw := range result.Keywords {
				if score, exists := result.Confidence[kw]; exists {
					truncatedConfidence[kw] = score
				}
			}
			result.Confidence = truncatedConfidence
		}
	}

	return result.Keywords, result.Confidence, nil
}

// GetType returns the agent type identifier for registration.
func (k *KeywordExtractor) GetType() string {
	return "keyword_extractor"
}

// Test helpers - Expose internal functions for unit testing

// TestParseKeywordResponse is a test helper that exposes parseKeywordResponse for testing.
// This allows unit tests to verify response parsing logic without invoking the full agent.
func TestParseKeywordResponse(response string, maxKeywords int) ([]string, map[string]float64, error) {
	return parseKeywordResponse(response, maxKeywords)
}

// TestCleanMarkdownFences is a test helper that exposes cleanMarkdownFences for testing.
// This allows unit tests to verify markdown fence removal logic independently.
func TestCleanMarkdownFences(s string) string {
	return cleanMarkdownFences(s)
}

// TestValidateInput is a test helper that exposes validateInput for testing.
// This allows unit tests to verify input validation logic independently.
func TestValidateInput(input map[string]interface{}) (string, string, int, error) {
	return validateInput(input)
}
