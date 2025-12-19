// -----------------------------------------------------------------------
// IMAP Service - IMAP email reading using user credentials
// Credentials are stored in KeyValue storage with imap_ prefix
// -----------------------------------------------------------------------

package imap

import (
	"context"
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"

	"github.com/emersion/go-imap"
	"github.com/emersion/go-imap/client"
	"github.com/emersion/go-message/mail"
	"github.com/ternarybob/arbor"
	"github.com/ternarybob/quaero/internal/interfaces"
)

// Config holds IMAP configuration loaded from KeyValue storage
type Config struct {
	Host     string
	Port     int
	Username string
	Password string
	UseTLS   bool
}

// Email represents a fetched email message
type Email struct {
	ID      uint32
	From    string
	Subject string
	Body    string
	Date    time.Time
}

// Service provides email reading functionality using user's IMAP credentials
type Service struct {
	kvStorage interfaces.KeyValueStorage
	logger    arbor.ILogger
}

// NewService creates a new IMAP service
func NewService(kvStorage interfaces.KeyValueStorage, logger arbor.ILogger) *Service {
	return &Service{
		kvStorage: kvStorage,
		logger:    logger,
	}
}

// GetConfig retrieves IMAP configuration from KeyValue storage
func (s *Service) GetConfig(ctx context.Context) (*Config, error) {
	config := &Config{
		Port:   993, // Default IMAP SSL port
		UseTLS: true,
	}

	// Load each config value from KV storage
	if host, err := s.kvStorage.Get(ctx, "imap_host"); err == nil && host != "" {
		config.Host = host
	}

	if portStr, err := s.kvStorage.Get(ctx, "imap_port"); err == nil && portStr != "" {
		if port, err := strconv.Atoi(portStr); err == nil {
			config.Port = port
		}
	}

	if username, err := s.kvStorage.Get(ctx, "imap_username"); err == nil {
		config.Username = username
	}

	if password, err := s.kvStorage.Get(ctx, "imap_password"); err == nil {
		config.Password = password
	}

	if tlsStr, err := s.kvStorage.Get(ctx, "imap_use_tls"); err == nil && tlsStr != "" {
		config.UseTLS = strings.ToLower(tlsStr) == "true" || tlsStr == "1"
	}

	return config, nil
}

// SetConfig saves IMAP configuration to KeyValue storage
func (s *Service) SetConfig(ctx context.Context, config *Config) error {
	if err := s.kvStorage.Set(ctx, "imap_host", config.Host, "IMAP server hostname"); err != nil {
		return fmt.Errorf("failed to set imap_host: %w", err)
	}

	if err := s.kvStorage.Set(ctx, "imap_port", strconv.Itoa(config.Port), "IMAP server port"); err != nil {
		return fmt.Errorf("failed to set imap_port: %w", err)
	}

	if err := s.kvStorage.Set(ctx, "imap_username", config.Username, "IMAP username (email address)"); err != nil {
		return fmt.Errorf("failed to set imap_username: %w", err)
	}

	if err := s.kvStorage.Set(ctx, "imap_password", config.Password, "IMAP password or app password"); err != nil {
		return fmt.Errorf("failed to set imap_password: %w", err)
	}

	tlsStr := "false"
	if config.UseTLS {
		tlsStr = "true"
	}
	if err := s.kvStorage.Set(ctx, "imap_use_tls", tlsStr, "Use TLS encryption"); err != nil {
		return fmt.Errorf("failed to set imap_use_tls: %w", err)
	}

	s.logger.Info().
		Str("host", config.Host).
		Int("port", config.Port).
		Msg("IMAP configuration saved")

	return nil
}

// IsConfigured checks if IMAP is configured with minimum required settings
func (s *Service) IsConfigured(ctx context.Context) bool {
	config, err := s.GetConfig(ctx)
	if err != nil {
		return false
	}

	return config.Host != "" && config.Username != "" && config.Password != ""
}

