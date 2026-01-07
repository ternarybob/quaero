// -----------------------------------------------------------------------
// Mailer Service - SMTP email sending using user credentials
// Credentials are stored in KeyValue storage with smtp_ prefix
// -----------------------------------------------------------------------

package mailer

import (
	"context"
	"crypto/rand"
	"crypto/tls"
	"encoding/base64"
	"fmt"
	"net/smtp"
	"strconv"
	"strings"

	"github.com/ternarybob/arbor"
	"github.com/ternarybob/quaero/internal/interfaces"
)

// Config holds SMTP configuration loaded from KeyValue storage
type Config struct {
	Host     string `json:"smtp_host"`
	Port     int    `json:"smtp_port"`
	Username string `json:"smtp_username"`
	Password string `json:"smtp_password"`
	From     string `json:"smtp_from"`
	FromName string `json:"smtp_from_name"`
	UseTLS   bool   `json:"smtp_use_tls"`
}

// Attachment represents an email attachment
type Attachment struct {
	Filename    string // Filename for the attachment
	ContentType string // MIME type (e.g., "text/markdown", "text/plain")
	Content     []byte // Raw content bytes
}

// Service provides email sending functionality using user's SMTP credentials
type Service struct {
	kvStorage interfaces.KeyValueStorage
	logger    arbor.ILogger
}

// NewService creates a new mailer service
// Uses KeyValue storage for SMTP credentials (survives reset_on_startup via variables.toml)
func NewService(kvStorage interfaces.KeyValueStorage, logger arbor.ILogger) *Service {
	return &Service{
		kvStorage: kvStorage,
		logger:    logger,
	}
}

// GetConfig retrieves SMTP configuration from KeyValue storage
func (s *Service) GetConfig(ctx context.Context) (*Config, error) {
	config := &Config{
		Port:     587,  // Default SMTP port
		UseTLS:   true, // Default to TLS
		FromName: "Quaero",
	}

	// Load each config value from KV storage
	if host, err := s.kvStorage.Get(ctx, "smtp_host"); err == nil && host != "" {
		config.Host = host
	}

	if portStr, err := s.kvStorage.Get(ctx, "smtp_port"); err == nil && portStr != "" {
		if port, err := strconv.Atoi(portStr); err == nil {
			config.Port = port
		}
	}

	if username, err := s.kvStorage.Get(ctx, "smtp_username"); err == nil {
		config.Username = username
	}

	if password, err := s.kvStorage.Get(ctx, "smtp_password"); err == nil {
		config.Password = password
	}

	if from, err := s.kvStorage.Get(ctx, "smtp_from"); err == nil && from != "" {
		config.From = from
	}

	if fromName, err := s.kvStorage.Get(ctx, "smtp_from_name"); err == nil && fromName != "" {
		config.FromName = fromName
	}

	if tlsStr, err := s.kvStorage.Get(ctx, "smtp_use_tls"); err == nil && tlsStr != "" {
		config.UseTLS = strings.ToLower(tlsStr) == "true" || tlsStr == "1"
	}

	return config, nil
}

// SetConfig saves SMTP configuration to KeyValue storage
func (s *Service) SetConfig(ctx context.Context, config *Config) error {
	// Save each config value to KV storage
	if err := s.kvStorage.Set(ctx, "smtp_host", config.Host, "SMTP server hostname"); err != nil {
		return fmt.Errorf("failed to set smtp_host: %w", err)
	}

	if err := s.kvStorage.Set(ctx, "smtp_port", strconv.Itoa(config.Port), "SMTP server port"); err != nil {
		return fmt.Errorf("failed to set smtp_port: %w", err)
	}

	if err := s.kvStorage.Set(ctx, "smtp_username", config.Username, "SMTP username (email address)"); err != nil {
		return fmt.Errorf("failed to set smtp_username: %w", err)
	}

	if err := s.kvStorage.Set(ctx, "smtp_password", config.Password, "SMTP password or app password"); err != nil {
		return fmt.Errorf("failed to set smtp_password: %w", err)
	}

	if err := s.kvStorage.Set(ctx, "smtp_from", config.From, "From email address"); err != nil {
		return fmt.Errorf("failed to set smtp_from: %w", err)
	}

	if err := s.kvStorage.Set(ctx, "smtp_from_name", config.FromName, "From display name"); err != nil {
		return fmt.Errorf("failed to set smtp_from_name: %w", err)
	}

	tlsStr := "false"
	if config.UseTLS {
		tlsStr = "true"
	}
	if err := s.kvStorage.Set(ctx, "smtp_use_tls", tlsStr, "Use TLS encryption"); err != nil {
		return fmt.Errorf("failed to set smtp_use_tls: %w", err)
	}

	s.logger.Info().
		Str("host", config.Host).
		Int("port", config.Port).
		Str("from", config.From).
		Msg("Mail configuration saved")

	return nil
}

