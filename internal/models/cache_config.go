// Package models provides cache configuration types for document caching.
package models

import (
	"fmt"
	"regexp"
	"strings"
	"time"
)

// CacheType represents the document caching strategy
type CacheType string

const (
	// CacheTypeNone disables caching for the step
	CacheTypeNone CacheType = "none"

	// CacheTypeRollingTime considers documents fresh if created within N hours from now
	CacheTypeRollingTime CacheType = "rolling_time"

	// CacheTypeHardTime considers documents fresh if created within current day (00:00 UTC boundary)
	CacheTypeHardTime CacheType = "hard_time"

	// CacheTypeAuto uses AI-based assessment to determine if content needs refresh
	// Currently implemented as rolling_time with 24h window (stub for future AI implementation)
	CacheTypeAuto CacheType = "auto"
)

// CacheConfig holds cache settings for job or step
type CacheConfig struct {
	Type      CacheType // Cache type (default: auto)
	Hours     int       // Cache window in hours (default: 24)
	Revisions int       // Number of revisions to keep (default: 1)
	Enabled   bool      // Whether caching is enabled (default: true)
}

// DefaultCacheConfig returns the default cache configuration
func DefaultCacheConfig() CacheConfig {
	return CacheConfig{
		Type:      CacheTypeAuto,
		Hours:     24,
		Revisions: 1,
		Enabled:   true,
	}
}

// ParseCacheConfig extracts cache config from a map[string]interface{}
// Handles various input formats for backward compatibility:
//   - cache = false → Enabled=false
//   - cache_type = "none" → Type=none
//   - cache_hours = 12 → Hours=12
//   - revisions = 3 → Revisions=3
func ParseCacheConfig(config map[string]interface{}) CacheConfig {
	result := DefaultCacheConfig()

	if config == nil {
		return result
	}

	// Check for cache = false (opt-out)
	if enabled, ok := config["cache"].(bool); ok {
		result.Enabled = enabled
		if !enabled {
			return result
		}
	}

	// Parse cache_type
	if cacheType, ok := config["cache_type"].(string); ok {
		switch strings.ToLower(cacheType) {
		case "none":
			result.Type = CacheTypeNone
			result.Enabled = false
		case "rolling_time":
			result.Type = CacheTypeRollingTime
		case "hard_time":
			result.Type = CacheTypeHardTime
		case "auto":
			result.Type = CacheTypeAuto
		}
	}

	// Parse cache_hours
	if hours, ok := config["cache_hours"].(float64); ok {
		result.Hours = int(hours)
	} else if hours, ok := config["cache_hours"].(int); ok {
		result.Hours = hours
	}

	// Parse revisions
	if revisions, ok := config["revisions"].(float64); ok {
		result.Revisions = int(revisions)
	} else if revisions, ok := config["revisions"].(int); ok {
		result.Revisions = revisions
	}

	// Validate bounds
	if result.Hours < 0 {
		result.Hours = 24
	}
	if result.Revisions < 1 {
		result.Revisions = 1
	}

	return result
}

// ResolveCacheConfig combines job-level and step-level cache settings.
// Step config overrides job config where specified.
func ResolveCacheConfig(jobConfig, stepConfig map[string]interface{}) CacheConfig {
	// Start with job-level config (or defaults if nil)
	result := ParseCacheConfig(jobConfig)

	if stepConfig == nil {
		return result
	}

	// Check for step-level opt-out
	if enabled, ok := stepConfig["cache"].(bool); ok {
		result.Enabled = enabled
		if !enabled {
			return result
		}
	}

	// Override with step-level cache_type if present
	if cacheType, ok := stepConfig["cache_type"].(string); ok {
		switch strings.ToLower(cacheType) {
		case "none":
			result.Type = CacheTypeNone
			result.Enabled = false
		case "rolling_time":
			result.Type = CacheTypeRollingTime
		case "hard_time":
			result.Type = CacheTypeHardTime
		case "auto":
			result.Type = CacheTypeAuto
		}
	}

	// Override with step-level cache_hours if present
	if hours, ok := stepConfig["cache_hours"].(float64); ok {
		result.Hours = int(hours)
	} else if hours, ok := stepConfig["cache_hours"].(int); ok {
		result.Hours = hours
	}

	// Override with step-level revisions if present
	if revisions, ok := stepConfig["revisions"].(float64); ok {
		result.Revisions = int(revisions)
	} else if revisions, ok := stepConfig["revisions"].(int); ok {
		result.Revisions = revisions
	}

	return result
}

