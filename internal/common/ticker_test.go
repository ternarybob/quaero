package common

import (
	"testing"
)

func TestParseTicker(t *testing.T) {
	tests := []struct {
		input        string
		wantExchange string
		wantCode     string
		wantString   string
		wantEODHD    string
	}{
		// Exchange-qualified format
		{"ASX:GNP", "ASX", "GNP", "ASX:GNP", "GNP.AU"},
		{"ASX:BCN", "ASX", "BCN", "ASX:BCN", "BCN.AU"},
		{"NYSE:AAPL", "NYSE", "AAPL", "NYSE:AAPL", "AAPL.US"},
		{"NASDAQ:MSFT", "NASDAQ", "MSFT", "NASDAQ:MSFT", "MSFT.US"},

		// Legacy format (no exchange - defaults to ASX)
		{"GNP", "ASX", "GNP", "ASX:GNP", "GNP.AU"},
		{"BCN", "ASX", "BCN", "ASX:BCN", "BCN.AU"},

		// Case normalization
		{"asx:gnp", "ASX", "GNP", "ASX:GNP", "GNP.AU"},
		{"gnp", "ASX", "GNP", "ASX:GNP", "GNP.AU"},

		// Whitespace handling
		{"  ASX:GNP  ", "ASX", "GNP", "ASX:GNP", "GNP.AU"},
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
