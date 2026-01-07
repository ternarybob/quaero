// -----------------------------------------------------------------------
// Ticker Data Providers - Implementations of ticker data provider interfaces
// These provide cached access to market data with on-demand generation
// -----------------------------------------------------------------------

package workers

import (
	"context"
	"fmt"
	"time"

	"github.com/ternarybob/arbor"
	"github.com/ternarybob/quaero/internal/common"
	"github.com/ternarybob/quaero/internal/interfaces"
	"github.com/ternarybob/quaero/internal/models"
)

// FundamentalsProvider implements FundamentalsDataProvider using MarketFundamentalsWorker.
// It first checks for cached documents, then generates fresh data if needed.
type FundamentalsProvider struct {
	documentStorage interfaces.DocumentStorage
	kvStorage       interfaces.KeyValueStorage
	logger          arbor.ILogger
	worker          *MarketFundamentalsWorker
	cacheHours      int
}

// Compile-time assertion
var _ interfaces.FundamentalsDataProvider = (*FundamentalsProvider)(nil)

// NewFundamentalsProvider creates a new FundamentalsProvider.
func NewFundamentalsProvider(
	documentStorage interfaces.DocumentStorage,
	kvStorage interfaces.KeyValueStorage,
	logger arbor.ILogger,
	debugEnabled bool,
) *FundamentalsProvider {
	worker := NewMarketFundamentalsWorker(documentStorage, kvStorage, logger, nil, debugEnabled)
	return &FundamentalsProvider{
		documentStorage: documentStorage,
		kvStorage:       kvStorage,
		logger:          logger,
		worker:          worker,
		cacheHours:      24, // Default 24 hour cache
	}
}

// SetCacheHours sets the cache duration in hours.
func (p *FundamentalsProvider) SetCacheHours(hours int) {
	p.cacheHours = hours
}

// GetFundamentals retrieves fundamental data for a ticker.
// Returns cached data if fresh, generates new data if missing/stale.
func (p *FundamentalsProvider) GetFundamentals(ctx context.Context, ticker string) (*interfaces.TickerDataResult, error) {
	parsed := common.ParseTicker(ticker)
	if parsed.Code == "" {
		return nil, fmt.Errorf("invalid ticker: %s", ticker)
	}

	sourceType := "market_fundamentals"
	sourceID := parsed.SourceID("stock_collector")

	// Check for cached document
	doc, err := p.documentStorage.GetDocumentBySource(sourceType, sourceID)
	if err == nil && doc != nil && p.isFresh(doc) {
		p.logger.Debug().
			Str("ticker", parsed.String()).
			Str("doc_id", doc.ID).
			Msg("Using cached fundamentals data")

		return &interfaces.TickerDataResult{
			Document:    doc,
			Status:      interfaces.TickerDataFresh,
			Ticker:      parsed.String(),
			LastUpdated: p.getDocumentDate(doc),
		}, nil
	}

	// Generate fresh data
	p.logger.Info().
		Str("ticker", parsed.String()).
		Msg("Generating fresh fundamentals data")

	// Use worker's processTicker directly (inline generation)
	docInfo, err := p.worker.processTicker(ctx, parsed, "2y", p.cacheHours, false, nil, "", nil)
	if err != nil {
		return &interfaces.TickerDataResult{
			Status: interfaces.TickerDataError,
			Ticker: parsed.String(),
			Error:  err,
		}, err
	}

	// Retrieve the generated document
	generatedDoc, err := p.documentStorage.GetDocument(docInfo.ID)
	if err != nil {
		return &interfaces.TickerDataResult{
			Status: interfaces.TickerDataError,
			Ticker: parsed.String(),
			Error:  err,
		}, err
	}

	return &interfaces.TickerDataResult{
		Document:    generatedDoc,
		Status:      interfaces.TickerDataGenerated,
		Ticker:      parsed.String(),
		LastUpdated: time.Now(),
	}, nil
}

// isFresh checks if a document is still fresh based on cache hours.
func (p *FundamentalsProvider) isFresh(doc *models.Document) bool {
	if doc == nil || doc.LastSynced == nil {
		return false
	}
	return time.Since(*doc.LastSynced) < time.Duration(p.cacheHours)*time.Hour
}

// getDocumentDate extracts the document date from metadata.
func (p *FundamentalsProvider) getDocumentDate(doc *models.Document) time.Time {
	if doc.LastSynced != nil {
		return *doc.LastSynced
	}
	return doc.CreatedAt
}

