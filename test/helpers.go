package test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"testing"
	"time"
)

// HTTPTestHelper provides helper methods for HTTP testing
type HTTPTestHelper struct {
	BaseURL string
	Client  *http.Client
	T       *testing.T
}

// NewHTTPTestHelper creates a new HTTP test helper
func NewHTTPTestHelper(t *testing.T) *HTTPTestHelper {
	return &HTTPTestHelper{
		BaseURL: GetTestServerURL(),
		Client:  &http.Client{Timeout: 30 * time.Second},
		T:       t,
	}
}

// GET makes a GET request and returns the response
func (h *HTTPTestHelper) GET(path string) (*http.Response, error) {
	url := h.BaseURL + path
	h.T.Logf("GET %s", url)
	return h.Client.Get(url)
}

// POST makes a POST request with JSON body
func (h *HTTPTestHelper) POST(path string, body interface{}) (*http.Response, error) {
	url := h.BaseURL + path
	h.T.Logf("POST %s", url)

	var reqBody io.Reader
	if body != nil {
		jsonBytes, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal request body: %w", err)
		}
		reqBody = bytes.NewBuffer(jsonBytes)
	}

	req, err := http.NewRequest(http.MethodPost, url, reqBody)
	if err != nil {
		return nil, err
	}

	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	return h.Client.Do(req)
}

// PUT makes a PUT request with JSON body
func (h *HTTPTestHelper) PUT(path string, body interface{}) (*http.Response, error) {
	url := h.BaseURL + path
	h.T.Logf("PUT %s", url)

	jsonBytes, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request body: %w", err)
	}

	req, err := http.NewRequest(http.MethodPut, url, bytes.NewBuffer(jsonBytes))
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/json")

	return h.Client.Do(req)
}

// DELETE makes a DELETE request
func (h *HTTPTestHelper) DELETE(path string) (*http.Response, error) {
	url := h.BaseURL + path
	h.T.Logf("DELETE %s", url)

	req, err := http.NewRequest(http.MethodDelete, url, nil)
	if err != nil {
		return nil, err
	}

	return h.Client.Do(req)
}

// AssertStatusCode verifies the response status code
func (h *HTTPTestHelper) AssertStatusCode(resp *http.Response, expected int) {
	if resp.StatusCode != expected {
		h.T.Errorf("Expected status code %d, got %d", expected, resp.StatusCode)
	}
}

// ParseJSONResponse parses JSON response into target
func (h *HTTPTestHelper) ParseJSONResponse(resp *http.Response, target interface{}) error {
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response body: %w", err)
	}

	h.T.Logf("Response body: %s", string(body))

	if err := json.Unmarshal(body, target); err != nil {
		return fmt.Errorf("failed to parse JSON response: %w", err)
	}

	return nil
}

// AssertJSONField checks if a JSON field has the expected value
func (h *HTTPTestHelper) AssertJSONField(resp *http.Response, field string, expected interface{}) {
	defer resp.Body.Close()

	var result map[string]interface{}
	if err := h.ParseJSONResponse(resp, &result); err != nil {
		h.T.Fatalf("Failed to parse JSON: %v", err)
	}

	actual, ok := result[field]
	if !ok {
		h.T.Errorf("Field '%s' not found in response", field)
		return
	}

	if actual != expected {
		h.T.Errorf("Field '%s': expected %v, got %v", field, expected, actual)
	}
}

// Retry retries a function until it succeeds or times out
func Retry(fn func() error, maxAttempts int, delay time.Duration) error {
	var lastErr error

	for i := 0; i < maxAttempts; i++ {
		err := fn()
		if err == nil {
			return nil
		}

		lastErr = err
		if i < maxAttempts-1 {
			time.Sleep(delay)
		}
	}

	return fmt.Errorf("retry failed after %d attempts: %w", maxAttempts, lastErr)
}
