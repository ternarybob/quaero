// -----------------------------------------------------------------------
// TickerNewsWorker - Aggregates news from EODHD and web search for tickers
// Creates a news summary document per ticker for caching and downstream use.
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
	"github.com/ternarybob/quaero/internal/common"
	"github.com/ternarybob/quaero/internal/eodhd"
	"github.com/ternarybob/quaero/internal/interfaces"
	"github.com/ternarybob/quaero/internal/models"
	"github.com/ternarybob/quaero/internal/queue"
	"github.com/ternarybob/quaero/internal/services/llm"
	"github.com/ternarybob/quaero/internal/workers/workerutil"
	"google.golang.org/genai"
)

// TickerNewsWorker fetches and aggregates news from multiple sources.
// Combines EODHD news API with web search for comprehensive coverage.
type TickerNewsWorker struct {
	documentStorage interfaces.DocumentStorage
	kvStorage       interfaces.KeyValueStorage
	searchService   interfaces.SearchService
	logger          arbor.ILogger
	jobMgr          *queue.Manager
	providerFactory *llm.ProviderFactory
	debugEnabled    bool
}

// Compile-time assertion
var _ interfaces.DefinitionWorker = (*TickerNewsWorker)(nil)

// NewsItem represents a single news article from any source
type NewsItem struct {
	Date      time.Time
	Headline  string
	Content   string
	Link      string
	Source    string // "eodhd" or "web_search"
	Sentiment float64
	Tags      []string
}

// TickerNewsSummary holds aggregated news for a ticker
type TickerNewsSummary struct {
	Ticker         common.Ticker
	EODHDNews      []NewsItem
	WebSearchNews  []NewsItem
	SentimentStats SentimentStats
	GeneratedAt    time.Time
	Period         string
	TotalNewsCount int
	EODHDCount     int
	WebSearchCount int
}

// SentimentStats summarizes sentiment across news items
type SentimentStats struct {
	PositiveCount int
	NeutralCount  int
	NegativeCount int
	AvgSentiment  float64
}

// NewTickerNewsWorker creates a new ticker news worker
func NewTickerNewsWorker(
	documentStorage interfaces.DocumentStorage,
	kvStorage interfaces.KeyValueStorage,
	searchService interfaces.SearchService,
	logger arbor.ILogger,
	jobMgr *queue.Manager,
	providerFactory *llm.ProviderFactory,
	debugEnabled bool,
) *TickerNewsWorker {
	return &TickerNewsWorker{
		documentStorage: documentStorage,
		kvStorage:       kvStorage,
		searchService:   searchService,
		logger:          logger,
		jobMgr:          jobMgr,
		providerFactory: providerFactory,
		debugEnabled:    debugEnabled,
	}
}

// GetType returns WorkerTypeTickerNews
func (w *TickerNewsWorker) GetType() models.WorkerType {
	return models.WorkerTypeTickerNews
}

