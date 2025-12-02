// -----------------------------------------------------------------------
// Last Modified: Wednesday, 8th October 2025 11:48:25 am
// Modified By: Bob McAllan
// -----------------------------------------------------------------------

package server

import (
	"bufio"
	"context"
	"fmt"
	"net"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/ternarybob/arbor"
)

// Context key for correlation ID
type contextKey string

const correlationIDKey contextKey = "correlation_id"

// withMiddleware wraps the router with middleware chain
func (s *Server) withMiddleware(handler http.Handler) http.Handler {
	// Apply middleware in reverse order (last applied = first executed)
	handler = s.recoveryMiddleware(handler)
	handler = s.corsMiddleware(handler)
	handler = s.loggingMiddleware(handler)
	handler = s.correlationIDMiddleware(handler)
	return handler
}

// withConditionalMiddleware applies middleware but bypasses it for WebSocket routes
func (s *Server) withConditionalMiddleware(handler http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Bypass middleware for WebSocket upgrade requests
		if r.URL.Path == "/ws" {
			// Only apply CORS for WebSocket (needed for cross-origin)
			w.Header().Set("Access-Control-Allow-Origin", "*")
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

			// Skip logging and other middleware that might interfere
			handler.ServeHTTP(w, r)
			return
		}

		// Apply full middleware chain for all other routes
		s.withMiddleware(handler).ServeHTTP(w, r)
	})
}

// correlationIDMiddleware extracts or generates a correlation ID for request tracking
func (s *Server) correlationIDMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Try to extract correlation ID from headers
		correlationID := r.Header.Get("X-Request-ID")
		if correlationID == "" {
			correlationID = r.Header.Get("X-Correlation-ID")
		}

		// Generate new correlation ID if not provided
		if correlationID == "" {
			correlationID = uuid.New().String()
		}

		// Add correlation ID to response header
		w.Header().Set("X-Correlation-ID", correlationID)

		// Store in request context
		ctx := context.WithValue(r.Context(), correlationIDKey, correlationID)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// loggingMiddleware logs HTTP requests and responses
func (s *Server) loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		// Create response writer wrapper to capture status code and bytes
		rw := &responseWriter{ResponseWriter: w, statusCode: http.StatusOK}

		// Call next handler
		next.ServeHTTP(rw, r)

		// Calculate duration in milliseconds
		durationMs := time.Since(start).Milliseconds()

		// Extract correlation ID from context
		correlationID, _ := r.Context().Value(correlationIDKey).(string)

		// Select log level and message based on status code
		var logMsg string
		var logEvent arbor.ILogEvent

		switch {
		case rw.statusCode >= 500:
			// 5xx errors - log as error
			logMsg = "HTTP request - server error"
			logEvent = s.app.Logger.Error()
		case rw.statusCode >= 400:
			// 4xx errors - log as warning
			logMsg = "HTTP request - client error"
			logEvent = s.app.Logger.Warn()
		default:
			// Success (2xx, 3xx) - log as trace (routine operation)
			logMsg = "HTTP request"
			logEvent = s.app.Logger.Trace()
		}

		// Build structured log event with fields
		logEvent.
			Str("correlation_id", correlationID).
			Str("method", r.Method).
			Str("path", r.URL.Path).
			Int("status", rw.statusCode).
			Int64("duration_ms", durationMs).
			Int("bytes", rw.bytesWritten).
			Str("remote", r.RemoteAddr)

		// Add query parameters if present
		if r.URL.RawQuery != "" {
			logEvent.Str("query", r.URL.RawQuery)
		}

		// Log the message
		logEvent.Msg(logMsg)
	})
}

// corsMiddleware handles CORS headers for browser extension
func (s *Server) corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Allow all origins for local development
		// In production, restrict to specific origins
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

		// Handle preflight requests
		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}

		next.ServeHTTP(w, r)
	})
}

// recoveryMiddleware recovers from panics and returns 500 error
func (s *Server) recoveryMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if err := recover(); err != nil {
				// Extract correlation ID from context
				correlationID, _ := r.Context().Value(correlationIDKey).(string)

				s.app.Logger.Error().
					Str("correlation_id", correlationID).
					Str("error", fmt.Sprintf("%v", err)).
					Str("path", r.URL.Path).
					Msg("Panic recovered")

				http.Error(w, "Internal server error", http.StatusInternalServerError)
			}
		}()

		next.ServeHTTP(w, r)
	})
}

// responseWriter wraps http.ResponseWriter to capture status code and bytes written
type responseWriter struct {
	http.ResponseWriter
	statusCode   int
	bytesWritten int
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}

func (rw *responseWriter) Write(b []byte) (int, error) {
	n, err := rw.ResponseWriter.Write(b)
	rw.bytesWritten += n
	return n, err
}

// Hijack implements http.Hijacker interface for WebSocket support
func (rw *responseWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	if hijacker, ok := rw.ResponseWriter.(http.Hijacker); ok {
		return hijacker.Hijack()
	}
	return nil, nil, fmt.Errorf("responseWriter does not implement http.Hijacker")
}
