package httpx

import "net/http"

// WithCORS allows cross-origin requests from a single allowed origin - the
// frontend, which is always a different origin from the API even in dev
// (Next.js on :3000, the API on :8080), and a different domain again in
// production (Vercel vs. Fly.io). Handles the browser's OPTIONS preflight.
func WithCORS(allowedOrigin string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Access-Control-Allow-Origin", allowedOrigin)
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

			if r.Method == http.MethodOptions {
				w.WriteHeader(http.StatusNoContent)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
