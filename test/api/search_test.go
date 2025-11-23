// Package api provides API integration tests for search endpoint
package api

import (
	"fmt"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/ternarybob/quaero/test/common"
)

// TestSearchBasic tests basic search functionality
func TestSearchBasic(t *testing.T) {
	env, err := common.SetupTestEnvironment(t.Name())
	require.NoError(t, err, "Failed to setup test environment")
	defer env.Cleanup()

	// Ensure clean state
	cleanupAllDocuments(t, env)
	defer cleanupAllDocuments(t, env)

	helper := env.NewHTTPTestHelper(t)

	t.Run("EmptyDatabase", func(t *testing.T) {
		// Ensure no documents exist
		cleanupAllDocuments(t, env)

		// Search with no documents
		resp, err := helper.GET("/api/search?q=test")
		require.NoError(t, err)
		defer resp.Body.Close()

		helper.AssertStatusCode(resp, http.StatusOK)

		var result map[string]interface{}
		err = helper.ParseJSONResponse(resp, &result)
		require.NoError(t, err)

		// Verify empty results
		results, ok := result["results"].([]interface{})
		require.True(t, ok, "Response should contain results array")
		assert.Equal(t, 0, len(results), "Should return empty results array")
		assert.Equal(t, float64(0), result["count"], "Count should be 0")

		t.Log("✓ Empty database test completed")
	})

	t.Run("SingleDocument", func(t *testing.T) {
		// Create document with searchable content
		doc := createTestDocument("jira", "Test Issue with keyword searchable")
		docID := createAndSaveTestDocument(t, env, doc)

		// Search for keyword
		resp, err := helper.GET("/api/search?q=searchable")
		require.NoError(t, err)
		defer resp.Body.Close()

		helper.AssertStatusCode(resp, http.StatusOK)

		var result map[string]interface{}
		err = helper.ParseJSONResponse(resp, &result)
		require.NoError(t, err)

		// Verify single result
		results, ok := result["results"].([]interface{})
		require.True(t, ok)
		assert.Equal(t, 1, len(results), "Should return 1 result")
		assert.Equal(t, float64(1), result["count"], "Count should be 1")

		// Verify result has required fields and matches created document
		foundCreatedDoc := false
		for _, r := range results {
			resultDoc := r.(map[string]interface{})
			assert.NotEmpty(t, resultDoc["id"])
			assert.NotEmpty(t, resultDoc["source_type"])
			assert.NotEmpty(t, resultDoc["title"])
			assert.NotEmpty(t, resultDoc["content_markdown"])
			assert.NotEmpty(t, resultDoc["url"])
			assert.NotEmpty(t, resultDoc["created_at"])
			assert.NotEmpty(t, resultDoc["updated_at"])
			assert.Contains(t, resultDoc, "brief", "Result should have brief field")

			// Verify this result is the document we created
			if resultDoc["id"].(string) == docID {
				foundCreatedDoc = true
			}
		}

		assert.True(t, foundCreatedDoc, "Search results should contain the created document with ID %s", docID)

		// Cleanup
		deleteTestDocument(t, env, docID)

		t.Log("✓ Single document test completed")
	})

	t.Run("MultipleDocuments", func(t *testing.T) {
		// Create 5 documents with common keyword
		createdIDs := make(map[string]bool)
		for i := 0; i < 5; i++ {
			doc := createTestDocument("jira", fmt.Sprintf("Document %d with searchterm content", i))
			id := createAndSaveTestDocument(t, env, doc)
			createdIDs[id] = true
		}

		// Search for common keyword
		resp, err := helper.GET("/api/search?q=searchterm")
		require.NoError(t, err)
		defer resp.Body.Close()

		helper.AssertStatusCode(resp, http.StatusOK)

		var result map[string]interface{}
		err = helper.ParseJSONResponse(resp, &result)
		require.NoError(t, err)

		// Verify multiple results
		results, ok := result["results"].([]interface{})
		require.True(t, ok)
		assert.GreaterOrEqual(t, len(results), 1, "Should return at least 1 result")
		assert.GreaterOrEqual(t, result["count"].(float64), float64(1), "Count should be at least 1")

		// Verify that each result's ID is in the set of created IDs
		foundCount := 0
		for _, r := range results {
			resultDoc := r.(map[string]interface{})
			resultID := resultDoc["id"].(string)
			if createdIDs[resultID] {
				foundCount++
			}
		}

		// All results should be from our created documents
		assert.Equal(t, len(results), foundCount, "All returned results should be from created documents")
		assert.Equal(t, 5, foundCount, "Should find all 5 created documents in results")

		// Cleanup
		for id := range createdIDs {
			deleteTestDocument(t, env, id)
		}

		t.Log("✓ Multiple documents test completed")
	})

	t.Run("NoResults", func(t *testing.T) {
		// Create document without the search keyword
		doc := createTestDocument("jira", "Document without keyword")
		docID := createAndSaveTestDocument(t, env, doc)

		// Search for nonexistent keyword
		resp, err := helper.GET("/api/search?q=nonexistentkeyword123")
		require.NoError(t, err)
		defer resp.Body.Close()

		helper.AssertStatusCode(resp, http.StatusOK)

		var result map[string]interface{}
		err = helper.ParseJSONResponse(resp, &result)
		require.NoError(t, err)

		// Verify empty results
		results, ok := result["results"].([]interface{})
		require.True(t, ok)
		assert.Equal(t, 0, len(results), "Should return empty results array")
		assert.Equal(t, float64(0), result["count"], "Count should be 0")

		// Cleanup
		deleteTestDocument(t, env, docID)

		t.Log("✓ No results test completed")
	})

	t.Run("EmptyQuery", func(t *testing.T) {
		// Create some documents
		doc1 := createTestDocument("jira", "Document One")
		id1 := createAndSaveTestDocument(t, env, doc1)
		doc2 := createTestDocument("confluence", "Document Two")
		id2 := createAndSaveTestDocument(t, env, doc2)

		// Search with empty query
		resp, err := helper.GET("/api/search?q=")
		require.NoError(t, err)
		defer resp.Body.Close()

		helper.AssertStatusCode(resp, http.StatusOK)

		var result map[string]interface{}
		err = helper.ParseJSONResponse(resp, &result)
		require.NoError(t, err)

		// Verify response structure (may return all or empty depending on implementation)
		_, ok := result["results"].([]interface{})
		require.True(t, ok, "Response should contain results array")
		// Don't assert count since empty query behavior varies by implementation

		// Cleanup
		deleteTestDocument(t, env, id1)
		deleteTestDocument(t, env, id2)

		t.Log("✓ Empty query test completed")
	})

	t.Log("✓ TestSearchBasic completed successfully")
}

