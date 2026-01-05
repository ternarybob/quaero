// -----------------------------------------------------------------------
// MarketNewsWorker - Fetches company announcements for any exchange
// Uses EODHD News API for non-ASX exchanges.
// For ASX tickers, delegates to MarketAnnouncementsWorker (Markit API has better ASX coverage).
// -----------------------------------------------------------------------

package workers

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
)

// MarketNewsWorker fetches company announcements for any exchange.
// For ASX tickers, delegates to MarketAnnouncementsWorker.
// For other exchanges, uses EODHD News API.
type MarketNewsWorker struct {
	documentStorage           interfaces.DocumentStorage
	kvStorage                 interfaces.KeyValueStorage
	logger                    arbor.ILogger
	jobMgr                    *queue.Manager
	marketAnnouncementsWorker *MarketAnnouncementsWorker
}

// Compile-time assertion
var _ interfaces.DefinitionWorker = (*MarketNewsWorker)(nil)

// Announcement represents a company announcement (exchange-agnostic)
type Announcement struct {
	Date      time.Time
	Headline  string
	Content   string
	Link      string
	Source    string
	Symbols   []string
	Tags      []string
	Sentiment *AnnouncementSentiment
}

// AnnouncementSentiment represents sentiment analysis
type AnnouncementSentiment struct {
	Polarity float64
	Positive float64
	Neutral  float64
	Negative float64
}

// NewMarketNewsWorker creates a new market news worker
func NewMarketNewsWorker(
	documentStorage interfaces.DocumentStorage,
	kvStorage interfaces.KeyValueStorage,
	logger arbor.ILogger,
	jobMgr *queue.Manager,
	marketAnnouncementsWorker *MarketAnnouncementsWorker,
) *MarketNewsWorker {
	return &MarketNewsWorker{
		documentStorage:           documentStorage,
		kvStorage:                 kvStorage,
		logger:                    logger,
		jobMgr:                    jobMgr,
		marketAnnouncementsWorker: marketAnnouncementsWorker,
	}
}

// getEODHDAPIKey retrieves the EODHD API key from KV storage
func (w *MarketNewsWorker) getEODHDAPIKey(ctx context.Context) string {
	if w.kvStorage == nil {
		w.logger.Warn().Msg("EODHD API key lookup failed: kvStorage is nil")
		return ""
	}
	apiKey, err := common.ResolveAPIKey(ctx, w.kvStorage, "eodhd_api_key", "")
	if err != nil {
		w.logger.Warn().Err(err).Str("key_name", "eodhd_api_key").Msg("Failed to resolve EODHD API key")
		return ""
	}
	if apiKey == "" {
		w.logger.Warn().Str("key_name", "eodhd_api_key").Msg("EODHD API key is empty")
	}
	return apiKey
}

// GetType returns WorkerTypeMarketNews
func (w *MarketNewsWorker) GetType() models.WorkerType {
	return models.WorkerTypeMarketNews
}

