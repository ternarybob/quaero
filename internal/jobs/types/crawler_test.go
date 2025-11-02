package types

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

func TestFormatJobError(t *testing.T) {
	tests := []struct {
		name        string
		category    string
		err         error
		url         string
		timeout     time.Duration
		expectedMsg string
	}{
		{
			name:        "HTTP 404 error with URL",
			category:    "Scraping",
			err:         errors.New("404 not found"),
			url:         "https://example.com/page1",
			timeout:     0,
			expectedMsg: "HTTP 404: Not Found for https://example.com/page1",
		},
		{
			name:        "Timeout with configured duration",
			category:    "Scraping",
			err:         errors.New("request timeout"),
			url:         "https://example.com/page2",
			timeout:     10 * time.Second,
			expectedMsg: "Timeout: Scraping timeout (10s) for https://example.com/page2",
		},
		{
			name:        "Timeout without configured duration",
			category:    "Scraping",
			err:         errors.New("timeout: 5s"),
			url:         "https://example.com/page3",
			timeout:     0,
			expectedMsg: "Timeout: Scraping timeout (5s) for https://example.com/page3",
		},
		{
			name:        "DeadlineExceeded with configured timeout",
			category:    "Scraping",
			err:         context.DeadlineExceeded,
			url:         "https://example.com/page4",
			timeout:     30 * time.Second,
			expectedMsg: "Timeout: Request exceeded 30s for https://example.com/page4",
		},
		{
			name:        "DeadlineExceeded without configured timeout",
			category:    "Scraping",
			err:         context.DeadlineExceeded,
			url:         "https://example.com/page5",
			timeout:     0,
			expectedMsg: "Timeout: Request exceeded deadline for https://example.com/page5",
		},
		{
			name:        "Timeout without URL with duration",
			category:    "Network",
			err:         errors.New("connection timeout"),
			url:         "",
			timeout:     15 * time.Second,
			expectedMsg: "Timeout: Request timeout (15s)",
		},
		{
			name:        "Timeout without URL and without duration",
			category:    "Network",
			err:         errors.New("timeout occurred"),
			url:         "",
			timeout:     0,
			expectedMsg: "Timeout: Request timeout",
		},
		{
			name:        "Network error with URL",
			category:    "Network",
			err:         errors.New("dial tcp: connection refused"),
			url:         "https://example.com/page6",
			timeout:     0,
			expectedMsg: "Network: dial tcp: connection refused (URL: https://example.com/page6)",
		},
		{
			name:        "Generic error with URL",
			category:    "Scraping",
			err:         errors.New("some unexpected error"),
			url:         "https://example.com/page7",
			timeout:     0,
			expectedMsg: "Scraping: some unexpected error (URL: https://example.com/page7)",
		},
		{
			name:        "Generic error without URL",
			category:    "Scraping",
			err:         errors.New("some unexpected error"),
			url:         "",
			timeout:     0,
			expectedMsg: "Scraping: some unexpected error",
		},
		{
			name:        "Long error message truncated",
			category:    "Scraping",
			err:         errors.New(strings.Repeat("a", 250)),
			url:         "",
			timeout:     0,
			expectedMsg: "Scraping: " + strings.Repeat("a", 187) + "...",
		},
		{
			name:        "DeadlineExceeded parsed from error message",
			category:    "Scraping",
			err:         errors.New("context.WithTimeout deadline exceeded: 20s"),
			url:         "https://example.com/page8",
			timeout:     0,
			expectedMsg: "Timeout: Scraping timeout (20s) for https://example.com/page8",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := formatJobError(tt.category, tt.err, tt.url, tt.timeout)
			if result != tt.expectedMsg {
				t.Errorf("formatJobError() = %v, want %v", result, tt.expectedMsg)
			}
		})
	}
}

// Test that the timeout parameter is properly respected over parsing from error
func TestFormatJobError_TimeoutPriority(t *testing.T) {
	// Test that configured timeout takes priority over parsing from error
	err := errors.New("context.WithTimeout deadline exceeded: 20s")
	url := "https://example.com/test"

	// With configured timeout
	result := formatJobError("Scraping", err, url, 30*time.Second)
	expected := "Timeout: Scraping timeout (30s) for https://example.com/test"
	if result != expected {
		t.Errorf("formatJobError() with configured timeout = %v, want %v", result, expected)
	}

	// Without configured timeout (should parse from error)
	result = formatJobError("Scraping", err, url, 0)
	expected = "Timeout: Scraping timeout (20s) for https://example.com/test"
	if result != expected {
		t.Errorf("formatJobError() with parsed timeout = %v, want %v", result, expected)
	}
}

// Test that messages are well-formed (balanced parentheses)
func TestFormatJobError_ParenthesisBalance(t *testing.T) {
	// Test various timeout scenarios to ensure parentheses are balanced
	testCases := []struct {
		name     string
		err      error
		timeout  time.Duration
		url      string
		category string
	}{
		{"timeout with duration", errors.New("timeout"), 10 * time.Second, "https://test.com", "Scraping"},
		{"timeout without duration", errors.New("timeout"), 0, "https://test.com", "Scraping"},
		{"deadline exceeded with duration", context.DeadlineExceeded, 15 * time.Second, "https://test.com", "Network"},
		{"deadline exceeded without duration", context.DeadlineExceeded, 0, "https://test.com", "Network"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := formatJobError(tc.category, tc.err, tc.url, tc.timeout)

			// Check that if there's a timeout duration AND it's NOT DeadlineExceeded, the message has balanced parentheses
			// Note: DeadlineExceeded uses direct formatting without parentheses for backward compatibility
			if tc.timeout > 0 && !errors.Is(tc.err, context.DeadlineExceeded) {
				if !strings.Contains(result, "(") || !strings.Contains(result, ")") {
					t.Errorf("formatJobError() = %v is missing balanced parentheses when timeout is provided", result)
				}
			}

			// Ensure no unmatched opening parentheses
			openCount := strings.Count(result, "(")
			closeCount := strings.Count(result, ")")
			if openCount != closeCount {
				t.Errorf("formatJobError() = %v has unbalanced parentheses: %d open, %d close", result, openCount, closeCount)
			}
		})
	}
}