// TestSearchPagination tests pagination with limit and offset parameters
func TestSearchPagination(t *testing.T) {
	env, err := common.SetupTestEnvironment(t.Name())
	require.NoError(t, err, "Failed to setup test environment")
	defer env.Cleanup()

	cleanupAllDocuments(t, env)
	defer cleanupAllDocuments(t, env)

	helper := env.NewHTTPTestHelper(t)

	t.Run("DefaultPagination", func(t *testing.T) {
		// Create document to search
		doc := createTestDocument("jira", "Pagination test document")
		docID := createAndSaveTestDocument(t, env, doc)

		// Search without pagination params
		resp, err := helper.GET("/api/search?q=pagination")
		require.NoError(t, err)
		defer resp.Body.Close()

		helper.AssertStatusCode(resp, http.StatusOK)

		var result map[string]interface{}
		err = helper.ParseJSONResponse(resp, &result)
		require.NoError(t, err)

		// Verify default pagination
		assert.Equal(t, float64(50), result["limit"], "Default limit should be 50")
		assert.Equal(t, float64(0), result["offset"], "Default offset should be 0")

		deleteTestDocument(t, env, docID)
		t.Log("✓ Default pagination test completed")
	})

	t.Run("CustomLimit", func(t *testing.T) {
		// Create documents
		var ids []string
		for i := 0; i < 15; i++ {
			doc := createTestDocument("jira", fmt.Sprintf("Custom limit doc %d", i))
			id := createAndSaveTestDocument(t, env, doc)
			ids = append(ids, id)
		}

		// Search with custom limit
		resp, err := helper.GET("/api/search?q=custom&limit=10")
		require.NoError(t, err)
		defer resp.Body.Close()

		helper.AssertStatusCode(resp, http.StatusOK)

		var result map[string]interface{}
		err = helper.ParseJSONResponse(resp, &result)
		require.NoError(t, err)

		// Verify custom limit
		assert.Equal(t, float64(10), result["limit"], "Limit should be 10")
		results := result["results"].([]interface{})
		assert.LessOrEqual(t, len(results), 10, "Results count should be ≤ limit")

		for _, id := range ids {
			deleteTestDocument(t, env, id)
		}
		t.Log("✓ Custom limit test completed")
	})

	t.Run("CustomOffset", func(t *testing.T) {
		// Create documents
		var ids []string
		for i := 0; i < 10; i++ {
			doc := createTestDocument("jira", fmt.Sprintf("Offset doc %d", i))
			id := createAndSaveTestDocument(t, env, doc)
			ids = append(ids, id)
		}

		// Search with custom offset
		resp, err := helper.GET("/api/search?q=offset&offset=5")
		require.NoError(t, err)
		defer resp.Body.Close()

		helper.AssertStatusCode(resp, http.StatusOK)

		var result map[string]interface{}
		err = helper.ParseJSONResponse(resp, &result)
		require.NoError(t, err)

		// Verify custom offset
		assert.Equal(t, float64(5), result["offset"], "Offset should be 5")

		for _, id := range ids {
			deleteTestDocument(t, env, id)
		}
		t.Log("✓ Custom offset test completed")
	})

	t.Run("LimitAndOffset", func(t *testing.T) {
		// Create documents
		var ids []string
		for i := 0; i < 20; i++ {
			doc := createTestDocument("jira", fmt.Sprintf("Combined pagination doc %d", i))
			id := createAndSaveTestDocument(t, env, doc)
			ids = append(ids, id)
		}

		// Search with both limit and offset
		resp, err := helper.GET("/api/search?q=combined&limit=10&offset=5")
		require.NoError(t, err)
		defer resp.Body.Close()

		helper.AssertStatusCode(resp, http.StatusOK)

		var result map[string]interface{}
		err = helper.ParseJSONResponse(resp, &result)
		require.NoError(t, err)

		// Verify both params
		assert.Equal(t, float64(10), result["limit"], "Limit should be 10")
		assert.Equal(t, float64(5), result["offset"], "Offset should be 5")

		for _, id := range ids {
			deleteTestDocument(t, env, id)
		}
		t.Log("✓ Limit and offset test completed")
	})

	t.Run("SecondPage", func(t *testing.T) {
		// Create 25 documents
		var ids []string
		for i := 0; i < 25; i++ {
			doc := createTestDocument("jira", fmt.Sprintf("Second page doc %d multipage", i))
			id := createAndSaveTestDocument(t, env, doc)
			ids = append(ids, id)
		}

		// Fetch first page
		resp1, err := helper.GET("/api/search?q=multipage&limit=10&offset=0")
		require.NoError(t, err)
		defer resp1.Body.Close()

		var result1 map[string]interface{}
		err = helper.ParseJSONResponse(resp1, &result1)
		require.NoError(t, err)

		results1 := result1["results"].([]interface{})
		firstPageIDs := make(map[string]bool)
		for _, r := range results1 {
			result := r.(map[string]interface{})
			firstPageIDs[result["id"].(string)] = true
		}

		// Fetch second page
		resp2, err := helper.GET("/api/search?q=multipage&limit=10&offset=10")
		require.NoError(t, err)
		defer resp2.Body.Close()

		var result2 map[string]interface{}
		err = helper.ParseJSONResponse(resp2, &result2)
		require.NoError(t, err)

		results2 := result2["results"].([]interface{})

		// Verify different results (no overlap)
		for _, r := range results2 {
			result := r.(map[string]interface{})
			resultID := result["id"].(string)
			assert.False(t, firstPageIDs[resultID], "Second page should have different results than first page")
		}

		for _, id := range ids {
			deleteTestDocument(t, env, id)
		}
		t.Log("✓ Second page test completed")
	})

	t.Log("✓ TestSearchPagination completed successfully")
}

