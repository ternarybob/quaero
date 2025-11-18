package api

import (
	"github.com/ternarybob/quaero/test/common"
	"net/http"
	"testing"
	"time"
)

// TestAgentJobExecution_KeywordExtraction verifies end-to-end keyword extraction via agent job
func TestAgentJobExecution_KeywordExtraction(t *testing.T) {
	env, err := common.SetupTestEnvironment("TestAgentJobExecution_KeywordExtraction")
	if err != nil {
		t.Fatalf("Failed to setup test environment: %v", err)
	}
	defer env.Cleanup()

	h := env.NewHTTPTestHelper(t)

	// 1. Create test document
	document := map[string]interface{}{
		"id":          "test-doc-agent-1",
		"source_type": "test",
		"title":       "Test Document for Keyword Extraction",
		"content_markdown": `# Advanced AI and Machine Learning Technologies

This document explores the latest developments in artificial intelligence and machine learning.
Deep learning neural networks have revolutionized computer vision and natural language processing.
Modern AI systems utilize transformer architectures for improved performance in language understanding
and generation tasks. Machine learning algorithms continue to advance with techniques like
reinforcement learning, generative adversarial networks, and federated learning. The integration
of AI into cloud computing platforms enables scalable deployment of intelligent systems across
enterprise environments.`,
		"url": "https://test.example.com/doc1",
	}

	docResp, err := h.POST("/api/documents", document)
	if err != nil {
		t.Fatalf("Failed to create document: %v", err)
	}
	h.AssertStatusCode(docResp, http.StatusCreated)

	var docResult map[string]interface{}
	h.ParseJSONResponse(docResp, &docResult)
	documentID := docResult["id"].(string)
	defer h.DELETE("/api/documents/" + documentID)
	t.Logf(" Created test document: %s", documentID)

	// 2. Create agent job definition
	jobDef := map[string]interface{}{
		"id":          "test-agent-job-def-1",
		"name":        "Test Agent Job - Keyword Extraction",
		"type":        "agent",
		"description": "Test keyword extraction",
		"enabled":     true,
		"steps": []map[string]interface{}{
			{
				"name":   "extract_keywords",
				"action": "agent",
				"config": map[string]interface{}{
					"agent_type": "keyword_extractor",
					"document_filter": map[string]interface{}{
						"source_type": "test",
					},
					"max_keywords": 10,
				},
				"on_error": "fail",
			},
		},
	}

	jobDefResp, err := h.POST("/api/job-definitions", jobDef)
	if err != nil {
		t.Fatalf("Failed to create job definition: %v", err)
	}
	h.AssertStatusCode(jobDefResp, http.StatusCreated)

	var jobDefResult map[string]interface{}
	h.ParseJSONResponse(jobDefResp, &jobDefResult)
	jobDefID := jobDefResult["id"].(string)
	defer h.DELETE("/api/job-definitions/" + jobDefID)
	t.Logf(" Created job definition: %s", jobDefID)

	// 3. Execute job definition
	execResp, err := h.POST("/api/job-definitions/"+jobDefID+"/execute", nil)
	if err != nil {
		t.Fatalf("Failed to execute job definition: %v", err)
	}
	h.AssertStatusCode(execResp, http.StatusAccepted)
	t.Log(" Job execution started")

	// 4. Poll for parent job creation
	var parentJobID string
	found := false

	for attempt := 0; attempt < 60; attempt++ {
		time.Sleep(500 * time.Millisecond)

		jobsResp, err := h.GET("/api/jobs")
		if err != nil {
			continue
		}

		var paginatedResponse struct {
			Jobs []map[string]interface{} `json:"jobs"`
		}
		if err := h.ParseJSONResponse(jobsResp, &paginatedResponse); err != nil {
			continue
		}

		// Look for parent job (job_type = "agent" and source_type = "job_definition")
		for _, job := range paginatedResponse.Jobs {
			jobType, _ := job["job_type"].(string)
			sourceType, _ := job["source_type"].(string)

			if jobType == "agent" && sourceType == "job_definition" {
				parentJobID = job["id"].(string)
				found = true
				break
			}
		}

		if found {
			break
		}
	}

	if !found {
		t.Fatal("Parent job was not created after job definition execution")
	}

	defer h.DELETE("/api/jobs/" + parentJobID)
	t.Logf(" Parent job created: %s", parentJobID)

	// 5. Poll for job completion
	deadline := time.Now().Add(5 * time.Minute)
	var finalStatus string

	for time.Now().Before(deadline) {
		time.Sleep(2 * time.Second)

		jobResp, err := h.GET("/api/jobs/" + parentJobID)
		if err != nil {
			continue
		}

		var job map[string]interface{}
		if err := h.ParseJSONResponse(jobResp, &job); err != nil {
			continue
		}

		status, ok := job["status"].(string)
		if !ok {
			continue
		}

		t.Logf("Job status: %s", status)

		if status == "completed" || status == "failed" {
			finalStatus = status
			break
		}
	}

	if finalStatus != "completed" {
		t.Fatalf("Expected job status 'completed', got: %s", finalStatus)
	}
	t.Log(" Job completed successfully")

	// 6. Verify document metadata updated
	docCheckResp, err := h.GET("/api/documents/" + documentID)
	if err != nil {
		t.Fatalf("Failed to fetch document: %v", err)
	}
	h.AssertStatusCode(docCheckResp, http.StatusOK)

	var updatedDoc map[string]interface{}
	h.ParseJSONResponse(docCheckResp, &updatedDoc)

	metadata, ok := updatedDoc["metadata"].(map[string]interface{})
	if !ok || metadata == nil {
		t.Fatal("Document metadata is missing")
	}

	keywordData, ok := metadata["keyword_extractor"].(map[string]interface{})
	if !ok || keywordData == nil {
		t.Fatal("keyword_extractor metadata is missing")
	}

	keywords, ok := keywordData["keywords"].([]interface{})
	if !ok || keywords == nil {
		t.Fatal("keywords array is missing in metadata")
	}

	if len(keywords) < 5 || len(keywords) > 15 {
		t.Errorf("Expected 5-15 keywords, got %d", len(keywords))
	}

	// Verify keywords are non-empty strings
	for i, kw := range keywords {
		kwStr, ok := kw.(string)
		if !ok || kwStr == "" {
			t.Errorf("Keyword at index %d is not a valid string: %v", i, kw)
		}
	}

	t.Logf(" Document metadata updated with %d keywords", len(keywords))
	t.Logf("Keywords extracted: %v", keywords)
}

