package types

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/ternarybob/quaero/internal/interfaces"
	"github.com/ternarybob/quaero/internal/queue"
)

// PreValidationJobDeps holds dependencies for pre-validation jobs
type PreValidationJobDeps struct {
	AuthStorage   interfaces.AuthStorage
	SourceStorage interfaces.SourceStorage
	HTTPClient    *http.Client
}

// PreValidationJob handles pre-flight validation jobs
type PreValidationJob struct {
	*BaseJob
	deps *PreValidationJobDeps
}

// NewPreValidationJob creates a new pre-validation job
func NewPreValidationJob(base *BaseJob, deps *PreValidationJobDeps) *PreValidationJob {
	return &PreValidationJob{
		BaseJob: base,
		deps:    deps,
	}
}

// Execute processes a pre-validation job
func (p *PreValidationJob) Execute(ctx context.Context, msg *queue.JobMessage) error {
	p.logger.Info().
		Str("message_id", msg.ID).
		Str("parent_id", msg.ParentID).
		Msg("Processing pre-validation job")

	// Validate message
	if err := p.Validate(msg); err != nil {
		return fmt.Errorf("invalid message: %w", err)
	}

	// Extract parent job ID
	parentID := msg.ParentID

	// Extract configuration
	sourceID := ""
	if sid, ok := msg.Config["source_id"].(string); ok {
		sourceID = sid
	}

	authID := ""
	if aid, ok := msg.Config["auth_id"].(string); ok {
		authID = aid
	}

	var seedURLs []string
	// Handle both []string and []interface{} for seed_urls
	if urls, ok := msg.Config["seed_urls"].([]string); ok {
		// Direct []string assignment
		seedURLs = urls
	} else if urls, ok := msg.Config["seed_urls"].([]interface{}); ok {
		// Convert []interface{} to []string
		for _, u := range urls {
			if urlStr, ok := u.(string); ok {
				seedURLs = append(seedURLs, urlStr)
			}
		}
	}

	// Log job start
	p.logger.LogJobStart("pre-validation", sourceID, msg.Config)

	// Validate Source Config (if provided)
	if sourceID != "" {
		sourceConfig, err := p.deps.SourceStorage.GetSource(ctx, sourceID)
		if err != nil {
			p.logger.Error().
				Err(err).
				Str("source_id", sourceID).
				Msg("Source config not found")
			return fmt.Errorf("source config not found: %w", err)
		}

		if err := sourceConfig.Validate(); err != nil {
			p.logger.Error().
				Err(err).
				Str("source_id", sourceID).
				Msg("Source config validation failed")
			return fmt.Errorf("source config validation failed: %w", err)
		}

		p.logger.Info().
			Str("source_id", sourceID).
			Str("base_url", sourceConfig.BaseURL).
			Msg("Source config validated")
	}

	// Validate Auth Config (if provided)
	if authID != "" {
		authConfig, err := p.deps.AuthStorage.GetCredentialsByID(ctx, authID)
		if err != nil {
			p.logger.Error().
				Err(err).
				Str("auth_id", authID).
				Msg("Auth config not found")
			return fmt.Errorf("auth config not found: %w", err)
		}

		// Validate auth credentials are present (check required fields)
		if authConfig == nil {
			return fmt.Errorf("auth config is nil")
		}

		if authConfig.ServiceType == "" {
			return fmt.Errorf("auth config service_type is required")
		}

		p.logger.Info().
			Str("auth_id", authID).
			Str("service_type", authConfig.ServiceType).
			Msg("Auth config validated")
	}

	// Validate Seed URLs (if provided)
	if len(seedURLs) > 0 {
		p.logger.Info().
			Int("seed_url_count", len(seedURLs)).
			Msg("Validating seed URLs")

		validatedCount := 0
		for _, url := range seedURLs {
			// Basic URL validation
			if url == "" {
				p.logger.Warn().
					Str("url", url).
					Msg("Empty seed URL found")
				continue
			}

			// Test URL accessibility with HEAD request
			req, err := http.NewRequestWithContext(ctx, "HEAD", url, nil)
			if err != nil {
				p.logger.Warn().
					Err(err).
					Str("url", url).
					Msg("Failed to create request for seed URL")
				continue
			}

			resp, err := p.deps.HTTPClient.Do(req)
			if err != nil {
				p.logger.Warn().
					Err(err).
					Str("url", url).
					Msg("Failed to reach seed URL")
				continue
			}
			resp.Body.Close()

			validatedCount++
		}

		p.logger.Info().
			Int("seed_urls_validated", validatedCount).
			Int("seed_urls_total", len(seedURLs)).
			Msg("Seed URL validation complete")
	}

	// Log completion
	p.logger.LogJobComplete(time.Since(time.Now()), 0)

	p.logger.Info().
		Str("message_id", msg.ID).
		Str("parent_id", parentID).
		Msg("Pre-validation job completed successfully")

	return nil
}

// Validate validates the pre-validation message
func (p *PreValidationJob) Validate(msg *queue.JobMessage) error {
	if msg.Config == nil {
		return fmt.Errorf("config is required")
	}

	// Validate source_id if present
	if sourceID, ok := msg.Config["source_id"].(string); ok && sourceID == "" {
		return fmt.Errorf("source_id cannot be empty")
	}

	// Validate auth_id if present
	if authID, ok := msg.Config["auth_id"].(string); ok && authID == "" {
		return fmt.Errorf("auth_id cannot be empty")
	}

	// Validate seed_urls if present
	if urls, ok := msg.Config["seed_urls"]; ok && urls != nil {
		switch v := urls.(type) {
		case []string:
			if len(v) == 0 {
				return fmt.Errorf("seed_urls cannot be empty")
			}
		case []interface{}:
			if len(v) == 0 {
				return fmt.Errorf("seed_urls cannot be empty")
			}
		default:
			return fmt.Errorf("seed_urls must be []string or []interface{}")
		}
	}

	return nil
}

// GetType returns the job type
func (p *PreValidationJob) GetType() string {
	return "pre_validation"
}
