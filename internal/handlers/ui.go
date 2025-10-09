// -----------------------------------------------------------------------
// Last Modified: Thursday, 9th October 2025 8:05:44 am
// Modified By: Bob McAllan
// -----------------------------------------------------------------------

package handlers

import (
	"html/template"
	"net/http"
	"os"
	"path/filepath"

	"github.com/ternarybob/arbor"
	"github.com/ternarybob/quaero/internal/common"
	"github.com/ternarybob/quaero/internal/interfaces"
)

type UIHandler struct {
	logger            arbor.ILogger
	staticDir         string
	jiraScraper       interfaces.JiraScraper
	confluenceScraper interfaces.ConfluenceScraper
	authService       interfaces.AuthService
	templates         *template.Template
}

func NewUIHandler(jira interfaces.JiraScraper, confluence interfaces.ConfluenceScraper, auth interfaces.AuthService) *UIHandler {
	staticDir := getStaticDir()

	// Parse all HTML templates including partials
	templates := template.Must(template.ParseGlob(filepath.Join(staticDir, "*.html")))
	template.Must(templates.ParseGlob(filepath.Join(staticDir, "partials", "*.html")))

	return &UIHandler{
		logger:            common.GetLogger(),
		staticDir:         staticDir,
		jiraScraper:       jira,
		confluenceScraper: confluence,
		authService:       auth,
		templates:         templates,
	}
}

// getStaticDir finds the pages directory
func getStaticDir() string {
	dirs := []string{
		"./pages",
		"../pages",
		"../../pages",
	}

	for _, dir := range dirs {
		if _, err := os.Stat(dir); err == nil {
			abs, _ := filepath.Abs(dir)
			return abs
		}
	}

	return "."
}

// IndexHandler serves the main HTML page
func (h *UIHandler) IndexHandler(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}

	data := map[string]interface{}{
		"Page": "home",
	}

	if err := h.templates.ExecuteTemplate(w, "index.html", data); err != nil {
		h.logger.Error().Err(err).Msg("Failed to render index")
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}

// JiraPageHandler serves the Jira data page
func (h *UIHandler) JiraPageHandler(w http.ResponseWriter, r *http.Request) {
	data := map[string]interface{}{
		"Page": "jira",
	}

	if err := h.templates.ExecuteTemplate(w, "jira.html", data); err != nil {
		h.logger.Error().Err(err).Msg("Failed to render jira")
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}

// ConfluencePageHandler serves the Confluence data page
func (h *UIHandler) ConfluencePageHandler(w http.ResponseWriter, r *http.Request) {
	data := map[string]interface{}{
		"Page": "confluence",
	}

	if err := h.templates.ExecuteTemplate(w, "confluence.html", data); err != nil {
		h.logger.Error().Err(err).Msg("Failed to render confluence")
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}

// DocumentsPageHandler serves the Documents page
func (h *UIHandler) DocumentsPageHandler(w http.ResponseWriter, r *http.Request) {
	data := map[string]interface{}{
		"Page": "documents",
	}

	if err := h.templates.ExecuteTemplate(w, "documents.html", data); err != nil {
		h.logger.Error().Err(err).Msg("Failed to render documents")
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}

// SettingsPageHandler serves the Settings page
func (h *UIHandler) SettingsPageHandler(w http.ResponseWriter, r *http.Request) {
	data := map[string]interface{}{
		"Page": "settings",
	}

	if err := h.templates.ExecuteTemplate(w, "settings.html", data); err != nil {
		h.logger.Error().Err(err).Msg("Failed to render settings")
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}

// EmbeddingsPageHandler serves the Embeddings page
func (h *UIHandler) EmbeddingsPageHandler(w http.ResponseWriter, r *http.Request) {
	data := map[string]interface{}{
		"Page": "embeddings",
	}

	if err := h.templates.ExecuteTemplate(w, "embeddings.html", data); err != nil {
		h.logger.Error().Err(err).Msg("Failed to render embeddings")
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}

// ChatPageHandler serves the Chat page
func (h *UIHandler) ChatPageHandler(w http.ResponseWriter, r *http.Request) {
	data := map[string]interface{}{
		"Page": "chat",
	}

	if err := h.templates.ExecuteTemplate(w, "chat.html", data); err != nil {
		h.logger.Error().Err(err).Msg("Failed to render chat")
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}

// StaticFileHandler serves static files (CSS, JS, favicon) from the pages/static directory
func (h *UIHandler) StaticFileHandler(w http.ResponseWriter, r *http.Request) {
	// List of allowed static files
	allowedFiles := map[string]string{
		"/static/common.css":            "static/common.css",
		"/static/websocket-manager.js":  "static/websocket-manager.js",
		"/static/alpine-components.js":  "static/alpine-components.js",
		"/favicon.ico":                  "static/favicon.ico",
		"/partials/navbar.html":         "partials/navbar.html",
		"/partials/footer.html":         "partials/footer.html",
		"/partials/service-status.html": "partials/service-status.html",
		"/partials/service-logs.html":   "partials/service-logs.html",
		"/partials/snackbar.html":       "partials/snackbar.html",
	}

	// Check if the requested path is allowed
	relativePath, allowed := allowedFiles[r.URL.Path]
	if !allowed {
		http.NotFound(w, r)
		return
	}

	// Construct the full path
	filePath := filepath.Join(h.staticDir, relativePath)

	// Check if file exists
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		http.NotFound(w, r)
		return
	}

	// Set appropriate content type
	ext := filepath.Ext(filePath)
	switch ext {
	case ".css":
		w.Header().Set("Content-Type", "text/css")
	case ".js":
		w.Header().Set("Content-Type", "application/javascript")
	case ".ico":
		w.Header().Set("Content-Type", "image/x-icon")
	case ".html":
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
	}

	// Serve the file
	http.ServeFile(w, r, filePath)
}