// FetchUnreadEmails fetches unread emails with optional subject filter
func (s *Service) FetchUnreadEmails(ctx context.Context, subjectFilter string) ([]Email, error) {
	config, err := s.GetConfig(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get IMAP config: %w", err)
	}

	if config.Host == "" || config.Username == "" || config.Password == "" {
		return nil, fmt.Errorf("IMAP not configured")
	}

	// Connect to IMAP server
	addr := fmt.Sprintf("%s:%d", config.Host, config.Port)
	var c *client.Client

	if config.UseTLS {
		c, err = client.DialTLS(addr, nil)
	} else {
		c, err = client.Dial(addr)
	}

	if err != nil {
		return nil, fmt.Errorf("failed to connect to IMAP server: %w", err)
	}
	defer c.Logout()

	// Login
	if err := c.Login(config.Username, config.Password); err != nil {
		return nil, fmt.Errorf("IMAP login failed: %w", err)
	}

	// Select INBOX
	mbox, err := c.Select("INBOX", false)
	if err != nil {
		return nil, fmt.Errorf("failed to select INBOX: %w", err)
	}

	if mbox.Messages == 0 {
		s.logger.Debug().Msg("No messages in INBOX")
		return []Email{}, nil
	}

	// Search for unseen messages
	criteria := imap.NewSearchCriteria()
	criteria.WithoutFlags = []string{imap.SeenFlag}

	seqNums, err := c.Search(criteria)
	if err != nil {
		return nil, fmt.Errorf("failed to search for unseen messages: %w", err)
	}

	if len(seqNums) == 0 {
		s.logger.Debug().Msg("No unseen messages")
		return []Email{}, nil
	}

	s.logger.Debug().Int("count", len(seqNums)).Msg("Found unseen messages")

	// Fetch messages
	seqSet := new(imap.SeqSet)
	seqSet.AddNum(seqNums...)

	messages := make(chan *imap.Message, len(seqNums))
	section := &imap.BodySectionName{}

	done := make(chan error, 1)
	go func() {
		done <- c.Fetch(seqSet, []imap.FetchItem{imap.FetchEnvelope, imap.FetchFlags, section.FetchItem()}, messages)
	}()

	var emails []Email
	for msg := range messages {
		if msg == nil {
			continue
		}

		// Check subject filter
		subject := msg.Envelope.Subject
		if subjectFilter != "" && !strings.Contains(strings.ToLower(subject), strings.ToLower(subjectFilter)) {
			continue
		}

		// Parse message body
		body, err := s.parseMessageBody(msg, section)
		if err != nil {
			s.logger.Warn().Err(err).Uint32("seq", msg.SeqNum).Msg("Failed to parse message body")
			continue
		}

		// Extract from address
		from := ""
		if len(msg.Envelope.From) > 0 {
			from = msg.Envelope.From[0].Address()
		}

		email := Email{
			ID:      msg.SeqNum,
			From:    from,
			Subject: subject,
			Body:    body,
			Date:    msg.Envelope.Date,
		}

		emails = append(emails, email)
	}

	if err := <-done; err != nil {
		return nil, fmt.Errorf("failed to fetch messages: %w", err)
	}

	return emails, nil
}

// MarkAsRead marks a message as read (seen)
func (s *Service) MarkAsRead(ctx context.Context, messageID uint32) error {
	config, err := s.GetConfig(ctx)
	if err != nil {
		return fmt.Errorf("failed to get IMAP config: %w", err)
	}

	if config.Host == "" || config.Username == "" || config.Password == "" {
		return fmt.Errorf("IMAP not configured")
	}

	// Connect to IMAP server
	addr := fmt.Sprintf("%s:%d", config.Host, config.Port)
	var c *client.Client

	if config.UseTLS {
		c, err = client.DialTLS(addr, nil)
	} else {
		c, err = client.Dial(addr)
	}

	if err != nil {
		return fmt.Errorf("failed to connect to IMAP server: %w", err)
	}
	defer c.Logout()

	// Login
	if err := c.Login(config.Username, config.Password); err != nil {
		return fmt.Errorf("IMAP login failed: %w", err)
	}

	// Select INBOX
	if _, err := c.Select("INBOX", false); err != nil {
		return fmt.Errorf("failed to select INBOX: %w", err)
	}

	// Mark as seen
	seqSet := new(imap.SeqSet)
	seqSet.AddNum(messageID)

	item := imap.FormatFlagsOp(imap.AddFlags, true)
	flags := []interface{}{imap.SeenFlag}

	if err := c.Store(seqSet, item, flags, nil); err != nil {
		return fmt.Errorf("failed to mark message as read: %w", err)
	}

	s.logger.Debug().Uint32("message_id", messageID).Msg("Marked message as read")
	return nil
}

// parseMessageBody extracts the text body from an IMAP message
func (s *Service) parseMessageBody(msg *imap.Message, section *imap.BodySectionName) (string, error) {
	if msg == nil {
		return "", fmt.Errorf("nil message")
	}

	r := msg.GetBody(section)
	if r == nil {
		return "", fmt.Errorf("no body section")
	}

	// Parse email message
	mr, err := mail.CreateReader(r)
	if err != nil {
		return "", fmt.Errorf("failed to create mail reader: %w", err)
	}

	// Extract text body
	var body string
	for {
		p, err := mr.NextPart()
		if err == io.EOF {
			break
		}
		if err != nil {
			return "", fmt.Errorf("failed to read next part: %w", err)
		}

		switch h := p.Header.(type) {
		case *mail.InlineHeader:
			contentType, _, _ := h.ContentType()
			if strings.HasPrefix(contentType, "text/plain") {
				b, err := io.ReadAll(p.Body)
				if err != nil {
					return "", fmt.Errorf("failed to read body: %w", err)
				}
				body = string(b)
			}
		}
	}

	return strings.TrimSpace(body), nil
}