// IsConfigured checks if SMTP is configured with minimum required settings
func (s *Service) IsConfigured(ctx context.Context) bool {
	config, err := s.GetConfig(ctx)
	if err != nil {
		return false
	}

	return config.Host != "" && config.Username != "" && config.Password != "" && config.From != ""
}

// SendEmail sends a plain text email
func (s *Service) SendEmail(ctx context.Context, to, subject, body string) error {
	return s.SendHTMLEmail(ctx, to, subject, "", body)
}

// SendHTMLEmail sends an email with HTML and/or plain text body
func (s *Service) SendHTMLEmail(ctx context.Context, to, subject, htmlBody, textBody string) error {
	config, err := s.GetConfig(ctx)
	if err != nil {
		return fmt.Errorf("failed to get mail config: %w", err)
	}

	if config.Host == "" {
		return fmt.Errorf("SMTP host not configured")
	}

	if config.Username == "" || config.Password == "" {
		return fmt.Errorf("SMTP credentials not configured")
	}

	if config.From == "" {
		return fmt.Errorf("from email not configured")
	}

	// Build email message
	var msg strings.Builder
	msg.WriteString(fmt.Sprintf("From: %s <%s>\r\n", config.FromName, config.From))
	msg.WriteString(fmt.Sprintf("To: %s\r\n", to))
	msg.WriteString(fmt.Sprintf("Subject: %s\r\n", subject))

	if htmlBody != "" {
		// Multipart message with HTML and text
		// Generate unique boundary to avoid conflicts with content
		boundary := generateBoundary()
		msg.WriteString("MIME-Version: 1.0\r\n")
		msg.WriteString(fmt.Sprintf("Content-Type: multipart/alternative; boundary=\"%s\"\r\n", boundary))
		msg.WriteString("\r\n")

		// Plain text part - use base64 encoding for safety with long lines
		if textBody != "" {
			msg.WriteString(fmt.Sprintf("--%s\r\n", boundary))
			msg.WriteString("Content-Type: text/plain; charset=\"UTF-8\"\r\n")
			msg.WriteString("Content-Transfer-Encoding: base64\r\n")
			msg.WriteString("\r\n")
			msg.WriteString(encodeBase64WithLineBreaks(textBody))
			msg.WriteString("\r\n")
		}

		// HTML part - use base64 encoding to handle large content and long lines
		// RFC 5322 limits line length to 998 chars; base64 ensures compliance
		msg.WriteString(fmt.Sprintf("--%s\r\n", boundary))
		msg.WriteString("Content-Type: text/html; charset=\"UTF-8\"\r\n")
		msg.WriteString("Content-Transfer-Encoding: base64\r\n")
		msg.WriteString("\r\n")
		msg.WriteString(encodeBase64WithLineBreaks(htmlBody))
		msg.WriteString("\r\n")

		msg.WriteString(fmt.Sprintf("--%s--\r\n", boundary))
	} else {
		// Plain text only
		msg.WriteString("Content-Type: text/plain; charset=\"UTF-8\"\r\n")
		msg.WriteString("\r\n")
		msg.WriteString(textBody)
	}

	// Connect and send
	addr := fmt.Sprintf("%s:%d", config.Host, config.Port)
	auth := smtp.PlainAuth("", config.Username, config.Password, config.Host)

	if config.UseTLS {
		// TLS connection (Gmail, etc.)
		return s.sendWithTLS(addr, auth, config.From, to, msg.String())
	}

	// Plain SMTP
	return smtp.SendMail(addr, auth, config.From, []string{to}, []byte(msg.String()))
}

