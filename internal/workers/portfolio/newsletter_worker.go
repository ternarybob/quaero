// -----------------------------------------------------------------------
// NewsletterWorker - Generates portfolio newsletter from news and metadata documents
// Synthesizes ticker news and company metadata into a comprehensive portfolio newsletter
// using LLM to create actionable insights.
// -----------------------------------------------------------------------

package portfolio

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/ternarybob/arbor"
	"github.com/ternarybob/quaero/internal/common"
	"github.com/ternarybob/quaero/internal/interfaces"
	"github.com/ternarybob/quaero/internal/models"
	"github.com/ternarybob/quaero/internal/queue"
	"github.com/ternarybob/quaero/internal/services/llm"
	"github.com/ternarybob/quaero/internal/workers/workerutil"
)

// NewsletterWorker generates portfolio newsletters from news and metadata documents.
// It consumes documents via filter_tags/input_tags and uses LLM to synthesize
// a "Monday morning newsletter" style summary.
type NewsletterWorker struct {
	documentStorage interfaces.DocumentStorage
	kvStorage       interfaces.KeyValueStorage
	searchService   interfaces.SearchService
	logger          arbor.ILogger
	jobMgr          *queue.Manager
	providerFactory *llm.ProviderFactory
	debugEnabled    bool
}

// Compile-time assertion
var _ interfaces.DefinitionWorker = (*NewsletterWorker)(nil)

// NewsletterContext holds data for newsletter generation
type NewsletterContext struct {
	PortfolioName     string
	Tickers           []common.Ticker
	NewsDocuments     []*models.Document
	MetadataDocuments []*models.Document
	TickersByGroup    map[string][]common.Ticker
	GeneratedAt       time.Time
}

// NewNewsletterWorker creates a new newsletter worker
func NewNewsletterWorker(
	documentStorage interfaces.DocumentStorage,
	kvStorage interfaces.KeyValueStorage,
	searchService interfaces.SearchService,
	logger arbor.ILogger,
	jobMgr *queue.Manager,
	providerFactory *llm.ProviderFactory,
	debugEnabled bool,
) *NewsletterWorker {
	return &NewsletterWorker{
		documentStorage: documentStorage,
		kvStorage:       kvStorage,
		searchService:   searchService,
		logger:          logger,
		jobMgr:          jobMgr,
		providerFactory: providerFactory,
		debugEnabled:    debugEnabled,
	}
}

// GetType returns WorkerTypePortfolioNewsletter
func (w *NewsletterWorker) GetType() models.WorkerType {
	return models.WorkerTypePortfolioNewsletter
}

// ReturnsChildJobs returns false - this worker executes inline
func (w *NewsletterWorker) ReturnsChildJobs() bool {
	return false
}

// ValidateConfig validates the worker configuration
func (w *NewsletterWorker) ValidateConfig(step models.JobStep) error {
	// input_tags or filter_tags is needed but can default to step name
	return nil
}

// Init initializes the newsletter worker
func (w *NewsletterWorker) Init(ctx context.Context, step models.JobStep, jobDef models.JobDefinition) (*interfaces.WorkerInitResult, error) {
	stepConfig := step.Config
	if stepConfig == nil {
		stepConfig = make(map[string]interface{})
	}

	// Get portfolio/group name from config or use default
	portfolio := workerutil.GetStringConfig(stepConfig, "portfolio", "default")

	// Get model from config or use default
	model := workerutil.GetStringConfig(stepConfig, "model", "gemini")

	// Get input tags for document filtering
	inputTags := workerutil.GetInputTags(stepConfig, step.Name)

	// Collect tickers from job def for validation
	tickers := workerutil.CollectTickersWithJobDef(stepConfig, jobDef)

	w.logger.Info().
		Str("phase", "init").
		Str("step_name", step.Name).
		Str("portfolio", portfolio).
		Str("model", model).
		Strs("input_tags", inputTags).
		Int("ticker_count", len(tickers)).
		Msg("Newsletter worker initialized")

	return &interfaces.WorkerInitResult{
		WorkItems: []interfaces.WorkItem{
			{
				ID:     fmt.Sprintf("newsletter-%s", portfolio),
				Name:   fmt.Sprintf("Generate newsletter for %s", portfolio),
				Type:   "portfolio_newsletter",
				Config: stepConfig,
			},
		},
		TotalCount:           1,
		Strategy:             interfaces.ProcessingStrategyInline,
		SuggestedConcurrency: 1,
		Metadata: map[string]interface{}{
			"portfolio":   portfolio,
			"model":       model,
			"input_tags":  inputTags,
			"tickers":     tickers,
			"step_config": stepConfig,
		},
	}, nil
}

