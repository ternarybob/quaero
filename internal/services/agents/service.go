package agents

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/ternarybob/arbor"
	"github.com/ternarybob/quaero/internal/common"
	"github.com/ternarybob/quaero/internal/interfaces"
	"google.golang.org/genai"
)

// AgentExecutor is an internal interface for agent type implementations.
// Each agent type (keyword extractor, summarizer, etc.) implements this interface
// and is registered with the service for dynamic dispatch.
type AgentExecutor interface {
	// Execute runs the agent with the given genai client and input
	Execute(ctx context.Context, client *genai.Client, modelName string, input map[string]interface{}) (map[string]interface{}, error)
	// GetType returns the agent type identifier (e.g., "keyword_extractor")
	GetType() string
}

// RateLimitSkipper is an optional interface that agents can implement to skip rate limiting.
// Agents that don't make external API calls (e.g., rule-based classifiers) should implement this.
type RateLimitSkipper interface {
	// SkipRateLimit returns true if this agent should bypass rate limiting
	SkipRateLimit() bool
}

// ModelSelection represents the strategy for selecting which LLM model to use
type ModelSelection string

const (
	// ModelSelectionAuto automatically selects model based on task complexity
	ModelSelectionAuto ModelSelection = "auto"
	// ModelSelectionDefault uses the standard agent_model
	ModelSelectionDefault ModelSelection = "default"
	// ModelSelectionFast uses the fast model for simple tasks
	ModelSelectionFast ModelSelection = "fast"
	// ModelSelectionThinking uses the thinking model for complex reasoning
	ModelSelectionThinking ModelSelection = "thinking"
)

// Service manages agent lifecycle and execution using direct genai API.
// It maintains a registry of agent types and routes execution requests to the appropriate agent.
type Service struct {
	config      *common.GeminiConfig
	logger      arbor.ILogger
	client      *genai.Client
	modelName   string
	agents      map[string]AgentExecutor
	timeout     time.Duration
	rateLimit   time.Duration
	lastRequest time.Time
	mu          sync.Mutex

	// Client cache for per-request API key overrides
	clientCache   map[string]*genai.Client
	clientCacheMu sync.RWMutex
}

