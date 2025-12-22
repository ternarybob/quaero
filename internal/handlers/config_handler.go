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
	logger         arbor.ILogger
	config         *common.Config // Original config (fallback)
	configSvc      interfaces.ConfigService
	storageManager interfaces.StorageManager // For clearing and reloading TOML config
}

func NewConfigHandler(logger arbor.ILogger, config *common.Config, configSvc interfaces.ConfigService, storageManager interfaces.StorageManager) *ConfigHandler {
	return &ConfigHandler{
		logger:         logger,
		config:         config,
		configSvc:      configSvc,
		storageManager: storageManager,
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
	Clear bool `json:"clear"` // If true, clears all TOML-loaded config before reloading
}

// ReloadConfigResponse represents the response for config reload
type ReloadConfigResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
}

// ReloadConfig reloads all TOML configuration from files
// POST /api/config/reload
// Body: {"clear": bool}
// When clear=true, deletes ALL TOML-loaded data (job definitions, connectors, variables) before reloading
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

	// Check storage manager is available
	if h.storageManager == nil {
		h.logger.Error().Msg("Storage manager not available")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(ReloadConfigResponse{
			Success: false,
			Message: "Storage manager not available",
		})
		return
	}

	ctx := r.Context()

	// Step 1: Clear all TOML-loaded config if requested
	if req.Clear {
		h.logger.Info().Msg("Clearing all TOML-loaded configuration data")
		if err := h.storageManager.ClearAllConfigData(ctx); err != nil {
			h.logger.Error().Err(err).Msg("Failed to clear configuration data")
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(ReloadConfigResponse{
				Success: false,
				Message: "Failed to clear configuration: " + err.Error(),
			})
			return
		}
	}

	// Step 2: Reload all TOML files
	// Load variables from files
	if h.config.Variables.Dir != "" {
		if err := h.storageManager.LoadVariablesFromFiles(ctx, h.config.Variables.Dir); err != nil {
			h.logger.Warn().Err(err).Str("dir", h.config.Variables.Dir).Msg("Failed to reload variables")
		}
	}

	// Load job definitions from files
	if h.config.Jobs.DefinitionsDir != "" {
		if err := h.storageManager.LoadJobDefinitionsFromFiles(ctx, h.config.Jobs.DefinitionsDir); err != nil {
			h.logger.Warn().Err(err).Str("dir", h.config.Jobs.DefinitionsDir).Msg("Failed to reload job definitions")
		}
	}

	// Load connectors from files
	if h.config.Connectors.Dir != "" {
		if err := h.storageManager.LoadConnectorsFromFiles(ctx, h.config.Connectors.Dir); err != nil {
			h.logger.Warn().Err(err).Str("dir", h.config.Connectors.Dir).Msg("Failed to reload connectors")
		}

		// Load email config (from same directory as connectors)
		if err := h.storageManager.LoadEmailFromFile(ctx, h.config.Connectors.Dir); err != nil {
			h.logger.Warn().Err(err).Str("dir", h.config.Connectors.Dir).Msg("Failed to reload email config")
		}
	}

	// Step 3: Invalidate config service cache so it picks up new KV values
	if h.configSvc != nil {
		h.configSvc.InvalidateCache()
	}

	message := "Configuration reloaded successfully"
	if req.Clear {
		message = "All configuration cleared and reloaded from TOML files"
	}

	h.logger.Info().Msg(message)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(ReloadConfigResponse{
		Success: true,
		Message: message,
	})
}
