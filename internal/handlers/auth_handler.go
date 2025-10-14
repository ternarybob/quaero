package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/ternarybob/arbor"
	"github.com/ternarybob/quaero/internal/interfaces"
)

// AuthUpdater interface for updating authentication
type AuthUpdater interface {
	UpdateAuth(authData *interfaces.AtlassianAuthData) error
	IsAuthenticated() bool
}

// AuthBroadcaster interface for broadcasting auth updates via WebSocket
type AuthBroadcaster interface {
	BroadcastAuth(authData *interfaces.AuthData)
}

// AuthHandler handles authentication-related HTTP requests
type AuthHandler struct {
	authService AuthUpdater
	wsHandler   AuthBroadcaster
	logger      arbor.ILogger
}

// NewAuthHandler creates a new auth handler
func NewAuthHandler(authService AuthUpdater, wsHandler AuthBroadcaster, logger arbor.ILogger) *AuthHandler {
	return &AuthHandler{
		authService: authService,
		wsHandler:   wsHandler,
		logger:      logger,
	}
}

// CaptureAuthHandler handles POST requests from Chrome extension with auth data
func (h *AuthHandler) CaptureAuthHandler(w http.ResponseWriter, r *http.Request) {
	if !RequireMethod(w, r, "POST") {
		return
	}

	// Parse request body
	var authData interfaces.AtlassianAuthData
	if err := json.NewDecoder(r.Body).Decode(&authData); err != nil {
		h.logger.Error().Err(err).Msg("Failed to parse auth data")
		WriteError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	h.logger.Info().
		Str("baseUrl", authData.BaseURL).
		Int("cookies", len(authData.Cookies)).
		Msg("Received authentication data from Chrome extension")

	// Update auth service with new credentials
	if err := h.authService.UpdateAuth(&authData); err != nil {
		h.logger.Error().Err(err).Msg("Failed to update authentication")
		WriteError(w, http.StatusInternalServerError, "Failed to store authentication")
		return
	}

	h.logger.Info().Msg("Authentication captured and stored successfully")

	// Broadcast to WebSocket clients
	if h.wsHandler != nil {
		// Convert AtlassianAuthData to AuthData for broadcast
		authDataForBroadcast := &interfaces.AuthData{
			BaseURL:   authData.BaseURL,
			Cookies:   authData.Cookies,
			Tokens:    authData.Tokens,
			Timestamp: authData.Timestamp,
		}
		h.wsHandler.BroadcastAuth(authDataForBroadcast)
	}

	// Return success response
	WriteJSON(w, http.StatusOK, map[string]interface{}{
		"status":  "success",
		"message": "Authentication captured successfully",
	})
}

// GetAuthStatusHandler returns the current authentication status
func (h *AuthHandler) GetAuthStatusHandler(w http.ResponseWriter, r *http.Request) {
	if !RequireMethod(w, r, "GET") {
		return
	}

	authenticated := h.authService.IsAuthenticated()

	WriteJSON(w, http.StatusOK, map[string]interface{}{
		"authenticated": authenticated,
	})
}