// CreateJobs generates the newsletter document from news and metadata
func (w *NewsletterWorker) CreateJobs(ctx context.Context, step models.JobStep, jobDef models.JobDefinition, stepID string, initResult *interfaces.WorkerInitResult) (string, error) {
	if initResult == nil {
		var err error
		initResult, err = w.Init(ctx, step, jobDef)
		if err != nil {
			return "", fmt.Errorf("failed to initialize newsletter worker: %w", err)
		}
	}

	// Get manager_id for job isolation
	managerID := workerutil.GetManagerID(ctx, w.jobMgr, stepID)

	portfolio, _ := initResult.Metadata["portfolio"].(string)
	model, _ := initResult.Metadata["model"].(string)
	inputTags, _ := initResult.Metadata["input_tags"].([]string)
	tickers, _ := initResult.Metadata["tickers"].([]common.Ticker)
	stepConfig, _ := initResult.Metadata["step_config"].(map[string]interface{})

	// Extract output tags
	outputTags := workerutil.GetOutputTags(stepConfig)

	w.logger.Info().
		Str("phase", "run").
		Str("step_name", step.Name).
		Str("portfolio", portfolio).
		Strs("input_tags", inputTags).
		Str("manager_id", managerID).
		Msg("Starting newsletter generation")

	if w.jobMgr != nil {
		w.jobMgr.AddJobLog(ctx, stepID, "info", fmt.Sprintf("Generating newsletter for %s", portfolio))
	}

	// Step 1: Collect news and metadata documents
	newsletterCtx, err := w.collectDocuments(ctx, inputTags, managerID, tickers, portfolio)
	if err != nil {
		return "", fmt.Errorf("failed to collect documents: %w", err)
	}

	if len(newsletterCtx.NewsDocuments) == 0 && len(newsletterCtx.MetadataDocuments) == 0 {
		return "", fmt.Errorf("no news or metadata documents found with tags %v for job %s", inputTags, managerID)
	}

	w.logger.Info().
		Int("news_count", len(newsletterCtx.NewsDocuments)).
		Int("metadata_count", len(newsletterCtx.MetadataDocuments)).
		Int("ticker_count", len(newsletterCtx.Tickers)).
		Msg("Documents collected for newsletter")

	if w.jobMgr != nil {
		w.jobMgr.AddJobLog(ctx, stepID, "info", fmt.Sprintf(
			"Found %d news documents and %d metadata documents for %d tickers",
			len(newsletterCtx.NewsDocuments), len(newsletterCtx.MetadataDocuments), len(newsletterCtx.Tickers),
		))
	}

	// Step 2: Generate newsletter via LLM
	newsletterContent, err := w.generateNewsletter(ctx, newsletterCtx, model, stepID)
	if err != nil {
		return "", fmt.Errorf("failed to generate newsletter: %w", err)
	}

	// Step 3: Create and save newsletter document
	doc := w.createNewsletterDocument(newsletterCtx, newsletterContent, stepID, managerID, outputTags)
	if err := w.documentStorage.SaveDocument(doc); err != nil {
		return "", fmt.Errorf("failed to save newsletter document: %w", err)
	}

	w.logger.Info().
		Str("document_id", doc.ID).
		Str("portfolio", portfolio).
		Int("tickers", len(newsletterCtx.Tickers)).
		Msg("Newsletter document created")

	if w.jobMgr != nil {
		w.jobMgr.AddJobLog(ctx, stepID, "info", fmt.Sprintf("Newsletter generated: %s", doc.Title))
	}

	return stepID, nil
}

