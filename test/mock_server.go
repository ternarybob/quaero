package test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"
)

// MockServer is a lightweight mock HTTP server for isolated testing
type MockServer struct {
	server  *http.Server
	sources map[string]interface{}
	auths   map[string]interface{}
	jobs    map[string]interface{}
	mu      sync.RWMutex
	port    int
}

// NewMockServer creates a new mock server instance
func NewMockServer(port int) *MockServer {
	ms := &MockServer{
		sources: make(map[string]interface{}),
		auths:   make(map[string]interface{}),
		jobs:    make(map[string]interface{}),
		port:    port,
	}

	mux := http.NewServeMux()

	// Source endpoints
	mux.HandleFunc("/api/sources", ms.handleSources)
	mux.HandleFunc("/api/sources/", ms.handleSourceByID)

	// Auth endpoints
	mux.HandleFunc("/api/auth/list", ms.handleAuthList)
	mux.HandleFunc("/api/auth/status", ms.handleAuthStatus)
	mux.HandleFunc("/api/auth", ms.handleAuth)

	// Job endpoints
	mux.HandleFunc("/api/jobs/", ms.handleJobByID)
	mux.HandleFunc("/api/jobs", ms.handleJobs)

	// Config endpoint
	mux.HandleFunc("/api/config", ms.handleConfig)

	// Status endpoint
	mux.HandleFunc("/api/status", ms.handleStatus)

	ms.server = &http.Server{
		Addr:    fmt.Sprintf(":%d", port),
		Handler: mux,
	}

	return ms
}

// Start starts the mock server in a goroutine
func (ms *MockServer) Start() error {
	go func() {
		if err := ms.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			fmt.Printf("Mock server error: %v\n", err)
		}
	}()

	// Wait a bit for server to start
	time.Sleep(100 * time.Millisecond)
	return nil
}

// Stop stops the mock server gracefully
func (ms *MockServer) Stop() error {
	if ms.server != nil {
		// Create context with timeout for graceful shutdown
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		// Try graceful shutdown first
		if err := ms.server.Shutdown(ctx); err != nil {
			// Fall back to immediate close on error
			return ms.server.Close()
		}
		return nil
	}
	return nil
}

// Reset clears all in-memory data
func (ms *MockServer) Reset() {
	ms.mu.Lock()
	defer ms.mu.Unlock()

	ms.sources = make(map[string]interface{})
	ms.auths = make(map[string]interface{})
	ms.jobs = make(map[string]interface{})
}

// handleSources handles GET and POST /api/sources
func (ms *MockServer) handleSources(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		ms.mu.RLock()
		defer ms.mu.RUnlock()

		sourceList := make([]interface{}, 0, len(ms.sources))
		for _, source := range ms.sources {
			sourceList = append(sourceList, source)
		}

		respondJSON(w, http.StatusOK, sourceList)

	case http.MethodPost:
		var source map[string]interface{}
		if err := json.NewDecoder(r.Body).Decode(&source); err != nil {
			respondError(w, http.StatusBadRequest, "Invalid request body")
			return
		}

		ms.mu.Lock()
		defer ms.mu.Unlock()

		id := generateID("src")
		source["id"] = id
		source["created_at"] = time.Now().Format(time.RFC3339)
		source["updated_at"] = time.Now().Format(time.RFC3339)

		ms.sources[id] = source

		respondJSON(w, http.StatusCreated, source)

	default:
		respondError(w, http.StatusMethodNotAllowed, "Method not allowed")
	}
}

// handleSourceByID handles GET, PUT, DELETE /api/sources/{id}
func (ms *MockServer) handleSourceByID(w http.ResponseWriter, r *http.Request) {
	// Extract ID from path
	id := strings.TrimPrefix(r.URL.Path, "/api/sources/")
	if id == "" {
		respondError(w, http.StatusBadRequest, "Missing source ID")
		return
	}

	switch r.Method {
	case http.MethodGet:
		ms.mu.RLock()
		defer ms.mu.RUnlock()

		source, ok := ms.sources[id]
		if !ok {
			respondError(w, http.StatusNotFound, "Source not found")
			return
		}

		respondJSON(w, http.StatusOK, source)

	case http.MethodPut:
		var updates map[string]interface{}
		if err := json.NewDecoder(r.Body).Decode(&updates); err != nil {
			respondError(w, http.StatusBadRequest, "Invalid request body")
			return
		}

		ms.mu.Lock()
		defer ms.mu.Unlock()

		source, ok := ms.sources[id]
		if !ok {
			respondError(w, http.StatusNotFound, "Source not found")
			return
		}

		// Merge updates
		sourceMap := source.(map[string]interface{})
		for k, v := range updates {
			sourceMap[k] = v
		}
		sourceMap["updated_at"] = time.Now().Format(time.RFC3339)

		ms.sources[id] = sourceMap

		respondJSON(w, http.StatusOK, sourceMap)

	case http.MethodDelete:
		ms.mu.Lock()
		defer ms.mu.Unlock()

		if _, ok := ms.sources[id]; !ok {
			respondError(w, http.StatusNotFound, "Source not found")
			return
		}

		delete(ms.sources, id)
		w.WriteHeader(http.StatusNoContent)

	default:
		respondError(w, http.StatusMethodNotAllowed, "Method not allowed")
	}
}

