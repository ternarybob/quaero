package agents

import (
	"context"
	"encoding/json"
	"fmt"

	"google.golang.org/adk/model"
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
	// Validate required input fields
	documentID, ok := input["document_id"].(string)
	if !ok || documentID == "" {
		return nil, fmt.Errorf("document_id is required and must be a non-empty string")
	}

	content, ok := input["content"].(string)
	if !ok || content == "" {
		return nil, fmt.Errorf("content is required and must be a non-empty string")
	}

	// Extract optional max_keywords parameter (default: 10)
	maxKeywords := 10
	if mk, ok := input["max_keywords"].(int); ok && mk > 0 {
		maxKeywords = mk
	}

	// Build prompt for model
	prompt := fmt.Sprintf(`You are a keyword extraction expert. Analyze the following document and extract the %d most semantically relevant keywords.

Rules:
- Extract only the most important and meaningful keywords
- Include both single words and short phrases (2-3 words max)
- Prioritize domain-specific terminology and technical concepts
- Avoid common stop words (the, is, and, etc.)
- Assign confidence scores (0.0-1.0) based on relevance and prominence

Respond ONLY with valid JSON in this exact format:
{
  "keywords": ["keyword1", "keyword2", ...],
  "confidence": {
    "keyword1": 0.95,
    "keyword2": 0.87,
    ...
  }
}

Document to analyze:
%s`, maxKeywords, content)

	// Generate content using ADK model
	// Create LLMRequest with genai.Content
	req := &model.LLMRequest{
		Contents: []*genai.Content{
			{
				Parts: []*genai.Part{
					genai.NewPartFromText(prompt),
				},
			},
		},
	}

	// GenerateContent returns an iterator, use stream=false for single response
	var response string
	for resp, err := range llmModel.GenerateContent(ctx, req, false) {
		if err != nil {
			return nil, fmt.Errorf("ADK model generation failed for document %s: %w", documentID, err)
		}

		// Extract text from response
		if resp == nil || resp.Content == nil || len(resp.Content.Parts) == 0 {
			return nil, fmt.Errorf("no response from ADK model for document %s", documentID)
		}

		// Get text from first part
		response = resp.Content.Parts[0].Text
		break // We only need the first response for non-streaming
	}

	// Parse JSON response
	var result struct {
		Keywords   []string           `json:"keywords"`
		Confidence map[string]float64 `json:"confidence"`
	}

	if err := json.Unmarshal([]byte(response), &result); err != nil {
		return nil, fmt.Errorf("failed to parse agent response for document %s: %w (response: %s)", documentID, err, response)
	}

	// Validate response structure
	if len(result.Keywords) == 0 {
		return nil, fmt.Errorf("agent returned no keywords for document %s", documentID)
	}

	if len(result.Confidence) == 0 {
		return nil, fmt.Errorf("agent returned no confidence scores for document %s", documentID)
	}

	// Return structured output
	return map[string]interface{}{
		"keywords":   result.Keywords,
		"confidence": result.Confidence,
	}, nil
}

// GetType returns the agent type identifier for registration.
func (k *KeywordExtractor) GetType() string {
	return "keyword_extractor"
}