// Init initializes the announcements worker
func (w *MarketNewsWorker) Init(ctx context.Context, step models.JobStep, jobDef models.JobDefinition) (*interfaces.WorkerInitResult, error) {
	stepConfig := step.Config
	if stepConfig == nil {
		return nil, fmt.Errorf("step config is required for announcements")
	}

	// Parse ticker
	ticker := parseTicker(stepConfig)
	if ticker.Code == "" {
		return nil, fmt.Errorf("ticker or asx_code is required in step config")
	}

	// For ASX, delegate to ASX worker
	if ticker.Exchange == "ASX" && w.marketAnnouncementsWorker != nil {
		w.logger.Info().
			Str("ticker", ticker.String()).
			Msg("Delegating to ASX announcements worker")
		return w.marketAnnouncementsWorker.Init(ctx, step, jobDef)
	}

	// Period for news (default Y1 = 12 months)
	period := "Y1"
	if p, ok := stepConfig["period"].(string); ok && p != "" {
		period = p
	}

	// Limit (default 50)
	limit := 50
	if l, ok := stepConfig["limit"].(float64); ok {
		limit = int(l)
	} else if l, ok := stepConfig["limit"].(int); ok {
		limit = l
	}

	w.logger.Info().
		Str("phase", "init").
		Str("step_name", step.Name).
		Str("ticker", ticker.String()).
		Str("period", period).
		Int("limit", limit).
		Msg("Announcements worker initialized")

	return &interfaces.WorkerInitResult{
		WorkItems: []interfaces.WorkItem{
			{
				ID:   ticker.String(),
				Name: fmt.Sprintf("Fetch %s announcements", ticker.String()),
				Type: "market_news",
				Config: map[string]interface{}{
					"ticker": ticker.String(),
					"period": period,
					"limit":  limit,
				},
			},
		},
		TotalCount:           1,
		Strategy:             interfaces.ProcessingStrategyInline,
		SuggestedConcurrency: 1,
		Metadata: map[string]interface{}{
			"ticker":      ticker.String(),
			"exchange":    ticker.Exchange,
			"code":        ticker.Code,
			"period":      period,
			"limit":       limit,
			"step_config": stepConfig,
		},
	}, nil
}

// CreateJobs fetches announcements and stores as documents
func (w *MarketNewsWorker) CreateJobs(ctx context.Context, step models.JobStep, jobDef models.JobDefinition, stepID string, initResult *interfaces.WorkerInitResult) (string, error) {
	if initResult == nil {
		var err error
		initResult, err = w.Init(ctx, step, jobDef)
		if err != nil {
			return "", fmt.Errorf("failed to initialize announcements worker: %w", err)
		}
	}

	tickerStr, _ := initResult.Metadata["ticker"].(string)
	ticker := common.ParseTicker(tickerStr)

	// For ASX, delegate to ASX worker
	if ticker.Exchange == "ASX" && w.marketAnnouncementsWorker != nil {
		w.logger.Info().
			Str("ticker", ticker.String()).
			Msg("Delegating to ASX announcements worker for execution")
		return w.marketAnnouncementsWorker.CreateJobs(ctx, step, jobDef, stepID, nil)
	}

	period, _ := initResult.Metadata["period"].(string)
	limit, _ := initResult.Metadata["limit"].(int)
	if limit == 0 {
		limit = 50
	}
	stepConfig, _ := initResult.Metadata["step_config"].(map[string]interface{})

	w.logger.Info().
		Str("phase", "run").
		Str("step_name", step.Name).
		Str("ticker", ticker.String()).
		Str("period", period).
		Int("limit", limit).
		Msg("Fetching announcements via EODHD News API")

	if w.jobMgr != nil {
		w.jobMgr.AddJobLog(ctx, stepID, "info", fmt.Sprintf("Fetching %s announcements from EODHD", ticker.String()))
	}

	// Fetch news from EODHD
	announcements, err := w.fetchAnnouncements(ctx, ticker, period, limit)
	if err != nil {
		w.logger.Error().Err(err).Str("ticker", ticker.String()).Msg("Failed to fetch announcements")
		if w.jobMgr != nil {
			w.jobMgr.AddJobLog(ctx, stepID, "error", fmt.Sprintf("Failed to fetch announcements: %v", err))
		}
		return "", fmt.Errorf("failed to fetch announcements: %w", err)
	}

	if len(announcements) == 0 {
		w.logger.Info().Str("ticker", ticker.String()).Msg("No announcements found")
		if w.jobMgr != nil {
			w.jobMgr.AddJobLog(ctx, stepID, "info", fmt.Sprintf("No announcements found for %s", ticker.String()))
		}
		return stepID, nil
	}

	// Extract output_tags
	var outputTags []string
	if stepConfig != nil {
		if tags, ok := stepConfig["output_tags"].([]interface{}); ok {
			for _, tag := range tags {
				if tagStr, ok := tag.(string); ok && tagStr != "" {
					outputTags = append(outputTags, tagStr)
				}
			}
		} else if tags, ok := stepConfig["output_tags"].([]string); ok {
			outputTags = tags
		}
	}

	// Create summary document
	doc := w.createAnnouncementsSummaryDoc(ctx, announcements, ticker, &jobDef, stepID, outputTags)
	if err := w.documentStorage.SaveDocument(doc); err != nil {
		w.logger.Warn().Err(err).Msg("Failed to save announcements summary")
		return "", fmt.Errorf("failed to save announcements summary: %w", err)
	}

	w.logger.Info().
		Str("ticker", ticker.String()).
		Int("count", len(announcements)).
		Msg("Announcements processed")

	if w.jobMgr != nil {
		w.jobMgr.AddJobLog(ctx, stepID, "info",
			fmt.Sprintf("%s - Found %d announcements", ticker.String(), len(announcements)))
	}

	return stepID, nil
}

