package handlers

import (
	"html/template"
	"net/http"
	"os"
	"path/filepath"

	"github.com/ternarybob/arbor"
)

type PageHandler struct {
	logger      arbor.ILogger
	templates   *template.Template
	clientDebug bool
}

func NewPageHandler(logger arbor.ILogger, clientDebug bool) *PageHandler {
	// Find pages directory (in bin/ after build)
	pagesDir := findPagesDir()

	// Parse all HTML templates including partials
	templates := template.Must(template.ParseGlob(filepath.Join(pagesDir, "*.html")))
	template.Must(templates.ParseGlob(filepath.Join(pagesDir, "partials", "*.html")))

	return &PageHandler{
		logger:      logger,
		templates:   templates,
		clientDebug: clientDebug,
	}
}

// findPagesDir locates the pages directory
func findPagesDir() string {
	// Check common locations
	dirs := []string{
		"./pages",     // Running from project root
		"../pages",    // Running from bin/
		"../../pages", // Running from deeper location
		".",           // Current directory (for deployed bin/)
	}

	for _, dir := range dirs {
		if _, err := os.Stat(dir); err == nil {
			abs, _ := filepath.Abs(dir)
			return abs
		}
	}

	return "."
}

// ServePagereates a handler function for serving a specific page template
func (h *PageHandler) ServePage(templateName string, pageName string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		data := map[string]interface{}{
			"Page":        pageName,
			"ClientDebug": h.clientDebug,
		}

		if err := h.templates.ExecuteTemplate(w, templateName, data); err != nil {
			h.logger.Error().
				Err(err).
				Str("template", templateName).
				Msg("Failed to render page")
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		}
	}
}

// StaticFileHandler serves static files (CSS, JS, images)
func (h *PageHandler) StaticFileHandler(w http.ResponseWriter, r *http.Request) {
	// Serve from static directory
	pagesDir := findPagesDir()
	staticDir := filepath.Join(pagesDir, "static")

	// Remove /static prefix from URL path
	path := r.URL.Path[len("/static/"):]
	fullPath := filepath.Join(staticDir, path)

	// Security check - prevent directory traversal
	if !filepath.HasPrefix(fullPath, staticDir) {
		http.NotFound(w, r)
		return
	}

	http.ServeFile(w, r, fullPath)
}

// PartialFileHandler serves partial HTML files for AJAX loading
func (h *PageHandler) PartialFileHandler(w http.ResponseWriter, r *http.Request) {
	// Serve from partials directory
	pagesDir := findPagesDir()
	partialsDir := filepath.Join(pagesDir, "partials")

	// Remove /partials prefix from URL path
	path := r.URL.Path[len("/partials/"):]
	fullPath := filepath.Join(partialsDir, path)

	// Security check - prevent directory traversal
	if !filepath.HasPrefix(fullPath, partialsDir) {
		http.NotFound(w, r)
		return
	}

	// Only serve .html files
	if filepath.Ext(fullPath) != ".html" {
		http.NotFound(w, r)
		return
	}

	// Read file content directly (don't use http.ServeFile to avoid template execution)
	content, err := os.ReadFile(fullPath)
	if err != nil {
		h.logger.Error().
			Err(err).
			Str("path", fullPath).
			Msg("Failed to read partial file")
		http.NotFound(w, r)
		return
	}

	// Set content type to HTML and serve raw content
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write(content)
}

// ServePartial serves partial HTML fragments for settings page accordion items
func (h *PageHandler) ServePartial(w http.ResponseWriter, r *http.Request) {
	// Extract partial name from /settings/ prefix
	partialName := r.URL.Path[len("/settings/"):]

	// Security validation using allowlist approach
	allowedPartials := map[string]bool{
		"auth-apikeys.html": true,
		"auth-cookies.html": true,
		"config.html":       true,
		"danger.html":       true,
		"status.html":       true,
		"logs.html":         true,
	}

	if !allowedPartials[partialName] {
		h.logger.Warn().
			Str("requested_partial", partialName).
			Str("remote_addr", r.RemoteAddr).
			Msg("Attempt to access non-allowed partial file")
		http.NotFound(w, r)
		return
	}

	// File path mapping: prepend "settings-" to match actual filename
	mappedFilename := "settings-" + partialName

	// Locate pages directory and construct full path
	pagesDir := findPagesDir()
	partialsDir := filepath.Join(pagesDir, "partials")
	fullPath := filepath.Join(partialsDir, mappedFilename)

	// File existence check
	if _, err := os.Stat(fullPath); os.IsNotExist(err) {
		h.logger.Error().
			Str("requested_partial", partialName).
			Str("mapped_filename", mappedFilename).
			Str("full_path", fullPath).
			Err(err).
			Msg("Partial file not found")
		http.NotFound(w, r)
		return
	}

	// Set Content-Type header for HTML fragments
	w.Header().Set("Content-Type", "text/html; charset=utf-8")

	// Serve the partial HTML file
	http.ServeFile(w, r, fullPath)
}
