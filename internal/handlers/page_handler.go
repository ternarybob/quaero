package handlers

import (
	"html/template"
	"net/http"
	"os"
	"path/filepath"

	"github.com/ternarybob/arbor"
)

type PageHandler struct {
	logger    arbor.ILogger
	templates *template.Template
}

func NewPageHandler(logger arbor.ILogger) *PageHandler {
	// Find pages directory (in bin/ after build)
	pagesDir := findPagesDir()

	// Parse all HTML templates including partials
	templates := template.Must(template.ParseGlob(filepath.Join(pagesDir, "*.html")))
	template.Must(templates.ParseGlob(filepath.Join(pagesDir, "partials", "*.html")))

	return &PageHandler{
		logger:    logger,
		templates: templates,
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
			"Page": pageName,
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