// ReturnsChildJobs returns false
func (w *MarketNewsWorker) ReturnsChildJobs() bool {
	return false
}

// ValidateConfig validates step configuration
func (w *MarketNewsWorker) ValidateConfig(step models.JobStep) error {
	if step.Config == nil {
		return fmt.Errorf("announcements step requires config")
	}
	ticker := parseTicker(step.Config)
	if ticker.Code == "" {
		return fmt.Errorf("announcements step requires 'ticker' or 'asx_code' in config")
	}
	return nil
}

// fetchAnnouncements fetches news from EODHD
func (w *MarketNewsWorker) fetchAnnouncements(ctx context.Context, ticker common.Ticker, period string, limit int) ([]Announcement, error) {
	// Get API key from KV store
	apiKey := w.getEODHDAPIKey(ctx)
	if apiKey == "" {
		return nil, fmt.Errorf("EODHD API key 'eodhd_api_key' not configured in KV store")
	}

	// Create EODHD client with resolved API key
	eodhdClient := eodhd.NewClient(apiKey, eodhd.WithLogger(w.logger))

	// Calculate date range
	to := time.Now()
	var from time.Time
	switch period {
	case "M1":
		from = to.AddDate(0, -1, 0)
	case "M3":
		from = to.AddDate(0, -3, 0)
	case "M6":
		from = to.AddDate(0, -6, 0)
	case "Y1":
		from = to.AddDate(-1, 0, 0)
	case "Y2":
		from = to.AddDate(-2, 0, 0)
	default:
		from = to.AddDate(-1, 0, 0)
	}

	// Fetch from EODHD
	symbol := ticker.EODHDSymbol()
	news, err := eodhdClient.GetNews(ctx, []string{symbol},
		eodhd.WithDateRange(from, to),
		eodhd.WithLimit(limit),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch news: %w", err)
	}

	// Convert to Announcement
	var announcements []Announcement
	for _, item := range news {
		ann := Announcement{
			Date:     item.Date,
			Headline: item.Title,
			Content:  item.Content,
			Link:     item.Link,
			Source:   "EODHD",
			Symbols:  item.Symbols,
			Tags:     item.Tags,
		}
		if item.Sentiment != nil {
			ann.Sentiment = &AnnouncementSentiment{
				Polarity: item.Sentiment.Polarity,
				Positive: item.Sentiment.Pos,
				Neutral:  item.Sentiment.Neu,
				Negative: item.Sentiment.Neg,
			}
		}
		announcements = append(announcements, ann)
	}

	// Sort by date descending
	sort.Slice(announcements, func(i, j int) bool {
		return announcements[i].Date.After(announcements[j].Date)
	})

	return announcements, nil
}

