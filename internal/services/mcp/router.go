package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/ternarybob/arbor"
	"github.com/ternarybob/quaero/internal/interfaces"
)

// ToolRouter orchestrates tool execution for the agent
type ToolRouter struct {
	documentService *DocumentService
	storage         interfaces.DocumentStorage
	logger          arbor.ILogger
}

// NewToolRouter creates a new MCP tool router
func NewToolRouter(
	storage interfaces.DocumentStorage,
	logger arbor.ILogger,
) *ToolRouter {
	return &ToolRouter{
		documentService: NewDocumentService(storage, logger),
		storage:         storage,
		logger:          logger,
	}
}

// ExecuteTool executes a tool call from the agent
// Returns a ToolResponse with the result
func (r *ToolRouter) ExecuteTool(ctx context.Context, toolUse *ToolUse) *ToolResponse {
	startTime := time.Now()

	r.logger.Info().
		Str("tool", toolUse.Name).
		Str("tool_use_id", toolUse.ID).
		Msg("Executing tool")

	// Execute the tool via the document service
	result, err := r.documentService.CallTool(ctx, toolUse.Name, toolUse.Arguments)

	duration := time.Since(startTime)

	if err != nil {
		r.logger.Error().
			Err(err).
			Str("tool", toolUse.Name).
			Str("tool_use_id", toolUse.ID).
			Dur("duration", duration).
			Msg("Tool execution failed")

		return &ToolResponse{
			ToolUseID: toolUse.ID,
			Content:   fmt.Sprintf("Error executing tool: %v", err),
			IsError:   true,
		}
	}

	// Convert ToolResult to ToolResponse
	response := r.convertToolResult(toolUse.ID, result)

	r.logger.Info().
		Str("tool", toolUse.Name).
		Str("tool_use_id", toolUse.ID).
		Int("content_length", len(response.Content)).
		Dur("duration", duration).
		Msg("Tool execution complete")

	return response
}

// convertToolResult converts MCP ToolResult to ToolResponse
func (r *ToolRouter) convertToolResult(toolUseID string, result *ToolResult) *ToolResponse {
	// Combine all content blocks into a single string
	var content string
	for i, block := range result.Content {
		if i > 0 {
			content += "\n\n"
		}
		if block.Type == "text" {
			content += block.Text
		} else if block.Type == "data" {
			content += fmt.Sprintf("[Binary data: %d bytes]", len(block.Data))
		}
	}

	return &ToolResponse{
		ToolUseID: toolUseID,
		Content:   content,
		IsError:   result.IsError,
	}
}

// GetAvailableTools returns a list of all available tools for the agent
func (r *ToolRouter) GetAvailableTools(ctx context.Context) ([]Tool, error) {
	toolList, err := r.documentService.ListTools(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list tools: %w", err)
	}

	return toolList.Tools, nil
}

// FormatToolsForPrompt formats the tool list for inclusion in the agent system prompt
func (r *ToolRouter) FormatToolsForPrompt(ctx context.Context) (string, error) {
	tools, err := r.GetAvailableTools(ctx)
	if err != nil {
		return "", err
	}

	var prompt string
	prompt += "# Available Tools\n\n"
	prompt += "You have access to the following tools. To use a tool, respond with a JSON object in this format:\n\n"
	prompt += "```json\n"
	prompt += "{\n"
	prompt += "  \"tool_use\": {\n"
	prompt += "    \"id\": \"unique_id\",\n"
	prompt += "    \"name\": \"tool_name\",\n"
	prompt += "    \"arguments\": {\"arg1\": \"value1\"}\n"
	prompt += "  }\n"
	prompt += "}\n"
	prompt += "```\n\n"

	for _, tool := range tools {
		prompt += fmt.Sprintf("## %s\n\n", tool.Name)
		prompt += fmt.Sprintf("%s\n\n", tool.Description)

		// Format input schema
		schemaJSON, err := json.MarshalIndent(tool.InputSchema, "", "  ")
		if err != nil {
			continue
		}
		prompt += "**Input Schema:**\n"
		prompt += "```json\n"
		prompt += string(schemaJSON)
		prompt += "\n```\n\n"
	}

	return prompt, nil
}
