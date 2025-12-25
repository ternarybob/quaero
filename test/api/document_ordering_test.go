package api

import (
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/ternarybob/quaero/test/common"
)

// TestDocumentOrderingByCreatedAt tests that documents are returned in the correct order
// when sorting by created_at. This is critical for the email worker which needs to pick
// the NEWEST document (most recently created) when sending emails.
func TestDocumentOrderingByCreatedAt(t *testing.T) {
	env, err := common.SetupTestEnvironment(t.Name())
	require.NoError(t, err, "Failed to setup test environment")
	defer env.Cleanup()

	helper := env.NewHTTPTestHelper(t)

	// Use unique tag for this test
	testTag := fmt.Sprintf("email-order-test-%d", time.Now().UnixNano())
	olderDocID := fmt.Sprintf("test-doc-older-%d", time.Now().UnixNano())
	newerDocID := fmt.Sprintf("test-doc-newer-%d", time.Now().UnixNano())

	// Create first document (older)
	oldDoc := map[string]interface{}{
		"id":               olderDocID,
		"title":            "Older Document",
		"content_markdown": "This is the OLDER document content",
		"source_type":      "test",
		"tags":             []string{testTag, "test"},
	}
	resp, err := helper.POST("/api/documents", oldDoc)
	require.NoError(t, err, "Failed to create older document")
	resp.Body.Close()
	require.Equal(t, 201, resp.StatusCode, "Failed to create older document")
	defer helper.DELETE(fmt.Sprintf("/api/documents/%s", olderDocID))

	// Wait to ensure different created_at timestamps
	time.Sleep(1 * time.Second)

	// Create second document (newer)
	newDoc := map[string]interface{}{
		"id":               newerDocID,
		"title":            "Newer Document",
		"content_markdown": "This is the NEWER document content",
		"source_type":      "test",
		"tags":             []string{testTag, "test"},
	}
	resp, err = helper.POST("/api/documents", newDoc)
	require.NoError(t, err, "Failed to create newer document")
	resp.Body.Close()
	require.Equal(t, 201, resp.StatusCode, "Failed to create newer document")
	defer helper.DELETE(fmt.Sprintf("/api/documents/%s", newerDocID))

	// Test: Order by created_at DESC (newest first) - this is what email worker uses
	t.Run("OrderByCreatedAtDescReturnsNewestFirst", func(t *testing.T) {
		url := fmt.Sprintf("/api/documents?tags=%s&order_by=created_at&order_dir=desc&limit=10", testTag)
		resp, err := helper.GET(url)
		require.NoError(t, err, "Failed to list documents")
		defer resp.Body.Close()
		require.Equal(t, 200, resp.StatusCode, "Failed to list documents")

		// API returns paginated response: {"documents": [...], "total": N}
		var result struct {
			Documents []map[string]interface{} `json:"documents"`
			Total     int                      `json:"total"`
		}
		err = json.NewDecoder(resp.Body).Decode(&result)
		require.NoError(t, err, "Failed to decode response")
		require.Len(t, result.Documents, 2, "Expected 2 documents")

		// First document should be the NEWER one (created second)
		assert.Equal(t, newerDocID, result.Documents[0]["id"].(string),
			"EMAIL BUG: First document should be NEWER when ordering by created_at DESC")
		assert.Equal(t, olderDocID, result.Documents[1]["id"].(string),
			"Second document should be OLDER")
	})

	// Test: Limit 1 with DESC should return only the newest
	t.Run("Limit1ReturnsNewest", func(t *testing.T) {
		url := fmt.Sprintf("/api/documents?tags=%s&order_by=created_at&order_dir=desc&limit=1", testTag)
		resp, err := helper.GET(url)
		require.NoError(t, err, "Failed to list documents")
		defer resp.Body.Close()
		require.Equal(t, 200, resp.StatusCode, "Failed to list documents")

		// API returns paginated response: {"documents": [...], "total": N}
		var result struct {
			Documents []map[string]interface{} `json:"documents"`
			Total     int                      `json:"total"`
		}
		err = json.NewDecoder(resp.Body).Decode(&result)
		require.NoError(t, err, "Failed to decode response")
		require.Len(t, result.Documents, 1, "Expected 1 document with limit=1")

		// The one document should be the NEWER one
		assert.Equal(t, newerDocID, result.Documents[0]["id"].(string),
			"EMAIL BUG: With limit=1 and DESC order, should return NEWEST document")
	})
}
