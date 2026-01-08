// -----------------------------------------------------------------------
// BaseMarketWorker - Base struct for market workers with document caching
// Provides cache-aware document retrieval with exchange-based staleness checking
// -----------------------------------------------------------------------

package market

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/ternarybob/arbor"
	"github.com/ternarybob/quaero/internal/common"
	"github.com/ternarybob/quaero/internal/interfaces"
	"github.com/ternarybob/quaero/internal/models"
	"github.com/ternarybob/quaero/internal/queue"
	"github.com/ternarybob/quaero/internal/services/exchange"
)

// DocumentStatus represents the cache status of a document
type DocumentStatus string

const (
	// DocumentStatusFresh indicates the document is available and not stale
	DocumentStatusFresh DocumentStatus = "fresh"
	// DocumentStatusStale indicates the document exists but is stale
	DocumentStatusStale DocumentStatus = "stale"
	// DocumentStatusMissing indicates no document was found
	DocumentStatusMissing DocumentStatus = "missing"
	// DocumentStatusPending indicates a refresh job has been queued
	DocumentStatusPending DocumentStatus = "pending"
)

// DocumentResult contains the result of a document retrieval with staleness info
type DocumentResult struct {
	// Document is the retrieved document (may be nil if missing)
	Document *models.Document
	// Status indicates the cache status
	Status DocumentStatus
	// IsStale indicates whether the document is stale (convenience field)
	IsStale bool
	// NextCheckTime is when to check for fresh data (from staleness checker)
	NextCheckTime time.Time
	// Reason provides a human-readable explanation
	Reason string
}

// BaseMarketWorker provides shared document caching functionality for market workers.
// Market workers can embed this struct to gain cache-aware document retrieval.
type BaseMarketWorker struct {
	documentStorage interfaces.DocumentStorage
	searchService   interfaces.SearchService
	exchangeService *exchange.Service
	kvStorage       interfaces.KeyValueStorage
	logger          arbor.ILogger
	jobMgr          *queue.Manager
	workerType      string
}

// NewBaseMarketWorker creates a new base market worker with caching capabilities.
// Parameters:
//   - documentStorage: For document persistence
//   - searchService: For tag-based document lookup
//   - exchangeService: For staleness checking (can be nil, will skip staleness checks)
//   - kvStorage: For KV storage access
//   - logger: For logging
//   - jobMgr: For job management (can be nil if not queueing refresh jobs)
//   - workerType: The worker type string (e.g., "market_fundamentals")
func NewBaseMarketWorker(
	documentStorage interfaces.DocumentStorage,
	searchService interfaces.SearchService,
	exchangeService *exchange.Service,
	kvStorage interfaces.KeyValueStorage,
	logger arbor.ILogger,
	jobMgr *queue.Manager,
	workerType string,
) *BaseMarketWorker {
	return &BaseMarketWorker{
		documentStorage: documentStorage,
		searchService:   searchService,
		exchangeService: exchangeService,
		kvStorage:       kvStorage,
		logger:          logger,
		jobMgr:          jobMgr,
		workerType:      workerType,
	}
}

// GetDocument retrieves a cached document for a ticker with staleness checking.
// Returns the document with status information:
//   - Fresh: Document exists and is not stale
//   - Stale: Document exists but is stale (may trigger refresh)
//   - Missing: No document found
//
// Parameters:
//   - ctx: Context for cancellation
//   - ticker: The ticker symbol (e.g., "CBA.AU" or "CBA")
func (b *BaseMarketWorker) GetDocument(ctx context.Context, ticker string) (*DocumentResult, error) {
	// Normalize ticker to uppercase
	ticker = strings.ToUpper(ticker)

	// Search for existing document by tags
	doc, err := b.findDocumentByTicker(ctx, ticker)
	if err != nil {
		return nil, fmt.Errorf("failed to search for document: %w", err)
	}

	// No document found
	if doc == nil {
		return &DocumentResult{
			Status: DocumentStatusMissing,
			Reason: fmt.Sprintf("no cached document found for ticker %s", ticker),
		}, nil
	}

	// Check staleness if exchange service is available
	if b.exchangeService != nil {
		stalenessResult, err := b.checkStaleness(ctx, ticker, doc)
		if err != nil {
			// Log warning but don't fail - return document with unknown staleness
			b.logger.Warn().Err(err).
				Str("ticker", ticker).
				Msg("Failed to check staleness, returning document as-is")

			return &DocumentResult{
				Document: doc,
				Status:   DocumentStatusFresh, // Assume fresh if we can't check
				Reason:   "staleness check failed, assuming fresh",
			}, nil
		}

		if stalenessResult.IsStale {
			return &DocumentResult{
				Document:      doc,
				Status:        DocumentStatusStale,
				IsStale:       true,
				NextCheckTime: stalenessResult.NextCheckTime,
				Reason:        stalenessResult.Reason,
			}, nil
		}

		return &DocumentResult{
			Document:      doc,
			Status:        DocumentStatusFresh,
			IsStale:       false,
			NextCheckTime: stalenessResult.NextCheckTime,
			Reason:        stalenessResult.Reason,
		}, nil
	}

	// No exchange service - return document without staleness info
	return &DocumentResult{
		Document: doc,
		Status:   DocumentStatusFresh,
		Reason:   "staleness checking not available",
	}, nil
}