// collectDocuments gathers news and metadata documents from storage
func (w *NewsletterWorker) collectDocuments(
	ctx context.Context,
	inputTags []string,
	managerID string,
	tickers []common.Ticker,
	portfolio string,
) (*NewsletterContext, error) {
	nlCtx := &NewsletterContext{
		PortfolioName:  portfolio,
		Tickers:        tickers,
		TickersByGroup: make(map[string][]common.Ticker),
		GeneratedAt:    time.Now(),
	}

	// Build search options
	searchOpts := interfaces.SearchOptions{
		Tags:  inputTags,
		JobID: managerID,
		Limit: 100,
	}

	// Search for all documents with input tags
	docs, err := w.searchService.Search(ctx, "", searchOpts)
	if err != nil {
		return nil, fmt.Errorf("failed to search documents: %w", err)
	}

	// Also try searching for ticker-news and ticker-metadata tags specifically
	newsSearchOpts := interfaces.SearchOptions{
		Tags:  []string{"ticker-news"},
		JobID: managerID,
		Limit: 100,
	}
	newsDocs, _ := w.searchService.Search(ctx, "", newsSearchOpts)

	metadataSearchOpts := interfaces.SearchOptions{
		Tags:  []string{"ticker-metadata"},
		JobID: managerID,
		Limit: 100,
	}
	metadataDocs, _ := w.searchService.Search(ctx, "", metadataSearchOpts)

	// Combine and deduplicate
	docMap := make(map[string]*models.Document)
	for _, doc := range docs {
		docMap[doc.ID] = doc
	}
	for _, doc := range newsDocs {
		docMap[doc.ID] = doc
	}
	for _, doc := range metadataDocs {
		docMap[doc.ID] = doc
	}

	// Categorize documents
	tickerSet := make(map[string]common.Ticker)
	for _, doc := range docMap {
		if doc.SourceType == "ticker_news" {
			nlCtx.NewsDocuments = append(nlCtx.NewsDocuments, doc)
			// Extract ticker from metadata
			if tickerStr, ok := doc.Metadata["ticker"].(string); ok {
				ticker := common.ParseTicker(tickerStr)
				tickerSet[ticker.String()] = ticker
			}
		} else if doc.SourceType == "ticker_metadata" {
			nlCtx.MetadataDocuments = append(nlCtx.MetadataDocuments, doc)
			// Extract ticker from metadata
			if tickerStr, ok := doc.Metadata["ticker"].(string); ok {
				ticker := common.ParseTicker(tickerStr)
				tickerSet[ticker.String()] = ticker
			}
		}
	}

	// Convert ticker set to slice and sort
	for _, ticker := range tickerSet {
		nlCtx.Tickers = append(nlCtx.Tickers, ticker)
	}
	sort.Slice(nlCtx.Tickers, func(i, j int) bool {
		return nlCtx.Tickers[i].String() < nlCtx.Tickers[j].String()
	})

	// Group by portfolio (default group if not specified)
	nlCtx.TickersByGroup[portfolio] = nlCtx.Tickers

	return nlCtx, nil
}

// generateNewsletter uses LLM to synthesize a newsletter
func (w *NewsletterWorker) generateNewsletter(
	ctx context.Context,
	nlCtx *NewsletterContext,
	model string,
	stepID string,
) (string, error) {
	// Build context string from news and metadata documents
	var contextBuilder strings.Builder

	// Add news summaries
	contextBuilder.WriteString("# NEWS DATA\n\n")
	for _, doc := range nlCtx.NewsDocuments {
		tickerStr := "Unknown"
		if t, ok := doc.Metadata["ticker"].(string); ok {
			tickerStr = t
		}
		contextBuilder.WriteString(fmt.Sprintf("## %s\n", tickerStr))
		// Include summary portion of content (first 2000 chars)
		content := doc.ContentMarkdown
		if len(content) > 2000 {
			content = content[:2000] + "..."
		}
		contextBuilder.WriteString(content)
		contextBuilder.WriteString("\n\n")
	}

	// Add metadata summaries
	contextBuilder.WriteString("# COMPANY DATA\n\n")
	for _, doc := range nlCtx.MetadataDocuments {
		tickerStr := "Unknown"
		if t, ok := doc.Metadata["ticker"].(string); ok {
			tickerStr = t
		}
		companyName := ""
		if cn, ok := doc.Metadata["company_name"].(string); ok {
			companyName = cn
		}
		industry := ""
		if ind, ok := doc.Metadata["industry"].(string); ok {
			industry = ind
		}
		location := ""
		if loc, ok := doc.Metadata["location"].(string); ok {
			location = loc
		}

		contextBuilder.WriteString(fmt.Sprintf("## %s - %s\n", tickerStr, companyName))
		contextBuilder.WriteString(fmt.Sprintf("Industry: %s, Location: %s\n", industry, location))
		// Include overview portion of content (first 1000 chars)
		content := doc.ContentMarkdown
		if len(content) > 1000 {
			content = content[:1000] + "..."
		}
		contextBuilder.WriteString(content)
		contextBuilder.WriteString("\n\n")
	}

	// Build ticker list
	var tickerList []string
	for _, t := range nlCtx.Tickers {
		tickerList = append(tickerList, t.String())
	}

	// Build the prompt
	currentDate := time.Now().Format("Monday, January 2, 2006")
	prompt := fmt.Sprintf(`You are a financial newsletter writer preparing a "Monday Morning Market Brief" for an investor.

The investor holds the following stocks: %s
Portfolio/Group: %s
Today's Date: %s

Based on the news and company data provided below, write a concise newsletter that:
1. Opens with a brief market overview relevant to the portfolio holdings
2. Summarizes the most important news affecting the portfolio this week
3. Highlights any geopolitical events or macro factors relevant to the holdings (e.g., if holding gold/silver miners, discuss precious metals markets; if holding resources companies, discuss commodity prices)
4. Notes any sector-specific trends
5. Provides a brief outlook for the coming week

Consider:
- Company locations and exchanges when analyzing global events
- Industry correlations (e.g., all tech stocks may be affected by similar factors)
- Market sentiment indicators from the news

Keep the tone professional but accessible. Use markdown formatting for headers and bullet points.
The newsletter should be 500-800 words.

---
%s`, strings.Join(tickerList, ", "), nlCtx.PortfolioName, currentDate, contextBuilder.String())

	// Make LLM request
	if w.jobMgr != nil {
		w.jobMgr.AddJobLog(ctx, stepID, "info", "Generating newsletter via LLM...")
	}

	request := &llm.ContentRequest{
		Model:       model,
		Temperature: 0.7,
		MaxTokens:   2000,
		Messages: []interfaces.Message{
			{Role: "user", Content: prompt},
		},
	}

	response, err := w.providerFactory.GenerateContent(ctx, request)
	if err != nil {
		return "", fmt.Errorf("LLM generation failed: %w", err)
	}

	return response.Text, nil
}

