// helpers.go - Shared helper functions for workers
// Common utilities for config parsing, ticker collection, and map operations

package workerutil

import (
	"context"
	"time"

	"github.com/ternarybob/arbor"
	"github.com/ternarybob/quaero/internal/common"
	"github.com/ternarybob/quaero/internal/interfaces"
	"github.com/ternarybob/quaero/internal/models"
)

// JobGetter is a minimal interface for getting job state.
// This allows workers to look up the manager_id from their step job.
type JobGetter interface {
	GetJob(ctx context.Context, jobID string) (interface{}, error)
}

// GetManagerID extracts the manager_id from the step job's metadata.
// This is used by workers to tag documents with the orchestrator job ID,
// enabling job isolation across all steps in the same pipeline.
//
// The manager_id is set by the dispatcher when creating step jobs and ensures
// that all documents from the same pipeline run share the same identifier,
// allowing downstream workers (like output_formatter) to find documents
// created by upstream workers.
//
// Parameters:
//   - ctx: context for the operation
//   - jobMgr: interface with GetJob method to retrieve the step job state
//   - stepID: the current step's job ID
//
// Returns the manager_id or stepID as fallback if manager_id not found.
func GetManagerID(ctx context.Context, jobMgr JobGetter, stepID string) string {
	if jobMgr == nil || stepID == "" {
		return stepID
	}

	jobInterface, err := jobMgr.GetJob(ctx, stepID)
	if err != nil {
		return stepID // Fallback to stepID if job lookup fails
	}

	// Type assert to QueueJobState
	jobState, ok := jobInterface.(*models.QueueJobState)
	if !ok || jobState == nil {
		return stepID
	}

	// First check the ManagerID field directly (if set)
	if jobState.ManagerID != nil && *jobState.ManagerID != "" {
		return *jobState.ManagerID
	}

	// Check metadata for manager_id
	if jobState.Metadata != nil {
		if managerID, ok := jobState.Metadata["manager_id"].(string); ok && managerID != "" {
			return managerID
		}
	}

	// Check config for manager_id (tool jobs created by orchestrator store it here)
	if jobState.Config != nil {
		if managerID, ok := jobState.Config["manager_id"].(string); ok && managerID != "" {
			return managerID
		}
	}

	return stepID // Ultimate fallback
}

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

// GetOutputTags extracts output_tags from step config.
// This enables pipeline routing where parent steps pass their output_tags to sub-workers,
// allowing downstream steps to find documents created by intermediate workers.
//
// Parameters:
//   - config: step configuration map
//
// Returns the output_tags array (may be empty if not configured)
func GetOutputTags(config map[string]interface{}) []string {
	if tags, ok := config["output_tags"].([]interface{}); ok && len(tags) > 0 {
		result := make([]string, 0, len(tags))
		for _, t := range tags {
			if s, ok := t.(string); ok && s != "" {
				result = append(result, s)
			}
		}
		return result
	}
	if tags, ok := config["output_tags"].([]string); ok && len(tags) > 0 {
		return tags
	}
	return nil
}

// MergeOutputTags merges parent step output_tags with sub-worker specific tags.
// The parent step's output_tags come first (for pipeline routing), followed by sub-worker tags.
// This enables intermediate workers like DataCollectionWorker to pass through pipeline routing tags
// while still adding their own descriptive tags.
func MergeOutputTags(parentOutputTags []string, subWorkerTags ...string) []string {
	result := make([]string, 0, len(parentOutputTags)+len(subWorkerTags))
	result = append(result, parentOutputTags...)
	result = append(result, subWorkerTags...)
	return result
}

