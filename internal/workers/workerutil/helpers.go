// helpers.go - Shared helper functions for workers
// Common utilities for config parsing, ticker collection, and map operations

package workerutil

import (
	"github.com/ternarybob/quaero/internal/common"
	"github.com/ternarybob/quaero/internal/models"
)

// ParseTicker parses a ticker from config, supporting both legacy ("GNP") and
// exchange-qualified ("ASX:GNP") formats.
func ParseTicker(config map[string]interface{}) common.Ticker {
	// Try ticker first (new format), then asx_code (legacy)
	if ticker, ok := config["ticker"].(string); ok && ticker != "" {
		return common.ParseTicker(ticker)
	}
	if asxCode, ok := config["asx_code"].(string); ok && asxCode != "" {
		return common.ParseTicker(asxCode)
	}
	return common.Ticker{}
}

// CollectTickers collects all tickers from step config only.
// Supports: ticker, asx_code (single) and tickers, asx_codes (array).
// For job-level variables support, use CollectTickersWithJobDef instead.
func CollectTickers(config map[string]interface{}) []common.Ticker {
	return CollectTickersWithJobDef(config, models.JobDefinition{})
}

// CollectTickersWithJobDef collects all tickers from both step config and job-level variables.
// Sources (in order of priority):
//  1. Step config: ticker, asx_code (single)
//  2. Step config: tickers, asx_codes (array)
//  3. Job-level: config.variables = [{ ticker = "..." }, { asx_code = "..." }, ...]
func CollectTickersWithJobDef(stepConfig map[string]interface{}, jobDef models.JobDefinition) []common.Ticker {
	var tickers []common.Ticker
	seen := make(map[string]bool)

	addTicker := func(t common.Ticker) {
		if t.Code != "" && !seen[t.String()] {
			seen[t.String()] = true
			tickers = append(tickers, t)
		}
	}

	// Source 1: Single ticker from step config (legacy)
	if stepConfig != nil {
		if t := ParseTicker(stepConfig); t.Code != "" {
			addTicker(t)
		}

		// Source 2: Array of tickers from step config
		if tickerArray, ok := stepConfig["tickers"].([]interface{}); ok {
			for _, v := range tickerArray {
				if s, ok := v.(string); ok && s != "" {
					addTicker(common.ParseTicker(s))
				}
			}
		}

		// Array of asx_codes (legacy) from step config
		if codeArray, ok := stepConfig["asx_codes"].([]interface{}); ok {
			for _, v := range codeArray {
				if s, ok := v.(string); ok && s != "" {
					addTicker(common.ParseTicker(s))
				}
			}
		}
	}

	// Source 3: Job-level variables (multiple tickers)
	if jobDef.Config != nil {
		if vars, ok := jobDef.Config["variables"].([]interface{}); ok {
			for _, v := range vars {
				varMap, ok := v.(map[string]interface{})
				if !ok {
					continue
				}
				// Try "ticker" key (e.g., "ASX:GNP" or "GNP")
				if ticker, ok := varMap["ticker"].(string); ok && ticker != "" {
					addTicker(common.ParseTicker(ticker))
				}
				// Try "asx_code" key
				if asxCode, ok := varMap["asx_code"].(string); ok && asxCode != "" {
					addTicker(common.ParseTicker(asxCode))
				}
			}
		}
	}

	return tickers
}

// GetString gets a string value from a map, returning empty string if not found.
func GetString(m map[string]interface{}, key string) string {
	if v, ok := m[key].(string); ok {
		return v
	}
	return ""
}

// GetInt64 gets an int64 value from a map, handling various numeric types.
func GetInt64(m map[string]interface{}, key string) int64 {
	switch v := m[key].(type) {
	case int:
		return int64(v)
	case int64:
		return v
	case float64:
		return int64(v)
	case float32:
		return int64(v)
	}
	return 0
}

// GetFloat64 gets a float64 value from a map, handling various numeric types.
func GetFloat64(m map[string]interface{}, key string) float64 {
	switch v := m[key].(type) {
	case float64:
		return v
	case float32:
		return float64(v)
	case int:
		return float64(v)
	case int64:
		return float64(v)
	}
	return 0
}

// GetBool gets a bool value from a map, returning false if not found.
func GetBool(m map[string]interface{}, key string) bool {
	if v, ok := m[key].(bool); ok {
		return v
	}
	return false
}

// GetStringConfig gets a string from config with a default value.
func GetStringConfig(config map[string]interface{}, key, defaultValue string) string {
	if v, ok := config[key].(string); ok {
		return v
	}
	return defaultValue
}

// GetIntConfig gets an int from config with a default value.
func GetIntConfig(config map[string]interface{}, key string, defaultValue int) int {
	if v, ok := config[key].(float64); ok {
		return int(v)
	}
	if v, ok := config[key].(int); ok {
		return v
	}
	return defaultValue
}

// GetStringSliceConfig gets a string slice from config with a default value.
func GetStringSliceConfig(config map[string]interface{}, key string, defaultValue []string) []string {
	if v, ok := config[key].([]string); ok {
		return v
	}
	if v, ok := config[key].([]interface{}); ok {
		result := make([]string, 0, len(v))
		for _, item := range v {
			if s, ok := item.(string); ok {
				result = append(result, s)
			}
		}
		if len(result) > 0 {
			return result
		}
	}
	return defaultValue
}

// GetInputTags extracts input_tags from step config, defaulting to [stepName] if not specified.
// This enables a consistent pipeline pattern where:
//   - Each step outputs documents tagged with its step name
//   - Downstream steps consume documents by specifying input_tags (defaults to their own step name)
//   - job_id ensures we only get documents from the current job
//
// Parameters:
//   - config: step configuration map
//   - stepName: name of the current step (used as default if input_tags not specified)
//
// Returns the input_tags array (never empty - at minimum contains stepName)
func GetInputTags(config map[string]interface{}, stepName string) []string {
	// Check if input_tags is explicitly configured
	if tags, ok := config["input_tags"].([]interface{}); ok && len(tags) > 0 {
		result := make([]string, 0, len(tags))
		for _, t := range tags {
			if s, ok := t.(string); ok && s != "" {
				result = append(result, s)
			}
		}
		if len(result) > 0 {
			return result
		}
	}
	if tags, ok := config["input_tags"].([]string); ok && len(tags) > 0 {
		return tags
	}

	// Default to step name as the input tag
	if stepName != "" {
		return []string{stepName}
	}

	return nil
}
