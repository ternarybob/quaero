package common

import (
	"testing"
)

func TestParseTicker(t *testing.T) {
	// Ensure default exchange is ASX for these tests
	originalDefault := DefaultExchange
	DefaultExchange = "ASX"
	defer func() { DefaultExchange = originalDefault }()

	tests := []struct {
		input        string
		wantExchange string
		wantCode     string
		wantString   string
		wantEODHD    string
	}{
		// Exchange-qualified format with colon separator
		{"ASX:GNP", "ASX", "GNP", "ASX:GNP", "GNP.AU"},
		{"ASX:BCN", "ASX", "BCN", "ASX:BCN", "BCN.AU"},
		{"NYSE:AAPL", "NYSE", "AAPL", "NYSE:AAPL", "AAPL.US"},
		{"NASDAQ:MSFT", "NASDAQ", "MSFT", "NASDAQ:MSFT", "MSFT.US"},

		// Exchange-qualified format with dot separator (EXCHANGE.CODE)
		{"ASX.GNP", "ASX", "GNP", "ASX:GNP", "GNP.AU"},
		{"ASX.BCN", "ASX", "BCN", "ASX:BCN", "BCN.AU"},
		{"NYSE.AAPL", "NYSE", "AAPL", "NYSE:AAPL", "AAPL.US"},
		{"NASDAQ.MSFT", "NASDAQ", "MSFT", "NASDAQ:MSFT", "MSFT.US"},

		// Legacy format (no exchange - defaults to ASX)
		{"GNP", "ASX", "GNP", "ASX:GNP", "GNP.AU"},
		{"BCN", "ASX", "BCN", "ASX:BCN", "BCN.AU"},

		// Case normalization
		{"asx:gnp", "ASX", "GNP", "ASX:GNP", "GNP.AU"},
		{"asx.gnp", "ASX", "GNP", "ASX:GNP", "GNP.AU"},
		{"gnp", "ASX", "GNP", "ASX:GNP", "GNP.AU"},

		// Whitespace handling
		{"  ASX:GNP  ", "ASX", "GNP", "ASX:GNP", "GNP.AU"},
		{"  ASX.GNP  ", "ASX", "GNP", "ASX:GNP", "GNP.AU"},
		{"  GNP  ", "ASX", "GNP", "ASX:GNP", "GNP.AU"},

		// Empty input
		{"", "", "", "", ""},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := ParseTicker(tt.input)

			if result.Exchange != tt.wantExchange {
				t.Errorf("Exchange = %q, want %q", result.Exchange, tt.wantExchange)
			}
			if result.Code != tt.wantCode {
				t.Errorf("Code = %q, want %q", result.Code, tt.wantCode)
			}
			if result.String() != tt.wantString {
				t.Errorf("String() = %q, want %q", result.String(), tt.wantString)
			}
			if result.EODHDSymbol() != tt.wantEODHD {
				t.Errorf("EODHDSymbol() = %q, want %q", result.EODHDSymbol(), tt.wantEODHD)
			}
		})
	}
}

func TestTicker_SourceID(t *testing.T) {
	tests := []struct {
		ticker     string
		suffix     string
		wantResult string
	}{
		{"ASX:GNP", "stock_collector", "asx:GNP:stock_collector"},
		{"ASX:GNP", "signals", "asx:GNP:signals"},
		{"ASX:GNP", "", "asx:GNP"},
		{"GNP", "stock_collector", "asx:GNP:stock_collector"},
		{"NYSE:AAPL", "stock_collector", "nyse:AAPL:stock_collector"},
	}

	for _, tt := range tests {
		t.Run(tt.ticker+"_"+tt.suffix, func(t *testing.T) {
			parsed := ParseTicker(tt.ticker)
			result := parsed.SourceID(tt.suffix)

			if result != tt.wantResult {
				t.Errorf("SourceID(%q) = %q, want %q", tt.suffix, result, tt.wantResult)
			}
		})
	}
}

func TestParseTickers(t *testing.T) {
	input := []string{"ASX:GNP", "ASX:BCN", "MYG", "  ", ""}
	result := ParseTickers(input)

	if len(result) != 3 {
		t.Errorf("ParseTickers returned %d tickers, want 3", len(result))
	}

	expected := []string{"GNP", "BCN", "MYG"}
	for i, ticker := range result {
		if ticker.Code != expected[i] {
			t.Errorf("result[%d].Code = %q, want %q", i, ticker.Code, expected[i])
		}
	}
}

