// -----------------------------------------------------------------------
// Last Modified: Wednesday, 8th October 2025 5:36:23 pm
// Modified By: Bob McAllan
// -----------------------------------------------------------------------

package server

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/ternarybob/quaero/internal/app"
)

// Server manages the HTTP server and routes
type Server struct {
	app          *app.App
	router       *http.ServeMux
	server       *http.Server
	shutdownChan chan struct{}
}

// New creates a new HTTP server with the given app
func New(application *app.App) *Server {
	s := &Server{
		app: application,
	}

	// Setup routes
	s.router = s.setupRoutes()

	// Create HTTP server with middleware, but bypass for WebSocket
	addr := fmt.Sprintf("%s:%d", application.Config.Server.Host, application.Config.Server.Port)
	s.server = &http.Server{
		Addr:         addr,
		Handler:      s.withConditionalMiddleware(s.router),
		ReadTimeout:  30 * time.Second,  // Increased to handle large request bodies
		WriteTimeout: 360 * time.Second, // 6 minutes - Extended for local LLM processing (chat can take 4+ minutes)
		IdleTimeout:  120 * time.Second, // Increased for long-polling connections
	}

	return s
}

// SetShutdownChannel sets the channel that will be signaled when HTTP shutdown is requested
func (s *Server) SetShutdownChannel(ch chan struct{}) {
	s.shutdownChan = ch
}

// Start starts the HTTP server
func (s *Server) Start() error {
	addr := fmt.Sprintf("%s:%d", s.app.Config.Server.Host, s.app.Config.Server.Port)

	s.app.Logger.Info().
		Str("address", addr).
		Msg("HTTP server starting")

	s.app.Logger.Info().Msg("Install Chrome extension and click icon when logged into Jira/Confluence")
	s.app.Logger.Info().
		Str("url", fmt.Sprintf("http://%s:%d", s.app.Config.Server.Host, s.app.Config.Server.Port)).
		Msg("Web UI available")

	if err := s.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		return fmt.Errorf("server failed: %w", err)
	}

	return nil
}

// Shutdown gracefully shuts down the server
func (s *Server) Shutdown(ctx context.Context) error {
	s.app.Logger.Info().Msg("Shutting down HTTP server...")

	if err := s.server.Shutdown(ctx); err != nil {
		return fmt.Errorf("server shutdown failed: %w", err)
	}

	s.app.Logger.Info().Msg("HTTP server stopped")
	return nil
}

// Handler returns the HTTP handler for testing
func (s *Server) Handler() http.Handler {
	return s.server.Handler
}

// ShutdownHandler handles HTTP shutdown requests (dev mode only)
func (s *Server) ShutdownHandler(w http.ResponseWriter, r *http.Request) {
	// Only allow POST requests
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	s.app.Logger.Info().Msg("Shutdown requested via HTTP endpoint")

	// Send response before shutting down
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("Shutting down gracefully...\n"))

	// Flush response to ensure client receives it
	if flusher, ok := w.(http.Flusher); ok {
		flusher.Flush()
	}

	// Trigger shutdown if channel is set (with small delay to ensure response sent)
	if s.shutdownChan != nil {
		go func() {
			time.Sleep(100 * time.Millisecond) // Allow response to be sent
			s.shutdownChan <- struct{}{}
		}()
	}
}
