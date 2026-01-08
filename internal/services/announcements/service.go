// Package announcements provides services for fetching and classifying company announcements.
// It supports multiple exchange sources (ASX, NYSE, etc.) with a clean provider-based architecture.
package announcements

import (
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"strings"
	"time"

	"github.com/ternarybob/arbor"
	"github.com/ternarybob/quaero/internal/common"
	"github.com/ternarybob/quaero/internal/interfaces"
)

// AnnouncementProvider defines the interface for exchange-specific announcement fetching.
type AnnouncementProvider interface {
	// Name returns the provider name (e.g., "ASX", "EODHD")
	Name() string

	// SupportsExchange returns true if this provider can handle the given exchange
	SupportsExchange(exchange string) bool

	// FetchAnnouncements retrieves announcements for a ticker within the given period
	FetchAnnouncements(ctx context.Context, ticker common.Ticker, period string, limit int) ([]RawAnnouncement, error)
}

// Service manages announcement fetching across multiple exchange providers.
type Service struct {
	providers  []AnnouncementProvider
	logger     arbor.ILogger
	httpClient *http.Client
	kvStorage  interfaces.KeyValueStorage
}

// NewService creates a new announcement service with the given providers.
// Providers are tried in order - first provider that supports the exchange is used.
func NewService(logger arbor.ILogger, httpClient *http.Client, kvStorage interfaces.KeyValueStorage) *Service {
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 30 * time.Second}
	}

	s := &Service{
		providers:  make([]AnnouncementProvider, 0),
		logger:     logger,
		httpClient: httpClient,
		kvStorage:  kvStorage,
	}

	// Register default providers in priority order
	// ASX provider is primary for ASX exchange
	s.RegisterProvider(NewASXProvider(logger, httpClient))

	// EODHD provider is fallback for ASX and primary for other exchanges
	s.RegisterProvider(NewEODHDProvider(logger, httpClient, kvStorage))

	return s
}

// RegisterProvider adds a provider to the service.
// Providers are tried in the order they are registered.
func (s *Service) RegisterProvider(provider AnnouncementProvider) {
	s.providers = append(s.providers, provider)
	s.logger.Debug().
		Str("provider", provider.Name()).
		Msg("Registered announcement provider")
}

// FetchAnnouncements retrieves announcements for a ticker, routing to the appropriate provider.
func (s *Service) FetchAnnouncements(ctx context.Context, ticker common.Ticker, period string, limit int) ([]RawAnnouncement, error) {
	if ticker.Code == "" {
		return nil, fmt.Errorf("empty ticker code")
	}

	// Default period to 3 years (36 months rolling)
	if period == "" {
		period = "Y3"
	}

	// Default limit
	if limit <= 0 {
		limit = 1000
	}

	s.logger.Debug().
		Str("ticker", ticker.String()).
		Str("exchange", ticker.Exchange).
		Str("period", period).
		Int("limit", limit).
		Msg("Fetching announcements")

	// Try providers in order
	var lastErr error
	for _, provider := range s.providers {
		if !provider.SupportsExchange(ticker.Exchange) {
			continue
		}

		s.logger.Debug().
			Str("provider", provider.Name()).
			Str("ticker", ticker.String()).
			Msg("Trying announcement provider")

		anns, err := provider.FetchAnnouncements(ctx, ticker, period, limit)
		if err != nil {
			s.logger.Warn().
				Err(err).
				Str("provider", provider.Name()).
				Msg("Provider failed, trying next")
			lastErr = err
			continue
		}

		if len(anns) > 0 {
			s.logger.Info().
				Str("provider", provider.Name()).
				Str("ticker", ticker.String()).
				Int("count", len(anns)).
				Msg("Fetched announcements successfully")
			return anns, nil
		}

		s.logger.Debug().
			Str("provider", provider.Name()).
			Msg("Provider returned no announcements, trying next")
	}

	if lastErr != nil {
		return nil, fmt.Errorf("all providers failed: %w", lastErr)
	}

	return nil, fmt.Errorf("no provider supports exchange: %s", ticker.Exchange)
}

// CalculateCutoffDate calculates the cutoff date based on period string.
func CalculateCutoffDate(period string) time.Time {
	now := time.Now()
	switch period {
	case "M1":
		return now.AddDate(0, -1, 0)
	case "M3":
		return now.AddDate(0, -3, 0)
	case "M6":
		return now.AddDate(0, -6, 0)
	case "Y1":
		return now.AddDate(-1, 0, 0)
	case "Y2":
		return now.AddDate(-2, 0, 0)
	case "Y3":
		return now.AddDate(-3, 0, 0)
	case "Y5":
		return now.AddDate(-5, 0, 0)
	default:
		return now.AddDate(-3, 0, 0) // Default 3 years
	}
}