// Init initializes the ticker news worker
func (w *TickerNewsWorker) Init(ctx context.Context, step models.JobStep, jobDef models.JobDefinition) (*interfaces.WorkerInitResult, error) {
	stepConfig := step.Config
	if stepConfig == nil {
		stepConfig = make(map[string]interface{})
	}

	// Collect tickers from step config and job-level variables first
	tickers := workerutil.CollectTickersWithJobDef(stepConfig, jobDef)

	// If no tickers found in config, try to extract from upstream documents
	// This enables pipeline patterns where navexa_portfolio outputs holdings
	// that downstream steps (fetch_news, fetch_metadata) need to process
	if len(tickers) == 0 && w.searchService != nil {
		// Get manager_id for job isolation (use empty string for init, will be resolved later)
		managerID := ""
		if w.jobMgr != nil {
			// Try to get manager_id from context or job state
			// During Init we may not have the stepID yet, so we search without job filter
			// and rely on input_tags for document matching
		}

		w.logger.Debug().
			Str("step_name", step.Name).
			Msg("No tickers in config, checking upstream documents via input_tags")

		tickers = workerutil.CollectTickersFromUpstreamDocs(ctx, w.searchService, stepConfig, step.Name, managerID, w.logger)
	}

	if len(tickers) == 0 {
		return nil, fmt.Errorf("no tickers specified - provide 'ticker', 'tickers', or config.variables")
	}

	// Period (default: M1 = 1 month)
	period := workerutil.GetStringConfig(stepConfig, "period", "M1")

	// Cache hours (default: 24)
	cacheHours := workerutil.GetIntConfig(stepConfig, "cache_hours", 24)

	// Force refresh
	forceRefresh := workerutil.GetBool(stepConfig, "force_refresh")

	w.logger.Info().
		Str("phase", "init").
		Str("step_name", step.Name).
		Int("ticker_count", len(tickers)).
		Str("period", period).
		Int("cache_hours", cacheHours).
		Bool("force_refresh", forceRefresh).
		Msg("Ticker news worker initialized")

	// Create work items for each ticker
	workItems := make([]interfaces.WorkItem, 0, len(tickers))
	for _, ticker := range tickers {
		workItems = append(workItems, interfaces.WorkItem{
			ID:   ticker.String(),
			Name: fmt.Sprintf("Fetch news for %s", ticker.String()),
			Type: "ticker_news",
			Config: map[string]interface{}{
				"ticker": ticker.String(),
				"period": period,
			},
		})
	}

	return &interfaces.WorkerInitResult{
		WorkItems:            workItems,
		TotalCount:           len(workItems),
		Strategy:             interfaces.ProcessingStrategyInline,
		SuggestedConcurrency: 1,
		Metadata: map[string]interface{}{
			"tickers":       tickers,
			"period":        period,
			"cache_hours":   cacheHours,
			"force_refresh": forceRefresh,
			"step_config":   stepConfig,
		},
	}, nil
}

// CreateJobs fetches news for all tickers and creates documents
func (w *TickerNewsWorker) CreateJobs(ctx context.Context, step models.JobStep, jobDef models.JobDefinition, stepID string, initResult *interfaces.WorkerInitResult) (string, error) {
	if initResult == nil {
		var err error
		initResult, err = w.Init(ctx, step, jobDef)
		if err != nil {
			return "", fmt.Errorf("failed to initialize ticker_news worker: %w", err)
		}
	}

	// Get manager_id for job isolation
	managerID := workerutil.GetManagerID(ctx, w.jobMgr, stepID)

	tickers, _ := initResult.Metadata["tickers"].([]common.Ticker)
	period, _ := initResult.Metadata["period"].(string)
	cacheHours, _ := initResult.Metadata["cache_hours"].(int)
	forceRefresh, _ := initResult.Metadata["force_refresh"].(bool)
	stepConfig, _ := initResult.Metadata["step_config"].(map[string]interface{})

	// Extract output tags
	outputTags := workerutil.GetOutputTags(stepConfig)

	// Get EODHD API key
	eodhdAPIKey, err := common.ResolveAPIKey(ctx, w.kvStorage, "eodhd_api_key", "")
	if err != nil {
		w.logger.Warn().Err(err).Msg("Failed to resolve EODHD API key - EODHD news will be skipped")
	}

	// Get Gemini API key for web search
	geminiAPIKey, err := common.ResolveAPIKey(ctx, w.kvStorage, "google_gemini_api_key", "")
	if err != nil {
		w.logger.Warn().Err(err).Msg("Failed to resolve Gemini API key - web search will be skipped")
	}

	w.logger.Info().
		Str("phase", "run").
		Str("step_name", step.Name).
		Int("ticker_count", len(tickers)).
		Str("period", period).
		Bool("has_eodhd_key", eodhdAPIKey != "").
		Bool("has_gemini_key", geminiAPIKey != "").
		Msg("Starting ticker news collection")

	if w.jobMgr != nil {
		w.jobMgr.AddJobLog(ctx, stepID, "info", fmt.Sprintf("Collecting news for %d tickers", len(tickers)))
	}

	// Process each ticker
	for _, ticker := range tickers {
		if err := w.processTickerNews(ctx, ticker, period, cacheHours, forceRefresh, eodhdAPIKey, geminiAPIKey, stepID, managerID, outputTags); err != nil {
			w.logger.Error().Err(err).Str("ticker", ticker.String()).Msg("Failed to process ticker news")
			if w.jobMgr != nil {
				w.jobMgr.AddJobLog(ctx, stepID, "warn", fmt.Sprintf("Failed to fetch news for %s: %v", ticker.String(), err))
			}
			// Continue with other tickers
		}
	}

	return stepID, nil
}

