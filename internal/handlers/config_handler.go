package handlers

import (
	"encoding/json"
	"io"
	"net/http"

	"github.com/ternarybob/arbor"
	"github.com/ternarybob/quaero/internal/common"
	"github.com/ternarybob/quaero/internal/interfaces"
)

type ConfigHandler struct {
	logger    arbor.ILogger
	config    *common.Config // Original config (fallback)
	configSvc interfaces.ConfigService
}

func NewConfigHandler(logger arbor.ILogger, config *common.Config, configSvc interfaces.ConfigService) *ConfigHandler {
	return &ConfigHandler{
		logger:    logger,
		config:    config,
		configSvc: configSvc,
	}
}

// ConfigResponse represents the configuration response
type ConfigResponse struct {
	Version string         `json:"version"`
	Build   string         `json:"build"`
	Port    int            `json:"port"`
	Host    string         `json:"host"`
	Config  *common.Config `json:"config"`
}

// GetConfig returns the application configuration as JSON with dynamically injected keys
func (h *ConfigHandler) GetConfig(w http.ResponseWriter, r *http.Request) {
	// Get config with injected keys from ConfigService
	config := h.config // Fallback to original config
	if h.configSvc != nil {
		injectedConfigRaw, err := h.configSvc.GetConfig(r.Context())
		if err != nil {
			h.logger.Warn().Err(err).Msg("Failed to get config with injected keys, using fallback")
		} else if injectedConfig, ok := injectedConfigRaw.(*common.Config); ok {
			config = injectedConfig
		} else {
			h.logger.Warn().Msg("ConfigService returned unexpected type, using fallback")
		}
	}

	// Use build flags (injected via -ldflags during build)
	response := ConfigResponse{
		Version: common.GetVersion(),
		Build:   common.GetBuild(),
		Port:    config.Server.Port,
		Host:    config.Server.Host,
		Config:  config,
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(response); err != nil {
		h.logger.Error().Err(err).Msg("Failed to encode config response")
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
}

// ReloadConfigRequest represents the request body for config reload
type ReloadConfigRequest struct {
	Clear bool `json:"clear"` // If true, clears KV store before reloading
}

// ReloadConfigResponse represents the response for config reload
type ReloadConfigResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
}

// ReloadConfig reloads configuration from files
// POST /api/config/reload
// Body: {"clear": bool}
func (h *ConfigHandler) ReloadConfig(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Parse request body
	var req ReloadConfigRequest
	if r.Body != nil {
		defer r.Body.Close()
		body, err := io.ReadAll(r.Body)
		if err != nil {
			h.logger.Error().Err(err).Msg("Failed to read reload config request body")
			http.Error(w, "Failed to read request body", http.StatusBadRequest)
			return
		}
		if len(body) > 0 {
			if err := json.Unmarshal(body, &req); err != nil {
				h.logger.Error().Err(err).Msg("Failed to parse reload config request")
				http.Error(w, "Invalid JSON body", http.StatusBadRequest)
				return
			}
		}
	}

	h.logger.Info().Bool("clear", req.Clear).Msg("Config reload requested")

	// Call the config service to reload
	if h.configSvc == nil {
		h.logger.Error().Msg("Config service not available")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(ReloadConfigResponse{
			Success: false,
			Message: "Config service not available",
		})
		return
	}

	if err := h.configSvc.ReloadConfig(r.Context(), req.Clear); err != nil {
		h.logger.Error().Err(err).Msg("Failed to reload configuration")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(ReloadConfigResponse{
			Success: false,
			Message: err.Error(),
		})
		return
	}

	message := "Configuration reloaded successfully"
	if req.Clear {
		message = "Configuration cleared and reloaded successfully"
	}

	h.logger.Info().Msg(message)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(ReloadConfigResponse{
		Success: true,
		Message: message,
	})
}
