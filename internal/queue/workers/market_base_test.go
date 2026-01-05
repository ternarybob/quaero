// -----------------------------------------------------------------------
// Unit tests for BaseMarketWorker
// Tests document caching with staleness checking
// -----------------------------------------------------------------------

package workers

import (
	"testing"
	"time"
)

// =============================================================================
// Test Cases for Tag Generation Functions
// =============================================================================

func TestGenerateMarketTags_CorrectFormat(t *testing.T) {
	tests := []struct {
		name       string
		ticker     string
		workerType string
		wantTags   []string
	}{
		{
			name:       "ASX ticker",
			ticker:     "CBA.AU",
			workerType: "market_fundamentals",
			wantTags: []string{
				"ticker:cba.au",
				"source_type:market_fundamentals",
				"exchange:au",
			},
		},
		{
			name:       "US ticker",
			ticker:     "AAPL.US",
			workerType: "market_data",
			wantTags: []string{
				"ticker:aapl.us",
				"source_type:market_data",
				"exchange:us",
			},
		},
		{
			name:       "Index ticker",
			ticker:     "AXJO.INDX",
			workerType: "market_data",
			wantTags: []string{
				"ticker:axjo.indx",
				"source_type:market_data",
				"exchange:indx",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tags := GenerateMarketTags(tt.ticker, tt.workerType, time.Now())

			// Check that all expected tags are present
			for _, wantTag := range tt.wantTags {
				found := false
				for _, gotTag := range tags {
					if gotTag == wantTag {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("Missing expected tag: %s (got tags: %v)", wantTag, tags)
				}
			}

			// Check date tag is present
			dateTagFound := false
			for _, tag := range tags {
				if len(tag) > 5 && tag[:5] == "date:" {
					dateTagFound = true
					break
				}
			}
			if !dateTagFound {
				t.Error("Missing date tag")
			}
		})
	}
}

func TestGenerateMarketTags_LowercaseTicker(t *testing.T) {
	// Test that tags are lowercase
	tags := GenerateMarketTags("CBA.AU", "market_fundamentals", time.Now())

	for _, tag := range tags {
		// Check that no tag contains uppercase letters (except date values)
		if tag[:5] != "date:" {
			for _, c := range tag {
				if c >= 'A' && c <= 'Z' {
					t.Errorf("Tag contains uppercase: %s", tag)
					break
				}
			}
		}
	}
}

func TestGenerateMarketTags_DateFormat(t *testing.T) {
	testDate := time.Date(2026, 1, 6, 0, 0, 0, 0, time.UTC)
	tags := GenerateMarketTags("CBA.AU", "market_fundamentals", testDate)

	expectedDateTag := "date:2026-01-06"
	found := false
	for _, tag := range tags {
		if tag == expectedDateTag {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("Expected date tag %s not found in tags: %v", expectedDateTag, tags)
	}
}

func TestNormalizeTickerForEODHD(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"CBA", "CBA.AU"},
		{"ASX:CBA", "CBA.AU"},
		{"CBA.AU", "CBA.AU"},
		{"cba.au", "CBA.AU"},
		{"AAPL.US", "AAPL.US"},
		{"aapl.us", "AAPL.US"},
		{"BTC-USD.CC", "BTC-USD.CC"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := NormalizeTickerForEODHD(tt.input)
			if got != tt.want {
				t.Errorf("NormalizeTickerForEODHD(%s) = %s, want %s", tt.input, got, tt.want)
			}
		})
	}
}

func TestNormalizeTickerForEODHD_DefaultsToAU(t *testing.T) {
	// Test that bare ticker codes default to AU exchange
	result := NormalizeTickerForEODHD("CBA")
	if result != "CBA.AU" {
		t.Errorf("Expected CBA to normalize to CBA.AU, got %s", result)
	}
}

func TestNormalizeTickerForEODHD_PreservesExchange(t *testing.T) {
	// Test that existing exchange codes are preserved
	result := NormalizeTickerForEODHD("AAPL.US")
	if result != "AAPL.US" {
		t.Errorf("Expected AAPL.US to remain AAPL.US, got %s", result)
	}

	result = NormalizeTickerForEODHD("BTC-USD.CC")
	if result != "BTC-USD.CC" {
		t.Errorf("Expected BTC-USD.CC to remain BTC-USD.CC, got %s", result)
	}
}

// =============================================================================
// Test Cases for DocumentStatus
// =============================================================================

func TestDocumentStatus_Values(t *testing.T) {
	// Test that document status values are defined correctly
	if DocumentStatusFresh != "fresh" {
		t.Errorf("DocumentStatusFresh = %s, want fresh", DocumentStatusFresh)
	}
	if DocumentStatusStale != "stale" {
		t.Errorf("DocumentStatusStale = %s, want stale", DocumentStatusStale)
	}
	if DocumentStatusMissing != "missing" {
		t.Errorf("DocumentStatusMissing = %s, want missing", DocumentStatusMissing)
	}
	if DocumentStatusPending != "pending" {
		t.Errorf("DocumentStatusPending = %s, want pending", DocumentStatusPending)
	}
}

// =============================================================================
// Test Cases for BaseMarketWorker
// =============================================================================

func TestBaseMarketWorker_GetWorkerType(t *testing.T) {
	baseWorker := NewBaseMarketWorker(
		nil, // documentStorage
		nil, // searchService
		nil, // exchangeService
		nil, // kvStorage
		nil, // logger (nil is ok for this test)
		nil, // jobMgr
		"market_fundamentals",
	)

	if got := baseWorker.GetWorkerType(); got != "market_fundamentals" {
		t.Errorf("GetWorkerType() = %s, want market_fundamentals", got)
	}
}

func TestBaseMarketWorker_SetExchangeService(t *testing.T) {
	baseWorker := NewBaseMarketWorker(
		nil, nil, nil, nil, nil, nil, "test_worker",
	)

	// Should not panic even with nil
	baseWorker.SetExchangeService(nil)

	// Verify it's stored
	if baseWorker.exchangeService != nil {
		t.Error("Expected exchangeService to be nil after setting nil")
	}
}

func TestBaseMarketWorker_SetSearchService(t *testing.T) {
	baseWorker := NewBaseMarketWorker(
		nil, nil, nil, nil, nil, nil, "test_worker",
	)

	// Should not panic even with nil
	baseWorker.SetSearchService(nil)

	// Verify it's stored
	if baseWorker.searchService != nil {
		t.Error("Expected searchService to be nil after setting nil")
	}
}

// =============================================================================
// Test Cases for DocumentResult
// =============================================================================

func TestDocumentResult_DefaultValues(t *testing.T) {
	result := &DocumentResult{
		Status: DocumentStatusMissing,
		Reason: "test reason",
	}

	if result.Document != nil {
		t.Error("Document should be nil by default")
	}
	if result.IsStale {
		t.Error("IsStale should be false by default")
	}
	if !result.NextCheckTime.IsZero() {
		t.Error("NextCheckTime should be zero by default")
	}
	if result.Status != DocumentStatusMissing {
		t.Errorf("Status = %s, want missing", result.Status)
	}
	if result.Reason != "test reason" {
		t.Errorf("Reason = %s, want 'test reason'", result.Reason)
	}
}