// SendEmailWithAttachments sends an email with HTML/text body and file attachments
func (s *Service) SendEmailWithAttachments(ctx context.Context, to, subject, htmlBody, textBody string, attachments []Attachment) error {
	if len(attachments) == 0 {
		// No attachments, use standard method
		return s.SendHTMLEmail(ctx, to, subject, htmlBody, textBody)
	}

	config, err := s.GetConfig(ctx)
	if err != nil {
		return fmt.Errorf("failed to get mail config: %w", err)
	}

	if config.Host == "" {
		return fmt.Errorf("SMTP host not configured")
	}

	if config.Username == "" || config.Password == "" {
		return fmt.Errorf("SMTP credentials not configured")
	}

	if config.From == "" {
		return fmt.Errorf("from email not configured")
	}

	// Generate boundaries for multipart message
	mixedBoundary := generateBoundary()
	altBoundary := generateBoundary()

	// Build email message
	var msg strings.Builder
	msg.WriteString(fmt.Sprintf("From: %s <%s>\r\n", config.FromName, config.From))
	msg.WriteString(fmt.Sprintf("To: %s\r\n", to))
	msg.WriteString(fmt.Sprintf("Subject: %s\r\n", subject))
	msg.WriteString("MIME-Version: 1.0\r\n")
	msg.WriteString(fmt.Sprintf("Content-Type: multipart/mixed; boundary=\"%s\"\r\n", mixedBoundary))
	msg.WriteString("\r\n")

	// Body part (multipart/alternative for HTML + text)
	msg.WriteString(fmt.Sprintf("--%s\r\n", mixedBoundary))
	msg.WriteString(fmt.Sprintf("Content-Type: multipart/alternative; boundary=\"%s\"\r\n", altBoundary))
	msg.WriteString("\r\n")

	// Plain text part
	if textBody != "" {
		msg.WriteString(fmt.Sprintf("--%s\r\n", altBoundary))
		msg.WriteString("Content-Type: text/plain; charset=\"UTF-8\"\r\n")
		msg.WriteString("Content-Transfer-Encoding: base64\r\n")
		msg.WriteString("\r\n")
		msg.WriteString(encodeBase64WithLineBreaks(textBody))
		msg.WriteString("\r\n")
	}

	// HTML part
	if htmlBody != "" {
		msg.WriteString(fmt.Sprintf("--%s\r\n", altBoundary))
		msg.WriteString("Content-Type: text/html; charset=\"UTF-8\"\r\n")
		msg.WriteString("Content-Transfer-Encoding: base64\r\n")
		msg.WriteString("\r\n")
		msg.WriteString(encodeBase64WithLineBreaks(htmlBody))
		msg.WriteString("\r\n")
	}

	msg.WriteString(fmt.Sprintf("--%s--\r\n", altBoundary))

	// Attachments
	for _, att := range attachments {
		msg.WriteString(fmt.Sprintf("--%s\r\n", mixedBoundary))
		contentType := att.ContentType
		if contentType == "" {
			contentType = "application/octet-stream"
		}
		msg.WriteString(fmt.Sprintf("Content-Type: %s; name=\"%s\"\r\n", contentType, att.Filename))
		msg.WriteString("Content-Transfer-Encoding: base64\r\n")
		msg.WriteString(fmt.Sprintf("Content-Disposition: attachment; filename=\"%s\"\r\n", att.Filename))
		msg.WriteString("\r\n")
		msg.WriteString(encodeBase64WithLineBreaks(string(att.Content)))
		msg.WriteString("\r\n")
	}

	msg.WriteString(fmt.Sprintf("--%s--\r\n", mixedBoundary))

	// Connect and send
	addr := fmt.Sprintf("%s:%d", config.Host, config.Port)
	auth := smtp.PlainAuth("", config.Username, config.Password, config.Host)

	if config.UseTLS {
		return s.sendWithTLS(addr, auth, config.From, to, msg.String())
	}

	return smtp.SendMail(addr, auth, config.From, []string{to}, []byte(msg.String()))
}

