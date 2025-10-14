package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/ternarybob/arbor"
	"github.com/ternarybob/quaero/internal/common"
)

type ConfigHandler struct {
	logger arbor.ILogger
	config *common.Config
}

func NewConfigHandler(logger arbor.ILogger, config *common.Config) *ConfigHandler {
	return &ConfigHandler{
		logger: logger,
		config: config,
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

// GetConfig returns the application configuration as JSON
func (h *ConfigHandler) GetConfig(w http.ResponseWriter, r *http.Request) {
	// Use build flags (injected via -ldflags during build)
	response := ConfigResponse{
		Version: common.GetVersion(),
		Build:   common.GetBuild(),
		Port:    h.config.Server.Port,
		Host:    h.config.Server.Host,
		Config:  h.config,
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(response); err != nil {
		h.logger.Error().Err(err).Msg("Failed to encode config response")
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
}
