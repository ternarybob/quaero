// Package navexa provides a client for the Navexa portfolio API.
package navexa

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/ternarybob/arbor"
	"golang.org/x/time/rate"
)

const (
	// DefaultBaseURL is the base URL for the Navexa API.
	DefaultBaseURL = "https://api.navexa.com.au"

	// DefaultTimeout is the default HTTP timeout.
	DefaultTimeout = 30 * time.Second

	// DefaultRateLimit is the default rate limit (requests per second).
	DefaultRateLimit = 5
)

// Client is a Navexa API client.
type Client struct {
	baseURL    string
	apiKey     string
	httpClient *http.Client
	logger     arbor.ILogger
	limiter    *rate.Limiter
}

// ClientOption configures the Client.
type ClientOption func(*Client)

// WithBaseURL sets a custom base URL.
func WithBaseURL(baseURL string) ClientOption {
	return func(c *Client) {
		c.baseURL = baseURL
	}
}

// WithHTTPClient sets a custom HTTP client.
func WithHTTPClient(httpClient *http.Client) ClientOption {
	return func(c *Client) {
		c.httpClient = httpClient
	}
}

// WithLogger sets a logger.
func WithLogger(logger arbor.ILogger) ClientOption {
	return func(c *Client) {
		c.logger = logger
	}
}

// WithRateLimit sets a custom rate limit.
func WithRateLimit(requestsPerSecond int) ClientOption {
	return func(c *Client) {
		c.limiter = rate.NewLimiter(rate.Limit(requestsPerSecond), requestsPerSecond)
	}
}

// NewClient creates a new Navexa API client.
func NewClient(apiKey string, opts ...ClientOption) *Client {
	c := &Client{
		baseURL: DefaultBaseURL,
		apiKey:  apiKey,
		httpClient: &http.Client{
			Timeout: DefaultTimeout,
		},
		limiter: rate.NewLimiter(rate.Limit(DefaultRateLimit), DefaultRateLimit),
	}

	for _, opt := range opts {
		opt(c)
	}

	return c
}

// APIError represents an error from the Navexa API.
type APIError struct {
	StatusCode int
	Message    string
	Endpoint   string
}

func (e *APIError) Error() string {
	return fmt.Sprintf("navexa API error: %s (status %d, endpoint: %s)", e.Message, e.StatusCode, e.Endpoint)
}

// get performs a GET request to the API.
func (c *Client) get(ctx context.Context, path string, params url.Values, result interface{}) error {
	// Wait for rate limiter
	if err := c.limiter.Wait(ctx); err != nil {
		return fmt.Errorf("rate limit exceeded: %w", err)
	}

	// Build URL
	reqURL := fmt.Sprintf("%s%s", c.baseURL, path)
	if params != nil && len(params) > 0 {
		reqURL = fmt.Sprintf("%s?%s", reqURL, params.Encode())
	}

	// Create request
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	// Set headers
	req.Header.Set("x-api-key", c.apiKey)
	req.Header.Set("Accept", "application/json")

	// Log request
	if c.logger != nil {
		c.logger.Debug().
			Str("url", c.baseURL+path).
			Msg("Navexa API request")
	}

	// Execute request
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	// Check status
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return &APIError{
			StatusCode: resp.StatusCode,
			Message:    string(body),
			Endpoint:   path,
		}
	}

	// Parse response
	if err := json.NewDecoder(resp.Body).Decode(result); err != nil {
		return fmt.Errorf("failed to decode response: %w", err)
	}

	return nil
}

// GetPortfolios retrieves all portfolios for the authenticated user.
func (c *Client) GetPortfolios(ctx context.Context) ([]Portfolio, error) {
	var portfolios []Portfolio
	if err := c.get(ctx, "/v1/portfolios", nil, &portfolios); err != nil {
		return nil, fmt.Errorf("failed to fetch portfolios: %w", err)
	}
	return portfolios, nil
}

// GetPortfolioByName finds a portfolio by name (case-insensitive contains match).
func (c *Client) GetPortfolioByName(ctx context.Context, name string) (*Portfolio, error) {
	portfolios, err := c.GetPortfolios(ctx)
	if err != nil {
		return nil, err
	}

	nameUpper := strings.ToUpper(name)
	for i := range portfolios {
		if strings.Contains(strings.ToUpper(portfolios[i].Name), nameUpper) {
			return &portfolios[i], nil
		}
	}

	return nil, fmt.Errorf("no portfolio found matching name '%s'", name)
}

// GetHoldings retrieves holdings for a portfolio.
func (c *Client) GetHoldings(ctx context.Context, portfolioID int) ([]Holding, error) {
	path := fmt.Sprintf("/v1/portfolios/%d/holdings", portfolioID)

	var holdings []Holding
	if err := c.get(ctx, path, nil, &holdings); err != nil {
		return nil, fmt.Errorf("failed to fetch holdings for portfolio %d: %w", portfolioID, err)
	}
	return holdings, nil
}