// sendWithTLS sends email using TLS connection (required for Gmail)
func (s *Service) sendWithTLS(addr string, auth smtp.Auth, from, to, msg string) error {
	host := strings.Split(addr, ":")[0]

	// Connect to SMTP server
	conn, err := tls.Dial("tcp", addr, &tls.Config{
		ServerName: host,
	})
	if err != nil {
		// Fallback to STARTTLS if direct TLS fails
		return s.sendWithSTARTTLS(addr, auth, from, to, msg)
	}
	defer conn.Close()

	client, err := smtp.NewClient(conn, host)
	if err != nil {
		return fmt.Errorf("failed to create SMTP client: %w", err)
	}
	defer client.Close()

	// Authenticate
	if err := client.Auth(auth); err != nil {
		return fmt.Errorf("SMTP authentication failed: %w", err)
	}

	// Set sender and recipient
	if err := client.Mail(from); err != nil {
		return fmt.Errorf("failed to set mail from: %w", err)
	}

	if err := client.Rcpt(to); err != nil {
		return fmt.Errorf("failed to set mail recipient: %w", err)
	}

	// Write message
	w, err := client.Data()
	if err != nil {
		return fmt.Errorf("failed to start data: %w", err)
	}

	_, err = w.Write([]byte(msg))
	if err != nil {
		return fmt.Errorf("failed to write message: %w", err)
	}

	err = w.Close()
	if err != nil {
		return fmt.Errorf("failed to close data writer: %w", err)
	}

	return client.Quit()
}

// sendWithSTARTTLS sends email using STARTTLS upgrade
func (s *Service) sendWithSTARTTLS(addr string, auth smtp.Auth, from, to, msg string) error {
	host := strings.Split(addr, ":")[0]

	client, err := smtp.Dial(addr)
	if err != nil {
		return fmt.Errorf("failed to connect to SMTP server: %w", err)
	}
	defer client.Close()

	// Upgrade to TLS
	tlsConfig := &tls.Config{
		ServerName: host,
	}
	if err := client.StartTLS(tlsConfig); err != nil {
		return fmt.Errorf("failed to start TLS: %w", err)
	}

	// Authenticate
	if err := client.Auth(auth); err != nil {
		return fmt.Errorf("SMTP authentication failed: %w", err)
	}

	// Set sender and recipient
	if err := client.Mail(from); err != nil {
		return fmt.Errorf("failed to set mail from: %w", err)
	}

	if err := client.Rcpt(to); err != nil {
		return fmt.Errorf("failed to set mail recipient: %w", err)
	}

	// Write message
	w, err := client.Data()
	if err != nil {
		return fmt.Errorf("failed to start data: %w", err)
	}

	_, err = w.Write([]byte(msg))
	if err != nil {
		return fmt.Errorf("failed to write message: %w", err)
	}

	err = w.Close()
	if err != nil {
		return fmt.Errorf("failed to close data writer: %w", err)
	}

	return client.Quit()
}

// SendTestEmail sends a test email to verify configuration
func (s *Service) SendTestEmail(ctx context.Context, to string) error {
	subject := "Quaero Test Email"
	body := "This is a test email from Quaero to verify your SMTP configuration is working correctly."

	if err := s.SendEmail(ctx, to, subject, body); err != nil {
		s.logger.Error().Err(err).Str("to", to).Msg("Failed to send test email")
		return err
	}

	s.logger.Info().Str("to", to).Msg("Test email sent successfully")
	return nil
}

// generateBoundary creates a unique MIME boundary string
// Uses crypto/rand for uniqueness to avoid collisions with content
func generateBoundary() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		// Fallback to a simple boundary if random fails
		return "quaero_boundary_fallback"
	}
	return fmt.Sprintf("quaero_%x", b)
}

// encodeBase64WithLineBreaks encodes content as base64 with 76-char line breaks
// per RFC 2045 for MIME content. This ensures compatibility with all mail servers
// and prevents line-length related corruption of large HTML content.
func encodeBase64WithLineBreaks(content string) string {
	encoded := base64.StdEncoding.EncodeToString([]byte(content))

	// Insert line breaks every 76 characters per RFC 2045
	var result strings.Builder
	const lineLen = 76

	for i := 0; i < len(encoded); i += lineLen {
		end := i + lineLen
		if end > len(encoded) {
			end = len(encoded)
		}
		result.WriteString(encoded[i:end])
		if end < len(encoded) {
			result.WriteString("\r\n")
		}
	}

	return result.String()
}
