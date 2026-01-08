// -----------------------------------------------------------------------
// CDSWorker - Capital Discipline Score Calculator
// Computes CDS (0-2) from fundamentals and announcement data
// -----------------------------------------------------------------------

package rating

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/ternarybob/arbor"
	"github.com/ternarybob/quaero/internal/common"
	"github.com/ternarybob/quaero/internal/interfaces"
	"github.com/ternarybob/quaero/internal/models"
	"github.com/ternarybob/quaero/internal/queue"
	"github.com/ternarybob/quaero/internal/services/rating"
	"github.com/ternarybob/quaero/internal/workers/workerutil"
)

// CDSWorker calculates Capital Discipline Score for tickers.
// This worker executes synchronously (no child jobs).
type CDSWorker struct {
	documentStorage       interfaces.DocumentStorage
	fundamentalsProvider  interfaces.FundamentalsDataProvider
	announcementsProvider interfaces.AnnouncementDataProvider
	logger                arbor.ILogger
	jobMgr                *queue.Manager
}

// Compile-time assertion
var _ interfaces.DefinitionWorker = (*CDSWorker)(nil)

// NewCDSWorker creates a new CDS rating worker.
func NewCDSWorker(
	documentStorage interfaces.DocumentStorage,
	fundamentalsProvider interfaces.FundamentalsDataProvider,
	announcementsProvider interfaces.AnnouncementDataProvider,
	logger arbor.ILogger,
	jobMgr *queue.Manager,
) *CDSWorker {
	return &CDSWorker{
		documentStorage:       documentStorage,
		fundamentalsProvider:  fundamentalsProvider,
		announcementsProvider: announcementsProvider,
		logger:                logger,
		jobMgr:                jobMgr,
	}
}

// GetType returns the worker type.
func (w *CDSWorker) GetType() models.WorkerType {
	return models.WorkerTypeRatingCDS
}

// ReturnsChildJobs indicates this worker processes inline (no child jobs).
func (w *CDSWorker) ReturnsChildJobs() bool {
	return false
}

// ValidateConfig validates the step configuration.
func (w *CDSWorker) ValidateConfig(step models.JobStep) error {
	return nil
}

// Init prepares work items from configuration.
func (w *CDSWorker) Init(ctx context.Context, step models.JobStep, jobDef models.JobDefinition) (*interfaces.WorkerInitResult, error) {
	tickers := workerutil.CollectTickersWithJobDef(step.Config, jobDef)

	// Get analysis period (default 36 months)
	months := 36
	if m, ok := step.Config["months"].(float64); ok {
		months = int(m)
	} else if m, ok := step.Config["months"].(int); ok {
		months = m
	}

	workItems := make([]interfaces.WorkItem, len(tickers))
	for i, t := range tickers {
		workItems[i] = interfaces.WorkItem{
			ID:   t.String(),
			Name: fmt.Sprintf("Calculate CDS for %s", t.String()),
		}
	}

	return &interfaces.WorkerInitResult{
		WorkItems:  workItems,
		TotalCount: len(tickers),
		Strategy:   interfaces.ProcessingStrategyInline,
		Metadata: map[string]interface{}{
			"tickers": tickers,
			"months":  months,
		},
	}, nil
}

// CreateJobs processes tickers and calculates CDS scores.
func (w *CDSWorker) CreateJobs(ctx context.Context, step models.JobStep, jobDef models.JobDefinition, stepID string, initResult *interfaces.WorkerInitResult) (string, error) {
	if initResult == nil {
		var err error
		initResult, err = w.Init(ctx, step, jobDef)
		if err != nil {
			return "", fmt.Errorf("failed to initialize rating_cds worker: %w", err)
		}
	}

	tickers := initResult.Metadata["tickers"].([]common.Ticker)
	months := initResult.Metadata["months"].(int)

	var outputTags []string
	if tags, ok := step.Config["output_tags"].([]interface{}); ok {
		for _, tag := range tags {
			if tagStr, ok := tag.(string); ok && tagStr != "" {
				outputTags = append(outputTags, tagStr)
			}
		}
	}

	for _, ticker := range tickers {
		if err := w.processTickerCDS(ctx, ticker, months, outputTags); err != nil {
			w.logger.Error().Err(err).Str("ticker", ticker.String()).Msg("failed to calculate CDS")
		}
	}

	return stepID, nil
}

// processTickerCDS calculates CDS for a single ticker.
func (w *CDSWorker) processTickerCDS(ctx context.Context, ticker common.Ticker, months int, outputTags []string) error {
	// 1. Get fundamentals
	fundResult, err := w.fundamentalsProvider.GetFundamentals(ctx, ticker.String())
	if err != nil {
		return fmt.Errorf("fundamentals not available: %w", err)
	}

	// 2. Get announcements
	annResult, err := w.announcementsProvider.GetAnnouncements(ctx, ticker.String(), 500)
	if err != nil {
		return fmt.Errorf("announcements not available: %w", err)
	}

	// 3. Extract data
	fundamentals := w.extractFundamentals(fundResult.Document)
	announcements := w.extractAnnouncements(annResult.Document)

	// 4. Calculate CDS
	cdsResult := rating.CalculateCDS(fundamentals, announcements, months)

	// 5. Save result
	return w.saveResultDocument(ctx, ticker, cdsResult, outputTags)
}

