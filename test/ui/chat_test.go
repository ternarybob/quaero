package ui

import (
	"context"
	"testing"
	"time"

	"github.com/chromedp/chromedp"
	"github.com/ternarybob/quaero/test"
)

func TestChatPageLoad(t *testing.T) {
	ctx, cancel := chromedp.NewContext(context.Background())
	defer cancel()

	ctx, cancel = context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	url := test.GetTestServerURL() + "/chat"
	var title string

	err := chromedp.Run(ctx,
		chromedp.Navigate(url),
		chromedp.WaitVisible(`body`, chromedp.ByQuery),
		chromedp.Title(&title),
	)

	if err != nil {
		t.Fatalf("Failed to load chat page: %v", err)
	}

	if title != "Chat - Quaero" {
		t.Errorf("Expected title 'Chat - Quaero', got '%s'", title)
	}
}

func TestChatElements(t *testing.T) {
	ctx, cancel := chromedp.NewContext(context.Background())
	defer cancel()

	ctx, cancel = context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	url := test.GetTestServerURL() + "/chat"

	// Check for presence of chat UI elements
	tests := []struct {
		name     string
		selector string
	}{
		{"Chat messages container", "#chat-messages"},
		{"Message input", "#user-message"},
		{"Send button", "#send-btn"},
		{"Clear button", "#clear-btn"},
		{"RAG checkbox", "#rag-enabled"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var visible bool
			err := chromedp.Run(ctx,
				chromedp.Navigate(url),
				chromedp.WaitVisible(`body`, chromedp.ByQuery),
				chromedp.Sleep(500*time.Millisecond),
				chromedp.Evaluate(`document.querySelector("`+tt.selector+`") !== null`, &visible),
			)

			if err != nil {
				t.Fatalf("Failed to check element '%s': %v", tt.name, err)
			}

			if !visible {
				t.Errorf("Element '%s' (selector: %s) not found on page", tt.name, tt.selector)
			}
		})
	}
}

func TestChatHealthCheck(t *testing.T) {
	ctx, cancel := chromedp.NewContext(context.Background())
	defer cancel()

	ctx, cancel = context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	url := test.GetTestServerURL() + "/chat"

	var statusText string
	err := chromedp.Run(ctx,
		chromedp.Navigate(url),
		chromedp.WaitVisible(`#live-status`, chromedp.ByQuery),
		chromedp.Sleep(2*time.Second), // Wait for health check to complete
		chromedp.Text(`#live-status`, &statusText, chromedp.ByQuery),
	)

	if err != nil {
		t.Fatalf("Failed to read live status: %v", err)
	}

	if statusText == "" {
		t.Error("Live status text is empty")
	}

	t.Logf("Chat health status: %s", statusText)
}
