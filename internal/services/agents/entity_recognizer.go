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

// EntityRecognizer implements the AgentExecutor interface for identifying key components in code.
// It uses Google Gemini API to analyze code documents and identify important entities.
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
//	    "components": ["ComponentA", ...],  // Key components/classes/structs defined
//	    "entry_points": ["main", ...],      // Entry point functions
//	    "exports": ["FuncA", "TypeB"],      // Exported/public items
//	    "dependencies": ["pkg/a", ...],     // Internal dependencies
//	    "interfaces": ["Reader", ...],      // Interfaces defined or implemented
//	    "patterns": ["singleton", ...]      // Design patterns identified
//	}
type EntityRecognizer struct{}

// Execute runs the entity recognizer agent.
//
// The agent analyzes code documents and identifies key components and entities.
// Results include components, entry points, exports, and design patterns.
func (e *EntityRecognizer) Execute(ctx context.Context, client *genai.Client, modelName string, input map[string]interface{}) (map[string]interface{}, error) {
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

	// Build prompt requesting JSON response with entities
	instruction := fmt.Sprintf(`You are a code analysis specialist focusing on entity recognition.

Task: Analyze this code document and identify key components, entry points, and important entities.%s

Rules:
- Identify key components (classes, structs, types, modules)
- Identify entry point functions (main, init, handlers, etc.)
- List exported/public items (functions, types, constants)
- Identify internal dependencies (imports from same project)
- List interfaces defined or implemented
- Identify design patterns used (singleton, factory, observer, etc.)

Output Format (JSON only, no markdown fences):
{
  "components": ["Component1", "Component2"],
  "entry_points": ["main", "init"],
  "exports": ["PublicFunc", "PublicType"],
  "dependencies": ["internal/pkg1", "internal/pkg2"],
  "interfaces": ["Interface1", "Interface2"],
  "patterns": ["pattern1", "pattern2"]
}

If a category doesn't apply, use an empty array.

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
	response = cleanEntityMarkdownFences(response)

	// Parse response
	result, err := parseEntityResponse(response)
	if err != nil {
		return nil, fmt.Errorf("failed to parse agent response for document %s: %w (response: %s)", documentID, err, response)
	}

	return result, nil
}

// cleanEntityMarkdownFences removes markdown code fences from response
func cleanEntityMarkdownFences(s string) string {
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

// parseEntityResponse parses JSON response into structured output
func parseEntityResponse(response string) (map[string]interface{}, error) {
	var result struct {
		Components   []string `json:"components"`
		EntryPoints  []string `json:"entry_points"`
		Exports      []string `json:"exports"`
		Dependencies []string `json:"dependencies"`
		Interfaces   []string `json:"interfaces"`
		Patterns     []string `json:"patterns"`
	}

	if err := json.Unmarshal([]byte(response), &result); err != nil {
		return nil, fmt.Errorf("failed to parse JSON: %w", err)
	}

	// Convert to map for flexible output
	output := map[string]interface{}{
		"components":   result.Components,
		"entry_points": result.EntryPoints,
		"exports":      result.Exports,
		"dependencies": result.Dependencies,
		"interfaces":   result.Interfaces,
		"patterns":     result.Patterns,
	}

	// Ensure arrays are not nil
	if output["components"] == nil {
		output["components"] = []string{}
	}
	if output["entry_points"] == nil {
		output["entry_points"] = []string{}
	}
	if output["exports"] == nil {
		output["exports"] = []string{}
	}
	if output["dependencies"] == nil {
		output["dependencies"] = []string{}
	}
	if output["interfaces"] == nil {
		output["interfaces"] = []string{}
	}
	if output["patterns"] == nil {
		output["patterns"] = []string{}
	}

	return output, nil
}

// GetType returns the agent type identifier for registration.
func (e *EntityRecognizer) GetType() string {
	return "entity_recognizer"
}
