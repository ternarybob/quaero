package handlers

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/ternarybob/arbor"
	"github.com/ternarybob/quaero/internal/services/mailer"
)

// MailerServiceInterface defines the methods needed from the mailer service
type MailerServiceInterface interface {
	GetConfig(ctx context.Context) (*mailer.Config, error)
	SetConfig(ctx context.Context, config *mailer.Config) error
	IsConfigured(ctx context.Context) bool
	SendTestEmail(ctx context.Context, to string) error
}

// MailerHandler handles mail configuration HTTP requests
type MailerHandler struct {
	mailerService MailerServiceInterface
	logger        arbor.ILogger
}

// NewMailerHandler creates a new mailer handler
func NewMailerHandler(mailerService MailerServiceInterface, logger arbor.ILogger) *MailerHandler {
	return &MailerHandler{
		mailerService: mailerService,
		logger:        logger,
	}
}

// GetConfigHandler handles GET /api/mail/config - retrieves mail configuration
func (h *MailerHandler) GetConfigHandler(w http.ResponseWriter, r *http.Request) {
	if !RequireMethod(w, r, "GET") {
		return
	}

	config, err := h.mailerService.GetConfig(r.Context())
	if err != nil {
		h.logger.Error().Err(err).Msg("Failed to get mail config")
		WriteError(w, http.StatusInternalServerError, "Failed to get mail configuration")
		return
	}

	// Mask password in response
	response := map[string]interface{}{
		"smtp_host":      config.Host,
		"smtp_port":      config.Port,
		"smtp_username":  config.Username,
		"smtp_password":  maskPassword(config.Password),
		"smtp_from":      config.From,
		"smtp_from_name": config.FromName,
		"smtp_use_tls":   config.UseTLS,
		"configured":     h.mailerService.IsConfigured(r.Context()),
	}

	h.logger.Debug().Msg("Retrieved mail configuration")
	WriteJSON(w, http.StatusOK, response)
}

// SetConfigHandler handles POST /api/mail/config - saves mail configuration
func (h *MailerHandler) SetConfigHandler(w http.ResponseWriter, r *http.Request) {
	if !RequireMethod(w, r, "POST") {
		return
	}

	var req struct {
		Host     string `json:"smtp_host"`
		Port     int    `json:"smtp_port"`
		Username string `json:"smtp_username"`
		Password string `json:"smtp_password"`
		From     string `json:"smtp_from"`
		FromName string `json:"smtp_from_name"`
		UseTLS   bool   `json:"smtp_use_tls"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.logger.Error().Err(err).Msg("Failed to parse request body")
		WriteError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	// Get existing config to preserve password if not provided
	existingConfig, _ := h.mailerService.GetConfig(r.Context())

	config := &mailer.Config{
		Host:     req.Host,
		Port:     req.Port,
		Username: req.Username,
		Password: req.Password,
		From:     req.From,
		FromName: req.FromName,
		UseTLS:   req.UseTLS,
	}

	// If password is masked/empty, preserve existing password
	if config.Password == "" || config.Password == "********" {
		if existingConfig != nil {
			config.Password = existingConfig.Password
		}
	}

	// Default port if not specified
	if config.Port == 0 {
		config.Port = 587
	}

	// Default from name if not specified
	if config.FromName == "" {
		config.FromName = "Quaero"
	}

	if err := h.mailerService.SetConfig(r.Context(), config); err != nil {
		h.logger.Error().Err(err).Msg("Failed to save mail config")
		WriteError(w, http.StatusInternalServerError, "Failed to save mail configuration")
		return
	}

	h.logger.Info().
		Str("host", config.Host).
		Int("port", config.Port).
		Str("from", config.From).
		Msg("Mail configuration saved")

	WriteJSON(w, http.StatusOK, map[string]interface{}{
		"status":  "success",
		"message": "Mail configuration saved successfully",
	})
}

// SendTestHandler handles POST /api/mail/test - sends a test email
func (h *MailerHandler) SendTestHandler(w http.ResponseWriter, r *http.Request) {
	if !RequireMethod(w, r, "POST") {
		return
	}

	var req struct {
		To string `json:"to"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.logger.Error().Err(err).Msg("Failed to parse request body")
		WriteError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	if req.To == "" {
		WriteError(w, http.StatusBadRequest, "Email address is required")
		return
	}

	if !h.mailerService.IsConfigured(r.Context()) {
		WriteError(w, http.StatusBadRequest, "Mail is not configured. Please configure SMTP settings first.")
		return
	}

	if err := h.mailerService.SendTestEmail(r.Context(), req.To); err != nil {
		h.logger.Error().Err(err).Str("to", req.To).Msg("Failed to send test email")
		WriteError(w, http.StatusInternalServerError, "Failed to send test email: "+err.Error())
		return
	}

	h.logger.Info().Str("to", req.To).Msg("Test email sent successfully")

	WriteJSON(w, http.StatusOK, map[string]interface{}{
		"status":  "success",
		"message": "Test email sent successfully to " + req.To,
	})
}

// maskPassword masks a password for display
func maskPassword(password string) string {
	if password == "" {
		return ""
	}
	return "********"
}