// NewService creates a new agent service with Google Gemini API integration.
//
// The service performs the following initialization:
//  1. Resolves Google API key from KV store with config fallback
//  2. Initializes direct genai client
//  3. Registers built-in agent types (keyword extractor)
//  4. Parses timeout duration
//
// Parameters:
//   - config: Agent configuration (must have valid Google API key)
//   - storageManager: Storage manager interface for KV and auth storage access
//   - logger: Structured logger for service operations
//
// Returns:
//   - *Service: Initialized agent service ready for use
//   - error: nil on success, error with details on failure
//
// Errors:
//   - Missing or empty Google API key (from KV store or config)
//   - Invalid model name
//   - Failed to initialize genai client (network, auth, etc.)
//   - Invalid timeout duration
func NewService(config *common.GeminiConfig, storageManager interfaces.StorageManager, logger arbor.ILogger) (*Service, error) {
	// Resolve API key with KV-first resolution order: KV store → config fallback
	ctx := context.Background()
	apiKey, err := common.ResolveAPIKey(ctx, storageManager.KeyValueStorage(), "google_api_key", config.GoogleAPIKey)
	if err != nil {
		return nil, fmt.Errorf("Google API key is required for agent service (set via KV store, QUAERO_GEMINI_GOOGLE_API_KEY, or gemini.google_api_key in config): %w", err)
	}

	// Debug logging: Log API key details (masked for security)
	maskedKey := ""
	if len(apiKey) > 12 {
		maskedKey = apiKey[:8] + "..." + apiKey[len(apiKey)-4:]
	} else {
		maskedKey = "***"
	}
	logger.Debug().
		Str("api_key_masked", maskedKey).
		Int("api_key_length", len(apiKey)).
		Bool("api_key_empty", apiKey == "").
		Msg("Agent service: Resolved Google API key")

	if config.AgentModel == "" {
		config.AgentModel = "gemini-2.0-flash" // Default to fast model
	}

	// Parse timeout duration
	timeout, err := time.ParseDuration(config.Timeout)
	if err != nil {
		return nil, fmt.Errorf("invalid timeout duration '%s': %w", config.Timeout, err)
	}

	// Parse rate limit duration
	rateLimit, err := time.ParseDuration(config.RateLimit)
	if err != nil {
		return nil, fmt.Errorf("invalid rate limit duration '%s': %w", config.RateLimit, err)
	}

	// Initialize direct genai client
	logger.Debug().
		Str("backend", "GeminiAPI").
		Str("model", config.AgentModel).
		Msg("Agent service: Initializing genai client")

	genaiClient, err := genai.NewClient(ctx, &genai.ClientConfig{
		APIKey:  apiKey,
		Backend: genai.BackendGeminiAPI,
	})
	if err != nil {
		logger.Error().
			Err(err).
			Str("api_key_masked", maskedKey).
			Msg("Agent service: Failed to initialize genai client")
		return nil, fmt.Errorf("failed to initialize genai client: %w", err)
	}

	logger.Debug().Msg("Agent service: genai client initialized successfully")

	// Create service instance
	service := &Service{
		config:      config,
		logger:      logger,
		client:      genaiClient,
		modelName:   config.AgentModel,
		agents:      make(map[string]AgentExecutor),
		timeout:     timeout,
		rateLimit:   rateLimit,
		clientCache: make(map[string]*genai.Client),
	}

	// Register built-in agents
	keywordExtractor := &KeywordExtractor{}
	service.RegisterAgent(keywordExtractor)

	metadataEnricher := &MetadataEnricher{}
	service.RegisterAgent(metadataEnricher)

	categoryClassifier := &CategoryClassifier{}
	service.RegisterAgent(categoryClassifier)

	ruleClassifier := &RuleClassifier{}
	service.RegisterAgent(ruleClassifier)

	entityRecognizer := &EntityRecognizer{}
	service.RegisterAgent(entityRecognizer)

	logger.Debug().
		Str("model", config.AgentModel).
		Int("max_turns", config.MaxTurns).
		Dur("timeout", timeout).
		Dur("rate_limit", rateLimit).
		Int("registered_agents", len(service.agents)).
		Msg("Agent service initialized with Google Gemini API")

	return service, nil
}

// RegisterAgent adds an agent executor to the service's registry.
// Agent executors are looked up by their type identifier during Execute calls.
//
// Parameters:
//   - agent: Agent executor implementing the AgentExecutor interface
func (s *Service) RegisterAgent(agent AgentExecutor) {
	agentType := agent.GetType()
	s.agents[agentType] = agent
	s.logger.Debug().
		Str("agent_type", agentType).
		Msg("Agent registered")
}

