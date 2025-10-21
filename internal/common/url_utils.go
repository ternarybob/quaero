package common

import (
	"fmt"
	"net/url"
	"strings"

	"github.com/ternarybob/arbor"
	"github.com/ternarybob/quaero/internal/models"
)

// ValidateBaseURL validates a base URL and detects test URL patterns
// Returns: (isValid bool, isTestURL bool, warnings []string, err error)
func ValidateBaseURL(baseURL string, logger arbor.ILogger) (bool, bool, []string, error) {
	warnings := []string{}

	// Parse URL
	parsedURL, err := url.Parse(baseURL)
	if err != nil {
		return false, false, warnings, fmt.Errorf("invalid URL format: %w", err)
	}

	// Validate scheme (must be http or https)
	if parsedURL.Scheme != "http" && parsedURL.Scheme != "https" {
		return false, false, warnings, fmt.Errorf("invalid URL scheme: %s (expected http or https)", parsedURL.Scheme)
	}

	// Validate host is not empty
	if parsedURL.Host == "" {
		return false, false, warnings, fmt.Errorf("URL host is empty")
	}

	// Check for test URL patterns
	isTestURL := false
	host := strings.ToLower(parsedURL.Host)

	// Pattern 1: localhost (any port)
	if strings.HasPrefix(host, "localhost") || strings.HasPrefix(host, "localhost:") {
		isTestURL = true
		warnings = append(warnings, fmt.Sprintf("Test URL detected: %s uses localhost", baseURL))
	}

	// Pattern 2: 127.0.0.1 (any port)
	if strings.HasPrefix(host, "127.0.0.1") {
		isTestURL = true
		warnings = append(warnings, fmt.Sprintf("Test URL detected: %s uses 127.0.0.1", baseURL))
	}

	// Pattern 3: 0.0.0.0 (any port)
	if strings.HasPrefix(host, "0.0.0.0") {
		isTestURL = true
		warnings = append(warnings, fmt.Sprintf("Test URL detected: %s uses 0.0.0.0", baseURL))
	}

	// Pattern 4: IPv6 localhost [::1]
	if strings.HasPrefix(host, "[::1]") {
		isTestURL = true
		warnings = append(warnings, fmt.Sprintf("Test URL detected: %s uses IPv6 localhost [::1]", baseURL))
	}

	// Pattern 5: Specific test server port 3333
	if strings.Contains(host, ":3333") {
		isTestURL = true
		warnings = append(warnings, fmt.Sprintf("Test URL detected: %s uses test server port 3333", baseURL))
	}

	// Log validation result
	if isTestURL {
		logger.Debug().
			Str("base_url", baseURL).
			Str("is_test_url", "true").
			Strs("warnings", warnings).
			Msg("Base URL validation: test URL detected")
	} else {
		logger.Debug().
			Str("base_url", baseURL).
			Str("is_test_url", "false").
			Msg("Base URL validation: production URL")
	}

	return true, isTestURL, warnings, nil
}

// DeriveSeedURLs determines the appropriate seed URLs based on source type and base URL.
// Seed URL derivation now relies solely on base URL and source type.
// For more specific targeting, users should provide explicit SeedURLs in the source configuration.
// This is a stateless helper function that can be used by any service or handler.
// The useHTMLSeeds flag controls whether to generate HTML page URLs (true) or REST API URLs (false).
func DeriveSeedURLs(source *models.SourceConfig, useHTMLSeeds bool, logger arbor.ILogger) []string {
	// Debug: Log derivation start
	logger.Debug().
		Str("source_type", string(source.Type)).
		Str("base_url", source.BaseURL).
		Str("use_html_seeds", fmt.Sprintf("%v", useHTMLSeeds)).
		Msg("Starting seed URL derivation")

	// If useHTMLSeeds is false, fall back to legacy REST API URL generation
	if !useHTMLSeeds {
		logger.Debug().Msg("Using legacy REST API URL generation")
		return deriveLegacySeedURLs(source, logger)
	}

	// Validate base URL format and detect test URLs
	isValid, isTestURL, warnings, err := ValidateBaseURL(source.BaseURL, logger)
	if !isValid || err != nil {
		logger.Warn().
			Err(err).
			Str("base_url", source.BaseURL).
			Msg("Base URL validation failed")
		return []string{}
	}

	// Log test URL warnings if detected
	if isTestURL && len(warnings) > 0 {
		for _, warning := range warnings {
			logger.Warn().
				Str("base_url", source.BaseURL).
				Msg(warning)
		}
	}

	// Parse base URL using net/url for proper normalization
	parsedURL, err := url.Parse(source.BaseURL)
	if err != nil {
		logger.Warn().
			Err(err).
			Str("base_url", source.BaseURL).
			Msg("Failed to parse base URL")
		return []string{}
	}

	// Debug: Log parsed URL components
	logger.Debug().
		Str("scheme", parsedURL.Scheme).
		Str("host", parsedURL.Host).
		Str("path", parsedURL.Path).
		Msg("Parsed base URL components")

	// Build base components
	baseRoot := fmt.Sprintf("%s://%s", parsedURL.Scheme, parsedURL.Host)
	basePath := parsedURL.Path
	if basePath != "" && basePath[len(basePath)-1] == '/' {
		basePath = basePath[:len(basePath)-1]
	}

	// Dispatch to appropriate helper based on source type
	switch source.Type {
	case models.SourceTypeJira:
		return deriveJiraSeeds(baseRoot, basePath, logger)
	case models.SourceTypeConfluence:
		return deriveConfluenceSeeds(baseRoot, basePath, logger)
	case models.SourceTypeGithub:
		return deriveGitHubSeeds(baseRoot, basePath, logger)
	default:
		logger.Warn().
			Str("source_type", string(source.Type)).
			Msg("Unknown source type")
		return []string{}
	}
}