// DownloadAndStorePDFs downloads PDFs for high-impact announcements and stores them in KV storage.
// Returns the announcements with PDFStorageKey populated for successfully downloaded PDFs.
// Only downloads up to maxDownloads PDFs to avoid excessive requests.
func (s *Service) DownloadAndStorePDFs(ctx context.Context, announcements []RawAnnouncement, code string, maxDownloads int) []RawAnnouncement {
	if s.kvStorage == nil {
		s.logger.Debug().Msg("kvStorage is nil, skipping PDF downloads")
		return announcements
	}

	if maxDownloads <= 0 {
		maxDownloads = 10 // Default limit
	}

	downloadCount := 0
	for i := range announcements {
		if downloadCount >= maxDownloads {
			break
		}

		ann := &announcements[i]
		if ann.PDFURL == "" || ann.DocumentKey == "" {
			continue
		}

		// Create storage key
		storageKey := fmt.Sprintf("pdf:%s:%s", strings.ToLower(code), ann.DocumentKey)

		// Check if already downloaded
		existingPDF, err := s.kvStorage.Get(ctx, storageKey)
		if err == nil && existingPDF != "" {
			ann.PDFStorageKey = storageKey
			ann.PDFDownloaded = true
			s.logger.Debug().
				Str("storage_key", storageKey).
				Msg("PDF already in storage, skipping download")
			continue
		}

		// Download PDF
		pdfContent, err := s.downloadASXPDF(ctx, ann.PDFURL, ann.DocumentKey)
		if err != nil {
			s.logger.Warn().
				Err(err).
				Str("document_key", ann.DocumentKey).
				Str("headline", ann.Headline).
				Msg("Failed to download PDF")
			continue
		}

		// Store as base64 in KV storage
		base64Content := base64.StdEncoding.EncodeToString(pdfContent)
		description := fmt.Sprintf("ASX PDF: %s - %s", ann.Date.Format("2006-01-02"), ann.Headline)

		if err := s.kvStorage.Set(ctx, storageKey, base64Content, description); err != nil {
			s.logger.Warn().
				Err(err).
				Str("storage_key", storageKey).
				Msg("Failed to store PDF in KV storage")
			continue
		}

		ann.PDFStorageKey = storageKey
		ann.PDFDownloaded = true
		downloadCount++

		s.logger.Info().
			Str("storage_key", storageKey).
			Str("headline", ann.Headline).
			Int("size_bytes", len(pdfContent)).
			Msg("Downloaded and stored PDF")
	}

	s.logger.Info().
		Str("code", code).
		Int("downloaded", downloadCount).
		Int("total", len(announcements)).
		Msg("PDF download complete")

	return announcements
}

// downloadASXPDF downloads a PDF from ASX, handling their WAF by establishing a session first.
// ASX uses a Web Application Firewall (WAF) that requires:
// 1. First visiting the redirect/HTML page to get session cookies
// 2. Then requesting the actual PDF with those cookies
func (s *Service) downloadASXPDF(ctx context.Context, pdfURL, documentKey string) ([]byte, error) {
	if pdfURL == "" {
		return nil, fmt.Errorf("empty PDF URL")
	}

	// Create a cookie jar to store session data
	jar, err := cookiejar.New(nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create cookie jar: %w", err)
	}

	// Create a client with the cookie jar
	client := &http.Client{
		Jar:     jar,
		Timeout: 60 * time.Second,
	}

	// Step 1: Visit the redirect/HTML page first to get session cookies
	initReq, err := http.NewRequestWithContext(ctx, "GET", pdfURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create init request: %w", err)
	}

	// Set browser-like headers to bypass WAF
	initReq.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")
	initReq.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,image/webp,*/*;q=0.8")
	initReq.Header.Set("Accept-Language", "en-US,en;q=0.5")
	initReq.Header.Set("Referer", "https://www.asx.com.au/")

	initResp, err := client.Do(initReq)
	if err != nil {
		return nil, fmt.Errorf("failed to get session cookies: %w", err)
	}
	initResp.Body.Close()

	s.logger.Debug().
		Str("pdf_url", pdfURL).
		Int("init_status", initResp.StatusCode).
		Msg("ASX PDF: Got session cookies")

	// Step 2: Try the direct PDF URL using the same client (with cookies)
	directPDFURL := fmt.Sprintf("https://announcements.asx.com.au/asxpdf/%s.pdf", documentKey)

	pdfReq, err := http.NewRequestWithContext(ctx, "GET", directPDFURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create PDF request: %w", err)
	}

	// Set browser-like headers
	pdfReq.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")
	pdfReq.Header.Set("Accept", "application/pdf,*/*")
	pdfReq.Header.Set("Referer", "https://www.asx.com.au/")

	pdfResp, err := client.Do(pdfReq)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch PDF: %w", err)
	}
	defer pdfResp.Body.Close()

	// If direct URL fails, try the original URL as fallback
	if pdfResp.StatusCode != http.StatusOK {
		pdfResp.Body.Close()

		fallbackReq, err := http.NewRequestWithContext(ctx, "GET", pdfURL, nil)
		if err != nil {
			return nil, fmt.Errorf("failed to create fallback request: %w", err)
		}
		fallbackReq.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36")
		fallbackReq.Header.Set("Accept", "application/pdf,*/*")

		pdfResp, err = client.Do(fallbackReq)
		if err != nil {
			return nil, fmt.Errorf("failed to fetch PDF from fallback: %w", err)
		}
		defer pdfResp.Body.Close()
	}

	if pdfResp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("PDF download failed with status: %d", pdfResp.StatusCode)
	}

	pdfContent, err := io.ReadAll(pdfResp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read PDF content: %w", err)
	}

	s.logger.Debug().
		Str("document_key", documentKey).
		Int("size_bytes", len(pdfContent)).
		Msg("ASX PDF: Downloaded successfully")

	return pdfContent, nil
}