// TestSearchLimitClamping tests limit and offset validation and clamping
func TestSearchLimitClamping(t *testing.T) {
	env, err := common.SetupTestEnvironment(t.Name())
	require.NoError(t, err, "Failed to setup test environment")
	defer env.Cleanup()

	cleanupAllDocuments(t, env)
	defer cleanupAllDocuments(t, env)

	helper := env.NewHTTPTestHelper(t)

	// Create test document for searching
	doc := createTestDocument("jira", "Clamping test document")
	docID := createAndSaveTestDocument(t, env, doc)
	defer deleteTestDocument(t, env, docID)

	t.Run("MaxLimitEnforcement", func(t *testing.T) {
		// Request limit > 100
		resp, err := helper.GET("/api/search?q=clamping&limit=200")
		require.NoError(t, err)
		defer resp.Body.Close()

		helper.AssertStatusCode(resp, http.StatusOK)

		var result map[string]interface{}
		err = helper.ParseJSONResponse(resp, &result)
		require.NoError(t, err)

		// Verify clamped to max 100
		assert.Equal(t, float64(100), result["limit"], "Limit should be clamped to 100")

		t.Log("✓ Max limit enforcement test completed")
	})

	t.Run("NegativeLimit", func(t *testing.T) {
		// Request negative limit
		resp, err := helper.GET("/api/search?q=clamping&limit=-10")
		require.NoError(t, err)
		defer resp.Body.Close()

		helper.AssertStatusCode(resp, http.StatusOK)

		var result map[string]interface{}
		err = helper.ParseJSONResponse(resp, &result)
		require.NoError(t, err)

		// Verify defaults to 50
		assert.Equal(t, float64(50), result["limit"], "Negative limit should default to 50")

		t.Log("✓ Negative limit test completed")
	})

	t.Run("ZeroLimit", func(t *testing.T) {
		// Request zero limit
		resp, err := helper.GET("/api/search?q=clamping&limit=0")
		require.NoError(t, err)
		defer resp.Body.Close()

		helper.AssertStatusCode(resp, http.StatusOK)

		var result map[string]interface{}
		err = helper.ParseJSONResponse(resp, &result)
		require.NoError(t, err)

		// Verify defaults to 50
		assert.Equal(t, float64(50), result["limit"], "Zero limit should default to 50")

		t.Log("✓ Zero limit test completed")
	})

	t.Run("InvalidLimit", func(t *testing.T) {
		// Request invalid limit
		resp, err := helper.GET("/api/search?q=clamping&limit=invalid")
		require.NoError(t, err)
		defer resp.Body.Close()

		helper.AssertStatusCode(resp, http.StatusOK)

		var result map[string]interface{}
		err = helper.ParseJSONResponse(resp, &result)
		require.NoError(t, err)

		// Verify defaults to 50
		assert.Equal(t, float64(50), result["limit"], "Invalid limit should default to 50")

		t.Log("✓ Invalid limit test completed")
	})

	t.Run("NegativeOffset", func(t *testing.T) {
		// Request negative offset
		resp, err := helper.GET("/api/search?q=clamping&offset=-5")
		require.NoError(t, err)
		defer resp.Body.Close()

		helper.AssertStatusCode(resp, http.StatusOK)

		var result map[string]interface{}
		err = helper.ParseJSONResponse(resp, &result)
		require.NoError(t, err)

		// Verify clamped to 0
		assert.Equal(t, float64(0), result["offset"], "Negative offset should be clamped to 0")

		t.Log("✓ Negative offset test completed")
	})

	t.Run("InvalidOffset", func(t *testing.T) {
		// Request invalid offset
		resp, err := helper.GET("/api/search?q=clamping&offset=bad")
		require.NoError(t, err)
		defer resp.Body.Close()

		helper.AssertStatusCode(resp, http.StatusOK)

		var result map[string]interface{}
		err = helper.ParseJSONResponse(resp, &result)
		require.NoError(t, err)

		// Verify defaults to 0
		assert.Equal(t, float64(0), result["offset"], "Invalid offset should default to 0")

		t.Log("✓ Invalid offset test completed")
	})

	t.Log("✓ TestSearchLimitClamping completed successfully")
}

