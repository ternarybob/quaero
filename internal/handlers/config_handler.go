package handlers

import (
	"encoding/json"
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
