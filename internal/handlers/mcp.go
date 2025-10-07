package handlers

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/ternarybob/arbor"
	"github.com/ternarybob/quaero/internal/services/mcp"
)

// MCPHandler handles MCP protocol requests
type MCPHandler struct {
	service *mcp.DocumentService
	logger  arbor.ILogger
}

// NewMCPHandler creates a new MCP handler
func NewMCPHandler(service *mcp.DocumentService, logger arbor.ILogger) *MCPHandler {
	return &MCPHandler{
		service: service,
		logger:  logger,
	}
}

// HandleRPC handles JSON-RPC 2.0 requests
func (h *MCPHandler) HandleRPC(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		h.sendError(w, nil, mcp.InvalidRequest, "Method must be POST", http.StatusMethodNotAllowed)
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		h.sendError(w, nil, mcp.ParseError, "Failed to read request body", http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	var req mcp.JSONRPCRequest
	if err := json.Unmarshal(body, &req); err != nil {
		h.sendError(w, nil, mcp.ParseError, "Invalid JSON", http.StatusBadRequest)
		return
	}

	// Validate JSON-RPC version
	if req.JSONRPC != "2.0" {
		h.sendError(w, req.ID, mcp.InvalidRequest, "Invalid JSON-RPC version", http.StatusBadRequest)
		return
	}

	h.logger.Debug().Str("method", req.Method).Msg("MCP RPC request")

	// Route to appropriate handler
	switch req.Method {
	case "resources/list":
		h.handleListResources(w, r, req)
	case "resources/read":
		h.handleReadResource(w, r, req)
	case "tools/list":
		h.handleListTools(w, r, req)
	case "tools/call":
		h.handleCallTool(w, r, req)
	default:
		h.sendError(w, req.ID, mcp.MethodNotFound, fmt.Sprintf("Unknown method: %s", req.Method), http.StatusNotFound)
	}
}

// handleListResources handles resources/list requests
func (h *MCPHandler) handleListResources(w http.ResponseWriter, r *http.Request, req mcp.JSONRPCRequest) {
	result, err := h.service.ListResources(r.Context())
	if err != nil {
		h.sendError(w, req.ID, mcp.InternalError, err.Error(), http.StatusInternalServerError)
		return
	}

	h.sendSuccess(w, req.ID, result)
}

// handleReadResource handles resources/read requests
func (h *MCPHandler) handleReadResource(w http.ResponseWriter, r *http.Request, req mcp.JSONRPCRequest) {
	uri, ok := req.Params["uri"].(string)
	if !ok {
		h.sendError(w, req.ID, mcp.InvalidParams, "Missing or invalid 'uri' parameter", http.StatusBadRequest)
		return
	}

	result, err := h.service.ReadResource(r.Context(), uri)
	if err != nil {
		h.sendError(w, req.ID, mcp.InternalError, err.Error(), http.StatusInternalServerError)
		return
	}

	h.sendSuccess(w, req.ID, result)
}

// handleListTools handles tools/list requests
func (h *MCPHandler) handleListTools(w http.ResponseWriter, r *http.Request, req mcp.JSONRPCRequest) {
	result, err := h.service.ListTools(r.Context())
	if err != nil {
		h.sendError(w, req.ID, mcp.InternalError, err.Error(), http.StatusInternalServerError)
		return
	}

	h.sendSuccess(w, req.ID, result)
}

// handleCallTool handles tools/call requests
func (h *MCPHandler) handleCallTool(w http.ResponseWriter, r *http.Request, req mcp.JSONRPCRequest) {
	name, ok := req.Params["name"].(string)
	if !ok {
		h.sendError(w, req.ID, mcp.InvalidParams, "Missing or invalid 'name' parameter", http.StatusBadRequest)
		return
	}

	args, ok := req.Params["arguments"].(map[string]interface{})
	if !ok {
		args = make(map[string]interface{})
	}

	result, err := h.service.CallTool(r.Context(), name, args)
	if err != nil {
		h.sendError(w, req.ID, mcp.InternalError, err.Error(), http.StatusInternalServerError)
		return
	}

	h.sendSuccess(w, req.ID, result)
}

// sendSuccess sends a successful JSON-RPC response
func (h *MCPHandler) sendSuccess(w http.ResponseWriter, id interface{}, result interface{}) {
	resp := mcp.JSONRPCResponse{
		JSONRPC: "2.0",
		ID:      id,
		Result:  result,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(resp)
}

// sendError sends an error JSON-RPC response
func (h *MCPHandler) sendError(w http.ResponseWriter, id interface{}, code int, message string, httpStatus int) {
	resp := mcp.JSONRPCResponse{
		JSONRPC: "2.0",
		ID:      id,
		Error: &mcp.RPCError{
			Code:    code,
			Message: message,
		},
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(httpStatus)
	json.NewEncoder(w).Encode(resp)
}

// InfoHandler returns MCP server information
func (h *MCPHandler) InfoHandler(w http.ResponseWriter, r *http.Request) {
	info := map[string]interface{}{
		"name":        "Quaero MCP Server",
		"version":     "1.0.0",
		"description": "Model Context Protocol server for Quaero document knowledge base",
		"capabilities": map[string]interface{}{
			"resources": true,
			"tools":     true,
		},
		"endpoints": map[string]string{
			"rpc":  "/mcp",
			"info": "/mcp/info",
		},
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"info":    info,
	})
}
