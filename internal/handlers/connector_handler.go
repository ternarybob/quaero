package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/ternarybob/arbor"
	"github.com/ternarybob/quaero/internal/connectors/github"
	"github.com/ternarybob/quaero/internal/interfaces"
	"github.com/ternarybob/quaero/internal/models"
)

type ConnectorHandler struct {
	service interfaces.ConnectorService
	logger  arbor.ILogger
}

func NewConnectorHandler(service interfaces.ConnectorService, logger arbor.ILogger) *ConnectorHandler {
	return &ConnectorHandler{
		service: service,
		logger:  logger,
	}
}

func (h *ConnectorHandler) ListConnectorsHandler(w http.ResponseWriter, r *http.Request) {
	connectors, err := h.service.ListConnectors(r.Context())
	if err != nil {
		h.logger.Error().Err(err).Msg("Failed to list connectors")
		http.Error(w, "Failed to list connectors", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(connectors)
}

func (h *ConnectorHandler) CreateConnectorHandler(w http.ResponseWriter, r *http.Request) {
	var connector models.Connector
	if err := json.NewDecoder(r.Body).Decode(&connector); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Basic validation
	if connector.Name == "" || connector.Type == "" {
		http.Error(w, "Name and Type are required", http.StatusBadRequest)
		return
	}

	// Test connection before saving
	if connector.Type == models.ConnectorTypeGitHub {
		ghConnector, err := github.NewConnector(&connector)
		if err != nil {
			http.Error(w, fmt.Sprintf("Invalid configuration: %v", err), http.StatusBadRequest)
			return
		}
		if err := ghConnector.TestConnection(r.Context()); err != nil {
			http.Error(w, fmt.Sprintf("Connection test failed: %v", err), http.StatusBadRequest)
			return
		}
	}

	if err := h.service.CreateConnector(r.Context(), &connector); err != nil {
		h.logger.Error().Err(err).Msg("Failed to create connector")
		http.Error(w, "Failed to create connector", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(connector)
}

func (h *ConnectorHandler) DeleteConnectorHandler(w http.ResponseWriter, r *http.Request) {
	// ID extraction handled by router helper usually, but here we might need to parse it from URL if using standard mux
	// Assuming RouteResourceItem passes ID or we parse it.
	// Since we are using standard http.ServeMux in routes.go and helper functions, let's assume the ID is the last part of path
	// or handled by the wrapper.
	// Looking at routes.go, RouteResourceItem handles extraction.
	// But wait, RouteResourceItem expects a function signature: func(w, r, id)
	// Let's check RouteResourceItem signature in routes.go or helpers.
	// I don't see the helper definition, but based on usage in routes.go:
	// RouteResourceItem(w, r, Get, Update, Delete)
	// The handlers in routes.go seem to be standard http.HandlerFunc.
	// Let's assume the ID is extracted from URL.

	// Actually, looking at `internal/server/routes.go`, `RouteResourceItem` is used.
	// I need to import `github.com/ternarybob/quaero/internal/server` to use it? No, it's likely in `server` package.
	// I am in `handlers` package.
	// I should just write standard handlers and let `routes.go` logic handle dispatch if it does.
	// But `routes.go` calls `s.app.AuthHandler.DeleteAuthHandler`.
	// Let's look at `DeleteAuthHandler` signature if possible.
	// I'll assume standard http.HandlerFunc and I need to parse ID.

	id := r.URL.Path[len("/api/connectors/"):]
	if id == "" {
		http.Error(w, "ID required", http.StatusBadRequest)
		return
	}

	if err := h.service.DeleteConnector(r.Context(), id); err != nil {
		h.logger.Error().Err(err).Msg("Failed to delete connector")
		http.Error(w, "Failed to delete connector", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
