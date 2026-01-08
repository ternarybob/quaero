// -----------------------------------------------------------------------
// ConsolidateWorker - Consolidates tagged documents into single output
// Collects documents by tag, sorts by ticker, and concatenates content
// No AI involved - pure document merging
// -----------------------------------------------------------------------

package market

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/ternarybob/arbor"
	"github.com/ternarybob/quaero/internal/interfaces"
	"github.com/ternarybob/quaero/internal/models"
	"github.com/ternarybob/quaero/internal/queue"
)

// ConsolidateWorker consolidates tagged documents into a single output document.
// This worker executes synchronously (no child jobs).
type ConsolidateWorker struct {
	searchService   interfaces.SearchService
	documentStorage interfaces.DocumentStorage
	logger          arbor.ILogger
	jobMgr          *queue.Manager
	debugEnabled    bool
}

// Compile-time assertion: ConsolidateWorker implements DefinitionWorker interface
var _ interfaces.DefinitionWorker = (*ConsolidateWorker)(nil)

// NewConsolidateWorker creates a new consolidation worker
func NewConsolidateWorker(
	searchService interfaces.SearchService,
	documentStorage interfaces.DocumentStorage,
	logger arbor.ILogger,
	jobMgr *queue.Manager,
	debugEnabled bool,
) *ConsolidateWorker {
	return &ConsolidateWorker{
		searchService:   searchService,
		documentStorage: documentStorage,
		logger:          logger,
		jobMgr:          jobMgr,
		debugEnabled:    debugEnabled,
	}
}

// GetType returns the worker type identifier
func (w *ConsolidateWorker) GetType() models.WorkerType {
	return models.WorkerTypeMarketConsolidate
}

// ReturnsChildJobs returns false as this worker executes synchronously
func (w *ConsolidateWorker) ReturnsChildJobs() bool {
	return false
}

// ValidateConfig validates the step configuration
func (w *ConsolidateWorker) ValidateConfig(step models.JobStep) error {
	stepConfig := step.Config
	if stepConfig == nil {
		return fmt.Errorf("step config is required")
	}
	// input_tags is required
	if _, ok := stepConfig["input_tags"]; !ok {
		return fmt.Errorf("input_tags is required")
	}
	return nil
}

// Init performs the initialization phase for the step
func (w *ConsolidateWorker) Init(ctx context.Context, step models.JobStep, jobDef models.JobDefinition) (*interfaces.WorkerInitResult, error) {
	stepConfig := step.Config
	if stepConfig == nil {
		stepConfig = make(map[string]interface{})
	}

	// Extract input_tags
	var inputTags []string
	if tags, ok := stepConfig["input_tags"].([]interface{}); ok {
		for _, t := range tags {
			if tag, ok := t.(string); ok && tag != "" {
				inputTags = append(inputTags, tag)
			}
		}
	} else if tags, ok := stepConfig["input_tags"].([]string); ok {
		inputTags = tags
	}

	w.logger.Info().
		Str("phase", "init").
		Str("step_name", step.Name).
		Strs("input_tags", inputTags).
		Msg("Market consolidate worker initialized")

	return &interfaces.WorkerInitResult{
		WorkItems: []interfaces.WorkItem{
			{
				ID:   "consolidate",
				Name: "Consolidate documents",
				Type: "market_consolidate",
				Config: map[string]interface{}{
					"input_tags": inputTags,
				},
			},
		},
		TotalCount:           1,
		Strategy:             interfaces.ProcessingStrategyInline,
		SuggestedConcurrency: 1,
		Metadata: map[string]interface{}{
			"input_tags":  inputTags,
			"step_config": stepConfig,
		},
	}, nil
}