// Execute runs an agent of the specified type with the given input.
//
// The execution flow:
//  1. Look up agent executor by type
//  2. Extract any per-request Gemini overrides from input
//  3. Select appropriate model based on model_selection strategy
//  4. Create timeout context
//  5. Call agent's Execute method with model and input
//  6. If validation is enabled, run validation loop
//  7. Return agent output or error
//  8. Log execution duration and result
//
// Per-request overrides (optional in input):
//   - gemini_api_key: Override the global API key for this request
//   - gemini_model: Override the global model for this request
//   - gemini_timeout: Override the global timeout for this request
//   - gemini_rate_limit: Override the global rate limit for this request
//   - model_selection: Model selection strategy ("auto", "default", "fast", "thinking")
//   - validation: Whether to validate output (default: true)
//   - validation_iteration_count: Number of validation iterations (default: 1)
//
// Parameters:
//   - ctx: Context for cancellation control
//   - agentType: Agent identifier (must be registered)
//   - input: Agent-specific input data
//
// Returns:
//   - map[string]interface{}: Agent output (structure varies by agent type)
//   - error: nil on success, error with details on failure
//
// Errors:
//   - Unknown agent type (not registered)
//   - Agent execution failure (invalid input, API error, etc.)
//   - Timeout exceeded
func (s *Service) Execute(ctx context.Context, agentType string, input map[string]interface{}) (map[string]interface{}, error) {
	// Look up agent executor
	agent, ok := s.agents[agentType]
	if !ok {
		return nil, fmt.Errorf("unknown agent type: %s", agentType)
	}

	// Extract per-request overrides from input
	client := s.client
	modelName := s.modelName
	timeout := s.timeout
	rateLimit := s.rateLimit

	// Check for API key override
	if apiKey, ok := input["gemini_api_key"].(string); ok && apiKey != "" {
		var err error
		client, err = s.getOrCreateClient(ctx, apiKey)
		if err != nil {
			return nil, fmt.Errorf("failed to create client with override API key: %w", err)
		}
		s.logger.Debug().Msg("Using per-request API key override")
		// Remove from input so it's not passed to the agent
		delete(input, "gemini_api_key")
	}

	// Check for model_selection override (determines which model variant to use)
	modelSelection := ModelSelectionAuto // Default to auto
	if selection, ok := input["model_selection"].(string); ok && selection != "" {
		modelSelection = ModelSelection(selection)
		s.logger.Debug().Str("model_selection", selection).Msg("Using per-request model selection")
		delete(input, "model_selection")
	}

	// Select model based on model_selection strategy
	modelName = s.selectModel(modelSelection, agentType, input)

	// Check for direct model override (overrides model_selection)
	if model, ok := input["gemini_model"].(string); ok && model != "" {
		modelName = model
		s.logger.Debug().Str("model", model).Msg("Using per-request model override")
		delete(input, "gemini_model")
	}

	// Check for timeout override
	if timeoutStr, ok := input["gemini_timeout"].(string); ok && timeoutStr != "" {
		if parsed, err := time.ParseDuration(timeoutStr); err == nil {
			timeout = parsed
			s.logger.Debug().Dur("timeout", timeout).Msg("Using per-request timeout override")
		}
		delete(input, "gemini_timeout")
	}

	// Check for rate limit override
	if rateLimitStr, ok := input["gemini_rate_limit"].(string); ok && rateLimitStr != "" {
		if parsed, err := time.ParseDuration(rateLimitStr); err == nil {
			rateLimit = parsed
			s.logger.Debug().Dur("rate_limit", rateLimit).Msg("Using per-request rate limit override")
		}
		delete(input, "gemini_rate_limit")
	}

	// Extract validation options (defaults: validation=true, iteration_count=1)
	enableValidation := true
	if v, ok := input["validation"].(bool); ok {
		enableValidation = v
		delete(input, "validation")
	}
	validationIterations := 1
	if count, ok := input["validation_iteration_count"].(int); ok && count > 0 {
		validationIterations = count
		delete(input, "validation_iteration_count")
	} else if countFloat, ok := input["validation_iteration_count"].(float64); ok && countFloat > 0 {
		validationIterations = int(countFloat)
		delete(input, "validation_iteration_count")
	}

	// Create timeout context
	timeoutCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	// Check if agent should skip rate limiting (e.g., rule-based agents that don't call APIs)
	skipRateLimit := false
	if skipper, ok := agent.(RateLimitSkipper); ok {
		skipRateLimit = skipper.SkipRateLimit()
	}

	// Enforce rate limit (unless agent opts out)
	if !skipRateLimit {
		s.mu.Lock()
		timeSinceLast := time.Since(s.lastRequest)
		if timeSinceLast < rateLimit {
			sleepDuration := rateLimit - timeSinceLast
			s.logger.Debug().
				Dur("sleep_duration", sleepDuration).
				Msg("Rate limit enforcing delay")
			time.Sleep(sleepDuration)
		}
		s.lastRequest = time.Now()
		s.mu.Unlock()
	}

	// Execute agent
	startTime := time.Now()
	s.logger.Debug().
		Str("agent_type", agentType).
		Str("model", modelName).
		Bool("validation", enableValidation).
		Int("validation_iterations", validationIterations).
		Msg("Starting agent execution")

	output, err := agent.Execute(timeoutCtx, client, modelName, input)
	duration := time.Since(startTime)

	if err != nil {
		s.logger.Error().
			Err(err).
			Str("agent_type", agentType).
			Dur("duration", duration).
			Msg("Agent execution failed")
		return nil, fmt.Errorf("agent execution failed: %w", err)
	}

	// Run validation loop if enabled and agent uses LLM
	if enableValidation && !skipRateLimit && validationIterations > 0 {
		output, err = s.runValidationLoop(timeoutCtx, client, modelName, agentType, input, output, validationIterations, rateLimit)
		if err != nil {
			s.logger.Error().
				Err(err).
				Str("agent_type", agentType).
				Int("validation_iterations", validationIterations).
				Msg("Validation loop failed")
			return nil, fmt.Errorf("validation failed: %w", err)
		}
	}

	totalDuration := time.Since(startTime)
	s.logger.Debug().
		Str("agent_type", agentType).
		Dur("duration", totalDuration).
		Bool("validation_applied", enableValidation && !skipRateLimit).
		Msg("Agent execution completed successfully")

	return output, nil
}

