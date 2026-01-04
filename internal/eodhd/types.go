// Package eodhd provides a client for the EODHD (End of Day Historical Data) API.
// This package centralizes all EODHD API interactions for the application.
package eodhd

import (
	"fmt"
	"time"
)

// QueryOption represents an optional parameter for API queries.
type QueryOption func(*queryParams)

// queryParams holds optional query parameters.
type queryParams struct {
	From   time.Time
	To     time.Time
	Period string // d, w, m
	Order  string // a (asc), d (desc)
	Limit  int
}

// WithDateRange sets the date range for the query.
func WithDateRange(from, to time.Time) QueryOption {
	return func(p *queryParams) {
		p.From = from
		p.To = to
	}
}

// WithPeriod sets the period (d=daily, w=weekly, m=monthly).
func WithPeriod(period string) QueryOption {
	return func(p *queryParams) {
		p.Period = period
	}
}

// WithOrder sets the order (a=ascending, d=descending).
func WithOrder(order string) QueryOption {
	return func(p *queryParams) {
		p.Order = order
	}
}

// WithLimit sets the maximum number of results.
func WithLimit(limit int) QueryOption {
	return func(p *queryParams) {
		p.Limit = limit
	}
}

// APIError represents an error from the EODHD API.
type APIError struct {
	StatusCode int
	Message    string
	Endpoint   string
}

func (e *APIError) Error() string {
	return fmt.Sprintf("EODHD API error: %s (status: %d, endpoint: %s)", e.Message, e.StatusCode, e.Endpoint)
}

// RateLimitError represents a rate limit error.
type RateLimitError struct {
	RetryAfter time.Duration
}

func (e *RateLimitError) Error() string {
	return fmt.Sprintf("EODHD rate limit exceeded, retry after %v", e.RetryAfter)
}
