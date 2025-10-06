package integration

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// Confluence-specific data structures

type spaceData struct {
	Key       string `json:"key"`
	Name      string `json:"name"`
	PageCount int    `json:"pageCount"`
}

// clearConfluenceData clears all Confluence data and verifies it's cleared
func clearConfluenceData(t *testing.T, serverURL string) {
	t.Helper()
	clearResp, err := http.Post(serverURL+"/api/data/confluence/clear", "application/json", nil)
	require.NoError(t, err)
	defer clearResp.Body.Close()

	require.Equal(t, http.StatusOK, clearResp.StatusCode)

	var result map[string]string
	require.NoError(t, json.NewDecoder(clearResp.Body).Decode(&result))
	require.Equal(t, "success", result["status"])
	t.Logf("✓ Confluence data cleared: %s", result["message"])
}

// scrapeAndWaitForSpaces scrapes spaces and waits for them to be available
func scrapeAndWaitForSpaces(t *testing.T, serverURL string, timeout time.Duration) []spaceData {
	t.Helper()

	// Trigger space scraping
	scrapeResp, err := http.Post(serverURL+"/api/scrape/spaces", "application/json", nil)
	require.NoError(t, err)
	defer scrapeResp.Body.Close()

	require.Equal(t, http.StatusOK, scrapeResp.StatusCode)

	var result map[string]string
	require.NoError(t, json.NewDecoder(scrapeResp.Body).Decode(&result))
	t.Logf("✓ Space scraping started: %s", result["message"])

	// Wait for spaces to be available
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			t.Fatalf("❌ Timeout: Spaces were not scraped within %v", timeout)
		case <-ticker.C:
			resp, err := http.Get(serverURL + "/api/collector/spaces")
			if err != nil {
				continue
			}

			body, err := io.ReadAll(resp.Body)
			resp.Body.Close()
			if err != nil {
				continue
			}

			var data struct {
				Data []spaceData `json:"data"`
			}

			if err := json.Unmarshal(body, &data); err != nil {
				continue
			}

			if len(data.Data) > 0 {
				t.Logf("✓ Spaces available: %d spaces", len(data.Data))
				return data.Data
			}
		}
	}
}

// collectAndWaitForPages collects pages for a space and waits for them
func collectAndWaitForPages(t *testing.T, serverURL string, spaceKey string, timeout time.Duration) int {
	t.Helper()

	// Trigger page collection
	requestBody := map[string]interface{}{
		"spaceKeys": []string{spaceKey},
	}

	requestJSON, err := json.Marshal(requestBody)
	require.NoError(t, err)

	collectResp, err := http.Post(
		serverURL+"/api/spaces/get-pages",
		"application/json",
		bytes.NewBuffer(requestJSON),
	)
	require.NoError(t, err)
	defer collectResp.Body.Close()

	require.Equal(t, http.StatusOK, collectResp.StatusCode)

	var result map[string]string
	require.NoError(t, json.NewDecoder(collectResp.Body).Decode(&result))
	t.Logf("✓ Page collection started: %s", result["message"])

	// Wait for pages to be available
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	pagesURL := fmt.Sprintf("%s/api/data/confluence/pages?spaceKey=%s", serverURL, spaceKey)

	for {
		select {
		case <-ctx.Done():
			t.Fatalf("❌ Timeout: Pages were not collected within %v", timeout)
		case <-ticker.C:
			resp, err := http.Get(pagesURL)
			if err != nil {
				continue
			}

			body, err := io.ReadAll(resp.Body)
			resp.Body.Close()
			if err != nil {
				continue
			}

			var data struct {
				Pages []map[string]interface{} `json:"pages"`
			}

			if err := json.Unmarshal(body, &data); err != nil {
				continue
			}

			count := len(data.Pages)
			if count > 0 {
				t.Logf("✓ Pages available: %d pages", count)
				return count
			}
		}
	}
}

// TestConfluencePagesCollection verifies the complete workflow of collecting pages
// This test follows the required workflow:
// 1. Clear all Confluence data - test passes if all Confluence data is deleted
// 2. Get spaces - test passes if space count > 0
// 3. Select space and get pages - test passes if page count > 0
func TestConfluencePagesCollection(t *testing.T) {
	serverURL := getServerURL()

	t.Log("=== Testing Confluence Pages Collection Workflow ===")

	// Step 1: Clear all Confluence data
	clearConfluenceData(t, serverURL)

	// Step 2: Scrape spaces and verify count > 0
	spaces := scrapeAndWaitForSpaces(t, serverURL, 30*time.Second)
	require.Greater(t, len(spaces), 0, "Space count should be > 0")

	// Step 3: Select a space and get pages - verify count > 0
	space := spaces[0]
	t.Logf("✓ Selected space: %s", space.Key)

	pageCount := collectAndWaitForPages(t, serverURL, space.Key, 60*time.Second)
	require.Greater(t, pageCount, 0, "Page count should be > 0")

	t.Log("\n✅ SUCCESS: Complete Confluence workflow verified")
}