// createAnnouncementsSummaryDoc creates a summary document
func (w *MarketNewsWorker) createAnnouncementsSummaryDoc(
	ctx context.Context,
	announcements []Announcement,
	ticker common.Ticker,
	jobDef *models.JobDefinition,
	parentJobID string,
	outputTags []string,
) *models.Document {
	var content strings.Builder

	content.WriteString(fmt.Sprintf("# %s Announcements Summary\n\n", ticker.String()))
	content.WriteString(fmt.Sprintf("**Total Announcements**: %d\n\n", len(announcements)))

	if len(announcements) > 0 {
		content.WriteString(fmt.Sprintf("**Date Range**: %s to %s\n\n",
			announcements[len(announcements)-1].Date.Format("2006-01-02"),
			announcements[0].Date.Format("2006-01-02")))
	}

	// Recent announcements (top 10)
	content.WriteString("## Recent Announcements\n\n")
	displayCount := 10
	if len(announcements) < displayCount {
		displayCount = len(announcements)
	}

	for i := 0; i < displayCount; i++ {
		ann := announcements[i]
		content.WriteString(fmt.Sprintf("### %s\n", ann.Headline))
		content.WriteString(fmt.Sprintf("**Date**: %s\n\n", ann.Date.Format("2006-01-02 15:04")))

		if ann.Sentiment != nil {
			sentiment := "Neutral"
			if ann.Sentiment.Polarity > 0.1 {
				sentiment = "Positive"
			} else if ann.Sentiment.Polarity < -0.1 {
				sentiment = "Negative"
			}
			content.WriteString(fmt.Sprintf("**Sentiment**: %s (%.2f)\n\n", sentiment, ann.Sentiment.Polarity))
		}

		// Truncate content if too long
		contentText := ann.Content
		if len(contentText) > 500 {
			contentText = contentText[:500] + "..."
		}
		content.WriteString(contentText + "\n\n")

		if ann.Link != "" {
			content.WriteString(fmt.Sprintf("[Read More](%s)\n\n", ann.Link))
		}

		content.WriteString("---\n\n")
	}

	// Sentiment summary
	if len(announcements) > 0 {
		var posCount, negCount, neuCount int
		for _, ann := range announcements {
			if ann.Sentiment != nil {
				if ann.Sentiment.Polarity > 0.1 {
					posCount++
				} else if ann.Sentiment.Polarity < -0.1 {
					negCount++
				} else {
					neuCount++
				}
			}
		}

		if posCount+negCount+neuCount > 0 {
			content.WriteString("## Sentiment Overview\n\n")
			content.WriteString("| Sentiment | Count | Percentage |\n")
			content.WriteString("|-----------|-------|------------|\n")
			total := posCount + negCount + neuCount
			content.WriteString(fmt.Sprintf("| Positive | %d | %.1f%% |\n", posCount, float64(posCount)/float64(total)*100))
			content.WriteString(fmt.Sprintf("| Neutral | %d | %.1f%% |\n", neuCount, float64(neuCount)/float64(total)*100))
			content.WriteString(fmt.Sprintf("| Negative | %d | %.1f%% |\n\n", negCount, float64(negCount)/float64(total)*100))
		}
	}

	// Build tags
	tags := []string{
		ticker.String(),
		strings.ToLower(ticker.Exchange),
		ticker.Code,
		"market_news",
		"news",
	}
	tags = append(tags, outputTags...)

	now := time.Now()
	sourceType := "announcements_summary"
	sourceID := ticker.SourceID("market_news")

	return &models.Document{
		ID:              uuid.New().String(),
		Title:           fmt.Sprintf("%s Announcements Summary", ticker.String()),
		ContentMarkdown: content.String(),
		DetailLevel:     models.DetailLevelFull,
		SourceType:      sourceType,
		SourceID:        sourceID,
		URL:             "",
		Tags:            tags,
		Metadata: map[string]interface{}{
			"ticker":             ticker.String(),
			"exchange":           ticker.Exchange,
			"code":               ticker.Code,
			"announcement_count": len(announcements),
			"source":             "eodhd",
			"job_id":             parentJobID,
		},
		CreatedAt:  now,
		UpdatedAt:  now,
		LastSynced: &now,
	}
}