// extractFundamentals extracts Fundamentals from document.
func (w *CDSWorker) extractFundamentals(doc *models.Document) rating.Fundamentals {
	if doc == nil {
		return rating.Fundamentals{}
	}
	m := doc.Metadata
	if m == nil {
		m = make(map[string]interface{})
	}

	var sharesOutstanding3YAgo *int64
	if v, ok := m["shares_outstanding_3y"].(float64); ok {
		val := int64(v)
		sharesOutstanding3YAgo = &val
	}

	return rating.Fundamentals{
		Ticker:                   workerutil.GetString(m, "ticker"),
		CompanyName:              workerutil.GetString(m, "company_name"),
		SharesOutstandingCurrent: workerutil.GetInt64(m, "shares_outstanding"),
		SharesOutstanding3YAgo:   sharesOutstanding3YAgo,
	}
}

// extractAnnouncements extracts announcements from document.
func (w *CDSWorker) extractAnnouncements(doc *models.Document) []rating.Announcement {
	if doc == nil || doc.Metadata == nil {
		return nil
	}

	annData, ok := doc.Metadata["announcements"].([]interface{})
	if !ok {
		return nil
	}

	var announcements []rating.Announcement
	for _, a := range annData {
		annMap, ok := a.(map[string]interface{})
		if !ok {
			continue
		}

		ann := rating.Announcement{
			Headline:         ratingGetString(annMap, "headline"),
			IsPriceSensitive: ratingGetBool(annMap, "is_price_sensitive"),
		}

		// Parse date
		if dateStr, ok := annMap["date"].(string); ok {
			if t, err := time.Parse(time.RFC3339, dateStr); err == nil {
				ann.Date = t
			} else if t, err := time.Parse("2006-01-02", dateStr); err == nil {
				ann.Date = t
			}
		}

		// Map announcement type
		ann.Type = mapAnnouncementType(ratingGetString(annMap, "type"))

		announcements = append(announcements, ann)
	}

	return announcements
}

// mapAnnouncementType maps string type to rating.AnnouncementType.
func mapAnnouncementType(typeStr string) rating.AnnouncementType {
	typeStr = strings.ToLower(typeStr)
	switch {
	case strings.Contains(typeStr, "trading halt"):
		return rating.TypeTradingHalt
	case strings.Contains(typeStr, "capital raise") || strings.Contains(typeStr, "placement"):
		return rating.TypeCapitalRaise
	case strings.Contains(typeStr, "quarterly") || strings.Contains(typeStr, "4c") || strings.Contains(typeStr, "appendix 4c"):
		return rating.TypeQuarterly
	case strings.Contains(typeStr, "annual") || strings.Contains(typeStr, "4e"):
		return rating.TypeAnnualReport
	case strings.Contains(typeStr, "drilling") || strings.Contains(typeStr, "exploration"):
		return rating.TypeDrilling
	case strings.Contains(typeStr, "acquisition") || strings.Contains(typeStr, "takeover"):
		return rating.TypeAcquisition
	case strings.Contains(typeStr, "contract") || strings.Contains(typeStr, "agreement"):
		return rating.TypeContract
	default:
		return rating.TypeOther
	}
}

// saveResultDocument saves CDS result as a document.
func (w *CDSWorker) saveResultDocument(ctx context.Context, ticker common.Ticker, result rating.CDSResult, outputTags []string) error {
	var content strings.Builder
	content.WriteString(fmt.Sprintf("# CDS Score: %s\n\n", ticker.Code))
	content.WriteString(fmt.Sprintf("**Score:** %d/2\n\n", result.Score))
	content.WriteString("## Components\n\n")
	content.WriteString("| Metric | Value |\n")
	content.WriteString("|--------|-------|\n")
	content.WriteString(fmt.Sprintf("| Shares CAGR | %.1f%% |\n", result.Components.SharesCAGR*100))
	content.WriteString(fmt.Sprintf("| Trading Halts/yr | %.1f |\n", result.Components.TradingHaltsPA))
	content.WriteString(fmt.Sprintf("| Capital Raises/yr | %.1f |\n", result.Components.CapitalRaisesPA))
	content.WriteString(fmt.Sprintf("| Analysis Period | %d months |\n", result.Components.AnalysisPeriodMo))
	content.WriteString(fmt.Sprintf("\n## Reasoning\n\n%s\n", result.Reasoning))

	tags := []string{"rating-cds", strings.ToLower(ticker.Code)}
	tags = append(tags, outputTags...)

	now := time.Now()
	doc := &models.Document{
		ID:              uuid.New().String(),
		Title:           fmt.Sprintf("CDS Score: %s", ticker.Code),
		ContentMarkdown: content.String(),
		DetailLevel:     models.DetailLevelFull,
		SourceType:      models.WorkerTypeRatingCDS.String(),
		SourceID:        ticker.SourceID("rating_cds"),
		Tags:            tags,
		Metadata: map[string]interface{}{
			"ticker":        ticker.String(),
			"score":         result.Score,
			"components":    result.Components,
			"reasoning":     result.Reasoning,
			"calculated_at": now.Format(time.RFC3339),
		},
		CreatedAt:  now,
		UpdatedAt:  now,
		LastSynced: &now,
	}

	return w.documentStorage.SaveDocument(doc)
}
