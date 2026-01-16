// -----------------------------------------------------------------------
// FormatterWorker - Prepares output documents for email delivery
// Creates markdown documents with email instructions and content.
// Supports variable substitution and ticker-based document collection.
//
// When multi_document=true, creates separate documents per ticker for
// individual PDF attachments in the email.
// -----------------------------------------------------------------------

package output

import (
	"bytes"
	"context"
	"fmt"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/ternarybob/arbor"
	"github.com/ternarybob/quaero/internal/interfaces"
	"github.com/ternarybob/quaero/internal/models"
	"github.com/ternarybob/quaero/internal/queue"
	"github.com/ternarybob/quaero/internal/workers/market"
	"github.com/ternarybob/quaero/internal/workers/workerutil"
	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/renderer/html"
)

// docWithTickerLocal is a local type for document with ticker
type docWithTickerLocal struct {
	doc    *models.Document
	ticker string
}

// FormatterWorker prepares output documents for email delivery.
// Creates a markdown document with embedded instructions for the email worker.
type FormatterWorker struct {
	searchService   interfaces.SearchService
	documentStorage interfaces.DocumentStorage
	logger          arbor.ILogger
	jobMgr          *queue.Manager
	debugEnabled    bool
	serverHost      string
	serverPort      int
}

// Compile-time assertion: FormatterWorker implements DefinitionWorker interface
var _ interfaces.DefinitionWorker = (*FormatterWorker)(nil)

// NewFormatterWorker creates a new output formatter worker
func NewFormatterWorker(
	searchService interfaces.SearchService,
	documentStorage interfaces.DocumentStorage,
	logger arbor.ILogger,
	jobMgr *queue.Manager,
	debugEnabled bool,
	serverHost string,
	serverPort int,
) *FormatterWorker {
	return &FormatterWorker{
		searchService:   searchService,
		documentStorage: documentStorage,
		logger:          logger,
		jobMgr:          jobMgr,
		debugEnabled:    debugEnabled,
		serverHost:      serverHost,
		serverPort:      serverPort,
	}
}

// GetType returns the worker type identifier
func (w *FormatterWorker) GetType() models.WorkerType {
	return models.WorkerTypeOutputFormatter
}

// ReturnsChildJobs returns false as this worker executes synchronously
func (w *FormatterWorker) ReturnsChildJobs() bool {
	return false
}

// ValidateConfig validates the step configuration
func (w *FormatterWorker) ValidateConfig(step models.JobStep) error {
	stepConfig := step.Config
	if stepConfig == nil {
		return fmt.Errorf("step config is required")
	}
	// input_tags is optional - defaults to [step.Name] if not specified
	return nil
}