func TestParseTickersFromInterface(t *testing.T) {
	tests := []struct {
		name  string
		input interface{}
		want  []string // expected codes
	}{
		{
			name:  "single string",
			input: "ASX:GNP",
			want:  []string{"GNP"},
		},
		{
			name:  "string slice",
			input: []string{"ASX:GNP", "ASX:BCN"},
			want:  []string{"GNP", "BCN"},
		},
		{
			name:  "interface slice (from TOML)",
			input: []interface{}{"ASX:GNP", "ASX:BCN", "MYG"},
			want:  []string{"GNP", "BCN", "MYG"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ParseTickersFromInterface(tt.input)

			if len(result) != len(tt.want) {
				t.Errorf("got %d tickers, want %d", len(result), len(tt.want))
				return
			}

			for i, ticker := range result {
				if ticker.Code != tt.want[i] {
					t.Errorf("result[%d].Code = %q, want %q", i, ticker.Code, tt.want[i])
				}
			}
		})
	}
}

func TestParseEODHDTicker(t *testing.T) {
	tests := []struct {
		input        string
		wantExchange string
		wantCode     string
	}{
		// Standard EODHD format
		{"CBA.AU", "AU", "CBA"},
		{"AAPL.US", "US", "AAPL"},
		{"VOD.LSE", "LSE", "VOD"},
		{"SAP.XETRA", "XETRA", "SAP"},

		// Crypto (code contains hyphen)
		{"BTC-USD.CC", "CC", "BTC-USD"},
		{"ETH-USD.CC", "CC", "ETH-USD"},

		// Forex
		{"EURUSD.FOREX", "FOREX", "EURUSD"},
		{"GBPUSD.FOREX", "FOREX", "GBPUSD"},

		// Indices
		{"AXJO.INDX", "INDX", "AXJO"},

		// Code with dot (e.g., BRK.B)
		{"BRK.B.US", "US", "BRK.B"},

		// Case normalization
		{"cba.au", "AU", "CBA"},
		{"aapl.us", "US", "AAPL"},

		// Whitespace handling
		{"  CBA.AU  ", "AU", "CBA"},

		// Invalid formats
		{"", "", ""},
		{"NODOT", "", ""},
		{".", "", ""},
		{".AU", "", ""},
		{"CBA.", "", ""},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := ParseEODHDTicker(tt.input)

			if result.Exchange != tt.wantExchange {
				t.Errorf("Exchange = %q, want %q", result.Exchange, tt.wantExchange)
			}
			if result.Code != tt.wantCode {
				t.Errorf("Code = %q, want %q", result.Code, tt.wantCode)
			}
		})
	}
}

func TestTicker_DetailsExchangeCode(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"CBA.AU", "AU"},
		{"AAPL.US", "US"},
		{"VOD.LSE", "LSE"},
		{"BTC-USD.CC", "CC"},
		{"EURUSD.FOREX", "FOREX"},
		// Unknown exchange returns as-is
		{"ABC.UNKNOWN", "UNKNOWN"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			parsed := ParseEODHDTicker(tt.input)
			result := parsed.DetailsExchangeCode()

			if result != tt.want {
				t.Errorf("DetailsExchangeCode() = %q, want %q", result, tt.want)
			}
		})
	}
}

func TestParseEODHDTickers(t *testing.T) {
	input := []string{"CBA.AU", "AAPL.US", "BTC-USD.CC", "", "INVALID"}
	result := ParseEODHDTickers(input)

	if len(result) != 3 {
		t.Errorf("ParseEODHDTickers returned %d tickers, want 3", len(result))
	}

	expected := []struct {
		code     string
		exchange string
	}{
		{"CBA", "AU"},
		{"AAPL", "US"},
		{"BTC-USD", "CC"},
	}

	for i, ticker := range result {
		if ticker.Code != expected[i].code {
			t.Errorf("result[%d].Code = %q, want %q", i, ticker.Code, expected[i].code)
		}
		if ticker.Exchange != expected[i].exchange {
			t.Errorf("result[%d].Exchange = %q, want %q", i, ticker.Exchange, expected[i].exchange)
		}
	}
}
