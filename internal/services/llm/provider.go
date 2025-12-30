package llm

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"
	"github.com/ternarybob/arbor"
	"github.com/ternarybob/quaero/internal/common"
	"github.com/ternarybob/quaero/internal/interfaces"
	"google.golang.org/genai"
)

// ProviderType represents the AI provider type
type ProviderType string

const (
	// ProviderGemini uses Google Gemini API
	ProviderGemini ProviderType = "gemini"
	// ProviderClaude uses Anthropic Claude API
	ProviderClaude ProviderType = "claude"
)

// ContentRequest represents a provider-agnostic content generation request
type ContentRequest struct {
	Messages          []interfaces.Message
	Model             string
	Temperature       float32
	MaxTokens         int
	SystemInstruction string
	ThinkingLevel     string                 // For providers that support extended thinking
	OutputSchema      map[string]interface{} // JSON schema for structured output (Gemini only)
}

// ContentResponse represents a provider-agnostic content generation response
type ContentResponse struct {
	Text     string
	Provider ProviderType
	Model    string
}

// Provider defines the interface for AI content generation
type Provider interface {
	GenerateContent(ctx context.Context, request *ContentRequest) (*ContentResponse, error)
	GetProviderType() ProviderType
	Close() error
}

// ProviderFactory creates and manages AI providers
type ProviderFactory struct {
	geminiConfig *common.GeminiConfig
	claudeConfig *common.ClaudeConfig
	llmConfig    *common.LLMConfig
	kvStorage    interfaces.KeyValueStorage
	logger       arbor.ILogger
	geminiClient *genai.Client
	claudeClient anthropic.Client
	geminiAPIKey string
	claudeAPIKey string
}

// NewProviderFactory creates a new provider factory
func NewProviderFactory(
	geminiConfig *common.GeminiConfig,
	claudeConfig *common.ClaudeConfig,
	llmConfig *common.LLMConfig,
	kvStorage interfaces.KeyValueStorage,
	logger arbor.ILogger,
) *ProviderFactory {
	return &ProviderFactory{
		geminiConfig: geminiConfig,
		claudeConfig: claudeConfig,
		llmConfig:    llmConfig,
		kvStorage:    kvStorage,
		logger:       logger,
	}
}

// DetectProvider determines the provider type from a model string.
// Model strings can be:
// - "claude-sonnet-4-20250514" -> Claude
// - "claude/claude-sonnet-4-20250514" -> Claude (with prefix)
// - "gemini-3-flash" -> Gemini
// - "gemini/gemini-3-flash" -> Gemini (with prefix)
// - Empty string -> uses default provider from config
func (f *ProviderFactory) DetectProvider(model string) ProviderType {
	if model == "" {
		return ProviderType(f.llmConfig.DefaultProvider)
	}

	model = strings.ToLower(model)

	// Check for explicit provider prefix
	if strings.HasPrefix(model, "claude/") || strings.HasPrefix(model, "anthropic/") {
		return ProviderClaude
	}
	if strings.HasPrefix(model, "gemini/") || strings.HasPrefix(model, "google/") {
		return ProviderGemini
	}

	// Check for model name patterns
	if strings.HasPrefix(model, "claude-") {
		return ProviderClaude
	}
	if strings.HasPrefix(model, "gemini-") {
		return ProviderGemini
	}

	// Default to configured provider
	return ProviderType(f.llmConfig.DefaultProvider)
}

// NormalizeModel removes provider prefix from model name if present
func (f *ProviderFactory) NormalizeModel(model string) string {
	// Remove provider prefixes
	prefixes := []string{"claude/", "anthropic/", "gemini/", "google/"}
	for _, prefix := range prefixes {
		if strings.HasPrefix(strings.ToLower(model), prefix) {
			return model[len(prefix):]
		}
	}
	return model
}

// GetDefaultModel returns the default model for a provider
func (f *ProviderFactory) GetDefaultModel(provider ProviderType) string {
	switch provider {
	case ProviderClaude:
		return f.claudeConfig.Model
	case ProviderGemini:
		return f.geminiConfig.Model
	default:
		return f.geminiConfig.Model
	}
}