// deriveLegacySeedURLs generates REST API endpoint URLs for backward compatibility
func deriveLegacySeedURLs(source *models.SourceConfig, logger arbor.ILogger) []string {
	parsedURL, err := url.Parse(source.BaseURL)
	if err != nil {
		logger.Warn().
			Err(err).
			Str("base_url", source.BaseURL).
			Msg("Failed to parse base URL")
		return []string{}
	}

	baseURL := fmt.Sprintf("%s://%s", parsedURL.Scheme, parsedURL.Host)

	switch source.Type {
	case models.SourceTypeJira:
		// Legacy REST API endpoint
		return []string{joinPath(baseURL, "rest", "api", "3", "project")}
	case models.SourceTypeConfluence:
		// Legacy REST API endpoint
		return []string{joinPath(baseURL, "wiki", "rest", "api", "space")}
	case models.SourceTypeGithub:
		// GitHub requires explicit seed URLs
		logger.Warn().Msg("GitHub requires explicit seed URLs")
		return []string{}
	default:
		logger.Warn().
			Str("source_type", string(source.Type)).
			Msg("Unknown source type")
		return []string{}
	}
}

// joinPath safely joins path segments, preventing duplicate slashes
func joinPath(segments ...string) string {
	result := ""
	for _, seg := range segments {
		if seg == "" {
			continue
		}
		if result == "" {
			result = seg
		} else if result[len(result)-1] == '/' {
			if seg[0] == '/' {
				result += seg[1:]
			} else {
				result += seg
			}
		} else {
			if seg[0] == '/' {
				result += seg
			} else {
				result += "/" + seg
			}
		}
	}
	return result
}

// hasWikiPath checks if basePath contains a "wiki" segment using segment-based detection
func hasWikiPath(basePath string) bool {
	if basePath == "" {
		return false
	}
	// Split basePath on "/" and check for a segment equal to "wiki"
	segments := strings.Split(basePath, "/")
	for _, segment := range segments {
		if segment == "wiki" {
			return true
		}
	}
	return false
}

// deriveJiraSeeds generates seed URLs for Jira sources based on base URL.
// Returns the default Jira projects listing page.
func deriveJiraSeeds(baseRoot, basePath string, logger arbor.ILogger) []string {
	logger.Debug().Msg("Using default Jira project listing page")
	return []string{joinPath(baseRoot, basePath, "projects")}
}

// deriveConfluenceSeeds generates seed URLs for Confluence sources based on base URL.
// Returns the default Confluence spaces directory page.
func deriveConfluenceSeeds(baseRoot, basePath string, logger arbor.ILogger) []string {
	// Determine if basePath already includes /wiki
	hasWiki := hasWikiPath(basePath)
	logger.Debug().
		Str("has_wiki_path", fmt.Sprintf("%v", hasWiki)).
		Str("base_path", basePath).
		Msg("Checking Confluence base path for /wiki segment")

	logger.Debug().Msg("Using default Confluence space directory page")
	if hasWiki {
		return []string{joinPath(baseRoot, basePath, "spaces")}
	}
	return []string{joinPath(baseRoot, basePath, "wiki", "spaces")}
}

// deriveGitHubSeeds generates seed URLs for GitHub sources based on base URL.
// GitHub requires explicit seed URLs - cannot derive from base URL alone.
func deriveGitHubSeeds(baseRoot, basePath string, logger arbor.ILogger) []string {
	logger.Warn().Msg("GitHub requires explicit seed URLs (org or user required)")
	return []string{}
}
