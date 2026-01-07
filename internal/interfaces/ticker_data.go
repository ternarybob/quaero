// Package interfaces provides service interfaces for dependency injection.
package interfaces

import (
	"context"
	"time"

	"github.com/ternarybob/quaero/internal/models"
)

// TickerDataStatus indicates the freshness/availability of cached ticker data.
type TickerDataStatus string

const (
	// TickerDataFresh indicates the data is available and not stale
	TickerDataFresh TickerDataStatus = "fresh"
	// TickerDataStale indicates data exists but is stale (was refreshed)
	TickerDataStale TickerDataStatus = "stale"
	// TickerDataGenerated indicates data was freshly generated (not from cache)
	TickerDataGenerated TickerDataStatus = "generated"
	// TickerDataError indicates an error occurred fetching/generating data
	TickerDataError TickerDataStatus = "error"
)

// TickerDataResult contains the result of a ticker data request.
// The requesting worker receives this result and extracts the data it needs.
type TickerDataResult struct {
	// Document is the underlying document containing the data
	Document *models.Document
	// Status indicates the data freshness/source
	Status TickerDataStatus
	// Ticker is the normalized ticker symbol
	Ticker string
	// LastUpdated is when the data was last updated
	LastUpdated time.Time
	// Error contains any error that occurred (when Status == TickerDataError)
	Error error
}

// PriceDataProvider provides historical price data for tickers.
// Implementations should first check for cached documents, then generate if needed.
// This interface enables separation of concerns between workers that need price data
// and the worker (MarketDataWorker) that fetches it.
type PriceDataProvider interface {
	// GetPriceData retrieves historical price data for a ticker.
	// If cached data exists and is fresh, returns it immediately.
	// If cached data is missing or stale, generates fresh data.
	// The period parameter controls how much history to return (e.g., "1y", "2y").
	GetPriceData(ctx context.Context, ticker string, period string) (*TickerDataResult, error)
}

// FundamentalsDataProvider provides fundamental data for tickers.
// Implementations should first check for cached documents, then generate if needed.
// This interface enables separation of concerns between workers that need fundamentals
// and the worker (MarketFundamentalsWorker) that fetches it.
type FundamentalsDataProvider interface {
	// GetFundamentals retrieves fundamental data (financials, earnings, etc.) for a ticker.
	// If cached data exists and is fresh, returns it immediately.
	// If cached data is missing or stale, generates fresh data.
	GetFundamentals(ctx context.Context, ticker string) (*TickerDataResult, error)
}

// AnnouncementDataProvider provides announcement data for tickers.
// This interface enables other workers to access ASX announcements data.
type AnnouncementDataProvider interface {
	// GetAnnouncements retrieves announcement data for a ticker.
	// If cached data exists and is fresh, returns it immediately.
	// If cached data is missing or stale, generates fresh data.
	GetAnnouncements(ctx context.Context, ticker string, limit int) (*TickerDataResult, error)
}

// TickerDataProviderFactory creates data providers for workers.
// This allows workers to be injected with the providers they need.
type TickerDataProviderFactory interface {
	// CreatePriceDataProvider creates a price data provider
	CreatePriceDataProvider() PriceDataProvider

	// CreateFundamentalsDataProvider creates a fundamentals data provider
	CreateFundamentalsDataProvider() FundamentalsDataProvider

	// CreateAnnouncementDataProvider creates an announcement data provider
	CreateAnnouncementDataProvider() AnnouncementDataProvider
}