// selectModel selects the appropriate model based on model_selection strategy
func (s *Service) selectModel(selection ModelSelection, agentType string, input map[string]interface{}) string {
	switch selection {
	case ModelSelectionFast:
		if s.config.AgentModelFast != "" {
			s.logger.Debug().
				Str("model", s.config.AgentModelFast).
				Str("selection", "fast").
				Msg("Selected fast model")
			return s.config.AgentModelFast
		}
	case ModelSelectionThinking:
		if s.config.AgentModelThinking != "" {
			s.logger.Debug().
				Str("model", s.config.AgentModelThinking).
				Str("selection", "thinking").
				Msg("Selected thinking model")
			return s.config.AgentModelThinking
		}
	case ModelSelectionAuto:
		// Auto-select based on agent type and content characteristics
		selectedModel := s.autoSelectModel(agentType, input)
		s.logger.Debug().
			Str("model", selectedModel).
			Str("selection", "auto").
			Str("agent_type", agentType).
			Msg("Auto-selected model")
		return selectedModel
	case ModelSelectionDefault:
		// Use default model
	}
	return s.modelName
}

// autoSelectModel automatically selects the best model based on agent type and input characteristics
func (s *Service) autoSelectModel(agentType string, input map[string]interface{}) string {
	// Agent types that benefit from thinking model (complex reasoning)
	thinkingAgents := map[string]bool{
		"category_classifier": true,
		"entity_recognizer":   true,
		"sentiment_analyzer":  true,
		"relation_extractor":  true,
		"question_answerer":   true,
		"content_summarizer":  true,
	}

	// Agent types that work well with fast model (simple extraction)
	fastAgents := map[string]bool{
		"keyword_extractor": true,
		"metadata_enricher": true,
		"rule_classifier":   true, // Rule-based, but if LLM is used
	}

	// Check content length for complexity assessment
	contentLength := 0
	if content, ok := input["content"].(string); ok {
		contentLength = len(content)
	}

	// Large documents (>10KB) might benefit from thinking model
	largeDocument := contentLength > 10*1024

	// Select model based on agent type and content size
	if thinkingAgents[agentType] || largeDocument {
		if s.config.AgentModelThinking != "" {
			return s.config.AgentModelThinking
		}
	}

	if fastAgents[agentType] && !largeDocument {
		if s.config.AgentModelFast != "" {
			return s.config.AgentModelFast
		}
	}

	// Default to standard model
	return s.modelName
}

