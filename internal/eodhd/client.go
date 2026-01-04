package eodhd

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
	// DefaultBaseURL is the base URL for the EODHD API.
	DefaultBaseURL = "https://eodhd.com/api"

	// DefaultTimeout is the default HTTP timeout.
	DefaultTimeout = 30 * time.Second

	// DefaultRateLimit is the default rate limit (requests per second).
	DefaultRateLimit = 10
)

// Client is an EODHD API client.
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

// NewClient creates a new EODHD API client.
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

// get performs a GET request to the API.
func (c *Client) get(ctx context.Context, path string, params url.Values, result interface{}) error {
	// Wait for rate limiter
	if err := c.limiter.Wait(ctx); err != nil {
		return &RateLimitError{RetryAfter: time.Second}
	}

	// Add API token
	if params == nil {
		params = url.Values{}
	}
	params.Set("api_token", c.apiKey)
	params.Set("fmt", "json")

	// Build URL
	reqURL := fmt.Sprintf("%s%s?%s", c.baseURL, path, params.Encode())

	// Create request
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	// Log request
	if c.logger != nil {
		c.logger.Debug().
			Str("url", c.baseURL+path).
			Msg("EODHD API request")
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

// GetEOD retrieves end-of-day price data for a symbol.
// Symbol format: TICKER.EXCHANGE (e.g., "AAPL.US", "GNP.AU")
func (c *Client) GetEOD(ctx context.Context, symbol string, opts ...QueryOption) (EODResponse, error) {
	params := &queryParams{
		Period: "d",
		Order:  "a",
	}
	for _, opt := range opts {
		opt(params)
	}

	queryParams := url.Values{}
	if !params.From.IsZero() {
		queryParams.Set("from", params.From.Format("2006-01-02"))
	}
	if !params.To.IsZero() {
		queryParams.Set("to", params.To.Format("2006-01-02"))
	}
	if params.Period != "" {
		queryParams.Set("period", params.Period)
	}
	if params.Order != "" {
		queryParams.Set("order", params.Order)
	}

	var result EODResponse
	if err := c.get(ctx, "/eod/"+symbol, queryParams, &result); err != nil {
		return nil, err
	}

	// Parse dates
	for i := range result {
		if t, err := time.Parse("2006-01-02", result[i].DateStr); err == nil {
			result[i].Date = t
		}
	}

	return result, nil
}

// GetFundamentals retrieves fundamental data for a symbol.
func (c *Client) GetFundamentals(ctx context.Context, symbol string) (*FundamentalsResponse, error) {
	var result FundamentalsResponse
	if err := c.get(ctx, "/fundamentals/"+symbol, nil, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// GetDividends retrieves dividend data for a symbol.
func (c *Client) GetDividends(ctx context.Context, symbol string, opts ...QueryOption) (DividendsResponse, error) {
	params := &queryParams{}
	for _, opt := range opts {
		opt(params)
	}

	queryParams := url.Values{}
	if !params.From.IsZero() {
		queryParams.Set("from", params.From.Format("2006-01-02"))
	}
	if !params.To.IsZero() {
		queryParams.Set("to", params.To.Format("2006-01-02"))
	}

	var result DividendsResponse
	if err := c.get(ctx, "/div/"+symbol, queryParams, &result); err != nil {
		return nil, err
	}

	// Parse dates
	for i := range result {
		if t, err := time.Parse("2006-01-02", result[i].DateStr); err == nil {
			result[i].Date = t
		}
	}

	return result, nil
}

// GetSplits retrieves stock split data for a symbol.
func (c *Client) GetSplits(ctx context.Context, symbol string, opts ...QueryOption) (SplitsResponse, error) {
	params := &queryParams{}
	for _, opt := range opts {
		opt(params)
	}

	queryParams := url.Values{}
	if !params.From.IsZero() {
		queryParams.Set("from", params.From.Format("2006-01-02"))
	}
	if !params.To.IsZero() {
		queryParams.Set("to", params.To.Format("2006-01-02"))
	}

	var result SplitsResponse
	if err := c.get(ctx, "/splits/"+symbol, queryParams, &result); err != nil {
		return nil, err
	}

	// Parse dates
	for i := range result {
		if t, err := time.Parse("2006-01-02", result[i].DateStr); err == nil {
			result[i].Date = t
		}
	}

	return result, nil
}

// GetNews retrieves news for one or more symbols.
// Symbols should be in TICKER.EXCHANGE format, comma-separated.
func (c *Client) GetNews(ctx context.Context, symbols []string, opts ...QueryOption) (NewsResponse, error) {
	params := &queryParams{
		Limit: 50,
	}
	for _, opt := range opts {
		opt(params)
	}

	queryParams := url.Values{}
	queryParams.Set("s", strings.Join(symbols, ","))
	if params.Limit > 0 {
		queryParams.Set("limit", fmt.Sprintf("%d", params.Limit))
	}
	if !params.From.IsZero() {
		queryParams.Set("from", params.From.Format("2006-01-02"))
	}
	if !params.To.IsZero() {
		queryParams.Set("to", params.To.Format("2006-01-02"))
	}

	var result NewsResponse
	if err := c.get(ctx, "/news", queryParams, &result); err != nil {
		return nil, err
	}

	// Parse dates
	for i := range result {
		if t, err := time.Parse("2006-01-02 15:04:05", result[i].DateStr); err == nil {
			result[i].Date = t
		} else if t, err := time.Parse("2006-01-02", result[i].DateStr); err == nil {
			result[i].Date = t
		}
	}

	return result, nil
}

// GetExchangesList retrieves the list of available exchanges.
func (c *Client) GetExchangesList(ctx context.Context) (ExchangesResponse, error) {
	var result ExchangesResponse
	if err := c.get(ctx, "/exchanges-list", nil, &result); err != nil {
		return nil, err
	}
	return result, nil
}

// GetRealTimeQuote retrieves real-time quote for a symbol.
// Note: This may require a higher tier subscription.
func (c *Client) GetRealTimeQuote(ctx context.Context, symbol string) (*EODData, error) {
	var result EODData
	if err := c.get(ctx, "/real-time/"+symbol, nil, &result); err != nil {
		return nil, err
	}
	return &result, nil
}