// Init performs the initialization phase for the step
func (w *FormatterWorker) Init(ctx context.Context, step models.JobStep, jobDef models.JobDefinition) (*interfaces.WorkerInitResult, error) {
	stepConfig := step.Config
	if stepConfig == nil {
		stepConfig = make(map[string]interface{})
	}

	// Extract input_tags, defaulting to step name if not specified
	inputTags := workerutil.GetInputTags(stepConfig, step.Name)

	// Collect tickers - supports both step config and job-level variables
	tickers := workerutil.CollectTickersWithJobDef(stepConfig, jobDef)

	w.logger.Info().
		Str("phase", "init").
		Str("step_name", step.Name).
		Strs("input_tags", inputTags).
		Int("ticker_count", len(tickers)).
		Msg("Output formatter worker initialized")

	return &interfaces.WorkerInitResult{
		WorkItems: []interfaces.WorkItem{
			{
				ID:   "format_output",
				Name: "Format output for email",
				Type: "output_formatter",
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
			"tickers":     tickers,
		},
	}, nil
}

// CreateJobs executes the output formatting synchronously
func (w *FormatterWorker) CreateJobs(ctx context.Context, step models.JobStep, jobDef models.JobDefinition, stepID string, initResult *interfaces.WorkerInitResult) (string, error) {
	// Call Init if not provided
	if initResult == nil {
		var err error
		initResult, err = w.Init(ctx, step, jobDef)
		if err != nil {
			return "", fmt.Errorf("failed to initialize output_formatter worker: %w", err)
		}
	}

	// Get manager_id for document search - all steps in the same pipeline share this ID
	// This allows us to find documents created by upstream workers in the same pipeline
	managerID := workerutil.GetManagerID(ctx, w.jobMgr, stepID)

	stepConfig, _ := initResult.Metadata["step_config"].(map[string]interface{})
	// Extract input_tags, defaulting to step name if not specified
	inputTags := workerutil.GetInputTags(stepConfig, step.Name)
	outputTags := workerutil.GetStringSliceConfig(stepConfig, "output_tags", nil)

	// Collect tickers for filtering
	tickers := workerutil.CollectTickersWithJobDef(stepConfig, jobDef)
	tickerSet := make(map[string]bool)
	for _, t := range tickers {
		tickerSet[strings.ToLower(t.Code)] = true
	}

	return w.processDocuments(ctx, step, stepConfig, stepID, managerID, inputTags, outputTags, tickerSet)
}

// processDocuments searches for documents and formats them for email
func (w *FormatterWorker) processDocuments(
	ctx context.Context,
	step models.JobStep,
	stepConfig map[string]interface{},
	stepID, managerID string,
	inputTags, outputTags []string,
	tickerSet map[string]bool,
) (string, error) {
	if len(inputTags) == 0 {
		return "", fmt.Errorf("input_tags is required and must not be empty")
	}

	// Extract email formatting options with defaults
	format := "inline" // inline | pdf | html | markdown
	if f, ok := stepConfig["format"].(string); ok && f != "" {
		format = strings.ToLower(f)
	}

	attachment := false
	if a, ok := stepConfig["attachment"].(bool); ok {
		attachment = a
	}

	style := "body" // proforma | body
	if s, ok := stepConfig["style"].(string); ok && s != "" {
		style = strings.ToLower(s)
	}

	// Extract optional title
	title := "Output Document"
	if t, ok := stepConfig["title"].(string); ok && t != "" {
		title = t
	}

	// Extract base_url for proforma links - defaults to configured server URL
	baseURL := fmt.Sprintf("http://%s:%d", w.serverHost, w.serverPort)
	if u, ok := stepConfig["base_url"].(string); ok && u != "" {
		baseURL = u
	}

	// Extract order configuration (default: ticker for alphanumeric ascending)
	order := "ticker" // ticker | created_at
	if o, ok := stepConfig["order"].(string); ok && o != "" {
		order = strings.ToLower(o)
	}

	// Extract multi_document option (default: false for backward compatibility)
	// When true, creates separate output document per ticker for individual PDF attachments
	multiDocument := false
	if md, ok := stepConfig["multi_document"].(bool); ok {
		multiDocument = md
	}

	w.logger.Info().
		Str("phase", "run").
		Str("step_name", step.Name).
		Str("step_id", stepID).
		Strs("input_tags", inputTags).
		Str("format", format).
		Bool("attachment", attachment).
		Str("style", style).
		Str("order", order).
		Bool("multi_document", multiDocument).
		Str("job_id", managerID).
		Msg("Starting output formatting")

	// Log step start for UI
	if w.jobMgr != nil {
		w.jobMgr.AddJobLog(ctx, stepID, "info", fmt.Sprintf("Formatting output with tags: %v", inputTags))
	}

	// Search for documents with input tags, filtered by current pipeline execution
	// JobID filter ensures we only get documents from this pipeline run
	// Uses managerID (orchestrator job ID) which is shared across all steps
	opts := interfaces.SearchOptions{
		Tags:     inputTags,
		Limit:    100,
		OrderBy:  "created_at",
		OrderDir: "desc",
		JobID:    managerID,
	}

	results, err := w.searchService.Search(ctx, "", opts)
	if err != nil {
		return "", fmt.Errorf("failed to search for documents: %w", err)
	}

	if len(results) == 0 {
		w.logger.Warn().Strs("input_tags", inputTags).Msg("No documents found to format")
		return "", fmt.Errorf("no documents found with tags: %v", inputTags)
	}

	// Load full documents and extract ticker tags for sorting
	// Documents are already filtered by JobID (managerID) in the search,
	// so we only get documents from this pipeline execution
	var docsWithTickers []docWithTickerLocal
	for _, result := range results {
		doc, err := w.documentStorage.GetDocument(result.ID)
		if err != nil {
			w.logger.Warn().Err(err).Str("id", result.ID).Msg("Failed to load document")
			continue
		}
		if doc == nil {
			continue
		}

		// Extract ticker tag
		ticker := ""
		for _, tag := range doc.Tags {
			if market.IsTickerTag(tag) {
				ticker = strings.ToUpper(tag)
				break
			}
		}

		// Filter by ticker if ticker set is provided
		if len(tickerSet) > 0 && ticker != "" {
			if !tickerSet[strings.ToLower(ticker)] {
				continue
			}
		}

		docsWithTickers = append(docsWithTickers, docWithTickerLocal{
			doc:    doc,
			ticker: ticker,
		})
	}

	// Sort documents based on order configuration
	switch order {
	case "created_at":
		sort.Slice(docsWithTickers, func(i, j int) bool {
			return docsWithTickers[i].doc.CreatedAt.Before(docsWithTickers[j].doc.CreatedAt)
		})
	case "ticker":
		fallthrough
	default:
		// Default: sort by ticker (alphanumeric ascending)
		sort.Slice(docsWithTickers, func(i, j int) bool {
			return docsWithTickers[i].ticker < docsWithTickers[j].ticker
		})
	}

	// Multi-document mode: create separate output document per ticker
	// This enables the email worker to create individual PDF attachments per stock
	if multiDocument && attachment {
		// Pass the config tickers to enable per-ticker output when input is a combined document
		var configTickerCodes []string
		for ticker := range tickerSet {
			configTickerCodes = append(configTickerCodes, ticker)
		}
		return w.processMultiDocumentMode(ctx, stepID, docsWithTickers, title, format, style, baseURL, inputTags, outputTags, managerID, order, configTickerCodes)
	}

	// Single document mode (default): merge all content into one document
	doc := w.buildOutputDocument(docsWithTickers, title, format, attachment, style, baseURL, inputTags, outputTags, managerID, order, "")

	// Save the output document
	if err := w.documentStorage.SaveDocument(doc); err != nil {
		return "", fmt.Errorf("failed to save output document: %w", err)
	}

	// Extract tickers for logging
	var tickers []string
	for _, dwt := range docsWithTickers {
		if dwt.ticker != "" {
			tickers = append(tickers, dwt.ticker)
		}
	}

	w.logger.Info().
		Str("doc_id", doc.ID).
		Int("source_count", len(docsWithTickers)).
		Strs("tickers", tickers).
		Str("format", format).
		Str("style", style).
		Msg("Output document created")

	if w.jobMgr != nil {
		w.jobMgr.AddJobLog(ctx, stepID, "info", fmt.Sprintf(
			"Created output document %s with %d sources (format=%s, style=%s)",
			doc.ID, len(docsWithTickers), format, style,
		))
	}

	return "", nil
}

// processMultiDocumentMode creates separate output documents for each ticker.
// This enables the email worker to create individual PDF attachments per stock.
//
// When input documents are already grouped by ticker (have individual ticker tags),
// it creates one output document per ticker group.
//
// When input is a single combined document (e.g., from summary worker), it creates
// one output document per ticker defined in configTickers, duplicating the combined
// content to each ticker's document. This enables multi_document mode to work with
// upstream workers that produce combined output.
func (w *FormatterWorker) processMultiDocumentMode(
	ctx context.Context,
	stepID string,
	docsWithTickers []docWithTickerLocal,
	title, format, style, baseURL string,
	inputTags, outputTags []string,
	managerID, order string,
	configTickers []string,
) (string, error) {
	// Group documents by ticker
	tickerGroups := w.groupDocsByTicker(docsWithTickers)

	// Check if we have a single combined document but multiple tickers in config
	// This happens when upstream worker (e.g., summary) creates one document with all ticker analysis
	usingConfigTickers := false
	if len(tickerGroups) <= 1 && len(configTickers) > 1 {
		w.logger.Info().
			Int("document_groups", len(tickerGroups)).
			Int("config_tickers", len(configTickers)).
			Msg("Single combined document detected, creating per-ticker outputs from config tickers")
		usingConfigTickers = true
	}

	if len(tickerGroups) == 0 && len(configTickers) == 0 {
		return "", fmt.Errorf("no documents with ticker tags found for multi-document mode")
	}

	var createdDocs []string
	var tickers []string

	if usingConfigTickers {
		// Combined content from input documents - we need to extract per-ticker content
		var combinedContent strings.Builder
		for _, dwt := range docsWithTickers {
			if dwt.doc.ContentMarkdown != "" {
				combinedContent.WriteString(dwt.doc.ContentMarkdown)
				combinedContent.WriteString("\n\n")
			}
		}
		fullContent := combinedContent.String()

		// Create a separate output document for each ticker from config
		// Extract only that ticker's content from the combined analysis
		for _, ticker := range configTickers {
			tickerCode := strings.ToLower(ticker)
			tickerUpper := strings.ToUpper(tickerCode)

			// Extract ticker-specific content from combined analysis
			tickerContent := w.extractTickerContent(fullContent, tickerUpper)

			if tickerContent == "" {
				w.logger.Warn().
					Str("ticker", tickerUpper).
					Msg("No ticker-specific content found in combined analysis, skipping")
				continue
			}

			// Build ticker-specific title
			tickerTitle := fmt.Sprintf("%s - ASX:%s", title, tickerUpper)

			// Build output document for this ticker with extracted content
			doc := w.buildOutputDocumentWithContent(tickerContent, tickerTitle, format, true, style, baseURL, inputTags, outputTags, managerID, order, tickerCode)

			// Save the output document
			if err := w.documentStorage.SaveDocument(doc); err != nil {
				w.logger.Error().Err(err).Str("ticker", tickerCode).Msg("Failed to save output document for ticker")
				continue
			}

			createdDocs = append(createdDocs, doc.ID)
			tickers = append(tickers, tickerUpper)

			w.logger.Info().
				Str("doc_id", doc.ID).
				Str("ticker", tickerCode).
				Int("content_length", len(tickerContent)).
				Msg("Created ticker-specific output document with extracted content")
		}
	} else {
		// Create a separate output document for each ticker group
		for ticker, docs := range tickerGroups {
			// Build ticker-specific title
			tickerTitle := fmt.Sprintf("%s - ASX:%s", title, strings.ToUpper(ticker))

			// Build output document for this ticker
			doc := w.buildOutputDocument(docs, tickerTitle, format, true, style, baseURL, inputTags, outputTags, managerID, order, ticker)

			// Save the output document
			if err := w.documentStorage.SaveDocument(doc); err != nil {
				w.logger.Error().Err(err).Str("ticker", ticker).Msg("Failed to save output document for ticker")
				continue
			}

			createdDocs = append(createdDocs, doc.ID)
			tickers = append(tickers, strings.ToUpper(ticker))

			w.logger.Info().
				Str("doc_id", doc.ID).
				Str("ticker", ticker).
				Int("source_count", len(docs)).
				Msg("Created ticker-specific output document")
		}
	}

	w.logger.Info().
		Int("document_count", len(createdDocs)).
		Strs("tickers", tickers).
		Str("format", format).
		Str("style", style).
		Bool("used_config_tickers", usingConfigTickers).
		Msg("Multi-document output: created per-ticker documents")

	// Create a summary document that acts as the "main" document for the email worker
	// This document will be the email body and will list the per-ticker documents as attachments
	summaryDoc := w.buildSummaryDocument(title, tickers, createdDocs, format, style, baseURL, inputTags, outputTags, managerID)
	if err := w.documentStorage.SaveDocument(summaryDoc); err != nil {
		return "", fmt.Errorf("failed to save summary document: %w", err)
	}

	w.logger.Info().
		Str("doc_id", summaryDoc.ID).
		Msg("Multi-document output: created summary document")

	if w.jobMgr != nil {
		w.jobMgr.AddJobLog(ctx, stepID, "info", fmt.Sprintf(
			"Created %d output documents and 1 summary document for tickers: %v",
			len(createdDocs), tickers,
		))
	}

	return "", nil
}

// buildSummaryDocument creates a summary document for multi-document mode.
// This document serves as the email body and lists the attachment IDs.
func (w *FormatterWorker) buildSummaryDocument(
	title string,
	tickers []string,
	attachmentIDs []string,
	format, style, baseURL string,
	inputTags, outputTags []string,
	managerID string,
) *models.Document {
	var sb strings.Builder

	// Email instructions section (YAML-like frontmatter)
	sb.WriteString("---\n")
	sb.WriteString("# Email Instructions (parsed by email worker)\n")
	sb.WriteString(fmt.Sprintf("format: %s\n", format))
	sb.WriteString("attachment: true\n") // Always true for summary doc to ensure attachments are processed
	sb.WriteString(fmt.Sprintf("style: %s\n", style))
	sb.WriteString(fmt.Sprintf("base_url: %s\n", baseURL))
	sb.WriteString(fmt.Sprintf("document_count: %d\n", len(attachmentIDs)))

	// List document IDs for attachment
	if len(attachmentIDs) > 0 {
		sb.WriteString("document_ids:\n")
		for _, id := range attachmentIDs {
			sb.WriteString(fmt.Sprintf("  - %s\n", id))
		}
	}
	sb.WriteString("---\n\n")

	// Build content for the email body
	var contentMarkdown strings.Builder
	contentMarkdown.WriteString(fmt.Sprintf("# %s\n\n", title))
	contentMarkdown.WriteString(fmt.Sprintf("**Generated**: %s\n\n", time.Now().Format("2 January 2006 3:04 PM MST")))

	if len(tickers) > 0 {
		contentMarkdown.WriteString("Please find attached the analysis reports for the following tickers:\n\n")
		for _, ticker := range tickers {
			contentMarkdown.WriteString(fmt.Sprintf("- **%s**\n", ticker))
		}
	} else {
		contentMarkdown.WriteString("Please find attached the analysis reports.\n")
	}

	// Add content to full document
	sb.WriteString(contentMarkdown.String())

	// Convert markdown content to HTML
	htmlContent := w.convertMarkdownToHTML(contentMarkdown.String())

	// Build tags
	dateTag := fmt.Sprintf("date:%s", time.Now().Format("2006-01-02"))
	finalTags := append(outputTags, dateTag, "email-output", "summary", "multi-document-summary")

	// Build metadata
	metadata := map[string]interface{}{
		"source_count":   len(attachmentIDs),
		"attachment_ids": attachmentIDs,
		"tickers":        tickers,
		"format":         format,
		"attachment":     true,
		"style":          style,
		"format_date":    time.Now().Format(time.RFC3339),
		"email_ready":    true,
		"multi_document": true, // Mark as multi-document parent
	}

	return &models.Document{
		ID:              uuid.New().String(),
		Title:           title + " - Summary",
		ContentMarkdown: sb.String(),
		ContentHTML:     htmlContent,
		Tags:            finalTags,
		Jobs:            []string{managerID},
		SourceType:      "output_formatter",
		CreatedAt:       time.Now().Add(1 * time.Second), // Ensure it's created after the attachments
		UpdatedAt:       time.Now().Add(1 * time.Second),
		Metadata:        metadata,
	}
}

// groupDocsByTicker groups documents by their ticker tag.
// Documents without a ticker tag are grouped under "unknown".
func (w *FormatterWorker) groupDocsByTicker(docs []docWithTickerLocal) map[string][]docWithTickerLocal {
	groups := make(map[string][]docWithTickerLocal)
	for _, dwt := range docs {
		ticker := strings.ToLower(dwt.ticker)
		if ticker == "" {
			ticker = "unknown"
		}
		groups[ticker] = append(groups[ticker], dwt)
	}
	return groups
}

// buildOutputDocument creates the output document with email instructions.
// Generates both markdown and HTML content for email worker priority selection.
// When ticker is provided (non-empty), adds the ticker tag to the output document
// for identification by the email worker when creating per-ticker PDF attachments.
func (w *FormatterWorker) buildOutputDocument(
	docsWithTickers []docWithTickerLocal,
	title, format string,
	attachment bool,
	style, baseURL string,
	inputTags, outputTags []string,
	managerID string,
	order string,
	ticker string, // optional: ticker code for multi-document mode
) *models.Document {
	var sb strings.Builder

	// Email instructions section (YAML-like frontmatter)
	sb.WriteString("---\n")
	sb.WriteString("# Email Instructions (parsed by email worker)\n")
	sb.WriteString(fmt.Sprintf("format: %s\n", format))
	sb.WriteString(fmt.Sprintf("attachment: %t\n", attachment))
	sb.WriteString(fmt.Sprintf("style: %s\n", style))
	sb.WriteString(fmt.Sprintf("base_url: %s\n", baseURL))
	sb.WriteString(fmt.Sprintf("order: %s\n", order))
	sb.WriteString(fmt.Sprintf("document_count: %d\n", len(docsWithTickers)))

	// List document IDs for attachment
	if len(docsWithTickers) > 0 {
		sb.WriteString("document_ids:\n")
		for _, dwt := range docsWithTickers {
			sb.WriteString(fmt.Sprintf("  - %s\n", dwt.doc.ID))
		}
	}
	sb.WriteString("---\n\n")

	// Build content based on style (for body after frontmatter)
	var contentMarkdown strings.Builder
	if style == "proforma" {
		// Proforma: list of documents with links (table format)
		contentMarkdown.WriteString(fmt.Sprintf("# %s\n\n", title))
		contentMarkdown.WriteString(fmt.Sprintf("**Generated**: %s\n", time.Now().Format("2 January 2006 3:04 PM MST")))
		contentMarkdown.WriteString(fmt.Sprintf("**Documents**: %d\n\n", len(docsWithTickers)))
		contentMarkdown.WriteString("| # | Document | Ticker | Link |\n")
		contentMarkdown.WriteString("|---|----------|--------|------|\n")
		for i, dwt := range docsWithTickers {
			link := fmt.Sprintf("%s/documents?document_id=%s", baseURL, dwt.doc.ID)
			contentMarkdown.WriteString(fmt.Sprintf("| %d | %s | %s | [View](%s) |\n", i+1, dwt.doc.Title, dwt.ticker, link))
		}
	} else {
		// Body: full document content (merged)
		contentMarkdown.WriteString(fmt.Sprintf("# %s\n\n", title))
		contentMarkdown.WriteString(fmt.Sprintf("**Generated**: %s\n", time.Now().Format("2 January 2006 3:04 PM MST")))
		contentMarkdown.WriteString(fmt.Sprintf("**Documents**: %d\n\n", len(docsWithTickers)))
		contentMarkdown.WriteString("---\n\n")

		for _, dwt := range docsWithTickers {
			if dwt.doc.ContentMarkdown != "" {
				contentMarkdown.WriteString(dwt.doc.ContentMarkdown)
				contentMarkdown.WriteString("\n\n---\n\n")
			}
		}
	}

	// Add content to full document (frontmatter + content)
	sb.WriteString(contentMarkdown.String())

	// Convert markdown content to HTML for email priority
	htmlContent := w.convertMarkdownToHTML(contentMarkdown.String())

	// Build tags
	dateTag := fmt.Sprintf("date:%s", time.Now().Format("2006-01-02"))
	finalTags := append(outputTags, dateTag, "email-output")

	// Add ticker tag if provided (multi-document mode)
	// This allows the email worker to identify per-ticker documents
	if ticker != "" {
		finalTags = append(finalTags, strings.ToLower(ticker))
	}

	// Extract tickers from documents
	var tickersList []string
	for _, dwt := range docsWithTickers {
		if dwt.ticker != "" {
			tickersList = append(tickersList, dwt.ticker)
		}
	}

	// Build metadata
	metadata := map[string]interface{}{
		"source_count": len(docsWithTickers),
		"source_tags":  inputTags,
		"tickers":      tickersList,
		"format":       format,
		"attachment":   attachment,
		"style":        style,
		"order":        order,
		"format_date":  time.Now().Format(time.RFC3339),
		"email_ready":  true,
	}

	// Add ticker to metadata if provided (for easy extraction by email worker)
	if ticker != "" {
		metadata["ticker"] = strings.ToUpper(ticker)
		metadata["multi_document"] = true
	}

	return &models.Document{
		ID:              uuid.New().String(),
		Title:           title,
		ContentMarkdown: sb.String(),
		ContentHTML:     htmlContent,
		Tags:            finalTags,
		Jobs:            []string{managerID},
		SourceType:      "output_formatter",
		CreatedAt:       time.Now(),
		UpdatedAt:       time.Now(),
		Metadata:        metadata,
	}
}

// convertMarkdownToHTML converts markdown content to styled HTML for email
func (w *FormatterWorker) convertMarkdownToHTML(markdown string) string {
	if markdown == "" {
		return ""
	}

	// Create goldmark instance with GitHub Flavored Markdown extensions
	md := goldmark.New(
		goldmark.WithExtensions(
			extension.GFM, // GitHub Flavored Markdown (tables, strikethrough, etc.)
		),
		goldmark.WithRendererOptions(
			html.WithHardWraps(),
			html.WithXHTML(),
			html.WithUnsafe(), // Allow raw HTML for colored indicators
		),
	)

	var buf bytes.Buffer
	if err := md.Convert([]byte(markdown), &buf); err != nil {
		w.logger.Error().Err(err).Msg("Failed to convert markdown to HTML")
		return ""
	}

	// Wrap in styled HTML email template
	return w.wrapInEmailTemplate(buf.String())
}

// wrapInEmailTemplate wraps HTML content in a styled email template
func (w *FormatterWorker) wrapInEmailTemplate(content string) string {
	return `<!DOCTYPE html>
<html>
<head>
  <meta charset="UTF-8">
  <meta name="viewport" content="width=device-width, initial-scale=1.0">
  <style>
    body {
      font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, 'Helvetica Neue', Arial, sans-serif;
      line-height: 1.6;
      color: #333;
      max-width: 800px;
      margin: 0 auto;
      padding: 20px;
      background-color: #f9f9f9;
    }
    .content {
      background-color: #fff;
      padding: 30px;
      border-radius: 8px;
      box-shadow: 0 1px 3px rgba(0,0,0,0.1);
    }
    h1 { color: #1a1a1a; font-size: 24px; margin-top: 0; border-bottom: 2px solid #eee; padding-bottom: 10px; }
    h2 { color: #2a2a2a; font-size: 20px; margin-top: 24px; }
    h3 { color: #3a3a3a; font-size: 16px; margin-top: 20px; }
    p { margin: 12px 0; }
    ul, ol { padding-left: 24px; margin: 12px 0; }
    li { margin: 6px 0; }
    strong { color: #1a1a1a; }
    code { background: #f4f4f4; padding: 2px 6px; border-radius: 3px; font-family: 'SF Mono', Monaco, 'Courier New', monospace; font-size: 14px; }
    pre { background: #f4f4f4; padding: 16px; border-radius: 6px; overflow-x: auto; font-family: 'SF Mono', Monaco, 'Courier New', monospace; font-size: 13px; }
    pre code { background: none; padding: 0; }
    blockquote { border-left: 4px solid #ddd; margin: 16px 0; padding-left: 16px; color: #666; }
    table { border-collapse: collapse; width: 100%; margin: 16px 0; }
    th, td { border: 1px solid #ddd; padding: 8px 12px; text-align: left; }
    th { background: #f4f4f4; font-weight: 600; }
    hr { border: none; border-top: 1px solid #eee; margin: 24px 0; }
    a { color: #0066cc; text-decoration: none; }
    a:hover { text-decoration: underline; }
    .footer { margin-top: 30px; padding-top: 20px; border-top: 1px solid #eee; font-size: 12px; color: #888; }
  </style>
</head>
<body>
  <div class="content">
    ` + content + `
  </div>
  <div class="footer">
    <p>This email was automatically generated by Quaero.</p>
  </div>
</body>
</html>`
}

// extractTickerContent extracts the content section for a specific ticker from combined analysis.
// It searches for common section header patterns that indicate the start of a ticker's analysis:
// - "# Stock Analysis Report: {Company} (ASX:{TICKER})"
// - "## ASX: {TICKER}" or "## ASX:{TICKER}"
// - "# {N}. {TICKER}" (numbered sections)
//
// Returns the extracted content including the header, or empty string if not found.
func (w *FormatterWorker) extractTickerContent(content, ticker string) string {
	if content == "" || ticker == "" {
		return ""
	}

	tickerUpper := strings.ToUpper(ticker)

	// Patterns to find the start of a ticker's section
	// Order matters - more specific patterns first
	patterns := []string{
		// "# Stock Analysis Report: Company Name (ASX:GNP)"
		fmt.Sprintf(`(?i)^#\s+Stock\s+Analysis\s+Report:.*\(ASX:%s\)`, tickerUpper),
		// "## ASX: GNP" or "## ASX:GNP"
		fmt.Sprintf(`(?i)^##\s+ASX:\s*%s\s`, tickerUpper),
		// "# 1. GNP" or "# 2. CGS (Company Name)"
		fmt.Sprintf(`(?i)^#\s+\d+\.\s+%s[\s\(]`, tickerUpper),
		// "# GNP Deep Dive" or similar
		fmt.Sprintf(`(?i)^#\s+%s\s`, tickerUpper),
	}

	// Patterns to find the end of a section (start of next ticker or major section)
	endPatterns := []string{
		`(?m)^#\s+Stock\s+Analysis\s+Report:`,  // Next stock analysis report
		`(?m)^##\s+ASX:\s*[A-Z]{2,5}\s`,        // Next ASX ticker section
		`(?m)^#\s+\d+\.\s+[A-Z]{2,5}[\s\(]`,    // Next numbered ticker section
		`(?m)^---\s*\n##\s+Worker\s+Debug`,     // Debug section at end
		`(?m)^##\s+Worker\s+Debug\s+Aggregate`, // Debug section header
	}

	lines := strings.Split(content, "\n")
	var startIdx int = -1
	var startPattern string

	// Find start of ticker section
	for i, line := range lines {
		for _, pattern := range patterns {
			re := regexp.MustCompile(pattern)
			if re.MatchString(line) {
				startIdx = i
				startPattern = pattern
				w.logger.Debug().
					Str("ticker", tickerUpper).
					Int("line", i).
					Str("pattern", pattern).
					Msg("Found ticker section start")
				break
			}
		}
		if startIdx >= 0 {
			break
		}
	}

	if startIdx < 0 {
		w.logger.Warn().
			Str("ticker", tickerUpper).
			Msg("Could not find ticker section in combined content")
		return ""
	}

	// Find end of ticker section (start of next section or end of content)
	endIdx := len(lines)

	// Start searching for end after the start line
	contentAfterStart := strings.Join(lines[startIdx+1:], "\n")

	for _, pattern := range endPatterns {
		re := regexp.MustCompile(pattern)
		match := re.FindStringIndex(contentAfterStart)
		if match != nil {
			// Calculate actual line index
			linesBeforeMatch := strings.Count(contentAfterStart[:match[0]], "\n")
			candidateEndIdx := startIdx + 1 + linesBeforeMatch

			// Don't match the same pattern we started with (avoid self-matching)
			matchedLine := lines[candidateEndIdx]
			startRe := regexp.MustCompile(startPattern)
			if startRe.MatchString(matchedLine) {
				continue
			}

			if candidateEndIdx < endIdx {
				endIdx = candidateEndIdx
				w.logger.Debug().
					Str("ticker", tickerUpper).
					Int("end_line", endIdx).
					Str("pattern", pattern).
					Msg("Found ticker section end")
			}
		}
	}

	// Extract the section
	extractedLines := lines[startIdx:endIdx]
	extracted := strings.Join(extractedLines, "\n")
	extracted = strings.TrimSpace(extracted)

	// Remove trailing horizontal rules
	extracted = strings.TrimSuffix(extracted, "---")
	extracted = strings.TrimSpace(extracted)

	w.logger.Info().
		Str("ticker", tickerUpper).
		Int("start_line", startIdx).
		Int("end_line", endIdx).
		Int("extracted_length", len(extracted)).
		Msg("Extracted ticker-specific content")

	return extracted
}

// buildOutputDocumentWithContent creates an output document with pre-extracted content.
// This is used when extracting ticker-specific content from a combined analysis.
func (w *FormatterWorker) buildOutputDocumentWithContent(
	content string,
	title, format string,
	attachment bool,
	style, baseURL string,
	inputTags, outputTags []string,
	managerID string,
	order string,
	ticker string,
) *models.Document {
	var sb strings.Builder

	// Email instructions section (YAML-like frontmatter)
	sb.WriteString("---\n")
	sb.WriteString("# Email Instructions (parsed by email worker)\n")
	sb.WriteString(fmt.Sprintf("format: %s\n", format))
	sb.WriteString(fmt.Sprintf("attachment: %t\n", attachment))
	sb.WriteString(fmt.Sprintf("style: %s\n", style))
	sb.WriteString(fmt.Sprintf("base_url: %s\n", baseURL))
	sb.WriteString(fmt.Sprintf("order: %s\n", order))
	sb.WriteString("document_count: 1\n")
	sb.WriteString("---\n\n")

	// Build content
	var contentMarkdown strings.Builder
	contentMarkdown.WriteString(fmt.Sprintf("# %s\n\n", title))
	contentMarkdown.WriteString(fmt.Sprintf("**Generated**: %s\n", time.Now().Format("2 January 2006 3:04 PM MST")))
	contentMarkdown.WriteString(fmt.Sprintf("**Documents**: 1\n\n"))
	contentMarkdown.WriteString("---\n\n")
	contentMarkdown.WriteString(content)
	contentMarkdown.WriteString("\n\n---\n\n")

	// Add content to full document (frontmatter + content)
	sb.WriteString(contentMarkdown.String())

	// Convert markdown content to HTML for email priority
	htmlContent := w.convertMarkdownToHTML(contentMarkdown.String())

	// Build tags
	dateTag := fmt.Sprintf("date:%s", time.Now().Format("2006-01-02"))
	finalTags := append(outputTags, dateTag, "email-output")

	// Add ticker tag for multi-document mode
	if ticker != "" {
		finalTags = append(finalTags, strings.ToLower(ticker))
	}

	// Build metadata
	metadata := map[string]interface{}{
		"source_count": 1,
		"source_tags":  inputTags,
		"tickers":      []string{strings.ToUpper(ticker)},
		"format":       format,
		"attachment":   attachment,
		"style":        style,
		"order":        order,
		"format_date":  time.Now().Format(time.RFC3339),
		"email_ready":  true,
	}

	// Add ticker to metadata
	if ticker != "" {
		metadata["ticker"] = strings.ToUpper(ticker)
		metadata["multi_document"] = true
	}

	return &models.Document{
		ID:              uuid.New().String(),
		Title:           title,
		ContentMarkdown: sb.String(),
		ContentHTML:     htmlContent,
		Tags:            finalTags,
		Jobs:            []string{managerID},
		SourceType:      "output_formatter",
		CreatedAt:       time.Now(),
		UpdatedAt:       time.Now(),
		Metadata:        metadata,
	}
}