// GetGeminiClient returns a Gemini client, creating one if necessary
func (f *ProviderFactory) GetGeminiClient(ctx context.Context) (*genai.Client, error) {
	if f.geminiClient != nil {
		return f.geminiClient, nil
	}

	// Resolve API key (supports both new "gemini_api_key" and legacy "google_api_key")
	apiKey, err := common.ResolveAPIKey(ctx, f.kvStorage, "gemini_api_key", f.geminiConfig.APIKey)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve Gemini API key: %w", err)
	}

	// Create client
	client, err := genai.NewClient(ctx, &genai.ClientConfig{
		APIKey:  apiKey,
		Backend: genai.BackendGeminiAPI,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create Gemini client: %w", err)
	}

	f.geminiClient = client
	f.geminiAPIKey = apiKey
	return client, nil
}

// GetClaudeClient returns a Claude client, creating one if necessary
func (f *ProviderFactory) GetClaudeClient(ctx context.Context) (anthropic.Client, error) {
	// Check if client is already initialized (non-zero value)
	if f.claudeAPIKey != "" {
		return f.claudeClient, nil
	}

	// Resolve API key
	apiKey, err := common.ResolveAPIKey(ctx, f.kvStorage, "anthropic_api_key", f.claudeConfig.APIKey)
	if err != nil {
		return anthropic.Client{}, fmt.Errorf("failed to resolve Anthropic API key: %w", err)
	}

	// Create client
	client := anthropic.NewClient(
		option.WithAPIKey(apiKey),
	)

	f.claudeClient = client
	f.claudeAPIKey = apiKey
	return client, nil
}

// GenerateContent generates content using the appropriate provider based on model
func (f *ProviderFactory) GenerateContent(ctx context.Context, request *ContentRequest) (*ContentResponse, error) {
	provider := f.DetectProvider(request.Model)
	model := f.NormalizeModel(request.Model)

	f.logger.Debug().
		Str("provider", string(provider)).
		Str("model", model).
		Int("message_count", len(request.Messages)).
		Msg("Generating content with provider")

	switch provider {
	case ProviderClaude:
		return f.generateWithClaude(ctx, request, model)
	case ProviderGemini:
		return f.generateWithGemini(ctx, request, model)
	default:
		return f.generateWithGemini(ctx, request, model)
	}
}

// generateWithClaude generates content using Claude API
func (f *ProviderFactory) generateWithClaude(ctx context.Context, request *ContentRequest, model string) (*ContentResponse, error) {
	client, err := f.GetClaudeClient(ctx)
	if err != nil {
		return nil, err
	}

	// Use default model if not specified
	if model == "" {
		model = f.claudeConfig.Model
	}

	// Convert messages to Claude format
	claudeMessages, systemText, err := convertMessagesToClaude(request.Messages)
	if err != nil {
		return nil, fmt.Errorf("failed to convert messages: %w", err)
	}

	// Use system instruction from request if provided
	if request.SystemInstruction != "" {
		systemText = request.SystemInstruction
	}

	// Build request parameters
	maxTokens := request.MaxTokens
	if maxTokens <= 0 {
		maxTokens = f.claudeConfig.MaxTokens
	}

	params := anthropic.MessageNewParams{
		Model:     anthropic.Model(model),
		MaxTokens: int64(maxTokens),
		Messages:  claudeMessages,
	}

	// Set temperature
	temp := request.Temperature
	if temp <= 0 {
		temp = f.claudeConfig.Temperature
	}
	if temp > 0 {
		params.Temperature = anthropic.Float(float64(temp))
	}

	// Set system message
	if systemText != "" {
		params.System = []anthropic.TextBlockParam{
			{Text: systemText},
		}
	}

	// Make API call with retry
	var resp *anthropic.Message
	var apiErr error
	retryConfig := NewDefaultRetryConfig()

	for attempt := 0; attempt <= retryConfig.MaxRetries; attempt++ {
		resp, apiErr = client.Messages.New(ctx, params)
		if apiErr == nil {
			break
		}

		if attempt == retryConfig.MaxRetries {
			break
		}

		// Calculate backoff
		backoff := time.Duration(attempt+1) * 2 * time.Second
		if IsRateLimitError(apiErr) {
			backoff = retryConfig.CalculateBackoff(attempt, 0)
		}

		f.logger.Warn().
			Int("attempt", attempt+1).
			Dur("backoff", backoff).
			Err(apiErr).
			Msg("Retrying Claude API call")

		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(backoff):
		}
	}

	if apiErr != nil {
		return nil, fmt.Errorf("Claude API call failed after %d retries: %w", retryConfig.MaxRetries, apiErr)
	}

	// Extract text from response
	var text strings.Builder
	for _, block := range resp.Content {
		if block.Type == "text" {
			text.WriteString(block.Text)
		}
	}

	if text.Len() == 0 {
		return nil, fmt.Errorf("empty response from Claude API")
	}

	return &ContentResponse{
		Text:     text.String(),
		Provider: ProviderClaude,
		Model:    model,
	}, nil
}

