package common

import (
	"fmt"
	"net/url"
	"strings"

	"github.com/ternarybob/arbor"
	"github.com/ternarybob/quaero/internal/models"
)

// DeriveSeedURLs determines the appropriate seed URLs based on source type
// This is a stateless helper function that can be used by any service or handler
// The useHTMLSeeds flag controls whether to generate HTML page URLs (true) or REST API URLs (false)
func DeriveSeedURLs(source *models.SourceConfig, useHTMLSeeds bool, logger arbor.ILogger) []string {
	// If useHTMLSeeds is false, fall back to legacy REST API URL generation
	if !useHTMLSeeds {
		return deriveLegacySeedURLs(source, logger)
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

	// Build base components
	baseRoot := fmt.Sprintf("%s://%s", parsedURL.Scheme, parsedURL.Host)
	basePath := parsedURL.Path
	if basePath != "" && basePath[len(basePath)-1] == '/' {
		basePath = basePath[:len(basePath)-1]
	}

	// Dispatch to appropriate helper based on source type
	switch source.Type {
	case models.SourceTypeJira:
		return deriveJiraSeeds(baseRoot, basePath, source.Filters, logger)
	case models.SourceTypeConfluence:
		return deriveConfluenceSeeds(baseRoot, basePath, source.Filters, logger)
	case models.SourceTypeGithub:
		return deriveGitHubSeeds(baseRoot, basePath, source.Filters, logger)
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
		// GitHub uses web URLs even in legacy mode
		if org, ok := source.Filters["org"].(string); ok {
			return []string{joinPath(baseURL, "orgs", org, "repos")}
		}
		if user, ok := source.Filters["user"].(string); ok {
			return []string{joinPath(baseURL, "users", user, "repos")}
		}
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

// deriveJiraSeeds generates seed URLs for Jira sources
func deriveJiraSeeds(baseRoot, basePath string, filters map[string]interface{}, logger arbor.ILogger) []string {
	// Check for projects filter ([]string first, then []interface{})
	if projects, ok := filters["projects"].([]string); ok {
		var seedURLs []string
		for _, projectKey := range projects {
			seedURLs = append(seedURLs, joinPath(baseRoot, basePath, "browse", projectKey))
		}
		if len(seedURLs) > 0 {
			return seedURLs
		}
	} else if projects, ok := filters["projects"].([]interface{}); ok {
		var seedURLs []string
		for _, proj := range projects {
			if projectKey, ok := proj.(string); ok {
				seedURLs = append(seedURLs, joinPath(baseRoot, basePath, "browse", projectKey))
			} else {
				logger.Warn().Msg("Invalid project key type in projects filter")
			}
		}
		if len(seedURLs) > 0 {
			return seedURLs
		}
	}
	// Check for project filter (single string)
	if project, ok := filters["project"].(string); ok {
		return []string{joinPath(baseRoot, basePath, "browse", project)}
	}
	// No filter - use project listing page
	return []string{joinPath(baseRoot, basePath, "projects")}
}

// deriveConfluenceSeeds generates seed URLs for Confluence sources
func deriveConfluenceSeeds(baseRoot, basePath string, filters map[string]interface{}, logger arbor.ILogger) []string {
	// Determine if basePath already includes /wiki
	hasWiki := hasWikiPath(basePath)

	// Check for spaces filter ([]string first, then []interface{})
	if spaces, ok := filters["spaces"].([]string); ok {
		var seedURLs []string
		for _, spaceKey := range spaces {
			if hasWiki {
				seedURLs = append(seedURLs, joinPath(baseRoot, basePath, "spaces", spaceKey))
			} else {
				seedURLs = append(seedURLs, joinPath(baseRoot, basePath, "wiki", "spaces", spaceKey))
			}
		}
		if len(seedURLs) > 0 {
			return seedURLs
		}
	} else if spaces, ok := filters["spaces"].([]interface{}); ok {
		var seedURLs []string
		for _, sp := range spaces {
			if spaceKey, ok := sp.(string); ok {
				if hasWiki {
					seedURLs = append(seedURLs, joinPath(baseRoot, basePath, "spaces", spaceKey))
				} else {
					seedURLs = append(seedURLs, joinPath(baseRoot, basePath, "wiki", "spaces", spaceKey))
				}
			} else {
				logger.Warn().Msg("Invalid space key type in spaces filter")
			}
		}
		if len(seedURLs) > 0 {
			return seedURLs
		}
	}
	// Check for space filter (single string)
	if space, ok := filters["space"].(string); ok {
		if hasWiki {
			return []string{joinPath(baseRoot, basePath, "spaces", space)}
		}
		return []string{joinPath(baseRoot, basePath, "wiki", "spaces", space)}
	}
	// No filter - use space directory page
	if hasWiki {
		return []string{joinPath(baseRoot, basePath, "spaces")}
	}
	return []string{joinPath(baseRoot, basePath, "wiki", "spaces")}
}

// deriveGitHubSeeds generates seed URLs for GitHub sources
func deriveGitHubSeeds(baseRoot, basePath string, filters map[string]interface{}, logger arbor.ILogger) []string {
	// Check for org filter
	if org, ok := filters["org"].(string); ok {
		return []string{joinPath(baseRoot, basePath, "orgs", org, "repos")}
	}
	// Check for user filter
	if user, ok := filters["user"].(string); ok {
		return []string{joinPath(baseRoot, basePath, "users", user, "repos")}
	}
	return []string{}
}