// TestAgentJobExecution_NoMatchingDocuments verifies job handles empty document set gracefully
func TestAgentJobExecution_NoMatchingDocuments(t *testing.T) {
	env, err := common.SetupTestEnvironment("TestAgentJobExecution_NoMatchingDocuments")
	if err != nil {
		t.Fatalf("Failed to setup test environment: %v", err)
	}
	defer env.Cleanup()

	h := env.NewHTTPTestHelper(t)

	// Create job definition with nonexistent source_type filter
	jobDef := map[string]interface{}{
		"id":          "test-agent-job-def-invalid",
		"name":        "Test Agent Job - Invalid Filter",
		"type":        "agent",
		"description": "Test with no matching documents",
		"enabled":     true,
		"steps": []map[string]interface{}{
			{
				"name":   "extract_keywords",
				"action": "agent",
				"config": map[string]interface{}{
					"agent_type": "keyword_extractor",
					"document_filter": map[string]interface{}{
						"source_type": "nonexistent",
					},
					"max_keywords": 10,
				},
				"on_error": "fail",
			},
		},
	}

	jobDefResp, err := h.POST("/api/job-definitions", jobDef)
	if err != nil {
		t.Fatalf("Failed to create job definition: %v", err)
	}
	h.AssertStatusCode(jobDefResp, http.StatusCreated)

	var jobDefResult map[string]interface{}
	h.ParseJSONResponse(jobDefResp, &jobDefResult)
	jobDefID := jobDefResult["id"].(string)
	defer h.DELETE("/api/job-definitions/" + jobDefID)

	// Execute job definition
	execResp, err := h.POST("/api/job-definitions/"+jobDefID+"/execute", nil)
	if err != nil {
		t.Fatalf("Failed to execute job definition: %v", err)
	}
	h.AssertStatusCode(execResp, http.StatusAccepted)

	// Poll for parent job
	var parentJobID string
	for attempt := 0; attempt < 60; attempt++ {
		time.Sleep(500 * time.Millisecond)

		jobsResp, err := h.GET("/api/jobs")
		if err != nil {
			continue
		}

		var paginatedResponse struct {
			Jobs []map[string]interface{} `json:"jobs"`
		}
		if err := h.ParseJSONResponse(jobsResp, &paginatedResponse); err != nil {
			continue
		}

		for _, job := range paginatedResponse.Jobs {
			jobType, _ := job["job_type"].(string)
			sourceType, _ := job["source_type"].(string)

			if jobType == "agent" && sourceType == "job_definition" {
				parentJobID = job["id"].(string)
				break
			}
		}

		if parentJobID != "" {
			break
		}
	}

	if parentJobID == "" {
		t.Fatal("Parent job not found")
	}

	defer h.DELETE("/api/jobs/" + parentJobID)

	// Wait a bit for job to complete
	time.Sleep(5 * time.Second)

	// Verify job completed gracefully (no documents to process)
	jobResp, err := h.GET("/api/jobs/" + parentJobID)
	if err != nil {
		t.Fatalf("Failed to fetch job: %v", err)
	}

	var job map[string]interface{}
	h.ParseJSONResponse(jobResp, &job)

	status, ok := job["status"].(string)
	if !ok {
		t.Fatal("Job status is missing")
	}

	if status != "completed" {
		t.Errorf("Expected status 'completed' (no documents to process), got: %s", status)
	}

	t.Log(" Job handled empty document set gracefully")
}