// handleAuthList handles GET /api/auth/list
func (ms *MockServer) handleAuthList(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		respondError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	ms.mu.RLock()
	defer ms.mu.RUnlock()

	authList := make([]interface{}, 0, len(ms.auths))
	for _, authInterface := range ms.auths {
		// Ensure each auth has site_domain derived from baseUrl
		auth := authInterface.(map[string]interface{})

		// Extract site_domain from baseUrl if not already present
		if _, hasSiteDomain := auth["site_domain"]; !hasSiteDomain {
			if baseURL, ok := auth["baseUrl"].(string); ok {
				// Extract host from URL (remove scheme)
				siteDomain := strings.TrimPrefix(baseURL, "https://")
				siteDomain = strings.TrimPrefix(siteDomain, "http://")
				// Remove path if present
				if idx := strings.Index(siteDomain, "/"); idx != -1 {
					siteDomain = siteDomain[:idx]
				}
				auth["site_domain"] = siteDomain
			}
		}

		authList = append(authList, auth)
	}

	respondJSON(w, http.StatusOK, authList)
}

// handleAuthStatus handles GET /api/auth/status
func (ms *MockServer) handleAuthStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		respondError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"authenticated": true,
	})
}

// handleAuth handles POST /api/auth and DELETE /api/auth/{id}
func (ms *MockServer) handleAuth(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodPost:
		var auth map[string]interface{}
		if err := json.NewDecoder(r.Body).Decode(&auth); err != nil {
			respondError(w, http.StatusBadRequest, "Invalid request body")
			return
		}

		ms.mu.Lock()
		defer ms.mu.Unlock()

		id := generateID("auth")
		auth["id"] = id
		auth["created_at"] = time.Now().Format(time.RFC3339)

		ms.auths[id] = auth

		// Return response with status=success as expected by tests
		respondJSON(w, http.StatusOK, map[string]interface{}{
			"status":     "success",
			"credential": auth,
		})

	case http.MethodDelete:
		// Extract ID from path
		id := strings.TrimPrefix(r.URL.Path, "/api/auth/")
		if id == "" {
			respondError(w, http.StatusBadRequest, "Missing auth ID")
			return
		}

		ms.mu.Lock()
		defer ms.mu.Unlock()

		if _, ok := ms.auths[id]; !ok {
			respondError(w, http.StatusNotFound, "Auth not found")
			return
		}

		delete(ms.auths, id)
		w.WriteHeader(http.StatusNoContent)

	default:
		respondError(w, http.StatusMethodNotAllowed, "Method not allowed")
	}
}

// handleJobs handles POST /api/jobs
func (ms *MockServer) handleJobs(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		respondError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	var job map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&job); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	ms.mu.Lock()
	defer ms.mu.Unlock()

	id := generateID("job")
	job["id"] = id
	job["status"] = "pending"
	job["created_at"] = time.Now().Format(time.RFC3339)

	ms.jobs[id] = job

	respondJSON(w, http.StatusCreated, job)
}

// handleJobByID handles GET /api/jobs/{id}
func (ms *MockServer) handleJobByID(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		respondError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	// Extract ID from path
	id := strings.TrimPrefix(r.URL.Path, "/api/jobs/")
	if id == "" {
		respondError(w, http.StatusBadRequest, "Missing job ID")
		return
	}

	ms.mu.RLock()
	defer ms.mu.RUnlock()

	job, ok := ms.jobs[id]
	if !ok {
		respondError(w, http.StatusNotFound, "Job not found")
		return
	}

	respondJSON(w, http.StatusOK, job)
}

// handleConfig handles GET /api/config
func (ms *MockServer) handleConfig(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		respondError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"server": map[string]interface{}{
			"host": "localhost",
			"port": ms.port,
		},
		"llm": map[string]interface{}{
			"mode": "mock",
		},
	})
}

// handleStatus handles GET /api/status
func (ms *MockServer) handleStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		respondError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"status": "ok",
	})
}

// generateID generates a unique ID with prefix
func generateID(prefix string) string {
	return fmt.Sprintf("%s_%d", prefix, time.Now().UnixNano())
}

// respondJSON sends a JSON response
func respondJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

// respondError sends an error response
func respondError(w http.ResponseWriter, status int, message string) {
	respondJSON(w, status, map[string]interface{}{
		"error": message,
	})
}