// PriceDataProvider implements PriceDataProvider using MarketDataWorker.
// It first checks for cached documents, then generates fresh data if needed.
type PriceProvider struct {
	documentStorage interfaces.DocumentStorage
	kvStorage       interfaces.KeyValueStorage
	logger          arbor.ILogger
	worker          *MarketDataWorker
	cacheHours      int
}

// Compile-time assertion
var _ interfaces.PriceDataProvider = (*PriceProvider)(nil)

// NewPriceProvider creates a new PriceProvider.
func NewPriceProvider(
	documentStorage interfaces.DocumentStorage,
	kvStorage interfaces.KeyValueStorage,
	logger arbor.ILogger,
) *PriceProvider {
	worker := NewMarketDataWorker(documentStorage, kvStorage, logger, nil)
	return &PriceProvider{
		documentStorage: documentStorage,
		kvStorage:       kvStorage,
		logger:          logger,
		worker:          worker,
		cacheHours:      24, // Default 24 hour cache
	}
}

// SetCacheHours sets the cache duration in hours.
func (p *PriceProvider) SetCacheHours(hours int) {
	p.cacheHours = hours
}

// GetPriceData retrieves price data for a ticker.
// Returns cached data if fresh, generates new data if missing/stale.
func (p *PriceProvider) GetPriceData(ctx context.Context, ticker string, period string) (*interfaces.TickerDataResult, error) {
	parsed := common.ParseTicker(ticker)
	if parsed.Code == "" {
		return nil, fmt.Errorf("invalid ticker: %s", ticker)
	}

	sourceType := models.WorkerTypeMarketData.String()
	sourceID := parsed.SourceID("market_data")

	// Check for cached document
	doc, err := p.documentStorage.GetDocumentBySource(sourceType, sourceID)
	if err == nil && doc != nil && p.isFresh(doc) {
		p.logger.Debug().
			Str("ticker", parsed.String()).
			Str("doc_id", doc.ID).
			Msg("Using cached price data")

		return &interfaces.TickerDataResult{
			Document:    doc,
			Status:      interfaces.TickerDataFresh,
			Ticker:      parsed.String(),
			LastUpdated: p.getDocumentDate(doc),
		}, nil
	}

	// Generate fresh data using worker's fetchMarketData
	p.logger.Info().
		Str("ticker", parsed.String()).
		Str("period", period).
		Msg("Generating fresh price data")

	// Create a synthetic step for the worker
	step := models.JobStep{
		Name: "price-data-provider",
		Type: models.WorkerTypeMarketData,
		Config: map[string]interface{}{
			"ticker": parsed.String(),
			"period": period,
		},
	}
	jobDef := models.JobDefinition{
		ID:   "ticker-data-provider",
		Name: "Ticker Data Provider",
	}

	// Call the worker's CreateJobs method
	_, err = p.worker.CreateJobs(ctx, step, jobDef, "", nil)
	if err != nil {
		return &interfaces.TickerDataResult{
			Status: interfaces.TickerDataError,
			Ticker: parsed.String(),
			Error:  err,
		}, err
	}

	// Retrieve the generated document
	generatedDoc, err := p.documentStorage.GetDocumentBySource(sourceType, sourceID)
	if err != nil || generatedDoc == nil {
		return &interfaces.TickerDataResult{
			Status: interfaces.TickerDataError,
			Ticker: parsed.String(),
			Error:  fmt.Errorf("failed to retrieve generated document: %w", err),
		}, err
	}

	return &interfaces.TickerDataResult{
		Document:    generatedDoc,
		Status:      interfaces.TickerDataGenerated,
		Ticker:      parsed.String(),
		LastUpdated: time.Now(),
	}, nil
}

// isFresh checks if a document is still fresh based on cache hours.
func (p *PriceProvider) isFresh(doc *models.Document) bool {
	if doc == nil || doc.LastSynced == nil {
		return false
	}
	return time.Since(*doc.LastSynced) < time.Duration(p.cacheHours)*time.Hour
}

// getDocumentDate extracts the document date from metadata.
func (p *PriceProvider) getDocumentDate(doc *models.Document) time.Time {
	if doc.LastSynced != nil {
		return *doc.LastSynced
	}
	return doc.CreatedAt
}