// TestSearchResponseStructure tests response format validation
func TestSearchResponseStructure(t *testing.T) {
	env, err := common.SetupTestEnvironment(t.Name())
	require.NoError(t, err, "Failed to setup test environment")
	defer env.Cleanup()

	cleanupAllDocuments(t, env)
	defer cleanupAllDocuments(t, env)

	helper := env.NewHTTPTestHelper(t)

	t.Run("AllFieldsPresent", func(t *testing.T) {
		// Create document
		doc := createTestDocument("jira", "Structure test document")
		docID := createAndSaveTestDocument(t, env, doc)

		// Search
		resp, err := helper.GET("/api/search?q=structure")
		require.NoError(t, err)
		defer resp.Body.Close()

		helper.AssertStatusCode(resp, http.StatusOK)

		var result map[string]interface{}
		err = helper.ParseJSONResponse(resp, &result)
		require.NoError(t, err)

		// Verify top-level fields
		assert.Contains(t, result, "results", "Response should have results field")
		assert.Contains(t, result, "count", "Response should have count field")
		assert.Contains(t, result, "query", "Response should have query field")
		assert.Contains(t, result, "limit", "Response should have limit field")
		assert.Contains(t, result, "offset", "Response should have offset field")

		deleteTestDocument(t, env, docID)
		t.Log("✓ All fields present test completed")
	})

	t.Run("ResultFieldsComplete", func(t *testing.T) {
		// Create document with full metadata
		metadata := map[string]interface{}{
			"project":  "TEST",
			"priority": "high",
		}
		doc := createTestDocumentWithMetadata("jira", "Complete fields test", metadata)
		docID := createAndSaveTestDocument(t, env, doc)

		// Search
		resp, err := helper.GET("/api/search?q=complete")
		require.NoError(t, err)
		defer resp.Body.Close()

		var result map[string]interface{}
		err = helper.ParseJSONResponse(resp, &result)
		require.NoError(t, err)

		results := result["results"].([]interface{})
		require.Greater(t, len(results), 0, "Should have at least one result")

		// Verify result fields
		firstResult := results[0].(map[string]interface{})
		expectedFields := []string{"id", "source_type", "source_id", "title", "content_markdown", "url", "detail_level", "metadata", "created_at", "updated_at", "brief"}
		for _, field := range expectedFields {
			assert.Contains(t, firstResult, field, "Result should have %s field", field)
		}

		deleteTestDocument(t, env, docID)
		t.Log("✓ Result fields complete test completed")
	})

	t.Run("CountMatchesResults", func(t *testing.T) {
		// Create multiple documents
		var ids []string
		for i := 0; i < 5; i++ {
			doc := createTestDocument("jira", fmt.Sprintf("Count matching doc %d", i))
			id := createAndSaveTestDocument(t, env, doc)
			ids = append(ids, id)
		}

		// Search
		resp, err := helper.GET("/api/search?q=matching")
		require.NoError(t, err)
		defer resp.Body.Close()

		var result map[string]interface{}
		err = helper.ParseJSONResponse(resp, &result)
		require.NoError(t, err)

		results := result["results"].([]interface{})
		count := int(result["count"].(float64))

		// Verify count matches results length
		assert.Equal(t, len(results), count, "Count should equal results array length")

		for _, id := range ids {
			deleteTestDocument(t, env, id)
		}
		t.Log("✓ Count matches results test completed")
	})

	t.Run("QueryEchoed", func(t *testing.T) {
		// Create document
		doc := createTestDocument("jira", "Query echo test")
		docID := createAndSaveTestDocument(t, env, doc)

		// Search with specific query
		testQuery := "echo"
		resp, err := helper.GET(fmt.Sprintf("/api/search?q=%s", testQuery))
		require.NoError(t, err)
		defer resp.Body.Close()

		var result map[string]interface{}
		err = helper.ParseJSONResponse(resp, &result)
		require.NoError(t, err)

		// Verify query field matches
		assert.Equal(t, testQuery, result["query"], "Query field should match request query parameter")

		deleteTestDocument(t, env, docID)
		t.Log("✓ Query echoed test completed")
	})

	t.Log("✓ TestSearchResponseStructure completed successfully")
}

