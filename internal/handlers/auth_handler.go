package handlers

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/ternarybob/arbor"
	"github.com/ternarybob/quaero/internal/interfaces"
	"github.com/ternarybob/quaero/internal/models"
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

// maskAPIKey masks an API key for safe display (first 4 + last 4 chars)
func maskAPIKey(apiKey string) string {
	if len(apiKey) <= 8 {
		return "••••••••"
	}
	return apiKey[:4] + "•••" + apiKey[len(apiKey)-4:]
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

	// Sanitize response - don't send cookies or tokens or API keys to client
	sanitized := make([]map[string]interface{}, len(credentials))
	for i, cred := range credentials {
		sanitized[i] = map[string]interface{}{
			"id":           cred.ID,
			"name":         cred.Name,
			"site_domain":  cred.SiteDomain,
			"service_type": cred.ServiceType,
			"auth_type":    cred.AuthType,
			"base_url":     cred.BaseURL,
			"created_at":   cred.CreatedAt,
			"updated_at":   cred.UpdatedAt,
		}

		// Include description if available
		if cred.Data != nil && cred.Data["description"] != nil {
			sanitized[i]["description"] = cred.Data["description"]
		}

		// Include data object if present
		if cred.Data != nil {
			sanitized[i]["data"] = cred.Data
		}

		// Mask API key if present (don't send actual API key value)
		if cred.AuthType == "api_key" && cred.APIKey != "" {
			sanitized[i]["api_key"] = maskAPIKey(cred.APIKey)
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

	// Sanitize response - don't send cookies or tokens or API keys
	sanitized := map[string]interface{}{
		"id":           cred.ID,
		"name":         cred.Name,
		"site_domain":  cred.SiteDomain,
		"service_type": cred.ServiceType,
		"auth_type":    cred.AuthType,
		"base_url":     cred.BaseURL,
		"created_at":   cred.CreatedAt,
		"updated_at":   cred.UpdatedAt,
	}

	// Mask API key if present (don't send actual API key value)
	if cred.AuthType == "api_key" && cred.APIKey != "" {
		sanitized["api_key"] = "***MASKED***"
	}

	WriteJSON(w, http.StatusOK, sanitized)
}

// CreateAPIKeyHandler creates a new API key credential
func (h *AuthHandler) CreateAPIKeyHandler(w http.ResponseWriter, r *http.Request) {
	if !RequireMethod(w, r, "POST") {
		return
	}

	// Parse request body
	var req struct {
		Name        string `json:"name"`
		APIKey      string `json:"api_key"`
		ServiceType string `json:"service_type"`
		Description string `json:"description"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.logger.Error().Err(err).Msg("Failed to parse API key request")
		WriteError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	// Validate required fields
	if req.Name == "" {
		WriteError(w, http.StatusBadRequest, "name is required")
		return
	}
	if req.APIKey == "" {
		WriteError(w, http.StatusBadRequest, "api_key is required")
		return
	}
	if req.ServiceType == "" {
		WriteError(w, http.StatusBadRequest, "service_type is required")
		return
	}

	h.logger.Info().
		Str("name", req.Name).
		Str("service_type", req.ServiceType).
		Msg("Creating new API key credential")

	// Create AuthCredentials with AuthType="api_key"
	credentials := &models.AuthCredentials{
		Name:        req.Name,
		SiteDomain:  "", // Empty for API keys
		ServiceType: req.ServiceType,
		APIKey:      req.APIKey,
		AuthType:    "api_key",
		Data:        make(map[string]interface{}),
	}

	// Add description to data if provided
	if req.Description != "" {
		credentials.Data["description"] = req.Description
	}

	// Store credentials
	if err := h.authStorage.StoreCredentials(r.Context(), credentials); err != nil {
		h.logger.Error().Err(err).Msg("Failed to store API key")
		WriteError(w, http.StatusInternalServerError, "Failed to store API key")
		return
	}

	h.logger.Info().Str("name", req.Name).Msg("API key created successfully")

	// Return sanitized response (exclude api_key)
	WriteJSON(w, http.StatusCreated, map[string]interface{}{
		"id":           credentials.ID,
		"name":         credentials.Name,
		"service_type": credentials.ServiceType,
		"auth_type":    credentials.AuthType,
		"created_at":   credentials.CreatedAt,
		"message":      "API key created successfully",
	})
}

// GetAPIKeyHandler retrieves a specific API key credential
func (h *AuthHandler) GetAPIKeyHandler(w http.ResponseWriter, r *http.Request) {
	if !RequireMethod(w, r, "GET") {
		return
	}

	// Extract ID from path: /api/auth/api-key/{id}
	path := r.URL.Path
	prefix := "/api/auth/api-key/"
	if !strings.HasPrefix(path, prefix) {
		WriteError(w, http.StatusBadRequest, "Invalid path")
		return
	}
	id := strings.TrimPrefix(path, prefix)

	if id == "" {
		WriteError(w, http.StatusBadRequest, "Missing API key ID")
		return
	}

	cred, err := h.authStorage.GetCredentialsByID(r.Context(), id)
	if err != nil {
		h.logger.Error().Err(err).Str("id", id).Msg("Failed to get credentials")
		WriteError(w, http.StatusInternalServerError, "Failed to get credentials")
		return
	}

	if cred == nil {
		WriteError(w, http.StatusNotFound, "API key not found")
		return
	}

	// Verify it's an API key credential
	if cred.AuthType != "api_key" {
		WriteError(w, http.StatusBadRequest, "Not an API key credential")
		return
	}

	// Sanitize response - mask API key value
	sanitized := map[string]interface{}{
		"id":           cred.ID,
		"name":         cred.Name,
		"site_domain":  cred.SiteDomain,
		"service_type": cred.ServiceType,
		"auth_type":    cred.AuthType,
		"base_url":     cred.BaseURL,
		"created_at":   cred.CreatedAt,
		"updated_at":   cred.UpdatedAt,
	}

	// Include description if available
	if cred.Data != nil && cred.Data["description"] != nil {
		sanitized["description"] = cred.Data["description"]
	}

	// Include data object if present
	if cred.Data != nil {
		sanitized["data"] = cred.Data
	}

	// Mask API key - don't send actual API key value
	if cred.APIKey != "" {
		sanitized["api_key"] = maskAPIKey(cred.APIKey)
	}

	WriteJSON(w, http.StatusOK, sanitized)
}

// UpdateAPIKeyHandler updates an existing API key credential
func (h *AuthHandler) UpdateAPIKeyHandler(w http.ResponseWriter, r *http.Request) {
	if !RequireMethod(w, r, "PUT") {
		return
	}

	// Extract ID from path: /api/auth/api-key/{id}
	path := r.URL.Path
	prefix := "/api/auth/api-key/"
	if !strings.HasPrefix(path, prefix) {
		WriteError(w, http.StatusBadRequest, "Invalid path")
		return
	}
	id := strings.TrimPrefix(path, prefix)

	if id == "" {
		WriteError(w, http.StatusBadRequest, "Missing API key ID")
		return
	}

	h.logger.Info().Str("id", id).Msg("Updating API key credential")

	// Get existing credentials
	cred, err := h.authStorage.GetCredentialsByID(r.Context(), id)
	if err != nil {
		h.logger.Error().Err(err).Str("id", id).Msg("Failed to get credentials")
		WriteError(w, http.StatusInternalServerError, "Failed to get credentials")
		return
	}

	if cred == nil {
		WriteError(w, http.StatusNotFound, "API key not found")
		return
	}

	// Verify it's an API key credential
	if cred.AuthType != "api_key" {
		WriteError(w, http.StatusBadRequest, "Cannot update non-API key credential")
		return
	}

	// Parse request body
	var req struct {
		Name        string `json:"name,omitempty"`
		APIKey      string `json:"api_key,omitempty"`
		ServiceType string `json:"service_type,omitempty"`
		Description string `json:"description,omitempty"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.logger.Error().Err(err).Msg("Failed to parse API key update request")
		WriteError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	// Update fields if provided
	if req.Name != "" {
		cred.Name = req.Name
	}
	if req.APIKey != "" {
		cred.APIKey = req.APIKey
	}
	if req.ServiceType != "" {
		cred.ServiceType = req.ServiceType
	}
	if req.Description != "" {
		if cred.Data == nil {
			cred.Data = make(map[string]interface{})
		}
		cred.Data["description"] = req.Description
	}

	// Store updated credentials
	if err := h.authStorage.StoreCredentials(r.Context(), cred); err != nil {
		h.logger.Error().Err(err).Str("id", id).Msg("Failed to update API key")
		WriteError(w, http.StatusInternalServerError, "Failed to update API key")
		return
	}

	h.logger.Info().Str("id", id).Msg("API key updated successfully")

	// Return sanitized response (exclude api_key)
	WriteJSON(w, http.StatusOK, map[string]interface{}{
		"id":           cred.ID,
		"name":         cred.Name,
		"service_type": cred.ServiceType,
		"auth_type":    cred.AuthType,
		"updated_at":   cred.UpdatedAt,
		"message":      "API key updated successfully",
	})
}

// DeleteAPIKeyHandler deletes an API key credential
func (h *AuthHandler) DeleteAPIKeyHandler(w http.ResponseWriter, r *http.Request) {
	if !RequireMethod(w, r, "DELETE") {
		return
	}

	// Extract ID from path: /api/auth/api-key/{id}
	path := r.URL.Path
	prefix := "/api/auth/api-key/"
	if !strings.HasPrefix(path, prefix) {
		WriteError(w, http.StatusBadRequest, "Invalid path")
		return
	}
	id := strings.TrimPrefix(path, prefix)

	if id == "" {
		WriteError(w, http.StatusBadRequest, "Missing API key ID")
		return
	}

	// Get existing credentials first to verify it's an API key
	cred, err := h.authStorage.GetCredentialsByID(r.Context(), id)
	if err != nil {
		h.logger.Error().Err(err).Str("id", id).Msg("Failed to get credentials")
		WriteError(w, http.StatusInternalServerError, "Failed to get credentials")
		return
	}

	if cred == nil {
		WriteError(w, http.StatusNotFound, "API key not found")
		return
	}

	// Verify it's an API key credential
	if cred.AuthType != "api_key" {
		WriteError(w, http.StatusBadRequest, "Not an API key credential")
		return
	}

	err = h.authStorage.DeleteCredentials(r.Context(), id)
	if err != nil {
		h.logger.Error().Err(err).Str("id", id).Msg("Failed to delete API key")
		WriteError(w, http.StatusInternalServerError, "Failed to delete credentials")
		return
	}

	h.logger.Info().Str("id", id).Msg("Deleted API key credentials")

	WriteJSON(w, http.StatusOK, map[string]interface{}{
		"status":  "success",
		"message": "API key deleted successfully",
	})
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
