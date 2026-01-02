// Package common provides shared utilities across the application.
package common

import (
	"strings"
)

// Ticker represents a parsed exchange-qualified ticker.
// Format: EXCHANGE:CODE (e.g., "ASX:GNP", "NYSE:AAPL")
type Ticker struct {
	// Exchange is the exchange code (e.g., "ASX", "NYSE", "NASDAQ")
	Exchange string
	// Code is the stock/security code (e.g., "GNP", "AAPL")
	Code string
	// Raw is the original ticker string
	Raw string
}

// ExchangeToSuffix maps exchange codes to EODHD API suffixes.
var ExchangeToSuffix = map[string]string{
	"ASX":    ".AU",
	"NYSE":   ".US",
	"NASDAQ": ".US",
	"LSE":    ".LSE",
	"TSX":    ".TO",
	"XETRA":  ".XETRA",
}

// ParseTicker parses an exchange-qualified ticker string.
// Supports formats:
//   - "ASX:GNP" -> Exchange="ASX", Code="GNP"
//   - "GNP" -> Exchange="ASX" (default), Code="GNP"
//   - "gnp" -> Exchange="ASX", Code="GNP" (normalized to uppercase)
func ParseTicker(ticker string) Ticker {
	ticker = strings.TrimSpace(ticker)
	if ticker == "" {
		return Ticker{}
	}

	// Check for exchange prefix
	if idx := strings.Index(ticker, ":"); idx > 0 {
		exchange := strings.ToUpper(ticker[:idx])
		code := strings.ToUpper(ticker[idx+1:])
		return Ticker{
			Exchange: exchange,
			Code:     code,
			Raw:      ticker,
		}
	}

	// No exchange prefix - default to ASX
	return Ticker{
		Exchange: "ASX",
		Code:     strings.ToUpper(ticker),
		Raw:      ticker,
	}
}

// String returns the full exchange-qualified ticker string.
func (t Ticker) String() string {
	if t.Exchange == "" || t.Code == "" {
		return t.Code
	}
	return t.Exchange + ":" + t.Code
}

// EODHDSymbol returns the EODHD API symbol format.
// Example: "ASX:GNP" -> "GNP.AU"
func (t Ticker) EODHDSymbol() string {
	if t.Code == "" {
		return ""
	}
	suffix, ok := ExchangeToSuffix[t.Exchange]
	if !ok {
		// Default to AU for unknown exchanges
		suffix = ".AU"
	}
	return t.Code + suffix
}

// SourceID returns a standardized source identifier for document storage.
// Example: "ASX:GNP" -> "asx:GNP"
func (t Ticker) SourceID(suffix string) string {
	if t.Code == "" {
		return ""
	}
	exchange := strings.ToLower(t.Exchange)
	if exchange == "" {
		exchange = "asx"
	}
	if suffix != "" {
		return exchange + ":" + t.Code + ":" + suffix
	}
	return exchange + ":" + t.Code
}

// ParseTickers parses a list of ticker strings.
func ParseTickers(tickers []string) []Ticker {
	result := make([]Ticker, 0, len(tickers))
	for _, t := range tickers {
		if parsed := ParseTicker(t); parsed.Code != "" {
			result = append(result, parsed)
		}
	}
	return result
}

// ParseTickersFromInterface parses tickers from interface{} (for TOML config).
func ParseTickersFromInterface(value interface{}) []Ticker {
	var result []Ticker

	switch v := value.(type) {
	case string:
		// Single ticker as string
		if parsed := ParseTicker(v); parsed.Code != "" {
			result = append(result, parsed)
		}
	case []string:
		// List of strings
		for _, s := range v {
			if parsed := ParseTicker(s); parsed.Code != "" {
				result = append(result, parsed)
			}
		}
	case []interface{}:
		// List from TOML/JSON
		for _, item := range v {
			if s, ok := item.(string); ok {
				if parsed := ParseTicker(s); parsed.Code != "" {
					result = append(result, parsed)
				}
			}
		}
	}

	return result
}
