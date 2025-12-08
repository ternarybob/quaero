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

// MetadataEnricher implements the AgentExecutor interface for extracting build/run/test metadata.
// It uses Google Gemini API to analyze code documents and identify how to build, run, and test the project.
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
//	    "build_commands": ["go build", ...],    // Commands to build the project
//	    "run_commands": ["go run main.go"],     // Commands to run the project
//	    "test_commands": ["go test ./..."],     // Commands to test the project
//	    "dependencies": ["github.com/..."],     // External dependencies
//	    "file_type": "source",                  // Type: source, config, test, docs
//	    "language": "go",                       // Detected programming language
//	    "framework": "none"                     // Detected framework if any
//	}
type MetadataEnricher struct{}

// Execute runs the metadata enricher agent.
//
// The agent analyzes code documents and extracts build/run/test metadata.
// Results include commands, dependencies, and file type classification.
func (m *MetadataEnricher) Execute(ctx context.Context, client *genai.Client, modelName string, input map[string]interface{}) (map[string]interface{}, error) {
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

	// Build prompt requesting JSON response with metadata
	instruction := fmt.Sprintf(`You are a code analysis specialist.

Task: Analyze this code document and extract build, run, and test metadata.%s

Rules:
- Identify build commands (compile, package, etc.)
- Identify run commands (execute, start, etc.)
- Identify test commands (unit tests, integration tests)
- List external dependencies/imports
- Classify the file type: source, config, test, docs, script, data
- Identify the programming language
- Identify any framework being used

Output Format (JSON only, no markdown fences):
{
  "build_commands": ["command1", "command2"],
  "run_commands": ["command1"],
  "test_commands": ["command1"],
  "dependencies": ["dep1", "dep2"],
  "file_type": "source",
  "language": "go",
  "framework": "none"
}

If a category doesn't apply, use an empty array or "none".

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
	response = cleanMetadataMarkdownFences(response)

	// Parse response
	result, err := parseMetadataResponse(response)
	if err != nil {
		return nil, fmt.Errorf("failed to parse agent response for document %s: %w (response: %s)", documentID, err, response)
	}

	return result, nil
}

// cleanMetadataMarkdownFences removes markdown code fences from response
func cleanMetadataMarkdownFences(s string) string {
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

// parseMetadataResponse parses JSON response into structured output
func parseMetadataResponse(response string) (map[string]interface{}, error) {
	var result struct {
		BuildCommands []string `json:"build_commands"`
		RunCommands   []string `json:"run_commands"`
		TestCommands  []string `json:"test_commands"`
		Dependencies  []string `json:"dependencies"`
		FileType      string   `json:"file_type"`
		Language      string   `json:"language"`
		Framework     string   `json:"framework"`
	}

	if err := json.Unmarshal([]byte(response), &result); err != nil {
		return nil, fmt.Errorf("failed to parse JSON: %w", err)
	}

	// Convert to map for flexible output
	output := map[string]interface{}{
		"build_commands": result.BuildCommands,
		"run_commands":   result.RunCommands,
		"test_commands":  result.TestCommands,
		"dependencies":   result.Dependencies,
		"file_type":      result.FileType,
		"language":       result.Language,
		"framework":      result.Framework,
	}

	// Ensure arrays are not nil
	if output["build_commands"] == nil {
		output["build_commands"] = []string{}
	}
	if output["run_commands"] == nil {
		output["run_commands"] = []string{}
	}
	if output["test_commands"] == nil {
		output["test_commands"] = []string{}
	}
	if output["dependencies"] == nil {
		output["dependencies"] = []string{}
	}

	return output, nil
}

// GetType returns the agent type identifier for registration.
func (m *MetadataEnricher) GetType() string {
	return "metadata_enricher"
}
