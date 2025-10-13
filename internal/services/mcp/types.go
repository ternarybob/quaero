package mcp

// MCP Protocol Types (JSON-RPC 2.0)

// Resource represents an MCP resource
type Resource struct {
	URI         string `json:"uri"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	MimeType    string `json:"mimeType,omitempty"`
}

// ResourceList represents a list of available resources
type ResourceList struct {
	Resources []Resource `json:"resources"`
}

// ResourceContent represents the content of a resource
type ResourceContent struct {
	URI      string `json:"uri"`
	MimeType string `json:"mimeType"`
	Text     string `json:"text,omitempty"`
	Blob     []byte `json:"blob,omitempty"`
}

// Tool represents an MCP tool
type Tool struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	InputSchema map[string]interface{} `json:"inputSchema"`
}

// ToolList represents a list of available tools
type ToolList struct {
	Tools []Tool `json:"tools"`
}

// ToolResult represents the result of a tool call
type ToolResult struct {
	Content []ContentBlock `json:"content"`
	IsError bool           `json:"isError,omitempty"`
}

// ContentBlock represents a block of content
type ContentBlock struct {
	Type string `json:"type"`
	Text string `json:"text,omitempty"`
	Data []byte `json:"data,omitempty"`
}

// JSONRPCRequest represents a JSON-RPC 2.0 request
type JSONRPCRequest struct {
	JSONRPC string                 `json:"jsonrpc"`
	ID      interface{}            `json:"id"`
	Method  string                 `json:"method"`
	Params  map[string]interface{} `json:"params,omitempty"`
}

// JSONRPCResponse represents a JSON-RPC 2.0 response
type JSONRPCResponse struct {
	JSONRPC string      `json:"jsonrpc"`
	ID      interface{} `json:"id"`
	Result  interface{} `json:"result,omitempty"`
	Error   *RPCError   `json:"error,omitempty"`
}

// RPCError represents a JSON-RPC error
type RPCError struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

// Error codes
const (
	ParseError     = -32700
	InvalidRequest = -32600
	MethodNotFound = -32601
	InvalidParams  = -32602
	InternalError  = -32603
)

// Agent-specific types for conversation loop

// AgentMessage represents a message in the agent conversation
type AgentMessage struct {
	Role    string `json:"role"`    // "user", "assistant", "system"
	Content string `json:"content"` // Message content
}

// AgentThought represents the agent's internal reasoning
type AgentThought struct {
	Type    string   `json:"type"`               // "thought", "action", "observation", "final_answer"
	Content string   `json:"content"`            // The thought or action content
	ToolUse *ToolUse `json:"tool_use,omitempty"` // Tool being called
}

// ToolUse represents a tool call by the agent
type ToolUse struct {
	ID        string                 `json:"id"`        // Unique ID for this tool call
	Name      string                 `json:"name"`      // Tool name
	Arguments map[string]interface{} `json:"arguments"` // Tool arguments
}

// ToolResponse represents the result of a tool execution
type ToolResponse struct {
	ToolUseID string `json:"tool_use_id"` // References the ToolUse ID
	Content   string `json:"content"`     // Tool result content
	IsError   bool   `json:"is_error"`    // Whether this is an error response
}

// AgentState represents the current state of the agent conversation
type AgentState struct {
	ConversationID string         `json:"conversation_id"`
	Messages       []AgentMessage `json:"messages"`       // Full conversation history
	Thoughts       []AgentThought `json:"thoughts"`       // Agent's reasoning process
	ToolCalls      []ToolUse      `json:"tool_calls"`     // Tools called so far
	ToolResponses  []ToolResponse `json:"tool_responses"` // Tool execution results
	TurnCount      int            `json:"turn_count"`     // Number of agent turns
	MaxTurns       int            `json:"max_turns"`      // Maximum allowed turns
	Complete       bool           `json:"complete"`       // Whether conversation is complete
}

// StreamingMessage represents a real-time update during agent execution
type StreamingMessage struct {
	Type       string                 `json:"type"`                  // "thought", "action", "observation", "final_answer", "error"
	Content    string                 `json:"content"`               // Message content
	ToolUse    *ToolUse               `json:"tool_use,omitempty"`    // Tool being called
	ToolResult *ToolResponse          `json:"tool_result,omitempty"` // Tool execution result
	Timestamp  string                 `json:"timestamp"`             // ISO8601 timestamp
	Metadata   map[string]interface{} `json:"metadata,omitempty"`    // Additional context
}
