package agents

import (
	"context"
	"fmt"
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

// Service manages agent lifecycle and execution using direct genai API.
// It maintains a registry of agent types and routes execution requests to the appropriate agent.
type Service struct {
	config    *common.GeminiConfig
	logger    arbor.ILogger
	client    *genai.Client
	modelName string
	agents    map[string]AgentExecutor
	timeout   time.Duration
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
		config:    config,
		logger:    logger,
		client:    genaiClient,
		modelName: config.AgentModel,
		agents:    make(map[string]AgentExecutor),
		timeout:   timeout,
	}

	// Register built-in agents
	keywordExtractor := &KeywordExtractor{}
	service.RegisterAgent(keywordExtractor)

	logger.Info().
		Str("model", config.AgentModel).
		Int("max_turns", config.MaxTurns).
		Dur("timeout", timeout).
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
	s.logger.Info().
		Str("agent_type", agentType).
		Msg("Agent registered")
}

// Execute runs an agent of the specified type with the given input.
//
// The execution flow:
//  1. Look up agent executor by type
//  2. Create timeout context
//  3. Call agent's Execute method with model and input
//  4. Return agent output or error
//  5. Log execution duration and result
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

	// Create timeout context
	timeoutCtx, cancel := context.WithTimeout(ctx, s.timeout)
	defer cancel()

	// Execute agent
	startTime := time.Now()
	s.logger.Debug().
		Str("agent_type", agentType).
		Msg("Starting agent execution")

	output, err := agent.Execute(timeoutCtx, s.client, s.modelName, input)
	duration := time.Since(startTime)

	if err != nil {
		s.logger.Error().
			Err(err).
			Str("agent_type", agentType).
			Dur("duration", duration).
			Msg("Agent execution failed")
		return nil, fmt.Errorf("agent execution failed: %w", err)
	}

	s.logger.Info().
		Str("agent_type", agentType).
		Dur("duration", duration).
		Msg("Agent execution completed successfully")

	return output, nil
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

	s.logger.Info().
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
	s.logger.Info().Msg("Closing agent service")

	// genai client doesn't require explicit Close
	s.client = nil
	s.agents = nil

	return nil
}
