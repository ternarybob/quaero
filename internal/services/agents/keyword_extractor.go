package agents

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"google.golang.org/adk/agent"
	"google.golang.org/adk/agent/llmagent"
	"google.golang.org/adk/model"
	"google.golang.org/adk/runner"
	"google.golang.org/genai"
)

// KeywordExtractor implements the AgentExecutor interface for extracting keywords from documents.
// It uses Google ADK's model to analyze document content and identify the most relevant keywords.
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
//   - llmModel: ADK model to use for extraction
//   - input: Map containing document_id, content, and max_keywords
//
// Returns:
//   - map[string]interface{}: Keywords and confidence scores
//   - error: nil on success, error with details on failure
//
// Errors:
//   - Missing required input fields (document_id, content)
//   - Invalid input types
//   - Agent execution failure
//   - Malformed agent response
func (k *KeywordExtractor) Execute(ctx context.Context, llmModel model.LLM, input map[string]interface{}) (map[string]interface{}, error) {
	// Validate input and extract parameters
	documentID, content, maxKeywords, err := validateInput(input)
	if err != nil {
		return nil, err
	}

	// Comment 3: Build prompt requesting JSON array or object with keywords/confidence
	instruction := fmt.Sprintf(`You are a keyword extraction specialist.

Task: Extract exactly %d of the most semantically relevant keywords from the document.

Rules:
- Single words or short phrases (2-3 words max)
- Domain-specific terminology and technical concepts
- No stop words (the, is, and, etc.)
- Extract exactly %d keywords (no more, no less)

Output Format Options (JSON only, no markdown fences):
1. Simple array: ["keyword1", "keyword2", "keyword3", ...]
2. Object with confidence: {"keywords": ["keyword1", "keyword2"], "confidence": {"keyword1": 0.95, "keyword2": 0.87}}

Choose option 2 if you can assign meaningful confidence scores (0.0-1.0), otherwise use option 1.

Document:
%s`, maxKeywords, maxKeywords, content)

	// Comment 1: Use ADK llmagent agent loop instead of direct GenerateContent
	// Create llmagent with instruction
	agentConfig := llmagent.Config{
		Name:        "keyword_extractor",
		Description: "Extracts keywords from documents",
		Model:       llmModel,
		Instruction: instruction,
		GenerateContentConfig: &genai.GenerateContentConfig{
			Temperature: genai.Ptr(float32(0.3)),
		},
	}

	llmAgent, err := llmagent.New(agentConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create llmagent for document %s: %w", documentID, err)
	}

	// Create runner to execute agent
	runnerConfig := runner.Config{
		Agent: llmAgent,
	}
	agentRunner, err := runner.New(runnerConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create runner for document %s: %w", documentID, err)
	}

	// Run agent with initial message
	userMsg := genai.NewPartFromText("Extract keywords from the document provided in the instruction.")
	initialContent := &genai.Content{
		Role:  "user",
		Parts: []*genai.Part{userMsg},
	}

	// Comment 2: Use correct ADK consumption pattern with iter.Seq2
	var response string
	for event, err := range agentRunner.Run(ctx, "user", "session_"+documentID, initialContent, agent.RunConfig{}) {
		if err != nil {
			return nil, fmt.Errorf("agent execution error for document %s: %w", documentID, err)
		}

		// Collect text from final response events
		if event != nil && event.IsFinalResponse() && event.Content != nil {
			for _, part := range event.Content.Parts {
				if part.Text != "" {
					response += part.Text
				}
			}
		}
	}

	if response == "" {
		return nil, fmt.Errorf("no response from agent for document %s", documentID)
	}

	// Comment 6: Robust markdown fence removal
	response = cleanMarkdownFences(response)

	// Comment 4: Flexible parsing - try array first, then object
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
// Comment 6: Enhanced cleanup for markdown fences
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
// Comment 4: Support both array-only and object formats
func parseKeywordResponse(response string, maxKeywords int) ([]string, map[string]float64, error) {
	// Try parsing as simple array first
	var keywords []string
	if err := json.Unmarshal([]byte(response), &keywords); err == nil {
		// Comment 5: Enforce max_keywords upper bound by truncating
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

	// Comment 5: Enforce max_keywords upper bound
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
