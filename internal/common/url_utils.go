package common

// URL utilities for source configuration validation.
//
// NOTE: Seed URL derivation functions have been removed. Jobs now specify start URLs directly
// instead of deriving them from source base URLs. This aligns with the architectural principle
// that sources define "WHAT to connect to" (base URL, auth, type) while jobs define
// "HOW to crawl" (start URLs, filtering, depth, concurrency).

import (
	"fmt"
	"net/url"
	"strings"

	"github.com/ternarybob/arbor"
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