// GetPerformance retrieves performance data for a portfolio.
func (c *Client) GetPerformance(ctx context.Context, portfolioID int, fromDate, toDate string) (*PerformanceResponse, error) {
	path := fmt.Sprintf("/v1/portfolios/%d/performance", portfolioID)

	params := url.Values{}
	params.Set("from", fromDate)
	params.Set("to", toDate)
	params.Set("isPortfolioGroup", "false")
	params.Set("groupBy", "holding")
	params.Set("showLocalCurrency", "false")

	var perf PerformanceResponse
	if err := c.get(ctx, path, params, &perf); err != nil {
		return nil, fmt.Errorf("failed to fetch performance for portfolio %d: %w", portfolioID, err)
	}
	return &perf, nil
}

// GetPortfolioWithHoldings fetches a portfolio by name and enriches holdings with performance data.
// This is a convenience method that combines GetPortfolioByName and GetPerformance.
func (c *Client) GetPortfolioWithHoldings(ctx context.Context, portfolioName string) (*PortfolioWithHoldings, error) {
	// Find portfolio by name
	portfolio, err := c.GetPortfolioByName(ctx, portfolioName)
	if err != nil {
		return nil, err
	}

	// Get performance data with holdings
	// Use portfolio creation date as from, today as to
	fromDate := portfolio.DateCreated
	if len(fromDate) > 10 {
		fromDate = fromDate[:10] // Extract YYYY-MM-DD from datetime
	}
	toDate := time.Now().Format("2006-01-02")

	perf, err := c.GetPerformance(ctx, portfolio.ID, fromDate, toDate)
	if err != nil {
		return nil, err
	}

	// Convert to enriched holdings
	holdings := make([]EnrichedHolding, len(perf.Holdings))
	for i, h := range perf.Holdings {
		var avgBuyPrice float64
		if h.TotalQuantity > 0 {
			avgBuyPrice = h.TotalReturn.TotalValue / h.TotalQuantity
		}

		holdings[i] = EnrichedHolding{
			Symbol:        h.Symbol,
			Name:          h.Name,
			Exchange:      h.Exchange,
			Quantity:      h.TotalQuantity,
			AvgBuyPrice:   avgBuyPrice,
			CurrentValue:  h.TotalReturn.TotalValue,
			HoldingWeight: h.HoldingWeight,
			CurrencyCode:  h.CurrencyCode,
		}
	}

	return &PortfolioWithHoldings{
		Portfolio: *portfolio,
		Holdings:  holdings,
		FetchedAt: time.Now().Format(time.RFC3339),
	}, nil
}

// GetPortfolioByIDWithHoldings fetches a portfolio by ID and enriches holdings with performance data.
func (c *Client) GetPortfolioByIDWithHoldings(ctx context.Context, portfolioID int) (*PortfolioWithHoldings, error) {
	// First get all portfolios to find the one with this ID
	portfolios, err := c.GetPortfolios(ctx)
	if err != nil {
		return nil, err
	}

	var portfolio *Portfolio
	for i := range portfolios {
		if portfolios[i].ID == portfolioID {
			portfolio = &portfolios[i]
			break
		}
	}
	if portfolio == nil {
		return nil, fmt.Errorf("portfolio with ID %d not found", portfolioID)
	}

	// Get performance data with holdings
	fromDate := portfolio.DateCreated
	if len(fromDate) > 10 {
		fromDate = fromDate[:10]
	}
	toDate := time.Now().Format("2006-01-02")

	perf, err := c.GetPerformance(ctx, portfolio.ID, fromDate, toDate)
	if err != nil {
		return nil, err
	}

	// Convert to enriched holdings
	holdings := make([]EnrichedHolding, len(perf.Holdings))
	for i, h := range perf.Holdings {
		var avgBuyPrice float64
		if h.TotalQuantity > 0 {
			avgBuyPrice = h.TotalReturn.TotalValue / h.TotalQuantity
		}

		holdings[i] = EnrichedHolding{
			Symbol:        h.Symbol,
			Name:          h.Name,
			Exchange:      h.Exchange,
			Quantity:      h.TotalQuantity,
			AvgBuyPrice:   avgBuyPrice,
			CurrentValue:  h.TotalReturn.TotalValue,
			HoldingWeight: h.HoldingWeight,
			CurrencyCode:  h.CurrencyCode,
		}
	}

	return &PortfolioWithHoldings{
		Portfolio: *portfolio,
		Holdings:  holdings,
		FetchedAt: time.Now().Format(time.RFC3339),
	}, nil
}
