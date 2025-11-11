// -----------------------------------------------------------------------
// Crawler Executor - Authentication Cookie Injection
// -----------------------------------------------------------------------

package processor

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/chromedp/cdproto/cdp"
	"github.com/chromedp/cdproto/network"
	"github.com/chromedp/chromedp"
	"github.com/ternarybob/arbor"
	"github.com/ternarybob/quaero/internal/interfaces"
	"github.com/ternarybob/quaero/internal/models"
)

// injectAuthCookies loads authentication credentials from storage and injects cookies into ChromeDP browser
func (e *CrawlerExecutor) injectAuthCookies(ctx context.Context, browserCtx context.Context, parentJobID, targetURL string, logger arbor.ILogger) error {
	logger.Debug().
		Str("parent_job_id", parentJobID).
		Str("target_url", targetURL).
		Msg("🔐 START: Cookie injection process initiated")

	// Check if authStorage is available
	if e.authStorage == nil {
		logger.Debug().Msg("🔐 SKIP: Auth storage not configured, skipping cookie injection")
		return nil
	}
	logger.Debug().Msg("🔐 OK: Auth storage is configured")

	// Get parent job from database to retrieve AuthID from job metadata
	logger.Debug().Str("parent_job_id", parentJobID).Msg("🔐 Fetching parent job from database")
	parentJobInterface, err := e.jobMgr.GetJob(ctx, parentJobID)
	if err != nil {
		logger.Error().Err(err).Str("parent_job_id", parentJobID).Msg("🔐 ERROR: Failed to get parent job for auth lookup")
		return fmt.Errorf("failed to get parent job: %w", err)
	}
	logger.Debug().Msg("🔐 OK: Parent job retrieved from database")

	// Extract JobModel from either JobModel or Job (which embeds JobModel)
	var authID string
	var jobModel *models.JobModel

	// Try Job first (embeds JobModel)
	if job, ok := parentJobInterface.(*models.Job); ok {
		logger.Debug().Msg("🔐 OK: Parent job is Job type (with embedded JobModel)")
		jobModel = job.JobModel
	} else if jm, ok := parentJobInterface.(*models.JobModel); ok {
		logger.Debug().Msg("🔐 OK: Parent job is JobModel type")
		jobModel = jm
	} else {
		logger.Error().
			Str("actual_type", fmt.Sprintf("%T", parentJobInterface)).
			Msg("🔐 ERROR: Parent job is neither Job nor JobModel type")
		return nil
	}

	if jobModel == nil {
		logger.Error().Msg("🔐 ERROR: JobModel is nil after extraction")
		return nil
	}

	logger.Debug().
		Int("metadata_count", len(jobModel.Metadata)).
		Msg("🔐 OK: JobModel extracted successfully")

	// Log all metadata keys for debugging
	metadataKeys := make([]string, 0, len(jobModel.Metadata))
	for k := range jobModel.Metadata {
		metadataKeys = append(metadataKeys, k)
	}
	logger.Debug().
		Strs("metadata_keys", metadataKeys).
		Msg("🔐 DEBUG: Parent job metadata keys")

	// Check metadata for auth_id
	if authIDVal, exists := jobModel.Metadata["auth_id"]; exists {
		if authIDStr, ok := authIDVal.(string); ok && authIDStr != "" {
			authID = authIDStr
			logger.Debug().
				Str("auth_id", authID).
				Msg("🔐 FOUND: Auth ID in job metadata")
		} else {
			logger.Debug().
				Str("auth_id_value", fmt.Sprintf("%v", authIDVal)).
				Msg("🔐 WARNING: auth_id exists but is not a valid string")
		}
	} else {
		logger.Debug().Msg("🔐 WARNING: auth_id NOT found in job metadata")
	}

	// If not in metadata, try job_definition_id
	if authID == "" {
		logger.Debug().Msg("🔐 Trying job_definition_id fallback")
		if jobDefID, exists := jobModel.Metadata["job_definition_id"]; exists {
			if jobDefIDStr, ok := jobDefID.(string); ok && jobDefIDStr != "" {
				logger.Debug().
					Str("job_def_id", jobDefIDStr).
					Msg("🔐 Found job_definition_id, fetching job definition")
				jobDef, err := e.jobDefStorage.GetJobDefinition(ctx, jobDefIDStr)
				if err != nil {
					logger.Error().Err(err).Str("job_def_id", jobDefIDStr).Msg("🔐 ERROR: Failed to get job definition for auth lookup")
					return fmt.Errorf("failed to get job definition: %w", err)
				}
				if jobDef != nil && jobDef.AuthID != "" {
					authID = jobDef.AuthID
					logger.Debug().
						Str("auth_id", authID).
						Str("job_def_id", jobDefIDStr).
						Msg("🔐 FOUND: Auth ID from job definition")
				} else {
					logger.Debug().
						Str("job_def_id", jobDefIDStr).
						Msg("🔐 WARNING: Job definition has no AuthID")
				}
			}
		} else {
			logger.Debug().Msg("🔐 WARNING: job_definition_id NOT found in metadata")
		}
	}

	if authID == "" {
		logger.Debug().Msg("🔐 SKIP: No auth_id found - skipping cookie injection")
		return nil
	}

	// Load authentication credentials from storage using AuthID
	logger.Debug().
		Str("auth_id", authID).
		Msg("🔐 Loading auth credentials from storage")
	authCreds, err := e.authStorage.GetCredentialsByID(ctx, authID)
	if err != nil {
		logger.Error().Err(err).Str("auth_id", authID).Msg("🔐 ERROR: Failed to load auth credentials from storage")
		return fmt.Errorf("failed to load auth credentials: %w", err)
	}

	if authCreds == nil {
		logger.Error().Str("auth_id", authID).Msg("🔐 ERROR: Auth credentials not found in storage")
		return fmt.Errorf("auth credentials not found for ID: %s", authID)
	}
	logger.Debug().
		Str("auth_id", authID).
		Str("site_domain", authCreds.SiteDomain).
		Msg("🔐 OK: Auth credentials loaded successfully")

	// Unmarshal cookies from JSON
	logger.Debug().Msg("🔐 Unmarshaling cookies from JSON")
	var extensionCookies []*interfaces.AtlassianExtensionCookie
	if err := json.Unmarshal(authCreds.Cookies, &extensionCookies); err != nil {
		logger.Error().Err(err).Msg("🔐 ERROR: Failed to unmarshal cookies from auth credentials")
		return fmt.Errorf("failed to unmarshal cookies: %w", err)
	}

	if len(extensionCookies) == 0 {
		logger.Debug().Msg("🔐 WARNING: No cookies found in auth credentials")
		return nil
	}

	logger.Debug().
		Int("cookie_count", len(extensionCookies)).
		Str("site_domain", authCreds.SiteDomain).
		Msg("🔐 SUCCESS: Cookies loaded - preparing to inject into browser")

	// Parse target URL to get domain
	targetURLParsed, err := url.Parse(targetURL)
	if err != nil {
		return fmt.Errorf("failed to parse target URL: %w", err)
	}

	// ===== PHASE 1: PRE-INJECTION DOMAIN DIAGNOSTICS =====
	logger.Debug().
		Str("target_url", targetURL).
		Str("target_domain", targetURLParsed.Host).
		Str("target_scheme", targetURLParsed.Scheme).
		Msg("🔐 DIAGNOSTIC: Target URL parsed for domain analysis")

	// Analyze each cookie's domain compatibility with target URL
	logger.Debug().Msg("🔐 DIAGNOSTIC: Analyzing cookie domain compatibility with target URL")
	for i, c := range extensionCookies {
		cookieDomain := c.Domain
		if cookieDomain == "" {
			logger.Debug().
				Int("cookie_index", i).
				Str("cookie_name", c.Name).
				Msg("🔐 DIAGNOSTIC: Cookie has no domain (will use target domain)")
			continue
		}

		// Normalize cookie domain (remove leading dot for comparison)
		normalizedCookieDomain := strings.TrimPrefix(cookieDomain, ".")
		targetHost := targetURLParsed.Host

		// Check domain matching logic
		var matchType string
		isMatch := false
		if normalizedCookieDomain == targetHost {
			matchType = "exact_match"
			isMatch = true
		} else if strings.HasSuffix(targetHost, "."+normalizedCookieDomain) {
			matchType = "parent_domain_match"
			isMatch = true
		} else if strings.HasSuffix(normalizedCookieDomain, "."+targetHost) {
			matchType = "subdomain_of_target"
			isMatch = false
		} else {
			matchType = "domain_mismatch"
			isMatch = false
		}

		logger.Debug().
			Int("cookie_index", i).
			Str("cookie_name", c.Name).
			Str("cookie_domain", cookieDomain).
			Str("normalized_cookie_domain", normalizedCookieDomain).
			Str("target_domain", targetHost).
			Str("match_type", matchType).
			Bool("will_be_sent", isMatch).
			Msg("🔐 DIAGNOSTIC: Cookie domain analysis")

		if !isMatch {
			logger.Debug().
				Str("cookie_name", c.Name).
				Str("cookie_domain", cookieDomain).
				Str("target_domain", targetHost).
				Msg("🔐 WARNING: Cookie domain mismatch - cookie may not be sent with requests")
		}

		// Check secure flag compatibility with scheme
		if c.Secure && targetURLParsed.Scheme != "https" {
			logger.Debug().
				Str("cookie_name", c.Name).
				Str("target_scheme", targetURLParsed.Scheme).
				Msg("🔐 WARNING: Secure cookie will not be sent to non-HTTPS URL")
		}
	}
	// ===== END PHASE 1 =====

	// Convert extension cookies to ChromeDP network cookies
	logger.Debug().
		Int("cookie_count", len(extensionCookies)).
		Str("target_domain", targetURLParsed.Host).
		Msg("🔐 Converting extension cookies to ChromeDP format")
	var chromeDPCookies []*network.CookieParam
	for i, c := range extensionCookies {
		// Calculate expiration timestamp
		var expires *cdp.TimeSinceEpoch
		if c.Expires > 0 {
			expiresTime := time.Unix(c.Expires, 0)
			// Only set expiration if it's in the future
			if expiresTime.After(time.Now()) {
				timestamp := cdp.TimeSinceEpoch(expiresTime)
				expires = &timestamp
			}
		}

		// Determine the domain to use for this cookie
		cookieDomain := c.Domain
		if cookieDomain == "" {
			cookieDomain = targetURLParsed.Host
		}
		// Remove leading dot if present (ChromeDP doesn't like it)
		cookieDomain = strings.TrimPrefix(cookieDomain, ".")

		chromeDPCookie := &network.CookieParam{
			Name:     c.Name,
			Value:    c.Value,
			Domain:   cookieDomain,
			Path:     c.Path,
			Secure:   c.Secure,
			HTTPOnly: c.HTTPOnly,
			Expires:  expires,
		}

		// Set SameSite attribute if available
		if c.SameSite != "" {
			switch strings.ToLower(c.SameSite) {
			case "strict":
				chromeDPCookie.SameSite = network.CookieSameSiteStrict
			case "lax":
				chromeDPCookie.SameSite = network.CookieSameSiteLax
			case "none":
				chromeDPCookie.SameSite = network.CookieSameSiteNone
			}
		}

		chromeDPCookies = append(chromeDPCookies, chromeDPCookie)

		logger.Debug().
			Int("cookie_index", i).
			Str("name", c.Name).
			Str("domain", cookieDomain).
			Str("path", c.Path).
			Bool("secure", c.Secure).
			Bool("http_only", c.HTTPOnly).
			Msg("🔐 OK: Prepared cookie for injection")
	}

	// ===== PHASE 2: NETWORK DOMAIN ENABLEMENT =====
	logger.Debug().Msg("🔐 DIAGNOSTIC: Enabling ChromeDP network domain for cookie operations")
	err = chromedp.Run(browserCtx, network.Enable())
	if err != nil {
		logger.Error().Err(err).Msg("🔐 ERROR: Failed to enable network domain")
		return fmt.Errorf("failed to enable network domain: %w", err)
	}
	logger.Debug().Msg("🔐 SUCCESS: Network domain enabled successfully")
	// ===== END PHASE 2 PART 1 =====

	// Inject cookies into browser using ChromeDP
	logger.Debug().
		Int("cookie_count", len(chromeDPCookies)).
		Msg("🔐 Starting browser cookie injection via ChromeDP")

	err = chromedp.Run(browserCtx,
		chromedp.ActionFunc(func(ctx context.Context) error {
			successCount := 0
			failCount := 0

			// Set all cookies
			for _, cookie := range chromeDPCookies {
				if err := network.SetCookie(cookie.Name, cookie.Value).
					WithDomain(cookie.Domain).
					WithPath(cookie.Path).
					WithSecure(cookie.Secure).
					WithHTTPOnly(cookie.HTTPOnly).
					WithSameSite(cookie.SameSite).
					WithExpires(cookie.Expires).
					Do(ctx); err != nil {
					failCount++
					logger.Error().
						Err(err).
						Str("cookie_name", cookie.Name).
						Str("domain", cookie.Domain).
						Str("path", cookie.Path).
						Msg("🔐 ERROR: Failed to inject cookie into browser")
					// Continue with other cookies even if one fails
				} else {
					successCount++
					logger.Debug().
						Str("cookie_name", cookie.Name).
						Str("domain", cookie.Domain).
						Msg("🔐 OK: Cookie injected successfully")
				}
			}

			logger.Debug().
				Int("success_count", successCount).
				Int("fail_count", failCount).
				Int("total_cookies", len(chromeDPCookies)).
				Msg("🔐 Cookie injection batch complete")

			return nil
		}),
	)

	if err != nil {
		logger.Error().
			Err(err).
			Str("target_url", targetURL).
			Int("cookies_attempted", len(chromeDPCookies)).
			Msg("🔐 ERROR: ChromeDP failed to inject cookies into browser")
		return fmt.Errorf("failed to inject cookies: %w", err)
	}

	logger.Info().
		Int("cookies_injected", len(chromeDPCookies)).
		Str("target_domain", targetURLParsed.Host).
		Msg("🔐 SUCCESS: Authentication cookies injected into browser")

	// ===== PHASE 2 PART 2: POST-INJECTION VERIFICATION =====
	logger.Debug().
		Str("target_url", targetURL).
		Msg("🔐 DIAGNOSTIC: Verifying cookies after injection using network.GetCookies()")

	var verifiedCookies []*network.Cookie
	err = chromedp.Run(browserCtx,
		chromedp.ActionFunc(func(ctx context.Context) error {
			cookies, err := network.GetCookies().WithURLs([]string{targetURL}).Do(ctx)
			if err != nil {
				return err
			}
			verifiedCookies = cookies
			return nil
		}),
	)

	if err != nil {
		logger.Error().
			Err(err).
			Str("target_url", targetURL).
			Msg("🔐 ERROR: Failed to verify cookies after injection")
		// Don't return error - continue with warning
	} else {
		logger.Debug().
			Int("verified_cookie_count", len(verifiedCookies)).
			Int("injected_cookie_count", len(chromeDPCookies)).
			Msg("🔐 DIAGNOSTIC: Cookie verification complete")

		// Log details of each verified cookie
		for i, cookie := range verifiedCookies {
			// Truncate value for security (show first 20 chars)
			valuePreview := cookie.Value
			if len(valuePreview) > 20 {
				valuePreview = valuePreview[:20] + "..."
			}

			expiryStr := "session"
			if cookie.Expires > 0 {
				expiryStr = time.Unix(int64(cookie.Expires), 0).Format(time.RFC3339)
			}

			logger.Debug().
				Int("cookie_index", i).
				Str("name", cookie.Name).
				Str("value_preview", valuePreview).
				Str("domain", cookie.Domain).
				Str("path", cookie.Path).
				Bool("secure", cookie.Secure).
				Bool("http_only", cookie.HTTPOnly).
				Str("same_site", string(cookie.SameSite)).
				Str("expires", expiryStr).
				Msg("🔐 DIAGNOSTIC: Verified cookie details")
		}

		// Compare injected vs verified cookies
		injectedCookieNames := make(map[string]bool)
		for _, cookie := range chromeDPCookies {
			injectedCookieNames[cookie.Name] = true
		}

		verifiedCookieNames := make(map[string]bool)
		for _, cookie := range verifiedCookies {
			verifiedCookieNames[cookie.Name] = true
		}

		// Check for missing cookies (injected but not verified)
		missingCookies := []string{}
		for name := range injectedCookieNames {
			if !verifiedCookieNames[name] {
				missingCookies = append(missingCookies, name)
			}
		}

		// Check for unexpected cookies (verified but not injected)
		unexpectedCookies := []string{}
		for name := range verifiedCookieNames {
			if !injectedCookieNames[name] {
				unexpectedCookies = append(unexpectedCookies, name)
			}
		}

		// Log mismatches
		if len(missingCookies) > 0 {
			logger.Error().
				Strs("missing_cookies", missingCookies).
				Int("missing_count", len(missingCookies)).
				Msg("🔐 ERROR: Cookies were injected but not verified (failed to persist)")
		}

		if len(unexpectedCookies) > 0 {
			logger.Debug().
				Strs("unexpected_cookies", unexpectedCookies).
				Int("unexpected_count", len(unexpectedCookies)).
				Msg("🔐 WARNING: Cookies verified but not injected (pre-existing or set by page)")
		}

		// Final verdict
		if len(verifiedCookies) == len(chromeDPCookies) && len(missingCookies) == 0 {
			logger.Debug().
				Int("cookie_count", len(verifiedCookies)).
				Msg("🔐 SUCCESS: All injected cookies verified successfully")
		} else {
			logger.Warn().
				Int("injected", len(chromeDPCookies)).
				Int("verified", len(verifiedCookies)).
				Int("missing", len(missingCookies)).
				Msg("🔐 WARNING: Cookie injection/verification mismatch detected")
		}
	}
	// ===== END PHASE 2 =====

	e.publishCrawlerJobLog(ctx, parentJobID, "info", fmt.Sprintf("Injected %d authentication cookies into browser", len(chromeDPCookies)), map[string]interface{}{
		"cookie_count":  len(chromeDPCookies),
		"site_domain":   authCreds.SiteDomain,
		"target_domain": targetURLParsed.Host,
	})

	return nil
}
