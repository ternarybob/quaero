// Package announcements provides services for fetching and classifying company announcements.
package announcements

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/ternarybob/arbor"
	"github.com/ternarybob/quaero/internal/common"
	"github.com/ternarybob/quaero/internal/eodhd"
	"github.com/ternarybob/quaero/internal/interfaces"
)

// EODHDProvider fetches announcements from EODHD API.
// Supports multiple exchanges using EODHD's fundamental data (dividends, earnings).
type EODHDProvider struct {
	logger     arbor.ILogger
	httpClient *http.Client
	kvStorage  interfaces.KeyValueStorage
}

// NewEODHDProvider creates a new EODHD announcement provider.
func NewEODHDProvider(logger arbor.ILogger, httpClient *http.Client, kvStorage interfaces.KeyValueStorage) *EODHDProvider {
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 30 * time.Second}
	}
	return &EODHDProvider{
		logger:     logger,
		httpClient: httpClient,
		kvStorage:  kvStorage,
	}
}

// Name returns the provider name.
func (p *EODHDProvider) Name() string {
	return "EODHD"
}

// SupportsExchange returns true for all exchanges (EODHD is the fallback provider).
func (p *EODHDProvider) SupportsExchange(exchange string) bool {
	// EODHD supports most major exchanges
	supportedExchanges := []string{
		"ASX", "AU", // Australia
		"NYSE", "NASDAQ", "US", "AMEX", // United States
		"LSE",       // London
		"TSX", "TO", // Toronto
		"XETRA", // Frankfurt
		"HK",    // Hong Kong
		"SG",    // Singapore
	}

	exchange = strings.ToUpper(exchange)
	for _, ex := range supportedExchanges {
		if exchange == ex {
			return true
		}
	}
	return false
}

// FetchAnnouncements retrieves announcements using EODHD's fundamental data APIs.
// Combines dividend history, earnings dates, and news for a comprehensive view.
func (p *EODHDProvider) FetchAnnouncements(ctx context.Context, ticker common.Ticker, period string, limit int) ([]RawAnnouncement, error) {
	apiKey := p.getAPIKey(ctx)
	if apiKey == "" {
		return nil, fmt.Errorf("EODHD API key 'eodhd_api_key' not configured in KV store")
	}

	// Calculate date range
	to := time.Now()
	from := CalculateCutoffDate(period)

	// Create EODHD client
	client := eodhd.NewClient(apiKey, eodhd.WithHTTPClient(p.httpClient))
	symbol := ticker.EODHDSymbol()

	p.logger.Debug().
		Str("symbol", symbol).
		Str("from", from.Format("2006-01-02")).
		Str("to", to.Format("2006-01-02")).
		Msg("Fetching announcements from EODHD")

	var allAnns []RawAnnouncement

	// Fetch dividends (historical corporate actions)
	divAnns, err := p.fetchDividends(ctx, client, symbol, from, to)
	if err != nil {
		p.logger.Warn().Err(err).Msg("Failed to fetch dividends from EODHD")
	} else {
		allAnns = append(allAnns, divAnns...)
	}

	// Fetch news (if no dividend data)
	if len(allAnns) == 0 {
		newsAnns, err := p.fetchNews(ctx, client, symbol, from, to, limit)
		if err != nil {
			p.logger.Warn().Err(err).Msg("Failed to fetch news from EODHD")
		} else {
			allAnns = append(allAnns, newsAnns...)
		}
	}

	// Apply limit
	if limit > 0 && len(allAnns) > limit {
		allAnns = allAnns[:limit]
	}

	return allAnns, nil
}

// getAPIKey retrieves EODHD API key from KV storage.
func (p *EODHDProvider) getAPIKey(ctx context.Context) string {
	if p.kvStorage == nil {
		return ""
	}
	key, err := p.kvStorage.Get(ctx, "eodhd_api_key")
	if err != nil || key == "" {
		return ""
	}
	return key
}

// fetchDividends retrieves dividend history and converts to announcements.
func (p *EODHDProvider) fetchDividends(ctx context.Context, client *eodhd.Client, symbol string, from, to time.Time) ([]RawAnnouncement, error) {
	divs, err := client.GetDividends(ctx, symbol,
		eodhd.WithDateRange(from, to),
	)
	if err != nil {
		return nil, err
	}

	var anns []RawAnnouncement
	for _, div := range divs {
		headline := fmt.Sprintf("Dividend: $%.4f per share", div.Value)
		if div.UnadjustedValue > 0 && div.UnadjustedValue != div.Value {
			headline = fmt.Sprintf("Dividend: $%.4f per share (unadjusted: $%.4f)", div.Value, div.UnadjustedValue)
		}

		anns = append(anns, RawAnnouncement{
			Date:           div.Date,
			Headline:       headline,
			Type:           "Dividend",
			PriceSensitive: true,
		})
	}

	return anns, nil
}

// fetchNews retrieves news and converts to announcements.
func (p *EODHDProvider) fetchNews(ctx context.Context, client *eodhd.Client, symbol string, from, to time.Time, limit int) ([]RawAnnouncement, error) {
	news, err := client.GetNews(ctx, []string{symbol},
		eodhd.WithDateRange(from, to),
		eodhd.WithLimit(limit),
	)
	if err != nil {
		return nil, err
	}

	var anns []RawAnnouncement
	for _, item := range news {
		anns = append(anns, RawAnnouncement{
			Date:           item.Date,
			Headline:       item.Title,
			Type:           "NEWS",
			PDFURL:         item.Link,
			PriceSensitive: false,
		})
	}

	return anns, nil
}
