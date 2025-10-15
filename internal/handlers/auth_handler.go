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
	authStorage interfaces.AuthStorage
	wsHandler   AuthBroadcaster
	logger      arbor.ILogger
}

// NewAuthHandler creates a new auth handler
func NewAuthHandler(authService AuthUpdater, authStorage interfaces.AuthStorage, wsHandler AuthBroadcaster, logger arbor.ILogger) *AuthHandler {
	return &AuthHandler{
		authService: authService,
		authStorage: authStorage,
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

// ListAuthHandler lists all stored authentication credentials
func (h *AuthHandler) ListAuthHandler(w http.ResponseWriter, r *http.Request) {
	if !RequireMethod(w, r, "GET") {
		return
	}

	credentials, err := h.authStorage.ListCredentials(r.Context())
	if err != nil {
		h.logger.Error().Err(err).Msg("Failed to list credentials")
		WriteError(w, http.StatusInternalServerError, "Failed to list credentials")
		return
	}

	// Sanitize response - don't send cookies or tokens to client
	sanitized := make([]map[string]interface{}, len(credentials))
	for i, cred := range credentials {
		sanitized[i] = map[string]interface{}{
			"id":           cred.ID,
			"name":         cred.Name,
			"site_domain":  cred.SiteDomain,
			"service_type": cred.ServiceType,
			"base_url":     cred.BaseURL,
			"created_at":   cred.CreatedAt,
			"updated_at":   cred.UpdatedAt,
		}
	}

	WriteJSON(w, http.StatusOK, sanitized)
}

// GetAuthHandler retrieves a specific authentication credential
func (h *AuthHandler) GetAuthHandler(w http.ResponseWriter, r *http.Request) {
	if !RequireMethod(w, r, "GET") {
		return
	}

	// Extract ID from path: /api/auth/{id}
	path := r.URL.Path
	id := path[len("/api/auth/"):]

	if id == "" {
		WriteError(w, http.StatusBadRequest, "Missing auth ID")
		return
	}

	cred, err := h.authStorage.GetCredentialsByID(r.Context(), id)
	if err != nil {
		h.logger.Error().Err(err).Str("id", id).Msg("Failed to get credentials")
		WriteError(w, http.StatusInternalServerError, "Failed to get credentials")
		return
	}

	if cred == nil {
		WriteError(w, http.StatusNotFound, "Authentication not found")
		return
	}

	// Sanitize response - don't send cookies or tokens
	sanitized := map[string]interface{}{
		"id":           cred.ID,
		"name":         cred.Name,
		"site_domain":  cred.SiteDomain,
		"service_type": cred.ServiceType,
		"base_url":     cred.BaseURL,
		"created_at":   cred.CreatedAt,
		"updated_at":   cred.UpdatedAt,
	}

	WriteJSON(w, http.StatusOK, sanitized)
}

// DeleteAuthHandler deletes an authentication credential
func (h *AuthHandler) DeleteAuthHandler(w http.ResponseWriter, r *http.Request) {
	if !RequireMethod(w, r, "DELETE") {
		return
	}

	// Extract ID from path: /api/auth/{id}
	path := r.URL.Path
	id := path[len("/api/auth/"):]

	if id == "" {
		WriteError(w, http.StatusBadRequest, "Missing auth ID")
		return
	}

	err := h.authStorage.DeleteCredentials(r.Context(), id)
	if err != nil {
		h.logger.Error().Err(err).Str("id", id).Msg("Failed to delete credentials")
		WriteError(w, http.StatusInternalServerError, "Failed to delete credentials")
		return
	}

	h.logger.Info().Str("id", id).Msg("Deleted authentication credentials")

	WriteJSON(w, http.StatusOK, map[string]interface{}{
		"status":  "success",
		"message": "Authentication deleted successfully",
	})
}
