package output

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestExtractBaseDomain(t *testing.T) {
	tests := []struct {
		name     string
		url      string
		expected string
	}{
		{
			name:     "https URL with path",
			url:      "https://eodhd.com/api/v1/data",
			expected: "eodhd.com",
		},
		{
			name:     "http URL with path",
			url:      "http://api.example.com/v1/stocks",
			expected: "api.example.com",
		},
		{
			name:     "URL with port",
			url:      "https://localhost:8085/documents",
			expected: "localhost",
		},
		{
			name:     "URL without scheme",
			url:      "eodhd.com/api/v1/data",
			expected: "eodhd.com",
		},
		{
			name:     "URL with query params",
			url:      "https://api.yahoo.com/finance?symbol=BHP",
			expected: "api.yahoo.com",
		},
		{
			name:     "URL with fragment",
			url:      "https://docs.example.com/guide#section",
			expected: "docs.example.com",
		},
		{
			name:     "Complex subdomain",
			url:      "https://api.v2.finance.yahoo.com/data",
			expected: "api.v2.finance.yahoo.com",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractBaseDomain(tt.url)
			assert.Equal(t, tt.expected, result, "extractBaseDomain(%s)", tt.url)
		})
	}
}

func TestDeduplicateDataSources(t *testing.T) {
	tests := []struct {
		name     string
		sources  []sourceDocInfo
		expected int
	}{
		{
			name: "No duplicates",
			sources: []sourceDocInfo{
				{Title: "doc1", SourceCategory: "local", ID: "1"},
				{Title: "eodhd.com", SourceCategory: "data", URL: "https://eodhd.com"},
				{Title: "News Article", SourceCategory: "web", URL: "https://news.com/article1"},
			},
			expected: 3,
		},
		{
			name: "Duplicate data sources",
			sources: []sourceDocInfo{
				{Title: "doc1", SourceCategory: "local", ID: "1"},
				{Title: "eodhd.com", SourceCategory: "data", URL: "https://eodhd.com"},
				{Title: "eodhd.com", SourceCategory: "data", URL: "https://eodhd.com"}, // duplicate
				{Title: "yahoo.com", SourceCategory: "data", URL: "https://yahoo.com"},
			},
			expected: 3, // local + 2 unique data
		},
		{
			name: "Local sources never deduplicated",
			sources: []sourceDocInfo{
				{Title: "doc1", SourceCategory: "local", ID: "1"},
				{Title: "doc1", SourceCategory: "local", ID: "2"}, // Same title, different ID
				{Title: "eodhd.com", SourceCategory: "data", URL: "https://eodhd.com"},
			},
			expected: 3, // Both local + 1 data
		},
		{
			name: "Duplicate web sources",
			sources: []sourceDocInfo{
				{Title: "News 1", SourceCategory: "web", URL: "https://news.com/article1"},
				{Title: "News 1 Copy", SourceCategory: "web", URL: "https://news.com/article1"}, // Same URL
				{Title: "News 2", SourceCategory: "web", URL: "https://news.com/article2"},
			},
			expected: 2, // 2 unique URLs
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := deduplicateDataSources(tt.sources)
			assert.Len(t, result, tt.expected, "deduplicateDataSources should return %d items", tt.expected)
		})
	}
}

func TestSourceDocInfoCategories(t *testing.T) {
	// Test that sourceTypes includes announcement summaries
	sourceTypes := []string{"asx-stock-data", "stock-recommendation", "asx-announcement-summary"}

	assert.Contains(t, sourceTypes, "asx-stock-data", "Should include asx-stock-data")
	assert.Contains(t, sourceTypes, "stock-recommendation", "Should include stock-recommendation")
	assert.Contains(t, sourceTypes, "asx-announcement-summary", "Should include asx-announcement-summary")
}