// CacheTagInfo holds parsed cache tag information
type CacheTagInfo struct {
	JobDefID    string
	RunDate     string
	StepName    string
	Revision    int
	ContentHash string // Content hash for cache invalidation (from hash: tag)
}

// GenerateCacheTags creates deterministic cache tags for a step execution.
// Tags are hierarchical and enable filtering at job and step levels.
// Format:
//   - jobdef:<job-definition-id>
//   - run:<YYYY-MM-DD>
//   - step:<step-name>
//   - rev:<revision-number>
//   - hash:<content-hash> (optional, if contentHash is non-empty)
func GenerateCacheTags(jobDefID, stepName string, revision int) []string {
	date := time.Now().UTC().Format("2006-01-02")
	return []string{
		fmt.Sprintf("jobdef:%s", sanitizeTag(jobDefID)),
		fmt.Sprintf("run:%s", date),
		fmt.Sprintf("step:%s", sanitizeTag(stepName)),
		fmt.Sprintf("rev:%d", revision),
	}
}

// GenerateCacheTagsWithHash creates cache tags including a content hash for cache invalidation.
// The content hash ensures that when prompt/template content changes, a cache miss occurs.
// Format:
//   - jobdef:<job-definition-id>
//   - run:<YYYY-MM-DD>
//   - step:<step-name>
//   - rev:<revision-number>
//   - hash:<content-hash> (if contentHash is non-empty)
func GenerateCacheTagsWithHash(jobDefID, stepName string, revision int, contentHash string) []string {
	tags := GenerateCacheTags(jobDefID, stepName, revision)
	if contentHash != "" {
		tags = append(tags, fmt.Sprintf("hash:%s", contentHash))
	}
	return tags
}

// sanitizeTag ensures tag is valid (lowercase, no spaces, safe chars only)
var tagSanitizeRegex = regexp.MustCompile(`[^a-z0-9\-_]`)

func sanitizeTag(s string) string {
	s = strings.ToLower(s)
	s = strings.ReplaceAll(s, " ", "-")
	return tagSanitizeRegex.ReplaceAllString(s, "")
}

// ParseCacheTags extracts cache tag info from a slice of tags
func ParseCacheTags(tags []string) *CacheTagInfo {
	info := &CacheTagInfo{Revision: 1}
	for _, tag := range tags {
		switch {
		case strings.HasPrefix(tag, "jobdef:"):
			info.JobDefID = strings.TrimPrefix(tag, "jobdef:")
		case strings.HasPrefix(tag, "run:"):
			info.RunDate = strings.TrimPrefix(tag, "run:")
		case strings.HasPrefix(tag, "step:"):
			info.StepName = strings.TrimPrefix(tag, "step:")
		case strings.HasPrefix(tag, "rev:"):
			fmt.Sscanf(tag, "rev:%d", &info.Revision)
		case strings.HasPrefix(tag, "hash:"):
			info.ContentHash = strings.TrimPrefix(tag, "hash:")
		}
	}
	return info
}

// MergeTags combines multiple tag slices, removing duplicates and empty strings
func MergeTags(tagSets ...[]string) []string {
	seen := make(map[string]bool)
	var result []string
	for _, tags := range tagSets {
		for _, tag := range tags {
			if tag != "" && !seen[tag] {
				seen[tag] = true
				result = append(result, tag)
			}
		}
	}
	return result
}