// generateWithGemini generates content using Gemini API
func (f *ProviderFactory) generateWithGemini(ctx context.Context, request *ContentRequest, model string) (*ContentResponse, error) {
	client, err := f.GetGeminiClient(ctx)
	if err != nil {
		return nil, err
	}

	// Use default model if not specified
	if model == "" {
		model = f.geminiConfig.Model
	}

	// Convert messages to Gemini format
	geminiContents, systemText, err := convertMessagesToGemini(request.Messages)
	if err != nil {
		return nil, fmt.Errorf("failed to convert messages: %w", err)
	}

	// Use system instruction from request if provided
	if request.SystemInstruction != "" {
		systemText = request.SystemInstruction
	}

	// Build config
	temp := request.Temperature
	if temp <= 0 {
		temp = f.geminiConfig.Temperature
	}

	config := &genai.GenerateContentConfig{
		Temperature: genai.Ptr(temp),
	}

	// Set system instruction
	if systemText != "" {
		config.SystemInstruction = genai.NewContentFromText(systemText, genai.RoleUser)
	}

	// Add thinking config if specified
	if request.ThinkingLevel != "" {
		parsedLevel := parseGeminiThinkingLevel(request.ThinkingLevel)
		if parsedLevel != "" {
			config.ThinkingConfig = &genai.ThinkingConfig{
				ThinkingLevel: parsedLevel,
			}
		}
	}

	// Add schema for structured output if specified
	// When schema is provided, Gemini enforces JSON output matching the schema
	if request.OutputSchema != nil && len(request.OutputSchema) > 0 {
		genaiSchema, err := convertToGenaiSchema(request.OutputSchema)
		if err != nil {
			f.logger.Error().Err(err).Msg("Failed to convert output schema")
			// Continue without schema rather than failing
		} else if genaiSchema != nil {
			config.ResponseMIMEType = "application/json"
			config.ResponseSchema = genaiSchema
			f.logger.Debug().
				Str("schema_type", string(genaiSchema.Type)).
				Msg("Using structured JSON output with schema")
		}
	}

	// Make API call with retry
	var resp *genai.GenerateContentResponse
	var apiErr error
	retryConfig := NewDefaultRetryConfig()

	for attempt := 0; attempt <= retryConfig.MaxRetries; attempt++ {
		resp, apiErr = client.Models.GenerateContent(ctx, model, geminiContents, config)
		if apiErr == nil {
			break
		}

		if attempt == retryConfig.MaxRetries {
			break
		}

		// Calculate backoff
		var backoff time.Duration
		if IsRateLimitError(apiErr) {
			apiDelay := ExtractRetryDelay(apiErr)
			backoff = retryConfig.CalculateBackoff(attempt, apiDelay)
		} else {
			backoff = time.Duration(attempt+1) * 2 * time.Second
		}

		f.logger.Warn().
			Int("attempt", attempt+1).
			Dur("backoff", backoff).
			Err(apiErr).
			Msg("Retrying Gemini API call")

		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(backoff):
		}
	}

	if apiErr != nil {
		return nil, fmt.Errorf("Gemini API call failed after %d retries: %w", retryConfig.MaxRetries, apiErr)
	}

	// Extract text from response
	if resp == nil || len(resp.Candidates) == 0 {
		return nil, fmt.Errorf("empty response from Gemini API")
	}

	responseText := resp.Text()
	if responseText == "" {
		return nil, fmt.Errorf("empty text in Gemini response")
	}

	return &ContentResponse{
		Text:     responseText,
		Provider: ProviderGemini,
		Model:    model,
	}, nil
}

