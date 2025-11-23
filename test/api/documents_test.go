package api

import (
	"fmt"
	"net/http"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/ternarybob/quaero/test/common"
)

// Counter for generating unique document IDs
var documentIDCounter uint64

// Helper functions for document test operations

// createTestDocument creates sample document data with required fields
func createTestDocument(sourceType, title string) map[string]interface{} {
	timestamp := time.Now().UnixNano()
	counter := atomic.AddUint64(&documentIDCounter, 1)
	return map[string]interface{}{
		"id":               fmt.Sprintf("doc_%d_%d", timestamp, counter),
		"source_type":      sourceType,
		"title":            title,
		"content_markdown": fmt.Sprintf("# %s\n\nThis is test content for %s", title, sourceType),
		"url":              fmt.Sprintf("https://example.com/%s/%d", sourceType, timestamp),
		"source_id":        fmt.Sprintf("%s-123", sourceType),
		"metadata": map[string]interface{}{
			"test_field": "test_value",
		},
		"tags": []string{"test"},
	}
}

// createTestDocumentWithMetadata creates document with custom metadata
func createTestDocumentWithMetadata(sourceType, title string, metadata map[string]interface{}) map[string]interface{} {
	doc := createTestDocument(sourceType, title)
	doc["metadata"] = metadata
	return doc
}

// createAndSaveTestDocument POSTs document and returns document ID
func createAndSaveTestDocument(t *testing.T, env *common.TestEnvironment, doc map[string]interface{}) string {
	helper := env.NewHTTPTestHelper(t)

	// POST document
	resp, err := helper.POST("/api/documents", doc)
	require.NoError(t, err, "Failed to create document")
	defer resp.Body.Close()

	helper.AssertStatusCode(resp, http.StatusCreated)

	// Parse response to get ID
	var result map[string]interface{}
	err = helper.ParseJSONResponse(resp, &result)
	require.NoError(t, err, "Failed to parse create response")

	id, ok := result["id"].(string)
	require.True(t, ok, "Response should contain id")
	require.NotEmpty(t, id, "Document ID should not be empty")

	t.Logf("Created test document: id=%s, source_type=%s", id, doc["source_type"])
	return id
}

// deleteTestDocument deletes document by ID
func deleteTestDocument(t *testing.T, env *common.TestEnvironment, id string) {
	helper := env.NewHTTPTestHelper(t)

	resp, err := helper.DELETE(fmt.Sprintf("/api/documents/%s", id))
	require.NoError(t, err, "Failed to delete document")
	defer resp.Body.Close()

	helper.AssertStatusCode(resp, http.StatusOK)

	var result map[string]interface{}
	err = helper.ParseJSONResponse(resp, &result)
	require.NoError(t, err, "Failed to parse delete response")

	assert.Equal(t, id, result["doc_id"], "Delete should return correct doc_id")
	t.Logf("Deleted test document: id=%s", id)
}

