package types

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/ternarybob/quaero/internal/common"
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

	// Log validation start
	if err := p.LogJobEvent(ctx, parentID, "info", "Starting pre-validation"); err != nil {
		p.logger.Warn().Err(err).Msg("Failed to log validation start event")
	}

	// Validate Source Config (if provided)
	if sourceID != "" {
		sourceConfig, err := p.deps.SourceStorage.GetSource(ctx, sourceID)
		if err != nil {
			p.logger.Error().
				Err(err).
				Str("source_id", sourceID).
				Msg("Source config not found")
			if logErr := p.LogJobEvent(ctx, parentID, "error",
				fmt.Sprintf("Source config not found: %s", err.Error())); logErr != nil {
				p.logger.Warn().Err(logErr).Msg("Failed to log error event")
			}
			return fmt.Errorf("source config not found: %w", err)
		}

		if err := sourceConfig.Validate(); err != nil {
			p.logger.Error().
				Err(err).
				Str("source_id", sourceID).
				Msg("Source config validation failed")
			if logErr := p.LogJobEvent(ctx, parentID, "error",
				fmt.Sprintf("Source config validation failed: %s", err.Error())); logErr != nil {
				p.logger.Warn().Err(logErr).Msg("Failed to log error event")
			}
			return fmt.Errorf("source config validation failed: %w", err)
		}

		p.logger.Info().
			Str("source_id", sourceID).
			Str("base_url", sourceConfig.BaseURL).
			Msg("Source config validated")
		if err := p.LogJobEvent(ctx, parentID, "info",
			fmt.Sprintf("Source config validated: base_url=%s", sourceConfig.BaseURL)); err != nil {
			p.logger.Warn().Err(err).Msg("Failed to log success event")
		}
	}

	// Validate Auth Credentials (if provided)
	if authID != "" {
		auth, err := p.deps.AuthStorage.GetCredentialsByID(ctx, authID)
		if err != nil {
			p.logger.Error().
				Err(err).
				Str("auth_id", authID).
				Msg("Auth credentials not found")
			if logErr := p.LogJobEvent(ctx, parentID, "error",
				fmt.Sprintf("Auth credentials not found: %s", err.Error())); logErr != nil {
				p.logger.Warn().Err(logErr).Msg("Failed to log error event")
			}
			return fmt.Errorf("auth credentials not found: %w", err)
		}

		// Check if cookies are available
		if len(auth.Cookies) == 0 {
			p.logger.Error().
				Str("auth_id", authID).
				Msg("Auth credentials missing cookies")
			if logErr := p.LogJobEvent(ctx, parentID, "error",
				"Auth credentials missing cookies"); logErr != nil {
				p.logger.Warn().Err(logErr).Msg("Failed to log error event")
			}
			return fmt.Errorf("auth credentials missing cookies")
		}

		// Unmarshal cookies to get count
		var cookies []interfaces.AtlassianExtensionCookie
		if err := json.Unmarshal(auth.Cookies, &cookies); err != nil {
			p.logger.Error().
				Err(err).
				Str("auth_id", authID).
				Msg("Failed to unmarshal auth cookies")
			if logErr := p.LogJobEvent(ctx, parentID, "error",
				fmt.Sprintf("Failed to unmarshal auth cookies: %s", err.Error())); logErr != nil {
				p.logger.Warn().Err(logErr).Msg("Failed to log error event")
			}
			return fmt.Errorf("failed to unmarshal auth cookies: %w", err)
		}

		cookieCount := len(cookies)
		p.logger.Info().
			Str("auth_id", authID).
			Int("cookie_count", cookieCount).
			Msg("Auth credentials validated")
		if err := p.LogJobEvent(ctx, parentID, "info",
			fmt.Sprintf("Auth credentials validated: %d cookies available", cookieCount)); err != nil {
			p.logger.Warn().Err(err).Msg("Failed to log success event")
		}
	}

	// Validate Seed URLs Accessibility
	if len(seedURLs) > 0 {
		failedURLs := []string{}
		successCount := 0

		for _, seedURL := range seedURLs {
			// Validate URL format
			isValid, _, warnings, err := common.ValidateBaseURL(seedURL, p.logger)
			if err != nil || !isValid {
				p.logger.Warn().
					Err(err).
					Str("seed_url", seedURL).
					Strs("warnings", warnings).
					Msg("Seed URL format invalid")
				failedURLs = append(failedURLs, seedURL)
				continue
			}

			// Check accessibility with HEAD request
			req, err := http.NewRequestWithContext(ctx, http.MethodHead, seedURL, nil)
			if err != nil {
				p.logger.Warn().
					Err(err).
					Str("seed_url", seedURL).
					Msg("Failed to create HEAD request")
				failedURLs = append(failedURLs, seedURL)
				continue
			}

			// Set timeout
			timeoutCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
			req = req.WithContext(timeoutCtx)

			resp, err := p.deps.HTTPClient.Do(req)
			cancel()

			if err != nil {
				p.logger.Warn().
					Err(err).
					Str("seed_url", seedURL).
					Msg("Seed URL not accessible")
				failedURLs = append(failedURLs, seedURL)
				continue
			}
			resp.Body.Close()

			if resp.StatusCode >= 400 {
				p.logger.Warn().
					Str("seed_url", seedURL).
					Int("status_code", resp.StatusCode).
					Msg("Seed URL returned error status")
				failedURLs = append(failedURLs, seedURL)
				continue
			}

			p.logger.Debug().
				Str("seed_url", seedURL).
				Msg("Seed URL accessible")
			successCount++
		}

		// Check if all URLs failed
		if len(failedURLs) == len(seedURLs) {
			p.logger.Error().
				Int("failed_count", len(failedURLs)).
				Int("total_count", len(seedURLs)).
				Msg("All seed URLs failed validation")
			if logErr := p.LogJobEvent(ctx, parentID, "error",
				"All seed URLs failed validation"); logErr != nil {
				p.logger.Warn().Err(logErr).Msg("Failed to log error event")
			}
			return fmt.Errorf("all seed URLs failed validation")
		}

		// Log warning if some failed
		if len(failedURLs) > 0 {
			p.logger.Warn().
				Int("failed_count", len(failedURLs)).
				Int("total_count", len(seedURLs)).
				Msg("Some seed URLs failed validation")
			if err := p.LogJobEvent(ctx, parentID, "warning",
				fmt.Sprintf("Some seed URLs failed validation: %d/%d", len(failedURLs), len(seedURLs))); err != nil {
				p.logger.Warn().Err(err).Msg("Failed to log warning event")
			}
		} else {
			p.logger.Info().
				Int("success_count", successCount).
				Msg("All seed URLs validated successfully")
		}
	}

	// Log validation completion
	if err := p.LogJobEvent(ctx, parentID, "info", "Pre-validation completed successfully"); err != nil {
		p.logger.Warn().Err(err).Msg("Failed to log completion event")
	}

	p.logger.Info().
		Str("message_id", msg.ID).
		Str("parent_id", parentID).
		Msg("Pre-validation job completed successfully")

	return nil
}

// Validate validates the pre-validation message
func (p *PreValidationJob) Validate(msg *queue.JobMessage) error {
	if msg.ParentID == "" {
		return fmt.Errorf("parent_id is required")
	}

	if msg.Config == nil {
		return fmt.Errorf("config is required")
	}

	return nil
}

// GetType returns the job type
func (p *PreValidationJob) GetType() string {
	return "pre_validation"
}
