package chat

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/ternarybob/arbor"
	"github.com/ternarybob/quaero/internal/interfaces"
	"github.com/ternarybob/quaero/internal/services/mcp"
)

// AgentConfig configures the agent conversation loop
type AgentConfig struct {
	MaxTurns      int           // Maximum agent turns before stopping
	MaxToolCalls  int           // Maximum tool calls per conversation
	Timeout       time.Duration // Overall timeout for agent conversation
	EnableLogging bool          // Whether to log agent reasoning
}

// DefaultAgentConfig returns sensible defaults
func DefaultAgentConfig() *AgentConfig {
	return &AgentConfig{
		MaxTurns:      10,
		MaxToolCalls:  15,
		Timeout:       5 * time.Minute,
		EnableLogging: true,
	}
}

// AgentLoop orchestrates the agent conversation
type AgentLoop struct {
	toolRouter *mcp.ToolRouter
	llmService interfaces.LLMService
	logger     arbor.ILogger
	config     *AgentConfig
}

// NewAgentLoop creates a new agent conversation loop
func NewAgentLoop(
	toolRouter *mcp.ToolRouter,
	llmService interfaces.LLMService,
	logger arbor.ILogger,
	config *AgentConfig,
) *AgentLoop {
	if config == nil {
		config = DefaultAgentConfig()
	}

	return &AgentLoop{
		toolRouter: toolRouter,
		llmService: llmService,
		logger:     logger,
		config:     config,
	}
}

// Execute runs the agent conversation loop
// It streams intermediate thoughts and tool results via the streamFunc callback
func (a *AgentLoop) Execute(
	ctx context.Context,
	userMessage string,
	streamFunc func(*mcp.StreamingMessage) error,
) (string, error) {
	startTime := time.Now()

	// Apply timeout
	ctx, cancel := context.WithTimeout(ctx, a.config.Timeout)
	defer cancel()

	// Initialize agent state
	state := &mcp.AgentState{
		ConversationID: uuid.New().String(),
		Messages:       []mcp.AgentMessage{},
		Thoughts:       []mcp.AgentThought{},
		ToolCalls:      []mcp.ToolUse{},
		ToolResponses:  []mcp.ToolResponse{},
		TurnCount:      0,
		MaxTurns:       a.config.MaxTurns,
		Complete:       false,
	}

	// Build agent system prompt with tool descriptions
	systemPrompt, err := a.buildSystemPrompt(ctx)
	if err != nil {
		return "", fmt.Errorf("failed to build system prompt: %w", err)
	}

	// Add system message
	state.Messages = append(state.Messages, mcp.AgentMessage{
		Role:    "system",
		Content: systemPrompt,
	})

	// Add user message
	state.Messages = append(state.Messages, mcp.AgentMessage{
		Role:    "user",
		Content: userMessage,
	})

	a.logger.Debug().
		Str("conversation_id", state.ConversationID).
		Str("user_message", userMessage).
		Msg("Starting agent conversation loop")

	// Main agent loop
	for state.TurnCount < state.MaxTurns && !state.Complete {
		select {
		case <-ctx.Done():
			return "", fmt.Errorf("agent loop timeout after %v", time.Since(startTime))
		default:
		}

		state.TurnCount++

		a.logger.Debug().
			Int("turn", state.TurnCount).
			Int("messages", len(state.Messages)).
			Msg("Agent turn start")

		// Get LLM response
		llmResponse, err := a.callLLM(ctx, state)
		if err != nil {
			return "", fmt.Errorf("LLM call failed on turn %d: %w", state.TurnCount, err)
		}

		// Parse LLM response for tool calls
		toolUse, isFinalAnswer := a.parseResponse(llmResponse)

		if isFinalAnswer {
			// Agent is done - return final answer
			state.Complete = true

			// Stream final answer
			if streamFunc != nil {
				streamFunc(&mcp.StreamingMessage{
					Type:      "final_answer",
					Content:   llmResponse,
					Timestamp: time.Now().Format(time.RFC3339),
				})
			}

			a.logger.Debug().
				Str("conversation_id", state.ConversationID).
				Int("turns", state.TurnCount).
				Int("tool_calls", len(state.ToolCalls)).
				Dur("duration", time.Since(startTime)).
				Msg("Agent conversation complete")

			return llmResponse, nil
		}

		if toolUse != nil {
			// Agent wants to use a tool
			a.logger.Debug().
				Str("tool", toolUse.Name).
				Str("tool_use_id", toolUse.ID).
				Msg("Agent requested tool use")

			// Check tool call limit
			if len(state.ToolCalls) >= a.config.MaxToolCalls {
				return "", fmt.Errorf("exceeded maximum tool calls (%d)", a.config.MaxToolCalls)
			}

			// Stream the agent's thought (the text before the tool call)
			if streamFunc != nil {
				thoughtContent := a.extractThought(llmResponse)
				streamFunc(&mcp.StreamingMessage{
					Type:      "thought",
					Content:   thoughtContent,
					Timestamp: time.Now().Format(time.RFC3339),
				})

				// Stream the action
				streamFunc(&mcp.StreamingMessage{
					Type:      "action",
					Content:   fmt.Sprintf("Using tool: %s", toolUse.Name),
					ToolUse:   toolUse,
					Timestamp: time.Now().Format(time.RFC3339),
				})
			}

			// Execute the tool
			toolResponse := a.toolRouter.ExecuteTool(ctx, toolUse)

			// Record tool call and response
			state.ToolCalls = append(state.ToolCalls, *toolUse)
			state.ToolResponses = append(state.ToolResponses, *toolResponse)

			// Stream the observation
			if streamFunc != nil {
				streamFunc(&mcp.StreamingMessage{
					Type:       "observation",
					Content:    fmt.Sprintf("Tool result: %s", truncate(toolResponse.Content, 200)),
					ToolResult: toolResponse,
					Timestamp:  time.Now().Format(time.RFC3339),
					Metadata: map[string]interface{}{
						"is_error":       toolResponse.IsError,
						"content_length": len(toolResponse.Content),
					},
				})
			}

			// Add assistant's tool use to conversation
			state.Messages = append(state.Messages, mcp.AgentMessage{
				Role:    "assistant",
				Content: llmResponse,
			})

			// Add tool result to conversation as a user message
			toolResultMsg := fmt.Sprintf("Tool '%s' returned:\n\n%s", toolUse.Name, toolResponse.Content)
			if toolResponse.IsError {
				toolResultMsg = fmt.Sprintf("Tool '%s' error:\n\n%s", toolUse.Name, toolResponse.Content)
			}

			state.Messages = append(state.Messages, mcp.AgentMessage{
				Role:    "user",
				Content: toolResultMsg,
			})

		} else {
			// No tool call detected, but also not marked as final answer
			// Treat as final answer anyway
			state.Complete = true

			if streamFunc != nil {
				streamFunc(&mcp.StreamingMessage{
					Type:      "final_answer",
					Content:   llmResponse,
					Timestamp: time.Now().Format(time.RFC3339),
				})
			}

			a.logger.Debug().
				Str("conversation_id", state.ConversationID).
				Int("turns", state.TurnCount).
				Dur("duration", time.Since(startTime)).
				Msg("Agent conversation complete (implicit final answer)")

			return llmResponse, nil
		}
	}

	// If we exit the loop without completing, return error
	if !state.Complete {
		return "", fmt.Errorf("agent did not complete within %d turns", a.config.MaxTurns)
	}

	return "", fmt.Errorf("agent loop exited unexpectedly")
}

