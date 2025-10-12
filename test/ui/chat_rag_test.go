// -----------------------------------------------------------------------
// Last Modified: Sunday, 13th October 2025 9:15:00 am
// Modified By: Claude Code
// -----------------------------------------------------------------------

package ui

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/chromedp/chromedp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestChatRAGCorpusSummaryUI tests the chat interface with RAG corpus summary queries
// This mirrors the API test but verifies the full UI interaction and response display
func TestChatRAGCorpusSummaryUI(t *testing.T) {
	config, err := LoadTestConfig()
	require.NoError(t, err, "Failed to load test config")

	// Create Chrome context
	opts := append(chromedp.DefaultExecAllocatorOptions[:],
		chromedp.Flag("headless", true),
		chromedp.Flag("disable-gpu", true),
		chromedp.Flag("no-sandbox", true),
		chromedp.WindowSize(1920, 1080),
	)

	allocCtx, cancel := chromedp.NewExecAllocator(context.Background(), opts...)
	defer cancel()

	ctx, cancel := chromedp.NewContext(allocCtx)
	defer cancel()

	// Set timeout for entire test
	ctx, cancel = context.WithTimeout(ctx, 15*time.Minute)
	defer cancel()

	// Navigate to chat page
	chatURL := config.ServerURL + "/chat"
	t.Logf("Navigating to chat page: %s", chatURL)

	err = chromedp.Run(ctx,
		chromedp.Navigate(chatURL),
		chromedp.WaitVisible(`body`, chromedp.ByQuery),
	)
	require.NoError(t, err, "Failed to navigate to chat page")

	takeScreenshot(ctx, t, "chat_page_loaded")

	// Verify RAG is enabled by default
	var ragEnabled bool
	err = chromedp.Run(ctx,
		chromedp.WaitVisible(`#rag-enabled`, chromedp.ByQuery),
		chromedp.Evaluate(`document.getElementById('rag-enabled').checked`, &ragEnabled),
	)
	require.NoError(t, err, "Failed to check RAG toggle state")
	assert.True(t, ragEnabled, "RAG should be enabled by default")
	t.Log("✓ RAG is enabled")

	// Test cases - same as API tests
	testCases := []struct {
		name                  string
		message               string
		expectedInResponse    []string
		maxWaitSeconds        int
		verifyCorpusSummary   bool
		verifyNumericResponse bool
	}{
		{
			name:    "Total document count query",
			message: "How many total documents are in the system?",
			expectedInResponse: []string{
				"380",
				"Quaero Corpus Summary",
			},
			maxWaitSeconds:        120,
			verifyCorpusSummary:   true,
			verifyNumericResponse: true,
		},
		{
			name:    "Jira document count query",
			message: "How many Jira issues are indexed?",
			expectedInResponse: []string{
				"350",
				"Jira",
			},
			maxWaitSeconds:        120,
			verifyCorpusSummary:   true,
			verifyNumericResponse: true,
		},
		{
			name:    "Confluence document count query",
			message: "How many Confluence pages are available?",
			expectedInResponse: []string{
				"29",
				"Confluence",
			},
			maxWaitSeconds:        120,
			verifyCorpusSummary:   true,
			verifyNumericResponse: true,
		},
		{
			name:    "Embedded document count query",
			message: "How many documents have embeddings?",
			expectedInResponse: []string{
				"380",
				"embedded",
			},
			maxWaitSeconds:        120,
			verifyCorpusSummary:   true,
			verifyNumericResponse: true,
		},
		{
			name:    "General corpus statistics query",
			message: "Tell me about the document corpus statistics",
			expectedInResponse: []string{
				"380",
				"350",
				"29",
				"statistics",
			},
			maxWaitSeconds:        120,
			verifyCorpusSummary:   true,
			verifyNumericResponse: true,
		},
	}

	for i, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Logf("=== Test %d/%d: %s ===", i+1, len(testCases), tc.name)
			t.Logf("Query: %s", tc.message)

			// Clear any previous messages
			err := chromedp.Run(ctx,
				chromedp.Click(`#clear-btn`, chromedp.ByQuery),
				chromedp.Sleep(500*time.Millisecond),
			)
			if err != nil {
				t.Logf("Clear chat button not found or failed: %v", err)
			}

			takeScreenshot(ctx, t, fmt.Sprintf("test%d_before_input", i+1))

			// Type message into input field
			messageSelector := `#user-message`
			err = chromedp.Run(ctx,
				chromedp.WaitVisible(messageSelector, chromedp.ByQuery),
				chromedp.SetValue(messageSelector, tc.message, chromedp.ByQuery),
			)
			require.NoError(t, err, "Failed to type message")

			takeScreenshot(ctx, t, fmt.Sprintf("test%d_message_typed", i+1))

			// Click send button
			sendButtonSelector := `#send-btn`
			err = chromedp.Run(ctx,
				chromedp.Click(sendButtonSelector, chromedp.ByQuery),
			)
			require.NoError(t, err, "Failed to click send button")

			t.Log("Message sent, waiting for response...")
			startTime := time.Now()

			// Wait for response with timeout
			responseSelector := `.message-block.is-assistant`
			waitCtx, waitCancel := context.WithTimeout(ctx, time.Duration(tc.maxWaitSeconds)*time.Second)
			defer waitCancel()

			err = chromedp.Run(waitCtx,
				chromedp.WaitVisible(responseSelector, chromedp.ByQuery),
				chromedp.Sleep(2*time.Second), // Wait for complete rendering
			)
			require.NoError(t, err, "Failed to receive response within %d seconds", tc.maxWaitSeconds)

			duration := time.Since(startTime)
			t.Logf("✓ Response received in %v", duration)

			takeScreenshot(ctx, t, fmt.Sprintf("test%d_response_received", i+1))

			// Extract response text
			var responseText string
			err = chromedp.Run(ctx,
				chromedp.Evaluate(`
					(() => {
						const assistantMessages = document.querySelectorAll('.message-block.is-assistant');
						if (assistantMessages.length > 0) {
							const lastMessage = assistantMessages[assistantMessages.length - 1];
							return lastMessage.textContent.trim();
						}
						return '';
					})()
				`, &responseText),
			)
			require.NoError(t, err, "Failed to extract response text")
			require.NotEmpty(t, responseText, "Response text should not be empty")

			t.Logf("Response preview: %s...", responseText[:min(100, len(responseText))])

			// Verify expected content in response
			for _, expected := range tc.expectedInResponse {
				assert.Contains(t, responseText, expected,
					"Response should contain '%s'", expected)
			}

			// Verify corpus summary is referenced
			if tc.verifyCorpusSummary {
				corpusSummaryFound := strings.Contains(responseText, "Corpus Summary") ||
					strings.Contains(responseText, "corpus summary") ||
					strings.Contains(responseText, "CORPUS SUMMARY")
				assert.True(t, corpusSummaryFound,
					"Response should reference the Corpus Summary document")
				if corpusSummaryFound {
					t.Log("✓ Response references Corpus Summary document")
				}
			}

			// Verify response contains numeric data
			if tc.verifyNumericResponse {
				hasNumbers := false
				for _, char := range responseText {
					if char >= '0' && char <= '9' {
						hasNumbers = true
						break
					}
				}
				assert.True(t, hasNumbers, "Response should contain numeric data")
				if hasNumbers {
					t.Log("✓ Response contains numeric data")
				}
			}

			// Check context documents in UI
			var contextDocsCount int
			err = chromedp.Run(ctx,
				chromedp.Evaluate(`
					(() => {
						// Check if context docs are displayed in UI
						const contextSection = document.querySelector('.context-docs, [data-context-docs]');
						if (contextSection) {
							return contextSection.querySelectorAll('.context-doc').length;
						}
						return 0;
					})()
				`, &contextDocsCount),
			)
			if err == nil && contextDocsCount > 0 {
				t.Logf("✓ Context documents displayed in UI: %d", contextDocsCount)
			}

			// Verify response time is reasonable
			if duration > time.Duration(tc.maxWaitSeconds)*time.Second {
				t.Errorf("Response took %v, expected < %d seconds", duration, tc.maxWaitSeconds)
			} else {
				t.Logf("✓ Response time within limit (%v < %ds)", duration, tc.maxWaitSeconds)
			}

			// Small delay between tests
			time.Sleep(1 * time.Second)
		})
	}

	takeScreenshot(ctx, t, "all_tests_complete")

	t.Log("")
	t.Log("=== RAG UI Verification Complete ===")
	t.Log("All chat queries executed successfully via UI")
	t.Log("RAG functionality verified through chat interface")
}

