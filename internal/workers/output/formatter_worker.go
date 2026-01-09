// -----------------------------------------------------------------------
// FormatterWorker - Prepares output documents for email delivery
// Creates a single markdown document with email instructions and content
// Supports variable substitution and ticker-based document collection
// -----------------------------------------------------------------------

package output

import (
	"bytes"
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

	w.logger.Info().
		Str("phase", "run").
		Str("step_name", step.Name).
		Str("step_id", stepID).
		Strs("input_tags", inputTags).
		Str("format", format).
		Bool("attachment", attachment).
		Str("style", style).
		Str("order", order).
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

	// Build the output document with email instructions
	doc := w.buildOutputDocument(docsWithTickers, title, format, attachment, style, baseURL, inputTags, outputTags, managerID, order)

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

// buildOutputDocument creates the output document with email instructions.
// Generates both markdown and HTML content for email worker priority selection.
func (w *FormatterWorker) buildOutputDocument(
	docsWithTickers []docWithTickerLocal,
	title, format string,
	attachment bool,
	style, baseURL string,
	inputTags, outputTags []string,
	managerID string,
	order string,
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

	// Extract tickers
	var tickers []string
	for _, dwt := range docsWithTickers {
		if dwt.ticker != "" {
			tickers = append(tickers, dwt.ticker)
		}
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
		Metadata: map[string]interface{}{
			"source_count": len(docsWithTickers),
			"source_tags":  inputTags,
			"tickers":      tickers,
			"format":       format,
			"attachment":   attachment,
			"style":        style,
			"order":        order,
			"format_date":  time.Now().Format(time.RFC3339),
			"email_ready":  true,
		},
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
