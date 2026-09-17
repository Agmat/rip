package httpx_test

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Agmat/rip/backend/internal/httpx"
)

func healthzHandler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		httpx.WriteJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	return httpx.WithRequestID(httpx.WithLogging(logger)(mux))
}

func TestHealthz_OK(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	rec := httptest.NewRecorder()

	healthzHandler().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/json" {
		t.Fatalf("Content-Type = %q, want application/json", ct)
	}

	var body map[string]string
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if body["status"] != "ok" {
		t.Fatalf("status field = %q, want ok", body["status"])
	}
}

func TestHealthz_RequestIDGeneratedWhenAbsent(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	rec := httptest.NewRecorder()

	healthzHandler().ServeHTTP(rec, req)

	id := rec.Header().Get(httpx.RequestIDHeader)
	if id == "" {
		t.Fatal("expected a generated X-Request-ID, got none")
	}
}

func TestHealthz_RequestIDEchoed(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	req.Header.Set(httpx.RequestIDHeader, "test-id-123")
	rec := httptest.NewRecorder()

	healthzHandler().ServeHTTP(rec, req)

	if got := rec.Header().Get(httpx.RequestIDHeader); got != "test-id-123" {
		t.Fatalf("X-Request-ID = %q, want %q", got, "test-id-123")
	}
}

func TestRequestIDFromContext_EmptyOutsideMiddleware(t *testing.T) {
	if id := httpx.RequestIDFromContext(context.Background()); id != "" {
		t.Fatalf("RequestIDFromContext on bare context = %q, want empty", id)
	}
}
