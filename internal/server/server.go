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
	app    *app.App
	router *http.ServeMux
	server *http.Server
}

// New creates a new HTTP server with the given app
func New(application *app.App) *Server {
	s := &Server{
		app: application,
	}

	// Setup routes
	s.router = s.setupRoutes()

	// Create HTTP server
	addr := fmt.Sprintf("%s:%d", application.Config.Server.Host, application.Config.Server.Port)
	s.server = &http.Server{
		Addr:         addr,
		Handler:      s.withMiddleware(s.router),
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	return s
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