// TestAgentJobExecution_MissingAPIKey verifies error handling when Google API key is missing
func TestAgentJobExecution_MissingAPIKey(t *testing.T) {
	env, err := common.SetupTestEnvironment("TestAgentJobExecution_MissingAPIKey")
	if err != nil {
		t.Fatalf("Failed to setup test environment: %v", err)
	}
	defer env.Cleanup()

	h := env.NewHTTPTestHelper(t)

	// This test documents expected behavior when API key is missing
	// In production, agent service initialization fails if config.Agent.GoogleAPIKey is empty
	// Agent executors are not registered if service initialization fails
	// Agent jobs will return 404 or validation errors if agent service is unavailable

	// Try to create agent job definition
	jobDef := map[string]interface{}{
		"id":          "test-agent-job-def-apikey",
		"name":        "Test Agent Job - API Key Check",
		"type":        "agent",
		"description": "Test API key requirement",
		"enabled":     true,
		"steps": []map[string]interface{}{
			{
				"name":   "extract_keywords",
				"action": "agent",
				"config": map[string]interface{}{
					"agent_type": "keyword_extractor",
					"document_filter": map[string]interface{}{
						"source_type": "test",
					},
				},
				"on_error": "fail",
			},
		},
	}

	jobDefResp, err := h.POST("/api/job-definitions", jobDef)
	if err != nil {
		t.Fatalf("Failed to create job definition: %v", err)
	}

	// Parse response to extract ID for cleanup
	var jobDefResult map[string]interface{}
	if jobDefResp.StatusCode == http.StatusCreated {
		h.ParseJSONResponse(jobDefResp, &jobDefResult)
		jobDefID := jobDefResult["id"].(string)
		defer h.DELETE("/api/job-definitions/" + jobDefID)

		// Agent service is configured (has API key), skip test
		t.Skip("Agent service is configured, skipping API key validation test")
		return
	}

	// Handle non-201 responses (expected when API key is missing)
	if jobDefResp.StatusCode != http.StatusCreated {
		// Expected failure modes when agent service is unavailable:
		// - 400 Bad Request: Invalid agent_type (agent executors not registered)
		// - 500 Internal Server Error: Agent service initialization failed
		if jobDefResp.StatusCode == http.StatusBadRequest || jobDefResp.StatusCode == http.StatusInternalServerError {
			t.Logf("Expected failure: Got status %d when agent service unavailable", jobDefResp.StatusCode)
		} else {
			t.Errorf("Unexpected status code: got %d, expected 400 or 500", jobDefResp.StatusCode)
		}
	}

	// Document expected behavior:
	// - Agent service initialization fails in app.go if config.Agent.GoogleAPIKey is empty
	// - Error message: "Google API key is required for agent service"
	// - Agent executors are not registered if service initialization fails
	// - Agent jobs will return 404 or validation errors if agent service is unavailable

	t.Log("Expected behavior when API key is missing:")
	t.Log("  - Agent service initialization fails: 'Google API key is required for agent service'")
	t.Log("  - Agent executors not registered")
	t.Log("  - Agent job execution returns error (404 or validation failure)")
}