// processTickerNews fetches and processes news for a single ticker
func (w *TickerNewsWorker) processTickerNews(
	ctx context.Context,
	ticker common.Ticker,
	period string,
	cacheHours int,
	forceRefresh bool,
	eodhdAPIKey, geminiAPIKey string,
	stepID, managerID string,
	outputTags []string,
) error {
	// Generate source ID for caching
	today := time.Now().Format("2006-01-02")
	sourceID := fmt.Sprintf("%s:%s", ticker.String(), today)

	// Check cache
	if !forceRefresh && cacheHours > 0 {
		existingDoc, err := w.documentStorage.GetDocumentBySource("ticker_news", sourceID)
		if err == nil && existingDoc != nil {
			if w.isCacheFresh(existingDoc, cacheHours) {
				w.logger.Info().
					Str("ticker", ticker.String()).
					Str("source_id", sourceID).
					Msg("Using cached ticker news")
				// Associate with current job for isolation
				if err := workerutil.AssociateDocumentWithJob(ctx, existingDoc, managerID, w.documentStorage, w.logger); err != nil {
					w.logger.Warn().Err(err).Msg("Failed to associate cached document with job")
				}
				return nil
			}
		}
	}

	summary := &TickerNewsSummary{
		Ticker:      ticker,
		GeneratedAt: time.Now(),
		Period:      period,
	}

	// Fetch EODHD news
	if eodhdAPIKey != "" {
		eodhdNews, err := w.fetchEODHDNews(ctx, ticker, period, eodhdAPIKey)
		if err != nil {
			w.logger.Warn().Err(err).Str("ticker", ticker.String()).Msg("Failed to fetch EODHD news")
		} else {
			summary.EODHDNews = eodhdNews
			summary.EODHDCount = len(eodhdNews)
		}
	}

	// Fetch web search news
	if geminiAPIKey != "" {
		webNews, err := w.fetchWebSearchNews(ctx, ticker, period, geminiAPIKey)
		if err != nil {
			w.logger.Warn().Err(err).Str("ticker", ticker.String()).Msg("Failed to fetch web search news")
		} else {
			summary.WebSearchNews = webNews
			summary.WebSearchCount = len(webNews)
		}
	}

	// Calculate totals and sentiment
	summary.TotalNewsCount = summary.EODHDCount + summary.WebSearchCount
	summary.SentimentStats = w.calculateSentimentStats(summary.EODHDNews, summary.WebSearchNews)

	// Create document
	doc := w.createNewsDocument(summary, stepID, managerID, outputTags, sourceID)
	if err := w.documentStorage.SaveDocument(doc); err != nil {
		return fmt.Errorf("failed to save news document: %w", err)
	}

	w.logger.Info().
		Str("ticker", ticker.String()).
		Int("eodhd_count", summary.EODHDCount).
		Int("web_count", summary.WebSearchCount).
		Int("total_count", summary.TotalNewsCount).
		Msg("Ticker news document created")

	if w.jobMgr != nil {
		w.jobMgr.AddJobLog(ctx, stepID, "info", fmt.Sprintf(
			"%s: %d news items (%d EODHD, %d web)",
			ticker.String(), summary.TotalNewsCount, summary.EODHDCount, summary.WebSearchCount,
		))
	}

	return nil
}

