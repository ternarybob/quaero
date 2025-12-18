package badger

import (
	"context"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/pelletier/go-toml/v2"
	"github.com/ternarybob/arbor"
	"github.com/ternarybob/quaero/internal/common"
	"github.com/ternarybob/quaero/internal/interfaces"
)

// EmailConfig represents email configuration in TOML format
// Format (email.toml):
// [email]
// smtp_host = "smtp.gmail.com"
// smtp_port = 587
// smtp_username = "user@gmail.com" or "{smtp_username}"
// smtp_password = "{smtp_password}"
// smtp_from = "user@gmail.com"
// smtp_from_name = "Quaero"
// smtp_use_tls = true
type EmailConfig struct {
	SMTPHost     string `toml:"smtp_host"`
	SMTPPort     int    `toml:"smtp_port"`
	SMTPUsername string `toml:"smtp_username"`
	SMTPPassword string `toml:"smtp_password"`
	SMTPFrom     string `toml:"smtp_from"`
	SMTPFromName string `toml:"smtp_from_name"`
	SMTPUseTLS   bool   `toml:"smtp_use_tls"`
}

// EmailFileFormat represents the TOML file structure with [email] section
type EmailFileFormat struct {
	Email EmailConfig `toml:"email"`
}

// LoadEmailFromFile loads email configuration from email.toml file in the specified directory
// It supports variable substitution using {variable_name} syntax in all fields
// Only the first [email] section is loaded
func LoadEmailFromFile(ctx context.Context, kvStorage interfaces.KeyValueStorage, dirPath string, logger arbor.ILogger) error {
	// Build path to email.toml file
	filePath := filepath.Join(dirPath, "email.toml")
	logger.Debug().Str("file", filePath).Msg("Loading email configuration from file")

	// Check if file exists
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		logger.Debug().Str("file", filePath).Msg("Email config file does not exist, skipping")
		return nil
	}

	// Load KV map for variable substitution
	var kvMap map[string]string
	if kvStorage != nil {
		var err error
		kvMap, err = kvStorage.GetAll(ctx)
		if err != nil {
			logger.Warn().Err(err).Msg("Failed to load KV map for email variable substitution")
			kvMap = make(map[string]string)
		}
	} else {
		kvMap = make(map[string]string)
	}

	// Read file content
	content, err := os.ReadFile(filePath)
	if err != nil {
		logger.Warn().Err(err).Str("file", filePath).Msg("Failed to read email config file")
		return nil // Non-fatal
	}

	// Parse TOML file
	var emailFile EmailFileFormat
	if err := toml.Unmarshal(content, &emailFile); err != nil {
		logger.Warn().Err(err).Str("file", filePath).Msg("Failed to parse email config file")
		return nil // Non-fatal
	}

	emailCfg := emailFile.Email

	// Check if any email config is present
	if emailCfg.SMTPHost == "" && emailCfg.SMTPUsername == "" {
		logger.Debug().Str("file", filePath).Msg("Email config file has no [email] section or empty config, skipping")
		return nil
	}

	// Perform variable substitution on all string fields
	smtpHost := common.ReplaceKeyReferences(emailCfg.SMTPHost, kvMap, logger)
	smtpUsername := common.ReplaceKeyReferences(emailCfg.SMTPUsername, kvMap, logger)
	smtpPassword := common.ReplaceKeyReferences(emailCfg.SMTPPassword, kvMap, logger)
	smtpFrom := common.ReplaceKeyReferences(emailCfg.SMTPFrom, kvMap, logger)
	smtpFromName := common.ReplaceKeyReferences(emailCfg.SMTPFromName, kvMap, logger)

	// Warn about unresolved variable references
	for field, value := range map[string]string{
		"smtp_host":      smtpHost,
		"smtp_username":  smtpUsername,
		"smtp_password":  smtpPassword,
		"smtp_from":      smtpFrom,
		"smtp_from_name": smtpFromName,
	} {
		if strings.Contains(value, "{") && strings.Contains(value, "}") {
			logger.Warn().
				Str("field", field).
				Msg("Email config field contains unresolved variable reference")
		}
	}

	// Store email config in KV storage with smtp_ prefix
	configItems := map[string]struct {
		value       string
		description string
	}{
		"smtp_host":      {value: smtpHost, description: "SMTP server hostname"},
		"smtp_port":      {value: strconv.Itoa(emailCfg.SMTPPort), description: "SMTP server port"},
		"smtp_username":  {value: smtpUsername, description: "SMTP username (email address)"},
		"smtp_password":  {value: smtpPassword, description: "SMTP password or app password"},
		"smtp_from":      {value: smtpFrom, description: "From email address"},
		"smtp_from_name": {value: smtpFromName, description: "From display name"},
		"smtp_use_tls":   {value: strconv.FormatBool(emailCfg.SMTPUseTLS), description: "Use TLS encryption"},
	}

	storedCount := 0
	for key, item := range configItems {
		// Skip empty values (except smtp_port which has a default of 0)
		if item.value == "" && key != "smtp_port" {
			continue
		}
		// Skip smtp_port if it's 0 (use mailer service default)
		if key == "smtp_port" && item.value == "0" {
			continue
		}

		if err := kvStorage.Set(ctx, key, item.value, item.description); err != nil {
			logger.Warn().Err(err).
				Str("key", key).
				Msg("Failed to store email config in KV storage")
			continue
		}
		storedCount++
	}

	logger.Info().
		Int("stored", storedCount).
		Str("host", smtpHost).
		Msg("Loaded email configuration from file")

	return nil
}