// CreateJobs executes the consolidation synchronously
func (w *ConsolidateWorker) CreateJobs(ctx context.Context, step models.JobStep, jobDef models.JobDefinition, stepID string, initResult *interfaces.WorkerInitResult) (string, error) {
	// Call Init if not provided
	if initResult == nil {
		var err error
		initResult, err = w.Init(ctx, step, jobDef)
		if err != nil {
			return "", fmt.Errorf("failed to initialize market_consolidate worker: %w", err)
		}
	}

	stepConfig, _ := initResult.Metadata["step_config"].(map[string]interface{})

	// Extract input_tags
	var inputTags []string
	if tags, ok := stepConfig["input_tags"].([]interface{}); ok {
		for _, t := range tags {
			if tag, ok := t.(string); ok && tag != "" {
				inputTags = append(inputTags, tag)
			}
		}
	} else if tags, ok := stepConfig["input_tags"].([]string); ok {
		inputTags = tags
	}

	if len(inputTags) == 0 {
		return "", fmt.Errorf("input_tags is required and must not be empty")
	}

	// Extract output_tags
	var outputTags []string
	if tags, ok := stepConfig["output_tags"].([]interface{}); ok {
		for _, t := range tags {
			if tag, ok := t.(string); ok && tag != "" {
				outputTags = append(outputTags, tag)
			}
		}
	} else if tags, ok := stepConfig["output_tags"].([]string); ok {
		outputTags = tags
	}

	// Extract optional title
	title := "Consolidated Documents"
	if t, ok := stepConfig["title"].(string); ok && t != "" {
		title = t
	}

	// Extract format option: split|combined (default: combined)
	// - combined: merge all documents into a single output document
	// - split: keep documents separate (pass through without merging)
	format := "combined"
	if f, ok := stepConfig["format"].(string); ok && f != "" {
		format = strings.ToLower(f)
	}

	w.logger.Info().
		Str("phase", "run").
		Str("step_name", step.Name).
		Str("step_id", stepID).
		Strs("input_tags", inputTags).
		Strs("output_tags", outputTags).
		Str("format", format).
		Msg("Starting document consolidation")

	// Log step start for UI
	if w.jobMgr != nil {
		w.jobMgr.AddJobLog(ctx, stepID, "info", fmt.Sprintf("Consolidating documents with tags: %v", inputTags))
	}

	// Search for documents with input tags
	opts := interfaces.SearchOptions{
		Tags:     inputTags,
		Limit:    100,
		OrderBy:  "created_at",
		OrderDir: "desc",
	}

	results, err := w.searchService.Search(ctx, "", opts)
	if err != nil {
		return "", fmt.Errorf("failed to search for documents: %w", err)
	}

	if len(results) == 0 {
		w.logger.Warn().Strs("input_tags", inputTags).Msg("No documents found to consolidate")
		return "", fmt.Errorf("no documents found with tags: %v", inputTags)
	}

	w.logger.Info().Int("count", len(results)).Msg("Found documents to consolidate")

	// Load full documents and extract ticker tags for sorting
	type docWithTicker struct {
		doc    *models.Document
		ticker string
	}

	var docsWithTickers []docWithTicker
	for _, result := range results {
		doc, err := w.documentStorage.GetDocument(result.ID)
		if err != nil {
			w.logger.Warn().Err(err).Str("id", result.ID).Msg("Failed to load document")
			continue
		}
		if doc == nil {
			continue
		}

		// Extract ticker tag (2-5 char lowercase tag that's not a known system tag)
		ticker := ""
		for _, tag := range doc.Tags {
			if IsTickerTag(tag) {
				ticker = strings.ToUpper(tag)
				break
			}
		}

		docsWithTickers = append(docsWithTickers, docWithTicker{
			doc:    doc,
			ticker: ticker,
		})
	}

	// Sort by ticker (alphanumeric ascending)
	sort.Slice(docsWithTickers, func(i, j int) bool {
		return docsWithTickers[i].ticker < docsWithTickers[j].ticker
	})

	// Build output tags
	dateTag := fmt.Sprintf("date:%s", time.Now().Format("2006-01-02"))
	finalTags := append(outputTags, dateTag)

	// Handle split format: add output tags to existing documents without merging
	if format == "split" {
		var tickers []string
		for _, dwt := range docsWithTickers {
			if dwt.ticker != "" {
				tickers = append(tickers, dwt.ticker)
			}

			// Add output tags to existing document
			existingTags := make(map[string]bool)
			for _, t := range dwt.doc.Tags {
				existingTags[t] = true
			}
			for _, t := range finalTags {
				if !existingTags[t] {
					dwt.doc.Tags = append(dwt.doc.Tags, t)
				}
			}
			dwt.doc.UpdatedAt = time.Now()

			// Save updated document
			if err := w.documentStorage.SaveDocument(dwt.doc); err != nil {
				w.logger.Warn().Err(err).Str("doc_id", dwt.doc.ID).Msg("Failed to update document tags")
			}
		}

		w.logger.Info().
			Int("doc_count", len(docsWithTickers)).
			Strs("tickers", tickers).
			Strs("added_tags", finalTags).
			Msg("Split format: added output tags to existing documents")

		if w.jobMgr != nil {
			w.jobMgr.AddJobLog(ctx, stepID, "info", fmt.Sprintf(
				"Split format: tagged %d documents with %v (tickers: %s)",
				len(docsWithTickers), outputTags, strings.Join(tickers, ", "),
			))
		}

		return "", nil
	}

	// Combined format (default): merge all documents into single output
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("# %s\n\n", title))
	sb.WriteString(fmt.Sprintf("**Generated**: %s\n", time.Now().Format("2 January 2006 3:04 PM MST")))
	sb.WriteString(fmt.Sprintf("**Documents Consolidated**: %d\n\n", len(docsWithTickers)))
	sb.WriteString("---\n\n")

	var tickers []string
	for _, dwt := range docsWithTickers {
		if dwt.ticker != "" {
			tickers = append(tickers, dwt.ticker)
		}

		// Add document content with separator
		if dwt.doc.ContentMarkdown != "" {
			sb.WriteString(dwt.doc.ContentMarkdown)
			sb.WriteString("\n\n---\n\n")
		}
	}

	// Create consolidated document
	consolidatedDoc := &models.Document{
		ID:              uuid.New().String(),
		Title:           title,
		ContentMarkdown: sb.String(),
		Tags:            finalTags,
		SourceType:      "consolidation",
		CreatedAt:       time.Now(),
		UpdatedAt:       time.Now(),
		Metadata: map[string]interface{}{
			"source_count":     len(docsWithTickers),
			"source_tags":      inputTags,
			"tickers":          tickers,
			"consolidation":    true,
			"parent_job_id":    stepID,
			"consolidate_date": time.Now().Format(time.RFC3339),
		},
	}

	// Save consolidated document
	if err := w.documentStorage.SaveDocument(consolidatedDoc); err != nil {
		return "", fmt.Errorf("failed to save consolidated document: %w", err)
	}

	w.logger.Info().
		Str("doc_id", consolidatedDoc.ID).
		Int("source_count", len(docsWithTickers)).
		Strs("tickers", tickers).
		Msg("Consolidated document saved")

	// Log to job
	if w.jobMgr != nil {
		w.jobMgr.AddJobLog(ctx, stepID, "info", fmt.Sprintf(
			"Consolidated %d documents into %s (tickers: %s)",
			len(docsWithTickers), consolidatedDoc.ID, strings.Join(tickers, ", "),
		))
	}

	return "", nil
}

// isTickerTag checks if a tag looks like a stock ticker (2-5 lowercase letters)
// Excludes known system tags
func IsTickerTag(tag string) bool {
	// Must be 2-5 characters
	if len(tag) < 2 || len(tag) > 5 {
		return false
	}

	// Must be lowercase
	if tag != strings.ToLower(tag) {
		return false
	}

	// Exclude known system tags
	excludedTags := map[string]bool{
		"date":      true,
		"email":     true,
		"smsf":      true,
		"job":       true,
		"summary":   true,
		"stock":     true,
		"asx":       true,
		"test":      true,
		"debug":     true,
		"daily":     true,
		"weekly":    true,
		"monthly":   true,
		"watchlist": true,
	}

	if excludedTags[tag] {
		return false
	}

	// Exclude tags with prefixes
	prefixes := []string{"date:", "job-", "asx-", "stock-", "email-"}
	for _, prefix := range prefixes {
		if strings.HasPrefix(tag, prefix) {
			return false
		}
	}

	// Must be all letters (ticker symbols)
	for _, r := range tag {
		if r < 'a' || r > 'z' {
			return false
		}
	}

	return true
}