// parseGeminiThinkingLevel converts a string thinking level to genai.ThinkingLevel
func parseGeminiThinkingLevel(level string) genai.ThinkingLevel {
	switch strings.ToUpper(level) {
	case "MINIMAL":
		return genai.ThinkingLevelMinimal
	case "LOW":
		return genai.ThinkingLevelLow
	case "MEDIUM":
		return genai.ThinkingLevelMedium
	case "HIGH":
		return genai.ThinkingLevelHigh
	default:
		return ""
	}
}

// Close closes all provider clients
func (f *ProviderFactory) Close() error {
	f.geminiClient = nil
	f.claudeClient = anthropic.Client{} // Reset to zero value
	f.claudeAPIKey = ""                 // Clear API key to mark as uninitialized
	return nil
}

// convertToGenaiSchema converts a map[string]interface{} representation of a JSON schema
// to a genai.Schema structure. This allows defining schemas in TOML templates.
func convertToGenaiSchema(schemaMap map[string]interface{}) (*genai.Schema, error) {
	if schemaMap == nil || len(schemaMap) == 0 {
		return nil, nil
	}

	schema := &genai.Schema{}

	// Type
	if typeStr, ok := schemaMap["type"].(string); ok {
		switch strings.ToLower(typeStr) {
		case "object":
			schema.Type = genai.TypeObject
		case "array":
			schema.Type = genai.TypeArray
		case "string":
			schema.Type = genai.TypeString
		case "number":
			schema.Type = genai.TypeNumber
		case "integer":
			schema.Type = genai.TypeInteger
		case "boolean":
			schema.Type = genai.TypeBoolean
		}
	}

	// Description
	if desc, ok := schemaMap["description"].(string); ok {
		schema.Description = desc
	}

	// Enum
	if enumVals, ok := schemaMap["enum"].([]interface{}); ok {
		for _, v := range enumVals {
			if s, ok := v.(string); ok {
				schema.Enum = append(schema.Enum, s)
			}
		}
	} else if enumVals, ok := schemaMap["enum"].([]string); ok {
		schema.Enum = enumVals
	}

	// Required
	if reqVals, ok := schemaMap["required"].([]interface{}); ok {
		for _, v := range reqVals {
			if s, ok := v.(string); ok {
				schema.Required = append(schema.Required, s)
			}
		}
	} else if reqVals, ok := schemaMap["required"].([]string); ok {
		schema.Required = reqVals
	}

	// Minimum/Maximum for integers
	if minVal, ok := schemaMap["minimum"].(int64); ok {
		f := float64(minVal)
		schema.Minimum = &f
	} else if minVal, ok := schemaMap["minimum"].(float64); ok {
		schema.Minimum = &minVal
	}
	if maxVal, ok := schemaMap["maximum"].(int64); ok {
		f := float64(maxVal)
		schema.Maximum = &f
	} else if maxVal, ok := schemaMap["maximum"].(float64); ok {
		schema.Maximum = &maxVal
	}

	// Items (for arrays)
	if itemsMap, ok := schemaMap["items"].(map[string]interface{}); ok {
		itemSchema, err := convertToGenaiSchema(itemsMap)
		if err != nil {
			return nil, fmt.Errorf("failed to convert items schema: %w", err)
		}
		schema.Items = itemSchema
	}

	// Properties (for objects)
	if propsMap, ok := schemaMap["properties"].(map[string]interface{}); ok {
		schema.Properties = make(map[string]*genai.Schema)
		for propName, propVal := range propsMap {
			if propMap, ok := propVal.(map[string]interface{}); ok {
				propSchema, err := convertToGenaiSchema(propMap)
				if err != nil {
					return nil, fmt.Errorf("failed to convert property '%s': %w", propName, err)
				}
				schema.Properties[propName] = propSchema
			}
		}
	}

	return schema, nil
}