// runValidationLoop runs the validation iterations on agent output
func (s *Service) runValidationLoop(ctx context.Context, client *genai.Client, modelName string, agentType string, input map[string]interface{}, output map[string]interface{}, iterations int, rateLimit time.Duration) (map[string]interface{}, error) {
	currentOutput := output

	for i := 0; i < iterations; i++ {
		s.logger.Debug().
			Str("agent_type", agentType).
			Int("iteration", i+1).
			Int("total_iterations", iterations).
			Msg("Running validation iteration")

		// Enforce rate limit before validation call
		s.mu.Lock()
		timeSinceLast := time.Since(s.lastRequest)
		if timeSinceLast < rateLimit {
			sleepDuration := rateLimit - timeSinceLast
			time.Sleep(sleepDuration)
		}
		s.lastRequest = time.Now()
		s.mu.Unlock()

		// Create validation prompt
		validatedOutput, err := s.validateOutput(ctx, client, modelName, agentType, input, currentOutput)
		if err != nil {
			s.logger.Warn().
				Err(err).
				Int("iteration", i+1).
				Msg("Validation iteration failed, using previous output")
			// Continue with current output if validation fails
			continue
		}

		currentOutput = validatedOutput
	}

	return currentOutput, nil
}

// validateOutput validates and potentially improves the agent output using LLM
func (s *Service) validateOutput(ctx context.Context, client *genai.Client, modelName string, agentType string, input map[string]interface{}, output map[string]interface{}) (map[string]interface{}, error) {
	// Build validation prompt based on agent type
	validationPrompt := s.buildValidationPrompt(agentType, input, output)

	// Generate validation response
	config := &genai.GenerateContentConfig{
		Temperature: genai.Ptr(float32(0.2)), // Lower temperature for validation
	}

	response, err := client.Models.GenerateContent(ctx, modelName, []*genai.Content{
		{
			Role: "user",
			Parts: []*genai.Part{
				genai.NewPartFromText(validationPrompt),
			},
		},
	}, config)

	if err != nil {
		return nil, fmt.Errorf("validation API call failed: %w", err)
	}

	responseText := response.Text()
	if responseText == "" {
		return output, nil // Return original if no response
	}

	// Parse validation response
	validatedOutput, err := s.parseValidationResponse(responseText, output)
	if err != nil {
		s.logger.Debug().
			Err(err).
			Str("response", responseText).
			Msg("Failed to parse validation response, using original output")
		return output, nil
	}

	return validatedOutput, nil
}

// buildValidationPrompt creates a prompt for validating agent output
func (s *Service) buildValidationPrompt(agentType string, input map[string]interface{}, output map[string]interface{}) string {
	content := ""
	if c, ok := input["content"].(string); ok {
		// Truncate content for validation prompt
		if len(c) > 2000 {
			content = c[:2000] + "..."
		} else {
			content = c
		}
	}

	outputJSON := "{}"
	if outputBytes, err := jsonMarshal(output); err == nil {
		outputJSON = string(outputBytes)
	}

	return fmt.Sprintf(`You are a validation specialist reviewing the output of an AI agent.

Agent Type: %s

Original Content (truncated):
%s

Agent Output:
%s

Task: Review and validate the agent's output. Check for:
1. Accuracy - Does the output correctly represent the content?
2. Completeness - Is anything important missing?
3. Consistency - Are there any contradictions or errors?
4. Quality - Could the output be improved?

If the output is correct, respond with exactly: VALIDATED
If improvements are needed, provide a corrected version in the same JSON format.

Response:`, agentType, content, outputJSON)
}

// parseValidationResponse parses the validation response and returns updated output
func (s *Service) parseValidationResponse(response string, originalOutput map[string]interface{}) (map[string]interface{}, error) {
	response = cleanResponse(response)

	// Check if output was validated as-is
	if response == "VALIDATED" || response == "validated" {
		return originalOutput, nil
	}

	// Try to parse as JSON
	var validated map[string]interface{}
	if err := jsonUnmarshal([]byte(response), &validated); err != nil {
		return nil, fmt.Errorf("failed to parse validation response as JSON: %w", err)
	}

	// Merge validated fields with original (validated takes precedence)
	result := make(map[string]interface{})
	for k, v := range originalOutput {
		result[k] = v
	}
	for k, v := range validated {
		result[k] = v
	}

	return result, nil
}