// TestChatRAGToggle tests enabling/disabling RAG and verifying behavior
func TestChatRAGToggle(t *testing.T) {
	config, err := LoadTestConfig()
	require.NoError(t, err, "Failed to load test config")

	opts := append(chromedp.DefaultExecAllocatorOptions[:],
		chromedp.Flag("headless", true),
		chromedp.Flag("disable-gpu", true),
		chromedp.Flag("no-sandbox", true),
		chromedp.WindowSize(1920, 1080),
	)

	allocCtx, cancel := chromedp.NewExecAllocator(context.Background(), opts...)
	defer cancel()

	ctx, cancel := chromedp.NewContext(allocCtx)
	defer cancel()

	ctx, cancel = context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()

	chatURL := config.ServerURL + "/chat"
	t.Logf("Navigating to chat page: %s", chatURL)

	err = chromedp.Run(ctx,
		chromedp.Navigate(chatURL),
		chromedp.WaitVisible(`body`, chromedp.ByQuery),
	)
	require.NoError(t, err, "Failed to navigate to chat page")

	takeScreenshot(ctx, t, "toggle_test_start")

	// Verify RAG toggle exists and is enabled by default
	var ragEnabled bool
	err = chromedp.Run(ctx,
		chromedp.WaitVisible(`#rag-enabled`, chromedp.ByQuery),
		chromedp.Evaluate(`document.getElementById('rag-enabled').checked`, &ragEnabled),
	)
	require.NoError(t, err, "Failed to check RAG toggle")
	assert.True(t, ragEnabled, "RAG should be enabled by default")
	t.Log("✓ RAG toggle found and is enabled by default")

	// Disable RAG
	err = chromedp.Run(ctx,
		chromedp.Click(`#rag-enabled`, chromedp.ByQuery),
		chromedp.Sleep(500*time.Millisecond),
		chromedp.Evaluate(`document.getElementById('rag-enabled').checked`, &ragEnabled),
	)
	require.NoError(t, err, "Failed to toggle RAG off")
	assert.False(t, ragEnabled, "RAG should be disabled after clicking toggle")
	t.Log("✓ RAG disabled successfully")

	takeScreenshot(ctx, t, "rag_disabled")

	// Send a test message with RAG disabled
	testMessage := "How many documents are in the system?"
	messageSelector := `#user-message`
	sendButtonSelector := `#send-btn`

	err = chromedp.Run(ctx,
		chromedp.SetValue(messageSelector, testMessage, chromedp.ByQuery),
		chromedp.Click(sendButtonSelector, chromedp.ByQuery),
		chromedp.WaitVisible(`.message-block.is-assistant`, chromedp.ByQuery),
		chromedp.Sleep(2*time.Second),
	)
	require.NoError(t, err, "Failed to send message with RAG disabled")

	var responseWithoutRAG string
	err = chromedp.Run(ctx,
		chromedp.Evaluate(`
			(() => {
				const messages = document.querySelectorAll('.message-block.is-assistant');
				return messages[messages.length - 1].textContent.trim();
			})()
		`, &responseWithoutRAG),
	)
	require.NoError(t, err, "Failed to get response")
	t.Logf("Response without RAG: %s...", responseWithoutRAG[:min(100, len(responseWithoutRAG))])

	takeScreenshot(ctx, t, "response_without_rag")

	// Re-enable RAG
	err = chromedp.Run(ctx,
		chromedp.Click(`#rag-enabled`, chromedp.ByQuery),
		chromedp.Sleep(500*time.Millisecond),
		chromedp.Evaluate(`document.getElementById('rag-enabled').checked`, &ragEnabled),
	)
	require.NoError(t, err, "Failed to toggle RAG on")
	assert.True(t, ragEnabled, "RAG should be enabled after clicking toggle again")
	t.Log("✓ RAG re-enabled successfully")

	// Clear and send same message with RAG enabled
	err = chromedp.Run(ctx,
		chromedp.Click(`#clear-btn`, chromedp.ByQuery),
		chromedp.Sleep(1*time.Second),
		chromedp.SetValue(messageSelector, testMessage, chromedp.ByQuery),
		chromedp.Click(sendButtonSelector, chromedp.ByQuery),
		chromedp.WaitVisible(`.message-block.is-assistant`, chromedp.ByQuery),
		chromedp.Sleep(2*time.Second),
	)
	require.NoError(t, err, "Failed to send message with RAG enabled")

	var responseWithRAG string
	err = chromedp.Run(ctx,
		chromedp.Evaluate(`
			(() => {
				const messages = document.querySelectorAll('.message-block.is-assistant');
				return messages[messages.length - 1].textContent.trim();
			})()
		`, &responseWithRAG),
	)
	require.NoError(t, err, "Failed to get response")
	t.Logf("Response with RAG: %s...", responseWithRAG[:min(100, len(responseWithRAG))])

	takeScreenshot(ctx, t, "response_with_rag")

	// Verify responses are different and RAG response contains corpus data
	assert.NotEqual(t, responseWithoutRAG, responseWithRAG,
		"Responses should differ between RAG enabled/disabled")

	// RAG response should contain specific numeric data
	ragHasNumbers := strings.Contains(responseWithRAG, "380") ||
		strings.Contains(responseWithRAG, "350") ||
		strings.Contains(responseWithRAG, "29")
	assert.True(t, ragHasNumbers,
		"RAG response should contain specific corpus statistics")

	if ragHasNumbers {
		t.Log("✓ RAG response contains corpus statistics")
	}

	t.Log("✓ RAG toggle test complete")
}

// Helper function for min
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
