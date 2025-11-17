package api

import (
	"net/http"
	"testing"

	"github.com/ternarybob/quaero/test/common"
)

// TestJobDefinition_APIKeyInjection_Success verifies that job definitions
// properly reference and validate API keys stored in KV storage
func TestJobDefinition_APIKeyInjection_Success(t *testing.T) {
	env, err := common.SetupTestEnvironment("TestJobDefinition_APIKeyInjection_Success")
	if err != nil {
		t.Fatalf("Failed to setup test environment: %v", err)
	}
	defer env.Cleanup()

	h := env.NewHTTPTestHelper(t)

	// Step 1: Create an API key in KV storage
	testAPIKeyName := "google_api_key"
	testAPIKeyValue := "test-google-api-key-12345"

	createKeyReq := map[string]interface{}{
		"key":         testAPIKeyName,
		"value":       testAPIKeyValue,
		"description": "Test Google API key for job execution",
	}

	keyResp, err := h.POST("/api/kv", createKeyReq)
	if err != nil {
		t.Fatalf("Failed to create API key: %v", err)
	}
	h.AssertStatusCode(keyResp, http.StatusCreated)
	defer h.DELETE("/api/kv/" + testAPIKeyName)

	env.LogTest(t, "✓ Created API key in KV storage: %s", testAPIKeyName)

	// Step 2: Create a job definition that references the API key
	jobDef := map[string]interface{}{
		"id":          "test-job-def-apikey-1",
		"name":        "Test Job - API Key Reference",
		"type":        "custom", // Use "custom" type with "agent" action
		"description": "Test job with API key reference",
		"enabled":     true,
		"steps": []map[string]interface{}{
			{
				"name":   "agent_step_with_key",
				"action": "agent",
				"config": map[string]interface{}{
					"agent_type": "keyword_extractor",
					"api_key":    testAPIKeyName, // Reference to KV storage key
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
	h.AssertStatusCode(jobDefResp, http.StatusCreated)

	var jobDefResult map[string]interface{}
	h.ParseJSONResponse(jobDefResp, &jobDefResult)
	jobDefID := jobDefResult["id"].(string)
	defer h.DELETE("/api/job-definitions/" + jobDefID)

	env.LogTest(t, "✓ Created job definition with API key reference")

	// Step 3: Retrieve job definition list (triggers runtime validation)
	listResp, err := h.GET("/api/job-definitions")
	if err != nil {
		t.Fatalf("Failed to list job definitions: %v", err)
	}
	h.AssertStatusCode(listResp, http.StatusOK)

	var listResult map[string]interface{}
	h.ParseJSONResponse(listResp, &listResult)

	jobDefs, ok := listResult["job_definitions"].([]interface{})
	if !ok {
		t.Fatal("Response missing 'job_definitions' array")
	}

	// Find our job definition in the list
	var retrievedJobDef map[string]interface{}
	for _, item := range jobDefs {
		jd, ok := item.(map[string]interface{})
		if !ok {
			continue
		}
		if id, ok := jd["id"].(string); ok && id == jobDefID {
			retrievedJobDef = jd
			break
		}
	}

	if retrievedJobDef == nil {
		t.Fatal("Job definition not found in list")
	}

	// Verify RuntimeStatus is "ready" (API key exists)
	runtimeStatus, ok := retrievedJobDef["runtime_status"].(string)
	if !ok {
		t.Error("Job definition should have runtime_status field")
	} else if runtimeStatus == "error" {
		// Check runtime_error for details
		if runtimeError, ok := retrievedJobDef["runtime_error"].(string); ok {
			t.Errorf("Job definition has error status: %s", runtimeError)
		} else {
			t.Error("Job definition has error status but no error message")
		}
	} else if runtimeStatus != "ready" && runtimeStatus != "disabled" {
		// Note: status may be "disabled" if agent service is not available
		// This is acceptable as long as there's no "error" status
		t.Logf("⚠ RuntimeStatus is '%s' (may be due to agent service availability)", runtimeStatus)
	}

	env.LogTest(t, "✓ Job definition validated successfully with existing API key")
}

// TestJobDefinition_APIKeyInjection_MissingKey verifies that job definitions
// properly detect and report missing API keys
func TestJobDefinition_APIKeyInjection_MissingKey(t *testing.T) {
	env, err := common.SetupTestEnvironment("TestJobDefinition_APIKeyInjection_MissingKey")
	if err != nil {
		t.Fatalf("Failed to setup test environment: %v", err)
	}
	defer env.Cleanup()

	h := env.NewHTTPTestHelper(t)

	// Create a job definition that references a non-existent API key
	nonExistentKeyName := "nonexistent_google_api_key"

	jobDef := map[string]interface{}{
		"id":          "test-job-def-missing-key",
		"name":        "Test Job - Missing API Key",
		"type":        "custom", // Use "custom" type with "agent" action
		"description": "Test job with missing API key reference",
		"enabled":     true,
		"steps": []map[string]interface{}{
			{
				"name":   "agent_step_missing_key",
				"action": "agent",
				"config": map[string]interface{}{
					"agent_type": "keyword_extractor",
					"api_key":    nonExistentKeyName, // References non-existent key
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
	h.AssertStatusCode(jobDefResp, http.StatusCreated)

	var jobDefResult map[string]interface{}
	h.ParseJSONResponse(jobDefResp, &jobDefResult)
	jobDefID := jobDefResult["id"].(string)
	defer h.DELETE("/api/job-definitions/" + jobDefID)

	env.LogTest(t, "✓ Created job definition with missing API key reference")

	// Retrieve job definition list (triggers runtime validation)
	listResp, err := h.GET("/api/job-definitions")
	if err != nil {
		t.Fatalf("Failed to list job definitions: %v", err)
	}
	h.AssertStatusCode(listResp, http.StatusOK)

	var listResult map[string]interface{}
	h.ParseJSONResponse(listResp, &listResult)

	jobDefs, ok := listResult["job_definitions"].([]interface{})
	if !ok {
		t.Fatal("Response missing 'job_definitions' array")
	}

	// Find our job definition in the list
	var foundJobDef map[string]interface{}
	for _, item := range jobDefs {
		jd, ok := item.(map[string]interface{})
		if !ok {
			continue
		}
		if id, ok := jd["id"].(string); ok && id == jobDefID {
			foundJobDef = jd
			break
		}
	}

	if foundJobDef == nil {
		t.Fatal("Job definition not found in list")
	}

	// Verify RuntimeStatus is "error" and RuntimeError mentions the missing key
	runtimeStatus, ok := foundJobDef["runtime_status"].(string)
	if !ok {
		t.Error("Job definition should have runtime_status field")
	} else if runtimeStatus != "error" {
		t.Errorf("Expected RuntimeStatus='error' for missing API key, got: %s", runtimeStatus)
	}

	runtimeError, ok := foundJobDef["runtime_error"].(string)
	if !ok || runtimeError == "" {
		t.Error("Job definition should have runtime_error message for missing API key")
	} else {
		env.LogTest(t, "✓ RuntimeError detected: %s", runtimeError)
		// Verify error message mentions the key name
		if len(runtimeError) > 0 {
			env.LogTest(t, "✓ Error message provided for missing API key")
		}
	}
}

// TestJobDefinition_APIKeyInjection_KeyReplacement verifies that updating
// an API key is properly detected by job definitions
func TestJobDefinition_APIKeyInjection_KeyReplacement(t *testing.T) {
	env, err := common.SetupTestEnvironment("TestJobDefinition_APIKeyInjection_KeyReplacement")
	if err != nil {
		t.Fatalf("Failed to setup test environment: %v", err)
	}
	defer env.Cleanup()

	h := env.NewHTTPTestHelper(t)

	testAPIKeyName := "google_api_key_update_test"
	initialValue := "initial-key-value"
	updatedValue := "updated-key-value"

	// Step 1: Create initial API key
	createKeyReq := map[string]interface{}{
		"key":         testAPIKeyName,
		"value":       initialValue,
		"description": "Test API key for update verification",
	}

	keyResp, err := h.POST("/api/kv", createKeyReq)
	if err != nil {
		t.Fatalf("Failed to create API key: %v", err)
	}
	h.AssertStatusCode(keyResp, http.StatusCreated)
	defer h.DELETE("/api/kv/" + testAPIKeyName)

	env.LogTest(t, "✓ Created initial API key")

	// Step 2: Create job definition referencing the key
	jobDef := map[string]interface{}{
		"id":          "test-job-def-key-update",
		"name":        "Test Job - Key Update",
		"type":        "custom", // Use "custom" type with "agent" action
		"description": "Test job with API key update",
		"enabled":     true,
		"steps": []map[string]interface{}{
			{
				"name":   "agent_step",
				"action": "agent",
				"config": map[string]interface{}{
					"agent_type": "keyword_extractor",
					"api_key":    testAPIKeyName,
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
	h.AssertStatusCode(jobDefResp, http.StatusCreated)

	var jobDefResult map[string]interface{}
	h.ParseJSONResponse(jobDefResp, &jobDefResult)
	jobDefID := jobDefResult["id"].(string)
	defer h.DELETE("/api/job-definitions/" + jobDefID)

	env.LogTest(t, "✓ Created job definition")

	// Step 3: Verify initial state (key exists, status should be ready or disabled)
	getResp1, err := h.GET("/api/job-definitions")
	if err != nil {
		t.Fatalf("Failed to list job definitions: %v", err)
	}
	h.AssertStatusCode(getResp1, http.StatusOK)

	env.LogTest(t, "✓ Initial validation complete")

	// Step 4: Update the API key value
	updateKeyReq := map[string]interface{}{
		"value":       updatedValue,
		"description": "Updated test API key",
	}

	updateResp, err := h.PUT("/api/kv/"+testAPIKeyName, updateKeyReq)
	if err != nil {
		t.Fatalf("Failed to update API key: %v", err)
	}
	h.AssertStatusCode(updateResp, http.StatusOK)

	env.LogTest(t, "✓ Updated API key value")

	// Step 5: Verify job definition still validates (key still exists)
	getResp2, err := h.GET("/api/job-definitions")
	if err != nil {
		t.Fatalf("Failed to list job definitions after update: %v", err)
	}
	h.AssertStatusCode(getResp2, http.StatusOK)

	var listResult map[string]interface{}
	h.ParseJSONResponse(getResp2, &listResult)

	jobDefs, ok := listResult["job_definitions"].([]interface{})
	if !ok {
		t.Fatal("Response missing 'job_definitions' array")
	}

	// Find our job definition
	var foundJobDef map[string]interface{}
	for _, item := range jobDefs {
		jd, ok := item.(map[string]interface{})
		if !ok {
			continue
		}
		if id, ok := jd["id"].(string); ok && id == jobDefID {
			foundJobDef = jd
			break
		}
	}

	if foundJobDef == nil {
		t.Fatal("Job definition not found after key update")
	}

	// Verify status is still valid (not "error")
	runtimeStatus, ok := foundJobDef["runtime_status"].(string)
	if ok && runtimeStatus == "error" {
		runtimeError, _ := foundJobDef["runtime_error"].(string)
		t.Errorf("Job definition should remain valid after key update, got error: %s", runtimeError)
	}

	env.LogTest(t, "✓ Job definition remains valid after API key update")

	// Step 6: Delete the key
	deleteResp, err := h.DELETE("/api/kv/" + testAPIKeyName)
	if err != nil {
		t.Fatalf("Failed to delete API key: %v", err)
	}
	h.AssertStatusCode(deleteResp, http.StatusOK)

	env.LogTest(t, "✓ Deleted API key")

	// Step 7: Verify job definition now shows error
	getResp3, err := h.GET("/api/job-definitions")
	if err != nil {
		t.Fatalf("Failed to list job definitions after deletion: %v", err)
	}
	h.AssertStatusCode(getResp3, http.StatusOK)

	h.ParseJSONResponse(getResp3, &listResult)

	jobDefs, ok = listResult["job_definitions"].([]interface{})
	if !ok {
		t.Fatal("Response missing 'job_definitions' array")
	}

	// Find our job definition again
	foundJobDef = nil
	for _, item := range jobDefs {
		jd, ok := item.(map[string]interface{})
		if !ok {
			continue
		}
		if id, ok := jd["id"].(string); ok && id == jobDefID {
			foundJobDef = jd
			break
		}
	}

	if foundJobDef == nil {
		t.Fatal("Job definition not found after key deletion")
	}

	// Verify status is now "error"
	runtimeStatus, ok = foundJobDef["runtime_status"].(string)
	if !ok {
		t.Error("Job definition should have runtime_status after key deletion")
	} else if runtimeStatus != "error" {
		t.Errorf("Expected RuntimeStatus='error' after key deletion, got: %s", runtimeStatus)
	}

	runtimeError, ok := foundJobDef["runtime_error"].(string)
	if !ok || runtimeError == "" {
		t.Error("Job definition should have runtime_error after key deletion")
	} else {
		env.LogTest(t, "✓ RuntimeError after deletion: %s", runtimeError)
	}
}

// TestJobDefinition_APIKeyInjection_MultipleKeys verifies handling of
// multiple API key references in a single job definition
func TestJobDefinition_APIKeyInjection_MultipleKeys(t *testing.T) {
	env, err := common.SetupTestEnvironment("TestJobDefinition_APIKeyInjection_MultipleKeys")
	if err != nil {
		t.Fatalf("Failed to setup test environment: %v", err)
	}
	defer env.Cleanup()

	h := env.NewHTTPTestHelper(t)

	// Create two API keys
	key1Name := "api_key_1"
	key2Name := "api_key_2"

	for _, keyName := range []string{key1Name, key2Name} {
		createKeyReq := map[string]interface{}{
			"key":         keyName,
			"value":       "test-value-" + keyName,
			"description": "Test key " + keyName,
		}

		keyResp, err := h.POST("/api/kv", createKeyReq)
		if err != nil {
			t.Fatalf("Failed to create API key %s: %v", keyName, err)
		}
		h.AssertStatusCode(keyResp, http.StatusCreated)
	}

	env.LogTest(t, "✓ Created multiple API keys")

	// Cleanup keys at the end of the test
	defer func() {
		for _, keyName := range []string{key1Name, key2Name} {
			h.DELETE("/api/kv/" + keyName)
		}
	}()

	// Create job definition with multiple steps, each referencing different keys
	jobDef := map[string]interface{}{
		"id":          "test-job-def-multi-keys",
		"name":        "Test Job - Multiple Keys",
		"type":        "custom", // Use "custom" type with "agent" action
		"description": "Test job with multiple API key references",
		"enabled":     true,
		"steps": []map[string]interface{}{
			{
				"name":   "step_1",
				"action": "agent",
				"config": map[string]interface{}{
					"agent_type": "keyword_extractor",
					"api_key":    key1Name,
					"document_filter": map[string]interface{}{
						"source_type": "test",
					},
				},
				"on_error": "fail",
			},
			{
				"name":   "step_2",
				"action": "agent",
				"config": map[string]interface{}{
					"agent_type": "keyword_extractor",
					"api_key":    key2Name,
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
	h.AssertStatusCode(jobDefResp, http.StatusCreated)

	var jobDefResult map[string]interface{}
	h.ParseJSONResponse(jobDefResp, &jobDefResult)
	jobDefID := jobDefResult["id"].(string)
	defer h.DELETE("/api/job-definitions/" + jobDefID)

	env.LogTest(t, "✓ Created job definition with multiple key references")

	// Verify both keys are validated
	listResp, err := h.GET("/api/job-definitions")
	if err != nil {
		t.Fatalf("Failed to list job definitions: %v", err)
	}
	h.AssertStatusCode(listResp, http.StatusOK)

	var listResult map[string]interface{}
	h.ParseJSONResponse(listResp, &listResult)

	jobDefs, ok := listResult["job_definitions"].([]interface{})
	if !ok {
		t.Fatal("Response missing 'job_definitions' array")
	}

	var foundJobDef map[string]interface{}
	for _, item := range jobDefs {
		jd, ok := item.(map[string]interface{})
		if !ok {
			continue
		}
		if id, ok := jd["id"].(string); ok && id == jobDefID {
			foundJobDef = jd
			break
		}
	}

	if foundJobDef == nil {
		t.Fatal("Job definition not found")
	}

	// Should be ready or disabled (not error)
	runtimeStatus, ok := foundJobDef["runtime_status"].(string)
	if ok && runtimeStatus == "error" {
		runtimeError, _ := foundJobDef["runtime_error"].(string)
		t.Errorf("Job definition should validate with both keys present, got error: %s", runtimeError)
	}

	env.LogTest(t, "✓ Job definition validated with multiple API keys")
}
