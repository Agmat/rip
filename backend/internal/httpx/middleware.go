package httpx

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"log/slog"
	"net/http"
	"time"
)

// RequestIDHeader is the header used both to accept a caller-supplied
// request ID and to echo it back in the response.
const RequestIDHeader = "X-Request-ID"

type contextKey int

const requestIDKey contextKey = iota

// RequestIDFromContext returns the request ID stored by WithRequestID, or
// "" if none is present (e.g. outside a request, or in a test that didn't
// go through the middleware).
func RequestIDFromContext(ctx context.Context) string {
	id, _ := ctx.Value(requestIDKey).(string)
	return id
}

// WithRequestID reads X-Request-ID from the incoming request, generating one
// if absent, echoes it on the response, and stores it in the request context
// for downstream handlers and logging to pick up.
func WithRequestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := r.Header.Get(RequestIDHeader)
		if id == "" {
			id = newRequestID()
		}
		w.Header().Set(RequestIDHeader, id)

		ctx := context.WithValue(r.Context(), requestIDKey, id)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func newRequestID() string {
	b := make([]byte, 16)
	// crypto/rand.Read only errors if the OS entropy source is unavailable,
	// which would mean the process can't do much else either; an all-zero
	// ID in that case is an acceptable degradation for request tracing.
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

// statusRecorder wraps http.ResponseWriter to capture the status code
// written, so logging middleware can report it after the handler returns.
type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (rec *statusRecorder) WriteHeader(status int) {
	rec.status = status
	rec.ResponseWriter.WriteHeader(status)
}

// WithLogging logs one structured line per request: method, path, status,
// duration, and request ID (must run after WithRequestID in the chain).
func WithLogging(logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}

			next.ServeHTTP(rec, r)

			logger.Info("request",
				"method", r.Method,
				"path", r.URL.Path,
				"status", rec.status,
				"duration_ms", time.Since(start).Milliseconds(),
				"request_id", RequestIDFromContext(r.Context()),
			)
		})
	}
}