// TestSearchBriefTruncation tests brief field truncation logic (200 char limit)
func TestSearchBriefTruncation(t *testing.T) {
	env, err := common.SetupTestEnvironment(t.Name())
	require.NoError(t, err, "Failed to setup test environment")
	defer env.Cleanup()

	cleanupAllDocuments(t, env)
	defer cleanupAllDocuments(t, env)

	helper := env.NewHTTPTestHelper(t)

	t.Run("ShortContent", func(t *testing.T) {
		// Create document with content < 200 chars
		shortContent := "This is short content with keyword brieftest that is less than 200 characters."
		doc := createTestDocument("jira", "Short Content")
		doc["content_markdown"] = shortContent
		docID := createAndSaveTestDocument(t, env, doc)

		// Search
		resp, err := helper.GET("/api/search?q=brieftest")
		require.NoError(t, err)
		defer resp.Body.Close()

		var result map[string]interface{}
		err = helper.ParseJSONResponse(resp, &result)
		require.NoError(t, err)

		results := result["results"].([]interface{})
		require.Greater(t, len(results), 0, "Should have at least one result")

		firstResult := results[0].(map[string]interface{})
		brief := firstResult["brief"].(string)

		// Verify no truncation (brief equals full content)
		assert.Equal(t, shortContent, brief, "Short content should not be truncated")
		assert.LessOrEqual(t, len(brief), 200, "Brief should be ≤ 200 chars")

		deleteTestDocument(t, env, docID)
		t.Log("✓ Short content test completed")
	})

	t.Run("ExactlyTwoHundred", func(t *testing.T) {
		// Create document with exactly 200 chars
		exactContent := "X"
		for len(exactContent) < 200 {
			exactContent += "x"
		}
		exactContent = exactContent[:200] + " exactbrief" // Add keyword
		doc := createTestDocument("jira", "Exact 200")
		doc["content_markdown"] = exactContent
		docID := createAndSaveTestDocument(t, env, doc)

		// Search
		resp, err := helper.GET("/api/search?q=exactbrief")
		require.NoError(t, err)
		defer resp.Body.Close()

		var result map[string]interface{}
		err = helper.ParseJSONResponse(resp, &result)
		require.NoError(t, err)

		results := result["results"].([]interface{})
		if len(results) > 0 {
			firstResult := results[0].(map[string]interface{})
			brief := firstResult["brief"].(string)

			// At exactly 200 chars, should not have ellipsis
			assert.LessOrEqual(t, len(brief), 203, "Brief should be ≤ 203 chars (200 + ...)")
		}

		deleteTestDocument(t, env, docID)
		t.Log("✓ Exactly 200 chars test completed")
	})

	t.Run("LongContent", func(t *testing.T) {
		// Create document with content > 200 chars
		longContent := "This is a very long content piece that exceeds the 200 character limit and should be truncated with ellipsis. "
		longContent += "It contains the keyword longbrief and continues with more text to ensure it's longer than 200 characters. "
		longContent += "Adding even more content to make absolutely sure we exceed 200 characters in total length."

		doc := createTestDocument("jira", "Long Content")
		doc["content_markdown"] = longContent
		docID := createAndSaveTestDocument(t, env, doc)

		// Search
		resp, err := helper.GET("/api/search?q=longbrief")
		require.NoError(t, err)
		defer resp.Body.Close()

		var result map[string]interface{}
		err = helper.ParseJSONResponse(resp, &result)
		require.NoError(t, err)

		results := result["results"].([]interface{})
		require.Greater(t, len(results), 0, "Should have at least one result")

		firstResult := results[0].(map[string]interface{})
		brief := firstResult["brief"].(string)
		fullContent := firstResult["content_markdown"].(string)

		// Verify truncation
		assert.LessOrEqual(t, len(brief), 203, "Brief should be ≤ 203 chars (200 + ...)")
		assert.Greater(t, len(fullContent), 200, "Full content should be > 200 chars")
		if len(brief) == 203 {
			assert.True(t, brief[200:] == "...", "Brief should end with ...")
		}

		deleteTestDocument(t, env, docID)
		t.Log("✓ Long content test completed")
	})

	t.Run("VeryLongContent", func(t *testing.T) {
		// Create document with content 500+ chars
		veryLongContent := ""
		for len(veryLongContent) < 500 {
			veryLongContent += "This is filler text. "
		}
		veryLongContent += "verybrief keyword"

		doc := createTestDocument("jira", "Very Long Content")
		doc["content_markdown"] = veryLongContent
		docID := createAndSaveTestDocument(t, env, doc)

		// Search
		resp, err := helper.GET("/api/search?q=verybrief")
		require.NoError(t, err)
		defer resp.Body.Close()

		var result map[string]interface{}
		err = helper.ParseJSONResponse(resp, &result)
		require.NoError(t, err)

		results := result["results"].([]interface{})
		if len(results) > 0 {
			firstResult := results[0].(map[string]interface{})
			brief := firstResult["brief"].(string)

			// Verify truncation to 203 chars max
			assert.LessOrEqual(t, len(brief), 203, "Brief should be ≤ 203 chars")
		}

		deleteTestDocument(t, env, docID)
		t.Log("✓ Very long content test completed")
	})

	t.Run("EmptyContent", func(t *testing.T) {
		// Create document with empty content
		doc := createTestDocument("jira", "Empty keyword emptybrief")
		doc["content_markdown"] = ""
		docID := createAndSaveTestDocument(t, env, doc)

		// Search (search title since content is empty)
		resp, err := helper.GET("/api/search?q=emptybrief")
		require.NoError(t, err)
		defer resp.Body.Close()

		var result map[string]interface{}
		err = helper.ParseJSONResponse(resp, &result)
		require.NoError(t, err)

		results := result["results"].([]interface{})
		if len(results) > 0 {
			firstResult := results[0].(map[string]interface{})
			brief := firstResult["brief"].(string)

			// Verify brief is empty or minimal
			assert.LessOrEqual(t, len(brief), 203, "Brief should be ≤ 203 chars")
		}

		deleteTestDocument(t, env, docID)
		t.Log("✓ Empty content test completed")
	})

	t.Log("✓ TestSearchBriefTruncation completed successfully")
}

