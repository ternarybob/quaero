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
	"INDX":   ".INDX", // For indices like AXJO (ASX 200)
}

// IndexCodeToEODHD maps common benchmark index codes to EODHD index symbols.
// These are used with the INDX exchange, e.g., "XJO" -> "AXJO" -> "AXJO.INDX"
var IndexCodeToEODHD = map[string]string{
	"XJO": "AXJO", // S&P/ASX 200
	"XSO": "AXSO", // S&P/ASX Small Ordinaries
	"XAO": "AORD", // All Ordinaries
}

// DefaultExchange is the default exchange used when parsing tickers without an exchange prefix.
// Can be overridden via [markets] default config in TOML.
var DefaultExchange = "ASX"

// SetDefaultExchange sets the default exchange for parsing tickers.
// Called during app initialization from config.
func SetDefaultExchange(exchange string) {
	if exchange != "" {
		DefaultExchange = strings.ToUpper(exchange)
	}
}

// ParseTicker parses an exchange-qualified ticker string.
// Supports formats:
//   - "ASX:GNP" -> Exchange="ASX", Code="GNP" (colon separator)
//   - "ASX.GNP" -> Exchange="ASX", Code="GNP" (dot separator)
//   - "GNP" -> Exchange=DefaultExchange (default), Code="GNP"
//   - "gnp" -> Exchange=DefaultExchange, Code="GNP" (normalized to uppercase)
//
// Note: EODHD uses CODE.EXCHANGE (e.g., "GNP.AU"), while our format uses EXCHANGE.CODE.
// Use EODHDSymbol() to convert to EODHD format.
func ParseTicker(ticker string) Ticker {
	ticker = strings.TrimSpace(ticker)
	if ticker == "" {
		return Ticker{}
	}

	// Check for exchange prefix with colon separator (EXCHANGE:CODE)
	if idx := strings.Index(ticker, ":"); idx > 0 {
		exchange := strings.ToUpper(ticker[:idx])
		code := strings.ToUpper(ticker[idx+1:])
		return Ticker{
			Exchange: exchange,
			Code:     code,
			Raw:      ticker,
		}
	}

	// Check for exchange prefix with dot separator (EXCHANGE.CODE)
	// Only match if the prefix is a known exchange to avoid conflicts with codes containing dots
	if idx := strings.Index(ticker, "."); idx > 0 {
		possibleExchange := strings.ToUpper(ticker[:idx])
		// Check if this is a known exchange
		if _, ok := ExchangeToSuffix[possibleExchange]; ok {
			code := strings.ToUpper(ticker[idx+1:])
			return Ticker{
				Exchange: possibleExchange,
				Code:     code,
				Raw:      ticker,
			}
		}
	}

	// No exchange prefix - use default exchange
	return Ticker{
		Exchange: DefaultExchange,
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

// EODHDSuffixToExchange maps EODHD API suffixes back to exchange-details API codes.
// These are the suffixes used in EODHD symbols (e.g., "CBA.AU" -> "AU").
var EODHDSuffixToExchange = map[string]string{
	"AU":    "AU",    // Australia (ASX)
	"US":    "US",    // United States (NYSE, NASDAQ, AMEX)
	"LSE":   "LSE",   // London Stock Exchange
	"XETRA": "XETRA", // Frankfurt Stock Exchange
	"TO":    "TO",    // Toronto Stock Exchange
	"PA":    "PA",    // Paris (Euronext)
	"HK":    "HK",    // Hong Kong
	"SG":    "SG",    // Singapore
	"TYO":   "TYO",   // Tokyo
	"CC":    "CC",    // Cryptocurrency
	"FOREX": "FOREX", // Foreign Exchange
	"INDX":  "INDX",  // Indices
}

// ParseEODHDTicker parses an EODHD-format ticker string.
// EODHD format: CODE.EXCHANGE (e.g., "CBA.AU", "AAPL.US", "BTC-USD.CC")
// Returns a Ticker with Exchange set to the EODHD exchange suffix.
//
// Examples:
//   - "CBA.AU" -> Exchange="AU", Code="CBA"
//   - "AAPL.US" -> Exchange="US", Code="AAPL"
//   - "BTC-USD.CC" -> Exchange="CC", Code="BTC-USD"
//   - "EURUSD.FOREX" -> Exchange="FOREX", Code="EURUSD"
func ParseEODHDTicker(symbol string) Ticker {
	symbol = strings.TrimSpace(symbol)
	if symbol == "" {
		return Ticker{}
	}

	// Find the last dot to split code and exchange
	// Use LastIndex because codes can contain dots (e.g., "BRK.B.US")
	lastDot := strings.LastIndex(symbol, ".")
	if lastDot < 0 || lastDot == len(symbol)-1 {
		// No dot or dot at end - invalid format
		return Ticker{}
	}

	code := symbol[:lastDot]
	exchange := strings.ToUpper(symbol[lastDot+1:])

	if code == "" || exchange == "" {
		return Ticker{}
	}

	return Ticker{
		Exchange: exchange,
		Code:     strings.ToUpper(code),
		Raw:      symbol,
	}
}

// DetailsExchangeCode returns the exchange code to use with the EODHD
// exchange-details API endpoint. For most exchanges this is the same
// as the Exchange field, but some may need mapping.
func (t Ticker) DetailsExchangeCode() string {
	if t.Exchange == "" {
		return ""
	}
	// Check if there's a specific mapping
	if mapped, ok := EODHDSuffixToExchange[t.Exchange]; ok {
		return mapped
	}
	// Return as-is for unknown exchanges
	return t.Exchange
}

// ParseEODHDTickers parses a list of EODHD-format ticker strings.
func ParseEODHDTickers(symbols []string) []Ticker {
	result := make([]Ticker, 0, len(symbols))
	for _, s := range symbols {
		if parsed := ParseEODHDTicker(s); parsed.Code != "" {
			result = append(result, parsed)
		}
	}
	return result
}