// GetDocuments retrieves cached documents for multiple tickers with staleness checking.
// Returns a map of ticker to DocumentResult for efficient batch processing.
//
// Parameters:
//   - ctx: Context for cancellation
//   - tickers: List of ticker symbols
func (b *BaseMarketWorker) GetDocuments(ctx context.Context, tickers []string) (map[string]*DocumentResult, error) {
	results := make(map[string]*DocumentResult, len(tickers))

	for _, ticker := range tickers {
		result, err := b.GetDocument(ctx, ticker)
		if err != nil {
			// Log error but continue processing other tickers
			b.logger.Warn().Err(err).
				Str("ticker", ticker).
				Msg("Failed to get document for ticker")

			results[ticker] = &DocumentResult{
				Status: DocumentStatusMissing,
				Reason: fmt.Sprintf("error retrieving document: %v", err),
			}
			continue
		}
		results[ticker] = result
	}

	return results, nil
}

// findDocumentByTicker searches for a document by ticker and worker type tags.
// Returns nil if no document found.
func (b *BaseMarketWorker) findDocumentByTicker(ctx context.Context, ticker string) (*models.Document, error) {
	if b.searchService == nil {
		return nil, fmt.Errorf("search service not available")
	}

	tickerLower := strings.ToLower(ticker)
	opts := interfaces.SearchOptions{
		Tags:  []string{"ticker:" + tickerLower, "source_type:" + b.workerType},
		Limit: 1,
	}

	results, err := b.searchService.Search(ctx, "", opts)
	if err != nil {
		return nil, fmt.Errorf("search failed: %w", err)
	}

	if len(results) == 0 {
		return nil, nil
	}

	return results[0], nil
}

// checkStaleness checks if a document's data is stale based on exchange trading schedule.
func (b *BaseMarketWorker) checkStaleness(ctx context.Context, ticker string, doc *models.Document) (*common.StalenessResult, error) {
	// Extract document date from metadata
	docDate := doc.CreatedAt
	if dateStr, ok := doc.Metadata["date"].(string); ok {
		if parsed, err := time.Parse("2006-01-02", dateStr); err == nil {
			docDate = parsed
		}
	}
	if dateVal, ok := doc.Metadata["eod_date"].(string); ok {
		if parsed, err := time.Parse("2006-01-02", dateVal); err == nil {
			docDate = parsed
		}
	}

	// Use exchange service to check staleness
	result, err := b.exchangeService.IsTickerStale(ctx, ticker, docDate)
	if err != nil {
		return nil, fmt.Errorf("staleness check failed: %w", err)
	}

	return result, nil
}

// GenerateMarketTags creates consistent tags for market documents.
// Tags follow the schema:
//   - ticker:{code}.{exchange} (lowercase)
//   - exchange:{exchange_code} (lowercase)
//   - source_type:{worker_type}
//   - date:{yyyy-mm-dd}
//
// Parameters:
//   - ticker: The EODHD format ticker (e.g., "CBA.AU")
//   - workerType: The worker type string
//   - date: The document date
func GenerateMarketTags(ticker, workerType string, date time.Time) []string {
	parsed := common.ParseEODHDTicker(ticker)
	exchangeCode := strings.ToLower(parsed.DetailsExchangeCode())
	tickerLower := strings.ToLower(ticker)

	tags := []string{
		"ticker:" + tickerLower,
		"source_type:" + workerType,
		"date:" + date.Format("2006-01-02"),
	}

	// Only add exchange tag if we have a valid exchange code
	if exchangeCode != "" {
		tags = append(tags, "exchange:"+exchangeCode)
	}

	return tags
}

// NormalizeTickerForEODHD converts various ticker formats to EODHD format.
// Examples:
//   - "CBA" -> "CBA.AU" (assumes ASX)
//   - "ASX:CBA" -> "CBA.AU"
//   - "CBA.AU" -> "CBA.AU" (no change)
func NormalizeTickerForEODHD(ticker string) string {
	// Already in EODHD format
	if strings.Contains(ticker, ".") {
		return strings.ToUpper(ticker)
	}

	// Parse using common ticker parser
	parsed := common.ParseTicker(ticker)
	if parsed.Code == "" {
		return strings.ToUpper(ticker)
	}

	// Default to AU exchange if not specified
	exchange := parsed.Exchange
	if exchange == "" || exchange == "ASX" {
		exchange = "AU"
	}

	return strings.ToUpper(parsed.Code) + "." + strings.ToUpper(exchange)
}

// GetWorkerType returns the worker type string for this base worker
func (b *BaseMarketWorker) GetWorkerType() string {
	return b.workerType
}

// SetExchangeService sets the exchange service (useful for testing or deferred initialization)
func (b *BaseMarketWorker) SetExchangeService(svc *exchange.Service) {
	b.exchangeService = svc
}

// SetSearchService sets the search service (useful for testing or deferred initialization)
func (b *BaseMarketWorker) SetSearchService(svc interfaces.SearchService) {
	b.searchService = svc
}