// TestAgentJobExecution_MultipleDocuments verifies agent processes multiple documents correctly
func TestAgentJobExecution_MultipleDocuments(t *testing.T) {
	env, err := common.SetupTestEnvironment("TestAgentJobExecution_MultipleDocuments")
	if err != nil {
		t.Fatalf("Failed to setup test environment: %v", err)
	}
	defer env.Cleanup()

	h := env.NewHTTPTestHelper(t)

	// Create 3 documents with different content
	documents := []map[string]interface{}{
		{
			"id":               "test-doc-multi-1",
			"source_type":      "test",
			"title":            "Technology Document",
			"content_markdown": "Artificial intelligence, machine learning, deep learning, neural networks, computer vision, natural language processing.",
			"url":              "https://test.example.com/tech",
		},
		{
			"id":               "test-doc-multi-2",
			"source_type":      "test",
			"title":            "Healthcare Document",
			"content_markdown": "Medical diagnosis, patient care, clinical trials, pharmaceutical research, treatment protocols, healthcare systems.",
			"url":              "https://test.example.com/health",
		},
		{
			"id":               "test-doc-multi-3",
			"source_type":      "test",
			"title":            "Finance Document",
			"content_markdown": "Investment strategies, portfolio management, risk assessment, financial markets, trading algorithms, asset allocation.",
			"url":              "https://test.example.com/finance",
		},
	}

	var documentIDs []string
	for _, doc := range documents {
		docResp, err := h.POST("/api/documents", doc)
		if err != nil {
			t.Fatalf("Failed to create document: %v", err)
		}
		h.AssertStatusCode(docResp, http.StatusCreated)

		var docResult map[string]interface{}
		h.ParseJSONResponse(docResp, &docResult)
		docID := docResult["id"].(string)
		documentIDs = append(documentIDs, docID)
		defer h.DELETE("/api/documents/" + docID)
	}

	t.Logf(" Created %d test documents", len(documentIDs))

	// Create and execute job definition
	jobDef := map[string]interface{}{
		"id":          "test-agent-job-def-multi",
		"name":        "Test Agent Job - Multiple Docs",
		"type":        "agent",
		"description": "Test multiple document processing",
		"enabled":     true,
		"steps": []map[string]interface{}{
			{
				"name":   "extract_keywords",
				"action": "agent",
				"config": map[string]interface{}{
					"agent_type": "keyword_extractor",
					"document_filter": map[string]interface{}{
						"source_type": "test",
					},
					"max_keywords": 10,
				},
				"on_error": "fail",
			},
		},
	}

	jobDefResp, err := h.POST("/api/job-definitions", jobDef)
	if err != nil {
		t.Fatalf("Failed to create job definition: %v", err)
	}
	h.AssertStatusCode(jobDefResp, http.StatusCreated)

	var jobDefResult map[string]interface{}
	h.ParseJSONResponse(jobDefResp, &jobDefResult)
	jobDefID := jobDefResult["id"].(string)
	defer h.DELETE("/api/job-definitions/" + jobDefID)

	// Execute
	execResp, err := h.POST("/api/job-definitions/"+jobDefID+"/execute", nil)
	if err != nil {
		t.Fatalf("Failed to execute job definition: %v", err)
	}
	h.AssertStatusCode(execResp, http.StatusAccepted)

	// Poll for parent job
	var parentJobID string
	for attempt := 0; attempt < 60; attempt++ {
		time.Sleep(500 * time.Millisecond)

		jobsResp, err := h.GET("/api/jobs")
		if err != nil {
			continue
		}

		var paginatedResponse struct {
			Jobs []map[string]interface{} `json:"jobs"`
		}
		if err := h.ParseJSONResponse(jobsResp, &paginatedResponse); err != nil {
			continue
		}

		for _, job := range paginatedResponse.Jobs {
			jobType, _ := job["job_type"].(string)
			sourceType, _ := job["source_type"].(string)

			if jobType == "agent" && sourceType == "job_definition" {
				parentJobID = job["id"].(string)
				break
			}
		}

		if parentJobID != "" {
			break
		}
	}

	if parentJobID == "" {
		t.Fatal("Parent job not found")
	}

	defer h.DELETE("/api/jobs/" + parentJobID)

	// Wait for completion
	deadline := time.Now().Add(10 * time.Minute)
	var finalStatus string

	for time.Now().Before(deadline) {
		time.Sleep(3 * time.Second)

		jobResp, err := h.GET("/api/jobs/" + parentJobID)
		if err != nil {
			continue
		}

		var job map[string]interface{}
		if err := h.ParseJSONResponse(jobResp, &job); err != nil {
			continue
		}

		status, ok := job["status"].(string)
		if !ok {
			continue
		}

		t.Logf("Job status: %s", status)

		if status == "completed" || status == "failed" {
			finalStatus = status
			break
		}
	}

	if finalStatus != "completed" {
		t.Fatalf("Expected job status 'completed', got: %s", finalStatus)
	}

	// Verify all documents processed
	for i, docID := range documentIDs {
		docResp, err := h.GET("/api/documents/" + docID)
		if err != nil {
			t.Fatalf("Failed to fetch document %s: %v", docID, err)
		}

		var doc map[string]interface{}
		h.ParseJSONResponse(docResp, &doc)

		metadata, ok := doc["metadata"].(map[string]interface{})
		if !ok || metadata == nil {
			t.Errorf("Document %d (%s) missing metadata", i+1, docID)
			continue
		}

		keywordData, ok := metadata["keyword_extractor"].(map[string]interface{})
		if !ok || keywordData == nil {
			t.Errorf("Document %d (%s) missing keyword_extractor metadata", i+1, docID)
			continue
		}

		keywords, ok := keywordData["keywords"].([]interface{})
		if !ok || keywords == nil {
			t.Errorf("Document %d (%s) missing keywords array", i+1, docID)
			continue
		}

		t.Logf(" Document %d (%s) processed with %d keywords", i+1, docID, len(keywords))
	}

	t.Log(" All documents processed successfully")
}