// cleanupAllDocuments deletes all documents via clear-all endpoint
func cleanupAllDocuments(t *testing.T, env *common.TestEnvironment) {
	helper := env.NewHTTPTestHelper(t)

	resp, err := helper.DELETE("/api/documents/clear-all")
	if err != nil {
		t.Logf("Failed to cleanup documents: %v", err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Logf("Cleanup returned status %d, continuing", resp.StatusCode)
		return
	}

	var result map[string]interface{}
	err = helper.ParseJSONResponse(resp, &result)
	if err != nil {
		t.Logf("Failed to parse cleanup response: %v", err)
		return
	}

	count, _ := result["documents_affected"].(float64)
	if count > 0 {
		t.Logf("Cleaned up %d documents", int(count))
	}
}

// Test functions

// TestDocumentsList tests GET /api/documents endpoint with pagination and filtering
func TestDocumentsList(t *testing.T) {
	env, err := common.SetupTestEnvironment(t.Name())
	require.NoError(t, err, "Failed to setup test environment")
	defer env.Cleanup()

	// Ensure clean state
	cleanupAllDocuments(t, env)
	defer cleanupAllDocuments(t, env)

	helper := env.NewHTTPTestHelper(t)

	t.Run("EmptyList", func(t *testing.T) {
		// Ensure no documents exist
		cleanupAllDocuments(t, env)

		// GET /api/documents
		resp, err := helper.GET("/api/documents")
		require.NoError(t, err)
		defer resp.Body.Close()

		helper.AssertStatusCode(resp, http.StatusOK)

		var result map[string]interface{}
		err = helper.ParseJSONResponse(resp, &result)
		require.NoError(t, err)

		documents, ok := result["documents"].([]interface{})
		require.True(t, ok, "Response should contain documents array")
		assert.Equal(t, 0, len(documents), "Should return empty array")

		t.Log("✓ Empty list test completed")
	})

	t.Run("SingleDocument", func(t *testing.T) {
		// Create one document
		doc := createTestDocument("jira", "Test Issue")
		docID := createAndSaveTestDocument(t, env, doc)

		// GET /api/documents
		resp, err := helper.GET("/api/documents")
		require.NoError(t, err)
		defer resp.Body.Close()

		helper.AssertStatusCode(resp, http.StatusOK)

		var result map[string]interface{}
		err = helper.ParseJSONResponse(resp, &result)
		require.NoError(t, err)

		documents, ok := result["documents"].([]interface{})
		require.True(t, ok)
		assert.Equal(t, 1, len(documents), "Should return 1 document")

		// Verify document fields
		firstDoc := documents[0].(map[string]interface{})
		assert.NotEmpty(t, firstDoc["id"])
		assert.NotEmpty(t, firstDoc["source_type"])
		assert.NotEmpty(t, firstDoc["title"])
		assert.NotEmpty(t, firstDoc["content_markdown"])
		assert.NotEmpty(t, firstDoc["created_at"])
		assert.NotEmpty(t, firstDoc["updated_at"])

		// Cleanup
		deleteTestDocument(t, env, docID)

		t.Log("✓ Single document test completed")
	})

	t.Run("MultipleDocuments", func(t *testing.T) {
		// Create 5 documents with different source types
		var ids []string
		for i := 0; i < 5; i++ {
			sourceType := "jira"
			if i%2 == 0 {
				sourceType = "confluence"
			}
			doc := createTestDocument(sourceType, fmt.Sprintf("Document %d", i))
			id := createAndSaveTestDocument(t, env, doc)
			ids = append(ids, id)
		}

		// GET /api/documents
		resp, err := helper.GET("/api/documents")
		require.NoError(t, err)
		defer resp.Body.Close()

		helper.AssertStatusCode(resp, http.StatusOK)

		var result map[string]interface{}
		err = helper.ParseJSONResponse(resp, &result)
		require.NoError(t, err)

		documents, ok := result["documents"].([]interface{})
		require.True(t, ok)
		assert.Equal(t, 5, len(documents), "Should return 5 documents")

		// Cleanup
		for _, id := range ids {
			deleteTestDocument(t, env, id)
		}

		t.Log("✓ Multiple documents test completed")
	})

	t.Run("Pagination", func(t *testing.T) {
		// Create 25 documents
		var ids []string
		for i := 0; i < 25; i++ {
			doc := createTestDocument("jira", fmt.Sprintf("Issue %d", i))
			id := createAndSaveTestDocument(t, env, doc)
			ids = append(ids, id)
		}

		// Test first page: limit=10 offset=0
		resp, err := helper.GET("/api/documents?limit=10&offset=0")
		require.NoError(t, err)
		defer resp.Body.Close()

		var result map[string]interface{}
		err = helper.ParseJSONResponse(resp, &result)
		require.NoError(t, err)

		documents := result["documents"].([]interface{})
		assert.LessOrEqual(t, len(documents), 10, "First page should have max 10 documents")
		assert.Equal(t, float64(10), result["limit"])
		assert.Equal(t, float64(0), result["offset"])

		// Test second page: limit=10 offset=10
		resp, err = helper.GET("/api/documents?limit=10&offset=10")
		require.NoError(t, err)
		defer resp.Body.Close()

		err = helper.ParseJSONResponse(resp, &result)
		require.NoError(t, err)

		documents = result["documents"].([]interface{})
		assert.LessOrEqual(t, len(documents), 10, "Second page should have max 10 documents")
		assert.Equal(t, float64(10), result["limit"])
		assert.Equal(t, float64(10), result["offset"])

		// Cleanup
		for _, id := range ids {
			deleteTestDocument(t, env, id)
		}

		t.Log("✓ Pagination test completed")
	})

	t.Run("FilterBySourceType", func(t *testing.T) {
		// Create documents with different source types
		jiraDoc := createTestDocument("jira", "Jira Issue")
		jiraID := createAndSaveTestDocument(t, env, jiraDoc)

		confluenceDoc := createTestDocument("confluence", "Confluence Page")
		confluenceID := createAndSaveTestDocument(t, env, confluenceDoc)

		// GET /api/documents?source_type=jira
		resp, err := helper.GET("/api/documents?source_type=jira")
		require.NoError(t, err)
		defer resp.Body.Close()

		var result map[string]interface{}
		err = helper.ParseJSONResponse(resp, &result)
		require.NoError(t, err)

		documents := result["documents"].([]interface{})
		for _, d := range documents {
			doc := d.(map[string]interface{})
			assert.Equal(t, "jira", doc["source_type"], "Should only return jira documents")
		}

		// Cleanup
		deleteTestDocument(t, env, jiraID)
		deleteTestDocument(t, env, confluenceID)

		t.Log("✓ Filter by source type test completed")
	})

	t.Log("✓ TestDocumentsList completed successfully")
}

// TestDocumentsCreate tests POST /api/documents endpoint
func TestDocumentsCreate(t *testing.T) {
	env, err := common.SetupTestEnvironment(t.Name())
	require.NoError(t, err, "Failed to setup test environment")
	defer env.Cleanup()

	cleanupAllDocuments(t, env)
	defer cleanupAllDocuments(t, env)

	helper := env.NewHTTPTestHelper(t)

	t.Run("Success", func(t *testing.T) {
		// POST valid document
		doc := createTestDocument("jira", "Test Issue")
		resp, err := helper.POST("/api/documents", doc)
		require.NoError(t, err)
		defer resp.Body.Close()

		helper.AssertStatusCode(resp, http.StatusCreated)

		var result map[string]interface{}
		err = helper.ParseJSONResponse(resp, &result)
		require.NoError(t, err)

		// Verify response contains expected fields
		assert.NotEmpty(t, result["id"])
		assert.Equal(t, "jira", result["source_type"])
		assert.NotEmpty(t, result["title"])

		// Verify document is retrievable
		docID := result["id"].(string)
		getResp, err := helper.GET(fmt.Sprintf("/api/documents/%s", docID))
		require.NoError(t, err)
		defer getResp.Body.Close()

		helper.AssertStatusCode(getResp, http.StatusOK)

		// Cleanup
		deleteTestDocument(t, env, docID)

		t.Log("✓ Success test completed")
	})

	t.Run("WithMetadata", func(t *testing.T) {
		// POST document with custom metadata
		metadata := map[string]interface{}{
			"project":  "TEST",
			"priority": "high",
			"assignee": "alice",
		}
		doc := createTestDocumentWithMetadata("jira", "Test with Metadata", metadata)
		docID := createAndSaveTestDocument(t, env, doc)

		// Verify metadata stored correctly
		getResp, err := helper.GET(fmt.Sprintf("/api/documents/%s", docID))
		require.NoError(t, err)
		defer getResp.Body.Close()

		var result map[string]interface{}
		err = helper.ParseJSONResponse(getResp, &result)
		require.NoError(t, err)

		storedMetadata, ok := result["metadata"].(map[string]interface{})
		require.True(t, ok)
		assert.Equal(t, "TEST", storedMetadata["project"])
		assert.Equal(t, "high", storedMetadata["priority"])

		// Cleanup
		deleteTestDocument(t, env, docID)

		t.Log("✓ With metadata test completed")
	})

	t.Run("InvalidJSON", func(t *testing.T) {
		// POST malformed JSON
		resp, err := helper.POSTBody("/api/documents", "application/json", []byte("invalid json {"))
		require.NoError(t, err)
		defer resp.Body.Close()

		helper.AssertStatusCode(resp, http.StatusBadRequest)

		t.Log("✓ Invalid JSON test completed")
	})

	t.Run("MissingID", func(t *testing.T) {
		// POST document without ID field
		doc := map[string]interface{}{
			"source_type":      "jira",
			"title":            "Missing ID",
			"content_markdown": "Test content",
		}

		resp, err := helper.POST("/api/documents", doc)
		require.NoError(t, err)
		defer resp.Body.Close()

		helper.AssertStatusCode(resp, http.StatusBadRequest)

		t.Log("✓ Missing ID test completed")
	})

	t.Run("MissingSourceType", func(t *testing.T) {
		// POST document without source_type field
		doc := map[string]interface{}{
			"id":               "doc_test_123",
			"title":            "Missing Source Type",
			"content_markdown": "Test content",
		}

		resp, err := helper.POST("/api/documents", doc)
		require.NoError(t, err)
		defer resp.Body.Close()

		helper.AssertStatusCode(resp, http.StatusBadRequest)

		t.Log("✓ Missing source type test completed")
	})

	t.Run("EmptyID", func(t *testing.T) {
		// POST document with empty string ID
		doc := map[string]interface{}{
			"id":               "",
			"source_type":      "jira",
			"title":            "Empty ID",
			"content_markdown": "Test content",
		}

		resp, err := helper.POST("/api/documents", doc)
		require.NoError(t, err)
		defer resp.Body.Close()

		helper.AssertStatusCode(resp, http.StatusBadRequest)

		t.Log("✓ Empty ID test completed")
	})

	t.Log("✓ TestDocumentsCreate completed successfully")
}

// TestDocumentsGet tests GET /api/documents/{id} endpoint
func TestDocumentsGet(t *testing.T) {
	env, err := common.SetupTestEnvironment(t.Name())
	require.NoError(t, err, "Failed to setup test environment")
	defer env.Cleanup()

	cleanupAllDocuments(t, env)
	defer cleanupAllDocuments(t, env)

	helper := env.NewHTTPTestHelper(t)

	t.Run("Success", func(t *testing.T) {
		// Create document
		doc := createTestDocument("jira", "Test Issue")
		docID := createAndSaveTestDocument(t, env, doc)

		// GET by ID
		resp, err := helper.GET(fmt.Sprintf("/api/documents/%s", docID))
		require.NoError(t, err)
		defer resp.Body.Close()

		helper.AssertStatusCode(resp, http.StatusOK)

		var result map[string]interface{}
		err = helper.ParseJSONResponse(resp, &result)
		require.NoError(t, err)

		// Verify all fields match
		assert.Equal(t, docID, result["id"])
		assert.Equal(t, "jira", result["source_type"])
		assert.Equal(t, "Test Issue", result["title"])
		assert.NotEmpty(t, result["content_markdown"])
		assert.NotEmpty(t, result["created_at"])
		assert.NotEmpty(t, result["updated_at"])

		// Cleanup
		deleteTestDocument(t, env, docID)

		t.Log("✓ Success test completed")
	})

	t.Run("NotFound", func(t *testing.T) {
		// GET with nonexistent ID
		resp, err := helper.GET("/api/documents/nonexistent-id")
		require.NoError(t, err)
		defer resp.Body.Close()

		helper.AssertStatusCode(resp, http.StatusNotFound)

		t.Log("✓ Not found test completed")
	})

	t.Run("EmptyID", func(t *testing.T) {
		// GET with empty ID path
		resp, err := helper.GET("/api/documents/")
		require.NoError(t, err)
		defer resp.Body.Close()

		// Should return 400 or 404
		assert.True(t, resp.StatusCode == http.StatusBadRequest || resp.StatusCode == http.StatusNotFound,
			"Empty ID should return 400 or 404")

		t.Log("✓ Empty ID test completed")
	})

	t.Log("✓ TestDocumentsGet completed successfully")
}

// TestDocumentsDelete tests DELETE /api/documents/{id} endpoint
func TestDocumentsDelete(t *testing.T) {
	env, err := common.SetupTestEnvironment(t.Name())
	require.NoError(t, err, "Failed to setup test environment")
	defer env.Cleanup()

	cleanupAllDocuments(t, env)
	defer cleanupAllDocuments(t, env)

	helper := env.NewHTTPTestHelper(t)

	t.Run("Success", func(t *testing.T) {
		// Create document
		doc := createTestDocument("jira", "Test Issue")
		docID := createAndSaveTestDocument(t, env, doc)

		// DELETE by ID
		resp, err := helper.DELETE(fmt.Sprintf("/api/documents/%s", docID))
		require.NoError(t, err)
		defer resp.Body.Close()

		helper.AssertStatusCode(resp, http.StatusOK)

		var result map[string]interface{}
		err = helper.ParseJSONResponse(resp, &result)
		require.NoError(t, err)

		assert.Equal(t, docID, result["doc_id"])
		assert.NotEmpty(t, result["message"])

		// Verify GET returns 404
		getResp, err := helper.GET(fmt.Sprintf("/api/documents/%s", docID))
		require.NoError(t, err)
		defer getResp.Body.Close()

		helper.AssertStatusCode(getResp, http.StatusNotFound)

		t.Log("✓ Success test completed")
	})

	t.Run("NotFound", func(t *testing.T) {
		// DELETE nonexistent ID
		resp, err := helper.DELETE("/api/documents/nonexistent-id")
		require.NoError(t, err)
		defer resp.Body.Close()

		// Should return 500 Internal Server Error (per handler implementation)
		helper.AssertStatusCode(resp, http.StatusInternalServerError)

		t.Log("✓ Not found test completed")
	})

	t.Run("EmptyID", func(t *testing.T) {
		// DELETE with empty ID
		resp, err := helper.DELETE("/api/documents/")
		require.NoError(t, err)
		defer resp.Body.Close()

		helper.AssertStatusCode(resp, http.StatusBadRequest)

		t.Log("✓ Empty ID test completed")
	})

	t.Log("✓ TestDocumentsDelete completed successfully")
}

// TestDocumentsStats tests GET /api/documents/stats endpoint
func TestDocumentsStats(t *testing.T) {
	env, err := common.SetupTestEnvironment(t.Name())
	require.NoError(t, err, "Failed to setup test environment")
	defer env.Cleanup()

	cleanupAllDocuments(t, env)
	defer cleanupAllDocuments(t, env)

	helper := env.NewHTTPTestHelper(t)

	t.Run("EmptyDatabase", func(t *testing.T) {
		// Ensure no documents exist
		cleanupAllDocuments(t, env)

		// GET stats
		resp, err := helper.GET("/api/documents/stats")
		require.NoError(t, err)
		defer resp.Body.Close()

		helper.AssertStatusCode(resp, http.StatusOK)

		var result map[string]interface{}
		err = helper.ParseJSONResponse(resp, &result)
		require.NoError(t, err)

		assert.Equal(t, float64(0), result["total_documents"])

		t.Log("✓ Empty database test completed")
	})

	t.Run("SingleDocument", func(t *testing.T) {
		// Create 1 document
		doc := createTestDocument("jira", "Single Doc")
		docID := createAndSaveTestDocument(t, env, doc)

		// GET stats
		resp, err := helper.GET("/api/documents/stats")
		require.NoError(t, err)
		defer resp.Body.Close()

		var result map[string]interface{}
		err = helper.ParseJSONResponse(resp, &result)
		require.NoError(t, err)

		assert.Equal(t, float64(1), result["total_documents"])

		// Cleanup
		deleteTestDocument(t, env, docID)

		t.Log("✓ Single document test completed")
	})

	t.Run("MultipleSourceTypes", func(t *testing.T) {
		// Create 3 jira, 2 confluence, 1 github
		var ids []string
		for i := 0; i < 3; i++ {
			doc := createTestDocument("jira", fmt.Sprintf("Jira %d", i))
			ids = append(ids, createAndSaveTestDocument(t, env, doc))
		}
		for i := 0; i < 2; i++ {
			doc := createTestDocument("confluence", fmt.Sprintf("Confluence %d", i))
			ids = append(ids, createAndSaveTestDocument(t, env, doc))
		}
		doc := createTestDocument("github", "GitHub Doc")
		ids = append(ids, createAndSaveTestDocument(t, env, doc))

		// GET stats
		resp, err := helper.GET("/api/documents/stats")
		require.NoError(t, err)
		defer resp.Body.Close()

		var result map[string]interface{}
		err = helper.ParseJSONResponse(resp, &result)
		require.NoError(t, err)

		assert.Equal(t, float64(6), result["total_documents"])

		documentsBySource, ok := result["documents_by_source"].(map[string]interface{})
		require.True(t, ok)
		assert.Equal(t, float64(3), documentsBySource["jira"])
		assert.Equal(t, float64(2), documentsBySource["confluence"])
		assert.Equal(t, float64(1), documentsBySource["github"])

		// Cleanup
		for _, id := range ids {
			deleteTestDocument(t, env, id)
		}

		t.Log("✓ Multiple source types test completed")
	})

	t.Log("✓ TestDocumentsStats completed successfully")
}

// TestDocumentsTags tests GET /api/documents/tags endpoint
func TestDocumentsTags(t *testing.T) {
	env, err := common.SetupTestEnvironment(t.Name())
	require.NoError(t, err, "Failed to setup test environment")
	defer env.Cleanup()

	cleanupAllDocuments(t, env)
	defer cleanupAllDocuments(t, env)

	helper := env.NewHTTPTestHelper(t)

	t.Run("EmptyDatabase", func(t *testing.T) {
		// Ensure no documents exist
		cleanupAllDocuments(t, env)

		// GET tags
		resp, err := helper.GET("/api/documents/tags")
		require.NoError(t, err)
		defer resp.Body.Close()

		helper.AssertStatusCode(resp, http.StatusOK)

		var result map[string]interface{}
		err = helper.ParseJSONResponse(resp, &result)
		require.NoError(t, err)

		tags, ok := result["tags"].([]interface{})
		require.True(t, ok, "Response should contain tags array")
		assert.Equal(t, 0, len(tags), "Tags should be empty")

		t.Log("✓ Empty database test completed")
	})

	t.Log("✓ TestDocumentsTags completed successfully")
}

// TestDocumentsClearAll tests DELETE /api/documents/clear-all endpoint
func TestDocumentsClearAll(t *testing.T) {
	env, err := common.SetupTestEnvironment(t.Name())
	require.NoError(t, err, "Failed to setup test environment")
	defer env.Cleanup()

	cleanupAllDocuments(t, env)
	defer cleanupAllDocuments(t, env)

	helper := env.NewHTTPTestHelper(t)

	t.Run("Success", func(t *testing.T) {
		// Create 10 documents
		for i := 0; i < 10; i++ {
			doc := createTestDocument("jira", fmt.Sprintf("Doc %d", i))
			createAndSaveTestDocument(t, env, doc)
		}

		// DELETE clear-all
		resp, err := helper.DELETE("/api/documents/clear-all")
		require.NoError(t, err)
		defer resp.Body.Close()

		helper.AssertStatusCode(resp, http.StatusOK)

		var result map[string]interface{}
		err = helper.ParseJSONResponse(resp, &result)
		require.NoError(t, err)

		assert.NotEmpty(t, result["message"])
		assert.Equal(t, float64(10), result["documents_affected"])

		// Verify list is empty
		listResp, err := helper.GET("/api/documents")
		require.NoError(t, err)
		defer listResp.Body.Close()

		var listResult map[string]interface{}
		err = helper.ParseJSONResponse(listResp, &listResult)
		require.NoError(t, err)

		documents := listResult["documents"].([]interface{})
		assert.Equal(t, 0, len(documents), "List should be empty after clear-all")

		t.Log("✓ Success test completed")
	})

	t.Run("EmptyDatabase", func(t *testing.T) {
		// Cleanup all first
		cleanupAllDocuments(t, env)

		// DELETE clear-all on empty database
		resp, err := helper.DELETE("/api/documents/clear-all")
		require.NoError(t, err)
		defer resp.Body.Close()

		helper.AssertStatusCode(resp, http.StatusOK)

		var result map[string]interface{}
		err = helper.ParseJSONResponse(resp, &result)
		require.NoError(t, err)

		assert.Equal(t, float64(0), result["documents_affected"])

		t.Log("✓ Empty database test completed")
	})

	t.Log("✓ TestDocumentsClearAll completed successfully")
}