// fetchEODHDNews fetches news from EODHD API
func (w *TickerNewsWorker) fetchEODHDNews(ctx context.Context, ticker common.Ticker, period, apiKey string) ([]NewsItem, error) {
	client := eodhd.NewClient(apiKey, eodhd.WithLogger(w.logger))

	// Calculate date range
	to := time.Now()
	from := w.calculateFromDate(to, period)

	// Fetch news
	symbol := ticker.EODHDSymbol()
	news, err := client.GetNews(ctx, []string{symbol},
		eodhd.WithDateRange(from, to),
		eodhd.WithLimit(50),
	)
	if err != nil {
		return nil, fmt.Errorf("EODHD news fetch failed: %w", err)
	}

	// Convert to NewsItem
	var items []NewsItem
	for _, n := range news {
		sentiment := 0.0
		if n.Sentiment != nil {
			sentiment = n.Sentiment.Polarity
		}
		items = append(items, NewsItem{
			Date:      n.Date,
			Headline:  n.Title,
			Content:   n.Content,
			Link:      n.Link,
			Source:    "eodhd",
			Sentiment: sentiment,
			Tags:      n.Tags,
		})
	}

	// Sort by date descending
	sort.Slice(items, func(i, j int) bool {
		return items[i].Date.After(items[j].Date)
	})

	return items, nil
}

