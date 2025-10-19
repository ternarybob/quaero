package main

import (
	"fmt"
	"net/http"
	"time"
)

// StartTestServer starts a simple HTTP server on port 3333 for testing browser automation
func StartTestServer() *http.Server {
	mux := http.NewServeMux()

	// Simple test page
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		html := `<!DOCTYPE html>
<html>
<head>
    <title>Test Server - Working</title>
</head>
<body>
    <h1 id="test-title">Browser Automation Test</h1>
    <p id="test-message">If you can see this, browser automation is working!</p>
    <button id="test-button">Click Me</button>
    <div id="test-output"></div>
    <script>
        document.getElementById('test-button').addEventListener('click', function() {
            document.getElementById('test-output').textContent = 'Button clicked!';
        });
    </script>
</body>
</html>`
		w.Header().Set("Content-Type", "text/html")
		w.Write([]byte(html))
	})

	// Status endpoint
	mux.HandleFunc("/status", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"status":"ok","server":"test","timestamp":"` + time.Now().Format(time.RFC3339) + `"}`))
	})

	// Mock Jira REST API endpoint for testing crawl → transform flow
	mux.HandleFunc("/rest/api/3/project", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{
			"title": "Fixture Project",
			"description": "Hello world",
			"_links": {
				"self": "http://localhost:3333/rest/api/3/project"
			}
		}`))
	})

	server := &http.Server{
		Addr:    ":3333",
		Handler: mux,
	}

	go func() {
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			fmt.Printf("Test server error: %v\n", err)
		}
	}()

	// Give server time to start
	time.Sleep(500 * time.Millisecond)

	return server
}