// createNewsletterDocument creates the newsletter document
func (w *NewsletterWorker) createNewsletterDocument(
	nlCtx *NewsletterContext,
	content string,
	stepID, managerID string,
	outputTags []string,
) *models.Document {
	var fullContent strings.Builder

	// Build ticker list for header
	var tickerList []string
	for _, t := range nlCtx.Tickers {
		tickerList = append(tickerList, t.String())
	}

	fullContent.WriteString(fmt.Sprintf("# %s Market Brief\n\n", nlCtx.PortfolioName))
	fullContent.WriteString(fmt.Sprintf("**Week of**: %s\n", nlCtx.GeneratedAt.Format("January 2, 2006")))
	fullContent.WriteString(fmt.Sprintf("**Holdings**: %s\n\n", strings.Join(tickerList, ", ")))
	fullContent.WriteString("---\n\n")
	fullContent.WriteString(content)
	fullContent.WriteString("\n\n---\n")
	fullContent.WriteString(fmt.Sprintf("*Generated by Quaero on %s*\n", nlCtx.GeneratedAt.Format("2006-01-02 15:04 MST")))

	// Build tags
	today := nlCtx.GeneratedAt.Format("2006-01-02")
	tags := []string{
		"newsletter",
		nlCtx.PortfolioName,
		"date:" + today,
	}
	// Add ticker tags
	for _, t := range nlCtx.Tickers {
		tags = append(tags, t.String())
	}
	tags = append(tags, outputTags...)

	// Build metadata
	metadata := map[string]interface{}{
		"portfolio":      nlCtx.PortfolioName,
		"tickers":        tickerList,
		"ticker_count":   len(nlCtx.Tickers),
		"news_count":     len(nlCtx.NewsDocuments),
		"metadata_count": len(nlCtx.MetadataDocuments),
		"generated_at":   nlCtx.GeneratedAt.Format(time.RFC3339),
		"job_id":         stepID,
	}

	// Generate source ID for potential caching
	sourceID := fmt.Sprintf("%s:%s", nlCtx.PortfolioName, today)

	now := time.Now()
	return &models.Document{
		ID:              uuid.New().String(),
		Title:           fmt.Sprintf("%s Market Brief - %s", nlCtx.PortfolioName, nlCtx.GeneratedAt.Format("Jan 2, 2006")),
		ContentMarkdown: fullContent.String(),
		DetailLevel:     models.DetailLevelFull,
		SourceType:      "portfolio_newsletter",
		SourceID:        sourceID,
		Tags:            tags,
		Jobs:            []string{managerID},
		Metadata:        metadata,
		CreatedAt:       now,
		UpdatedAt:       now,
		LastSynced:      &now,
	}
}
