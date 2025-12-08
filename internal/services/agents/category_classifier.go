package agents

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"time"

	"google.golang.org/genai"
)

// CategoryClassifier implements the AgentExecutor interface for classifying document/file purpose.
// It uses Google Gemini API to analyze code documents and classify their role in the project.
//
// Input Format:
//
//	{
//	    "document_id": "doc_123",           // Document identifier
//	    "content": "Document text...",      // Full document content to analyze
//	    "title": "main.go"                  // Optional: document title/filename
//	}
//
// Output Format:
//
//	{
//	    "category": "source",               // Primary category: source, test, config, docs, script, data
//	    "subcategory": "implementation",    // More specific classification
//	    "purpose": "Main entry point",      // Brief description of file purpose
//	    "importance": "high",               // Importance level: high, medium, low
//	    "tags": ["entrypoint", "main"]      // Relevant tags for the file
//	}
type CategoryClassifier struct{}

// Execute runs the category classifier agent.
//
// The agent analyzes code documents and classifies their purpose and role.
// Results include category, subcategory, purpose description, and tags.
func (c *CategoryClassifier) Execute(ctx context.Context, client *genai.Client, modelName string, input map[string]interface{}) (map[string]interface{}, error) {
	// Validate document_id
	documentID, ok := input["document_id"].(string)
	if !ok || documentID == "" {
		return nil, fmt.Errorf("document_id is required and must be a non-empty string")
	}

	// Validate content
	content, ok := input["content"].(string)
	if !ok || content == "" {
		return nil, fmt.Errorf("content is required and must be a non-empty string")
	}

	// Optional title
	title, _ := input["title"].(string)
	titleHint := ""
	if title != "" {
		titleHint = fmt.Sprintf("\nFilename: %s", title)
	}

	// Build prompt requesting JSON response with classification
	instruction := fmt.Sprintf(`You are a code classification specialist.

Task: Analyze this code document and classify its purpose and role in the project.%s

Rules:
- Identify the primary category: source, test, config, docs, script, data, build, ci
- Identify a more specific subcategory (e.g., implementation, interface, unit-test, integration-test, readme, etc.)
- Describe the purpose of this file in one sentence
- Rate the importance: high (core functionality), medium (supporting), low (auxiliary)
- Suggest relevant tags for categorization

Output Format (JSON only, no markdown fences):
{
  "category": "source",
  "subcategory": "implementation",
  "purpose": "Brief description of what this file does",
  "importance": "high",
  "tags": ["tag1", "tag2"]
}

Document Content:
%s`, titleHint, content)

	// Generate content with retry logic
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
			break
		}

		if attempt == maxRetries {
			break
		}

		// Calculate backoff duration
		backoff := time.Duration(float64(initialBackoff) * float64(uint(1)<<uint(attempt)) * backoffMultiplier)
		if backoff > maxBackoff {
			backoff = maxBackoff
		}

		select {
		case <-ctx.Done():
			return nil, fmt.Errorf("context cancelled during retry for document %s: %w", documentID, ctx.Err())
		case <-time.After(backoff):
		}
	}

	if apiErr != nil {
		return nil, fmt.Errorf("failed to generate content for document %s (model: %s) after %d retries: %w", documentID, modelName, maxRetries, apiErr)
	}

	// Extract text from response
	response := genaiResponse.Text()
	if response == "" {
		return nil, fmt.Errorf("no response from API for document %s", documentID)
	}

	// Clean markdown fences
	response = cleanCategoryMarkdownFences(response)

	// Parse response
	result, err := parseCategoryResponse(response)
	if err != nil {
		return nil, fmt.Errorf("failed to parse agent response for document %s: %w (response: %s)", documentID, err, response)
	}

	return result, nil
}

// cleanCategoryMarkdownFences removes markdown code fences from response
func cleanCategoryMarkdownFences(s string) string {
	s = strings.TrimSpace(s)

	fencePattern := regexp.MustCompile(`(?s)^\s*` + "```" + `(?:json|JSON)?\s*\n?(.*?)\n?\s*` + "```" + `\s*$`)
	if matches := fencePattern.FindStringSubmatch(s); len(matches) > 1 {
		s = matches[1]
	}

	s = strings.TrimPrefix(s, "```json")
	s = strings.TrimPrefix(s, "```JSON")
	s = strings.TrimPrefix(s, "```")
	s = strings.TrimSuffix(s, "```")

	return strings.TrimSpace(s)
}

// parseCategoryResponse parses JSON response into structured output
func parseCategoryResponse(response string) (map[string]interface{}, error) {
	var result struct {
		Category    string   `json:"category"`
		Subcategory string   `json:"subcategory"`
		Purpose     string   `json:"purpose"`
		Importance  string   `json:"importance"`
		Tags        []string `json:"tags"`
	}

	if err := json.Unmarshal([]byte(response), &result); err != nil {
		return nil, fmt.Errorf("failed to parse JSON: %w", err)
	}

	// Convert to map for flexible output
	output := map[string]interface{}{
		"category":    result.Category,
		"subcategory": result.Subcategory,
		"purpose":     result.Purpose,
		"importance":  result.Importance,
		"tags":        result.Tags,
	}

	// Ensure tags array is not nil
	if output["tags"] == nil {
		output["tags"] = []string{}
	}

	// Set defaults for empty strings
	if output["category"] == "" {
		output["category"] = "unknown"
	}
	if output["importance"] == "" {
		output["importance"] = "medium"
	}

	return output, nil
}

// GetType returns the agent type identifier for registration.
func (c *CategoryClassifier) GetType() string {
	return "category_classifier"
}