// CollectTickersFromUpstreamDocs searches for upstream documents via input_tags and extracts tickers.
// This enables workers to get tickers from upstream steps like navexa_portfolio which produces
// documents with holdings data containing ticker symbols.
//
// Parameters:
//   - ctx: context for the operation
//   - searchService: for tag-based document lookup
//   - stepConfig: step configuration map (contains input_tags)
//   - stepName: name of the current step (used as default input_tag)
//   - managerID: job isolation ID for document filtering
//   - logger: for logging (can be nil)
//
// Returns slice of tickers extracted from upstream documents.
// Handles navexa_portfolio documents (extracts from holdings array).
func CollectTickersFromUpstreamDocs(
	ctx context.Context,
	searchService interfaces.SearchService,
	stepConfig map[string]interface{},
	stepName string,
	managerID string,
	logger arbor.ILogger,
) []common.Ticker {
	if searchService == nil {
		return nil
	}

	// Get input tags for document filtering
	inputTags := GetInputTags(stepConfig, stepName)
	if len(inputTags) == 0 {
		return nil
	}

	var tickers []common.Ticker
	seen := make(map[string]bool)

	addTicker := func(t common.Ticker) {
		if t.Code != "" && !seen[t.String()] {
			seen[t.String()] = true
			tickers = append(tickers, t)
		}
	}

	// Search for documents with input tags
	searchOpts := interfaces.SearchOptions{
		Tags:  inputTags,
		JobID: managerID,
		Limit: 100,
	}

	docs, err := searchService.Search(ctx, "", searchOpts)
	if err != nil {
		if logger != nil {
			logger.Debug().Err(err).Strs("input_tags", inputTags).Msg("Failed to search for upstream documents")
		}
		return nil
	}

	if logger != nil {
		logger.Debug().
			Strs("input_tags", inputTags).
			Str("manager_id", managerID).
			Int("doc_count", len(docs)).
			Msg("Found upstream documents for ticker extraction")
	}

	// Extract tickers from document metadata
	for _, doc := range docs {
		if doc.Metadata == nil {
			continue
		}

		// Handle navexa_portfolio documents - extract from holdings array
		if doc.SourceType == "navexa_portfolio" {
			holdings, ok := doc.Metadata["holdings"].([]interface{})
			if !ok {
				// Try typed array
				if holdingsTyped, ok := doc.Metadata["holdings"].([]map[string]interface{}); ok {
					for _, h := range holdingsTyped {
						ticker := extractTickerFromHolding(h)
						addTicker(ticker)
					}
				}
				continue
			}
			for _, h := range holdings {
				holdingMap, ok := h.(map[string]interface{})
				if !ok {
					continue
				}
				ticker := extractTickerFromHolding(holdingMap)
				addTicker(ticker)
			}
		}
	}

	if logger != nil && len(tickers) > 0 {
		var tickerStrs []string
		for _, t := range tickers {
			tickerStrs = append(tickerStrs, t.String())
		}
		logger.Info().
			Strs("tickers", tickerStrs).
			Int("count", len(tickers)).
			Msg("Extracted tickers from upstream documents")
	}

	return tickers
}

// extractTickerFromHolding extracts a Ticker from a holding map.
// Handles exchange and symbol fields, mapping common exchange codes.
func extractTickerFromHolding(holding map[string]interface{}) common.Ticker {
	symbol, _ := holding["symbol"].(string)
	exchange, _ := holding["exchange"].(string)

	if symbol == "" {
		return common.Ticker{}
	}

	// Map common exchange codes
	mappedExchange := mapExchangeCode(exchange)

	// Build ticker string and parse
	if mappedExchange != "" {
		return common.ParseTicker(mappedExchange + ":" + symbol)
	}
	return common.ParseTicker(symbol)
}

// mapExchangeCode maps various exchange code formats to standard codes.
func mapExchangeCode(exchange string) string {
	switch exchange {
	case "AU", "ASX", "AUS":
		return "ASX"
	case "US", "NYSE", "NASDAQ":
		return "US"
	case "LSE", "LON", "UK":
		return "LSE"
	case "TSX", "TSE", "CA":
		return "TSX"
	default:
		return exchange
	}
}

// AssociateDocumentWithJob associates an existing document with a job execution.
// This is used when a worker reuses a cached document from a previous job.
// It appends the job ID to the document's Jobs array (if not already present),
// enabling multiple concurrent jobs to reference the same document.
//
// This solves the job isolation problem where:
// 1. First run creates documents with job_id_1 in Jobs array
// 2. Second run finds cached documents (still fresh), doesn't recreate them
// 3. Without this function, second run's output_formatter finds NOTHING (filters by job_id_2)
// 4. With this function, cached documents are associated with current job via Jobs array
//
// Parameters:
//   - ctx: context for the operation
//   - doc: the document to associate (must not be nil)
//   - jobID: the current job's ID to associate the document with
//   - storage: document storage for saving the updated document
//   - logger: logger for tracking the association
//
// Returns error if the document could not be saved.
func AssociateDocumentWithJob(ctx context.Context, doc *models.Document, jobID string, storage interfaces.DocumentStorage, logger arbor.ILogger) error {
	if doc == nil {
		return nil // Nothing to associate
	}

	if jobID == "" {
		return nil // No job to associate with
	}

	// Check if job ID already exists in Jobs array (avoid duplicates)
	for _, existingJob := range doc.Jobs {
		if existingJob == jobID {
			logger.Debug().
				Str("doc_id", doc.ID).
				Str("job_id", jobID).
				Msg("Document already associated with job, skipping")
			return nil
		}
	}

	// Append job ID to Jobs array
	doc.Jobs = append(doc.Jobs, jobID)

	// Update timestamp
	doc.UpdatedAt = time.Now()

	// Save the document
	if err := storage.SaveDocument(doc); err != nil {
		logger.Error().
			Err(err).
			Str("doc_id", doc.ID).
			Str("job_id", jobID).
			Msg("Failed to save document after job association")
		return err
	}

	logger.Info().
		Str("doc_id", doc.ID).
		Str("job_id", jobID).
		Int("total_jobs", len(doc.Jobs)).
		Msg("Associated cached document with current job")

	return nil
}