// buildSystemPrompt constructs the full system prompt with tool descriptions
func (a *AgentLoop) buildSystemPrompt(ctx context.Context) (string, error) {
	toolsSection, err := a.toolRouter.FormatToolsForPrompt(ctx)
	if err != nil {
		return "", fmt.Errorf("failed to format tools: %w", err)
	}

	return AgentSystemPromptBase + "\n\n" + toolsSection, nil
}

// callLLM sends the conversation to the LLM and gets a response
func (a *AgentLoop) callLLM(ctx context.Context, state *mcp.AgentState) (string, error) {
	// Convert agent messages to LLM messages
	messages := make([]interfaces.Message, len(state.Messages))
	for i, msg := range state.Messages {
		messages[i] = interfaces.Message{
			Role:    msg.Role,
			Content: msg.Content,
		}
	}

	// Call LLM
	response, err := a.llmService.Chat(ctx, messages)
	if err != nil {
		return "", err
	}

	return response, nil
}

// parseResponse extracts tool use from LLM response
// Returns (toolUse, isFinalAnswer)
func (a *AgentLoop) parseResponse(response string) (*mcp.ToolUse, bool) {
	// Look for JSON tool_use block
	jsonPattern := regexp.MustCompile("(?s)```json\\s*\\{\\s*\"tool_use\"\\s*:\\s*(\\{[^}]*\\})\\s*\\}\\s*```")
	matches := jsonPattern.FindStringSubmatch(response)

	if len(matches) > 1 {
		// Found a tool use
		var toolUseWrapper struct {
			ToolUse mcp.ToolUse `json:"tool_use"`
		}

		fullJSON := fmt.Sprintf(`{"tool_use":%s}`, matches[1])
		if err := json.Unmarshal([]byte(fullJSON), &toolUseWrapper); err == nil {
			return &toolUseWrapper.ToolUse, false
		}
	}

	// Alternative: Look for standalone JSON block
	altPattern := regexp.MustCompile("(?s)```json\\s*(\\{.*?\"tool_use\".*?\\})\\s*```")
	altMatches := altPattern.FindStringSubmatch(response)

	if len(altMatches) > 1 {
		var toolUseWrapper struct {
			ToolUse mcp.ToolUse `json:"tool_use"`
		}

		if err := json.Unmarshal([]byte(altMatches[1]), &toolUseWrapper); err == nil {
			return &toolUseWrapper.ToolUse, false
		}
	}

	// No tool use found - check if it's a final answer
	// Consider it final if there's substantive text and no tool call
	if len(strings.TrimSpace(response)) > 20 {
		return nil, true
	}

	return nil, false
}

// extractThought extracts the text before a tool call JSON block
func (a *AgentLoop) extractThought(response string) string {
	jsonPattern := regexp.MustCompile("(?s)```json")
	loc := jsonPattern.FindStringIndex(response)

	if loc != nil && loc[0] > 0 {
		thought := strings.TrimSpace(response[:loc[0]])
		return thought
	}

	return response
}

// truncate truncates a string to maxLen characters
func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
