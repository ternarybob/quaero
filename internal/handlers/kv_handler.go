package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/url"

	"github.com/ternarybob/arbor"
	"github.com/ternarybob/quaero/internal/interfaces"
)

// KVServiceInterface defines the methods needed from the KV service
type KVServiceInterface interface {
	Get(ctx context.Context, key string) (string, error)
	GetPair(ctx context.Context, key string) (*interfaces.KeyValuePair, error)
	Set(ctx context.Context, key string, value string, description string) error
	Delete(ctx context.Context, key string) error
	List(ctx context.Context) ([]interfaces.KeyValuePair, error)
	GetAll(ctx context.Context) (map[string]string, error)
}

// KVHandler handles variables (key/value) storage HTTP requests
type KVHandler struct {
	kvService KVServiceInterface
	logger    arbor.ILogger
}

// NewKVHandler creates a new KV handler for managing variables
func NewKVHandler(kvService KVServiceInterface, logger arbor.ILogger) *KVHandler {
	return &KVHandler{
		kvService: kvService,
		logger:    logger,
	}
}

// ListKVHandler handles GET /api/kv - lists all variables (key/value pairs)
func (h *KVHandler) ListKVHandler(w http.ResponseWriter, r *http.Request) {
	if !RequireMethod(w, r, "GET") {
		return
	}

	pairs, err := h.kvService.List(r.Context())
	if err != nil {
		h.logger.Error().Err(err).Msg("Failed to list key/value pairs")
		WriteError(w, http.StatusInternalServerError, "Failed to list key/value pairs")
		return
	}

	// Sanitize values in response - mask sensitive data
	sanitized := make([]map[string]interface{}, len(pairs))
	for i, pair := range pairs {
		sanitized[i] = map[string]interface{}{
			"key":         pair.Key,
			"value":       h.maskValue(pair.Value),
			"description": pair.Description,
			"created_at":  pair.CreatedAt,
			"updated_at":  pair.UpdatedAt,
		}
	}

	h.logger.Debug().Int("count", len(pairs)).Msg("Listed key/value pairs")
	WriteJSON(w, http.StatusOK, sanitized)
}

// GetKVHandler handles GET /api/kv/{key} - retrieves a specific variable (key/value pair)
func (h *KVHandler) GetKVHandler(w http.ResponseWriter, r *http.Request) {
	if !RequireMethod(w, r, "GET") {
		return
	}

	// Extract key from path: /api/kv/{key}
	path := r.URL.Path
	encodedKey := path[len("/api/kv/"):]

	// URL-decode the key to handle special characters
	key, err := url.QueryUnescape(encodedKey)
	if err != nil {
		h.logger.Error().Err(err).Str("encoded_key", encodedKey).Msg("Failed to decode key")
		WriteError(w, http.StatusBadRequest, "Invalid key encoding")
		return
	}

	if key == "" {
		WriteError(w, http.StatusBadRequest, "Missing key parameter")
		return
	}

	// Get full key/value pair with metadata
	pair, err := h.kvService.GetPair(r.Context(), key)
	if err != nil {
		if errors.Is(err, interfaces.ErrKeyNotFound) {
			WriteError(w, http.StatusNotFound, "Key not found")
			return
		}
		h.logger.Error().Err(err).Str("key", key).Msg("Failed to get key/value pair")
		WriteError(w, http.StatusInternalServerError, "Failed to retrieve key/value pair")
		return
	}

	// Return full value (unmasked) for editing purposes
	// Note: ListKVHandler returns masked values for security, but GET specific key returns full value
	response := map[string]interface{}{
		"key":         pair.Key,
		"value":       pair.Value, // Return full value for editing
		"description": pair.Description,
		"created_at":  pair.CreatedAt,
		"updated_at":  pair.UpdatedAt,
	}

	h.logger.Debug().Str("key", key).Msg("Retrieved key/value pair")
	WriteJSON(w, http.StatusOK, response)
}