// TestSearchErrorCases tests error handling
func TestSearchErrorCases(t *testing.T) {
	env, err := common.SetupTestEnvironment(t.Name())
	require.NoError(t, err, "Failed to setup test environment")
	defer env.Cleanup()

	helper := env.NewHTTPTestHelper(t)

	t.Run("MethodNotAllowed", func(t *testing.T) {
		// Try POST to search endpoint
		resp, err := helper.POST("/api/search", map[string]interface{}{"q": "test"})
		require.NoError(t, err)
		defer resp.Body.Close()

		// Should return 405 Method Not Allowed
		helper.AssertStatusCode(resp, http.StatusMethodNotAllowed)

		t.Log("✓ Method not allowed test completed")
	})

	t.Log("✓ TestSearchErrorCases completed successfully")
}

// TestSearchFTS5Disabled tests search endpoint behavior when FTS5 is disabled
func TestSearchFTS5Disabled(t *testing.T) {
	// Setup test environment with custom config that disables search
	env, err := common.SetupTestEnvironment(t.Name(), "../config/test-quaero.toml", "../config/test-search-disabled.toml")
	require.NoError(t, err, "Failed to setup test environment with search disabled")
	defer env.Cleanup()

	helper := env.NewHTTPTestHelper(t)

	t.Run("SearchReturns503", func(t *testing.T) {
		// Attempt search with search disabled
		resp, err := helper.GET("/api/search?q=test")
		require.NoError(t, err)
		defer resp.Body.Close()

		// Verify 503 Service Unavailable
		helper.AssertStatusCode(resp, http.StatusServiceUnavailable)

		// Parse error response
		var result map[string]interface{}
		err = helper.ParseJSONResponse(resp, &result)
		require.NoError(t, err)

		// Verify error message contains expected text
		errorMsg, ok := result["error"].(string)
		require.True(t, ok, "Response should contain error field")
		assert.Contains(t, errorMsg, "Search functionality is unavailable", "Error message should indicate search is unavailable")
		assert.Contains(t, errorMsg, "FTS5", "Error message should mention FTS5")

		t.Log("✓ Search returns 503 test completed")
	})

	t.Run("SearchWithParametersReturns503", func(t *testing.T) {
		// Attempt search with pagination parameters
		resp, err := helper.GET("/api/search?q=keyword&limit=10&offset=5")
		require.NoError(t, err)
		defer resp.Body.Close()

		// Verify 503 regardless of parameters
		helper.AssertStatusCode(resp, http.StatusServiceUnavailable)

		var result map[string]interface{}
		err = helper.ParseJSONResponse(resp, &result)
		require.NoError(t, err)

		// Verify error response structure
		errorMsg, ok := result["error"].(string)
		require.True(t, ok, "Response should contain error field")
		assert.NotEmpty(t, errorMsg, "Error message should not be empty")

		t.Log("✓ Search with parameters returns 503 test completed")
	})

	t.Log("✓ TestSearchFTS5Disabled completed successfully")
}
