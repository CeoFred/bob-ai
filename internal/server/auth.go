package server

import (
	"crypto/subtle"
	"net/http"
	"strings"
)

// AuthMiddleware enforces API token validation if a token is configured.
func AuthMiddleware(token string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Health check is always public
		if r.URL.Path == "/health" {
			next.ServeHTTP(w, r)
			return
		}

		// If no token is configured, allow requests (useful for local development or Tailscale-only networks)
		if token == "" {
			next.ServeHTTP(w, r)
			return
		}

		provided := extractToken(r)
		if subtle.ConstantTimeCompare([]byte(provided), []byte(token)) != 1 {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = w.Write([]byte(`{"error":"unauthorized: invalid or missing API token"}`))
			return
		}

		next.ServeHTTP(w, r)
	})
}

func extractToken(r *http.Request) string {
	// 1. Authorization: Bearer <token>
	authHeader := r.Header.Get("Authorization")
	if strings.HasPrefix(strings.ToLower(authHeader), "bearer ") {
		return strings.TrimSpace(authHeader[7:])
	}

	// 2. X-API-Token header
	if custom := r.Header.Get("X-API-Token"); custom != "" {
		return strings.TrimSpace(custom)
	}

	// 3. Query parameter for WebSockets or EventSource
	if q := r.URL.Query().Get("token"); q != "" {
		return strings.TrimSpace(q)
	}

	return ""
}