// CreateKVHandler handles POST /api/kv - creates a new variable (key/value pair)
func (h *KVHandler) CreateKVHandler(w http.ResponseWriter, r *http.Request) {
	if !RequireMethod(w, r, "POST") {
		return
	}

	// Parse request body
	var req struct {
		Key         string `json:"key"`
		Value       string `json:"value"`
		Description string `json:"description"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.logger.Error().Err(err).Msg("Failed to parse request body")
		WriteError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	// Validate required fields
	if req.Key == "" {
		WriteError(w, http.StatusBadRequest, "Key is required")
		return
	}

	if req.Value == "" {
		WriteError(w, http.StatusBadRequest, "Value is required")
		return
	}

	// Store the key/value pair
	if err := h.kvService.Set(r.Context(), req.Key, req.Value, req.Description); err != nil {
		h.logger.Error().Err(err).Str("key", req.Key).Msg("Failed to create key/value pair")
		WriteError(w, http.StatusInternalServerError, "Failed to create key/value pair")
		return
	}

	h.logger.Info().Str("key", req.Key).Msg("Created key/value pair")

	WriteJSON(w, http.StatusCreated, map[string]interface{}{
		"status":  "success",
		"message": "Key/value pair created successfully",
		"key":     req.Key,
	})
}

// UpdateKVHandler handles PUT /api/kv/{key} - updates an existing variable (key/value pair)
// Supports full replacement (value + description) or description-only updates
func (h *KVHandler) UpdateKVHandler(w http.ResponseWriter, r *http.Request) {
	if !RequireMethod(w, r, "PUT") {
		return
	}

	// Extract key from path: /api/kv/{key}
	path := r.URL.Path
	encodedKey := path[len("/api/kv/"):]

	// URL-decode the key to handle special characters
	key, err := url.QueryUnescape(encodedKey)
	if err != nil {
		h.logger.Error().Err(err).Str("encoded_key", encodedKey).Msg("Failed to decode key")
		WriteError(w, http.StatusBadRequest, "Invalid key encoding")
		return
	}

	if key == "" {
		WriteError(w, http.StatusBadRequest, "Missing key parameter")
		return
	}

	// Parse request body
	var req struct {
		Value       string `json:"value"`
		Description string `json:"description"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.logger.Error().Err(err).Msg("Failed to parse request body")
		WriteError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	// If value is empty, fetch current value for description-only update
	valueToSet := req.Value
	if valueToSet == "" {
		currentPair, err := h.kvService.GetPair(r.Context(), key)
		if err != nil {
			if errors.Is(err, interfaces.ErrKeyNotFound) {
				WriteError(w, http.StatusNotFound, "Key not found - cannot update description for non-existent key")
				return
			}
			h.logger.Error().Err(err).Str("key", key).Msg("Failed to get current value for description-only update")
			WriteError(w, http.StatusInternalServerError, "Failed to retrieve current value")
			return
		}
		valueToSet = currentPair.Value
		h.logger.Debug().Str("key", key).Msg("Description-only update - preserving existing value")
	}

	// Update the key/value pair (Set handles upsert)
	if err := h.kvService.Set(r.Context(), key, valueToSet, req.Description); err != nil {
		h.logger.Error().Err(err).Str("key", key).Msg("Failed to update key/value pair")
		WriteError(w, http.StatusInternalServerError, "Failed to update key/value pair")
		return
	}

	h.logger.Info().Str("key", key).Msg("Updated key/value pair")

	WriteJSON(w, http.StatusOK, map[string]interface{}{
		"status":  "success",
		"message": "Key/value pair updated successfully",
		"key":     key,
	})
}

// DeleteKVHandler handles DELETE /api/kv/{key} - deletes a variable (key/value pair)
func (h *KVHandler) DeleteKVHandler(w http.ResponseWriter, r *http.Request) {
	if !RequireMethod(w, r, "DELETE") {
		return
	}

	// Extract key from path: /api/kv/{key}
	path := r.URL.Path
	encodedKey := path[len("/api/kv/"):]

	// URL-decode the key to handle special characters
	key, err := url.QueryUnescape(encodedKey)
	if err != nil {
		h.logger.Error().Err(err).Str("encoded_key", encodedKey).Msg("Failed to decode key")
		WriteError(w, http.StatusBadRequest, "Invalid key encoding")
		return
	}

	if key == "" {
		WriteError(w, http.StatusBadRequest, "Missing key parameter")
		return
	}

	// Delete the key/value pair
	if err := h.kvService.Delete(r.Context(), key); err != nil {
		if errors.Is(err, interfaces.ErrKeyNotFound) {
			WriteError(w, http.StatusNotFound, "Key not found")
			return
		}
		h.logger.Error().Err(err).Str("key", key).Msg("Failed to delete key/value pair")
		WriteError(w, http.StatusInternalServerError, "Failed to delete key/value pair")
		return
	}

	h.logger.Info().Str("key", key).Msg("Deleted key/value pair")

	WriteJSON(w, http.StatusOK, map[string]interface{}{
		"status":  "success",
		"message": "Key/value pair deleted successfully",
	})
}

// maskValue masks sensitive variable values for API responses
// If length < 8: returns "••••••••"
// Otherwise: returns first 4 chars + "..." + last 4 chars (e.g., "sk-1...xyz9")
func (h *KVHandler) maskValue(value string) string {
	if len(value) < 8 {
		return "••••••••"
	}

	return value[:4] + "..." + value[len(value)-4:]
}