// fetchWebSearchNews fetches news via Gemini web search
func (w *TickerNewsWorker) fetchWebSearchNews(ctx context.Context, ticker common.Ticker, period, apiKey string) ([]NewsItem, error) {
	// Create Gemini client
	client, err := genai.NewClient(ctx, &genai.ClientConfig{
		APIKey:  apiKey,
		Backend: genai.BackendGeminiAPI,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create Gemini client: %w", err)
	}

	// Build search query
	companyName := ticker.Code // We'll use ticker code; could be enhanced with company name
	periodDesc := w.periodToDescription(period)
	query := fmt.Sprintf("Latest news and announcements for %s stock exchange:%s %s",
		companyName, ticker.Exchange, periodDesc)

	// Configure search tool
	searchTool := &genai.Tool{GoogleSearch: &genai.GoogleSearch{}}
	config := &genai.GenerateContentConfig{
		Tools: []*genai.Tool{searchTool},
	}

	// Current date for context
	currentDate := time.Now().Format("January 2, 2006")
	systemPrompt := fmt.Sprintf(`You are a financial news researcher. Today's date is %s.
Search for the latest news about %s (%s) stock.
Focus on:
- Recent company announcements
- Earnings reports
- Market analysis
- Significant price movements
- Regulatory filings

Return a summary of the key news items found.`, currentDate, ticker.Code, ticker.Exchange)

	// Execute search with timeout
	searchCtx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()

	resp, err := client.Models.GenerateContent(
		searchCtx,
		"gemini-2.0-flash",
		[]*genai.Content{
			genai.NewContentFromText(systemPrompt+"\n\nQuery: "+query, genai.RoleUser),
		},
		config,
	)
	if err != nil {
		return nil, fmt.Errorf("web search failed: %w", err)
	}

	// Extract content
	var content string
	var sources []string
	if len(resp.Candidates) > 0 && resp.Candidates[0].Content != nil {
		for _, part := range resp.Candidates[0].Content.Parts {
			if part.Text != "" {
				content = part.Text
			}
		}
		// Extract sources from grounding metadata
		if resp.Candidates[0].GroundingMetadata != nil && resp.Candidates[0].GroundingMetadata.GroundingChunks != nil {
			for _, chunk := range resp.Candidates[0].GroundingMetadata.GroundingChunks {
				if chunk.Web != nil {
					sources = append(sources, chunk.Web.URI)
				}
			}
		}
	}

	// Create a single news item representing the web search results
	if content != "" {
		return []NewsItem{
			{
				Date:      time.Now(),
				Headline:  fmt.Sprintf("Web Search Summary: %s", ticker.String()),
				Content:   content,
				Link:      strings.Join(sources, "\n"),
				Source:    "web_search",
				Sentiment: 0, // Web search doesn't provide sentiment
				Tags:      []string{"web-search", "aggregated"},
			},
		}, nil
	}

	return nil, nil
}

// calculateFromDate calculates the start date based on period string
func (w *TickerNewsWorker) calculateFromDate(to time.Time, period string) time.Time {
	switch period {
	case "W1":
		return to.AddDate(0, 0, -7)
	case "M1":
		return to.AddDate(0, -1, 0)
	case "M3":
		return to.AddDate(0, -3, 0)
	case "M6":
		return to.AddDate(0, -6, 0)
	case "Y1":
		return to.AddDate(-1, 0, 0)
	default:
		return to.AddDate(0, -1, 0) // Default to 1 month
	}
}

// periodToDescription converts period code to human-readable description
func (w *TickerNewsWorker) periodToDescription(period string) string {
	switch period {
	case "W1":
		return "past week"
	case "M1":
		return "past month"
	case "M3":
		return "past 3 months"
	case "M6":
		return "past 6 months"
	case "Y1":
		return "past year"
	default:
		return "recent"
	}
}

// calculateSentimentStats calculates sentiment statistics across news items
func (w *TickerNewsWorker) calculateSentimentStats(eodhdNews, webNews []NewsItem) SentimentStats {
	stats := SentimentStats{}
	var totalSentiment float64
	count := 0

	for _, item := range eodhdNews {
		if item.Sentiment > 0.1 {
			stats.PositiveCount++
		} else if item.Sentiment < -0.1 {
			stats.NegativeCount++
		} else {
			stats.NeutralCount++
		}
		totalSentiment += item.Sentiment
		count++
	}

	// Web search items don't have sentiment
	stats.NeutralCount += len(webNews)

	if count > 0 {
		stats.AvgSentiment = totalSentiment / float64(count)
	}

	return stats
}

// isCacheFresh checks if document is within cache window
func (w *TickerNewsWorker) isCacheFresh(doc *models.Document, cacheHours int) bool {
	if doc == nil || doc.LastSynced == nil {
		return false
	}
	cacheWindow := time.Duration(cacheHours) * time.Hour
	return time.Since(*doc.LastSynced) < cacheWindow
}

// createNewsDocument creates a news summary document
func (w *TickerNewsWorker) createNewsDocument(
	summary *TickerNewsSummary,
	stepID, managerID string,
	outputTags []string,
	sourceID string,
) *models.Document {
	var content strings.Builder

	content.WriteString(fmt.Sprintf("# %s News Summary\n\n", summary.Ticker.String()))
	content.WriteString(fmt.Sprintf("**Period**: %s\n", summary.Period))
	content.WriteString(fmt.Sprintf("**Generated**: %s\n", summary.GeneratedAt.Format("2006-01-02 15:04 MST")))
	content.WriteString(fmt.Sprintf("**Total Articles**: %d (%d EODHD, %d Web Search)\n\n",
		summary.TotalNewsCount, summary.EODHDCount, summary.WebSearchCount))

	// Sentiment overview
	if summary.EODHDCount > 0 {
		content.WriteString("## Sentiment Overview\n\n")
		content.WriteString("| Sentiment | Count |\n")
		content.WriteString("|-----------|-------|\n")
		content.WriteString(fmt.Sprintf("| Positive | %d |\n", summary.SentimentStats.PositiveCount))
		content.WriteString(fmt.Sprintf("| Neutral | %d |\n", summary.SentimentStats.NeutralCount))
		content.WriteString(fmt.Sprintf("| Negative | %d |\n\n", summary.SentimentStats.NegativeCount))
	}

	// EODHD News section
	if len(summary.EODHDNews) > 0 {
		content.WriteString("## Recent News (EODHD)\n\n")
		displayCount := 10
		if len(summary.EODHDNews) < displayCount {
			displayCount = len(summary.EODHDNews)
		}

		for i := 0; i < displayCount; i++ {
			item := summary.EODHDNews[i]
			content.WriteString(fmt.Sprintf("### %s\n", item.Headline))
			content.WriteString(fmt.Sprintf("**Date**: %s\n", item.Date.Format("2006-01-02")))

			sentiment := "Neutral"
			if item.Sentiment > 0.1 {
				sentiment = "Positive"
			} else if item.Sentiment < -0.1 {
				sentiment = "Negative"
			}
			content.WriteString(fmt.Sprintf("**Sentiment**: %s (%.2f)\n\n", sentiment, item.Sentiment))

			// Truncate content
			text := item.Content
			if len(text) > 500 {
				text = text[:500] + "..."
			}
			content.WriteString(text + "\n\n")

			if item.Link != "" {
				content.WriteString(fmt.Sprintf("[Read More](%s)\n\n", item.Link))
			}
			content.WriteString("---\n\n")
		}
	}

	// Web Search section
	if len(summary.WebSearchNews) > 0 {
		content.WriteString("## Web Search Results\n\n")
		for _, item := range summary.WebSearchNews {
			content.WriteString(item.Content)
			content.WriteString("\n\n")

			if item.Link != "" {
				content.WriteString("**Sources**:\n")
				for _, link := range strings.Split(item.Link, "\n") {
					if link != "" {
						content.WriteString(fmt.Sprintf("- <%s>\n", link))
					}
				}
				content.WriteString("\n")
			}
		}
	}

	// Build tags
	tags := []string{
		"ticker-news",
		strings.ToLower(summary.Ticker.String()),
		strings.ToLower(summary.Ticker.Exchange),
		strings.ToLower(summary.Ticker.Code),
		"date:" + summary.GeneratedAt.Format("2006-01-02"),
	}
	tags = append(tags, outputTags...)

	// Build metadata
	metadata := map[string]interface{}{
		"ticker":           summary.Ticker.String(),
		"exchange":         summary.Ticker.Exchange,
		"code":             summary.Ticker.Code,
		"news_count":       summary.TotalNewsCount,
		"eodhd_count":      summary.EODHDCount,
		"web_search_count": summary.WebSearchCount,
		"sentiment_summary": map[string]interface{}{
			"positive": summary.SentimentStats.PositiveCount,
			"neutral":  summary.SentimentStats.NeutralCount,
			"negative": summary.SentimentStats.NegativeCount,
			"average":  summary.SentimentStats.AvgSentiment,
		},
		"period":     summary.Period,
		"fetched_at": summary.GeneratedAt.Format(time.RFC3339),
		"job_id":     stepID,
	}

	now := time.Now()
	return &models.Document{
		ID:              uuid.New().String(),
		Title:           fmt.Sprintf("%s News Summary", summary.Ticker.String()),
		ContentMarkdown: content.String(),
		DetailLevel:     models.DetailLevelFull,
		SourceType:      "ticker_news",
		SourceID:        sourceID,
		Tags:            tags,
		Jobs:            []string{managerID},
		Metadata:        metadata,
		CreatedAt:       now,
		UpdatedAt:       now,
		LastSynced:      &now,
	}
}

// ReturnsChildJobs returns false
func (w *TickerNewsWorker) ReturnsChildJobs() bool {
	return false
}

// ValidateConfig validates step configuration
func (w *TickerNewsWorker) ValidateConfig(step models.JobStep) error {
	// Tickers can come from step config or job-level variables
	// So minimal validation here - Init will do the full check
	return nil
}
