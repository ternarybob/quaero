package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/ternarybob/quaero/test"
)

// TestAuthListEndpoint tests the GET /api/auth/list endpoint
func TestAuthListEndpoint(t *testing.T) {
	baseURL := test.MustGetTestServerURL()

	// Test listing authentications
	resp, err := http.Get(baseURL + "/api/auth/list")
	if err != nil {
		t.Fatalf("Failed to get auth list: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}

	// Parse response
	var auths []map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&auths); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	// Should be an array (possibly empty)
	t.Logf("Found %d authentication credentials", len(auths))
}

// TestAuthCaptureEndpoint tests capturing auth from Chrome extension
func TestAuthCaptureEndpoint(t *testing.T) {
	baseURL := test.MustGetTestServerURL()

	// Create test auth data (simulating Chrome extension)
	authData := map[string]interface{}{
		"baseUrl":   "https://test.atlassian.net",
		"userAgent": "Mozilla/5.0 Test",
		"cookies": []map[string]interface{}{
			{
				"name":     "cloud.session.token",
				"value":    "test-token",
				"domain":   ".atlassian.net",
				"path":     "/",
				"secure":   true,
				"httpOnly": true,
			},
		},
		"tokens": map[string]string{
			"cloudId":  "test-cloud-id",
			"atlToken": "test-atl-token",
		},
	}

	jsonData, err := json.Marshal(authData)
	if err != nil {
		t.Fatalf("Failed to marshal auth data: %v", err)
	}

	// Send POST request
	resp, err := http.Post(baseURL+"/api/auth", "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		t.Fatalf("Failed to post auth data: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}

	// Verify response
	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if status, ok := result["status"].(string); !ok || status != "success" {
		t.Errorf("Expected success status, got %v", result["status"])
	}

	t.Log("✓ Authentication capture endpoint works")
}

// TestAuthStatusEndpoint tests the GET /api/auth/status endpoint
func TestAuthStatusEndpoint(t *testing.T) {
	baseURL := test.MustGetTestServerURL()

	resp, err := http.Get(baseURL + "/api/auth/status")
	if err != nil {
		t.Fatalf("Failed to get auth status: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}

	// Parse response
	var status map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&status); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	// Should have authenticated field
	if _, ok := status["authenticated"]; !ok {
		t.Error("Response missing 'authenticated' field")
	}

	t.Logf("✓ Auth status: authenticated=%v", status["authenticated"])
}