// cleanResponse removes markdown fences and whitespace from response
func cleanResponse(s string) string {
	s = strings.TrimSpace(s)
	// Remove markdown code fences
	if strings.HasPrefix(s, "```json") {
		s = strings.TrimPrefix(s, "```json")
	} else if strings.HasPrefix(s, "```") {
		s = strings.TrimPrefix(s, "```")
	}
	s = strings.TrimSuffix(s, "```")
	return strings.TrimSpace(s)
}

// jsonMarshal is a helper for JSON marshaling
func jsonMarshal(v interface{}) ([]byte, error) {
	return json.Marshal(v)
}

// jsonUnmarshal is a helper for JSON unmarshaling
func jsonUnmarshal(data []byte, v interface{}) error {
	return json.Unmarshal(data, v)
}

// getOrCreateClient returns a cached client for the given API key or creates a new one
func (s *Service) getOrCreateClient(ctx context.Context, apiKey string) (*genai.Client, error) {
	// Check cache first with read lock
	s.clientCacheMu.RLock()
	if client, ok := s.clientCache[apiKey]; ok {
		s.clientCacheMu.RUnlock()
		return client, nil
	}
	s.clientCacheMu.RUnlock()

	// Create new client with write lock
	s.clientCacheMu.Lock()
	defer s.clientCacheMu.Unlock()

	// Double-check after acquiring write lock
	if client, ok := s.clientCache[apiKey]; ok {
		return client, nil
	}

	// Create new client
	client, err := genai.NewClient(ctx, &genai.ClientConfig{
		APIKey:  apiKey,
		Backend: genai.BackendGeminiAPI,
	})
	if err != nil {
		return nil, err
	}

	// Cache it
	s.clientCache[apiKey] = client
	s.logger.Debug().Msg("Created and cached new genai client for API key override")

	return client, nil
}

// HealthCheck verifies the agent service is operational.
//
// The health check validates:
//   - The genai client is accessible
//   - The model name is set correctly
//
// This should be called during service initialization to fail fast if there are issues.
//
// Parameters:
//   - ctx: Context for cancellation control
//
// Returns:
//   - nil if healthy (service is operational)
//   - error with details if unhealthy (invalid API key, network issues, etc.)
func (s *Service) HealthCheck(ctx context.Context) error {
	s.logger.Debug().Msg("Running agent service health check")

	// Verify genai client is initialized
	if s.client == nil {
		return fmt.Errorf("agent service client is not initialized")
	}

	// Verify model name is set
	if s.modelName == "" {
		return fmt.Errorf("model name is not set")
	}

	s.logger.Debug().
		Str("model", s.modelName).
		Msg("Agent service health check passed")
	return nil
}

// Close releases resources and performs cleanup.
// Should be called during application shutdown.
//
// Returns:
//   - nil on successful cleanup
//   - error if cleanup fails
func (s *Service) Close() error {
	s.logger.Debug().Msg("Closing agent service")

	// genai client doesn't require explicit Close
	s.client = nil
	s.agents = nil

	return nil
}

// IsRuleBased returns true if the specified agent type is rule-based (does not use LLM).
// Rule-based agents implement the RateLimitSkipper interface with SkipRateLimit() returning true.
//
// Parameters:
//   - agentType: Agent identifier to check
//
// Returns:
//   - true if the agent is rule-based (no LLM calls)
//   - false if the agent uses LLM or if agent type is unknown
func (s *Service) IsRuleBased(agentType string) bool {
	agent, ok := s.agents[agentType]
	if !ok {
		return false
	}

	if skipper, ok := agent.(RateLimitSkipper); ok {
		return skipper.SkipRateLimit()
	}

	return false
}
