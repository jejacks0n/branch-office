package server

import (
	"crypto/subtle"
	"net/http"
	"net/url"
	"strings"
)

const tokenHeader = "X-Broffice-Token"

// SetAuthToken enables token authentication for all API requests. An empty
// token disables it.
func (s *Server) SetAuthToken(token string) {
	s.authToken = token
}

// securityMiddleware enforces the API security policy. Static SPA assets stay
// public (the token is delivered to the browser via URL fragment), while
// every /api/ route requires the token when one is configured. Without token
// auth, state-changing requests must be same-origin, and request bodies must
// be JSON so browsers cannot smuggle "simple" cross-site requests past the
// origin check.
func (s *Server) securityMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		s.harden(w)

		if !strings.HasPrefix(r.URL.Path, "/api/") {
			next.ServeHTTP(w, r)
			return
		}

		if s.authToken != "" {
			provided := requestToken(r)
			if subtle.ConstantTimeCompare([]byte(provided), []byte(s.authToken)) != 1 {
				writeError(w, http.StatusUnauthorized, "unauthorized: missing or invalid token (open the access URL printed by broffice, or send "+tokenHeader+")")
				return
			}
		} else if !isReadMethod(r.Method) {
			// Token auth is the primary CSRF defense; this origin check only
			// guards the opt-in -no-auth mode.
			if origin := r.Header.Get("Origin"); origin != "" && !sameOrigin(r, origin) {
				writeError(w, http.StatusForbidden, "cross-origin API requests are not allowed")
				return
			}
		}

		if !isReadMethod(r.Method) {
			if ct := r.Header.Get("Content-Type"); ct != "" && !strings.HasPrefix(ct, "application/json") {
				writeError(w, http.StatusUnsupportedMediaType, "Content-Type must be application/json")
				return
			}
		}

		next.ServeHTTP(w, r)
	})
}

func isReadMethod(method string) bool {
	return method == http.MethodGet || method == http.MethodHead || method == http.MethodOptions
}

func requestToken(r *http.Request) string {
	if token := r.Header.Get(tokenHeader); token != "" {
		return token
	}
	// EventSource cannot set request headers, so SSE falls back to a query param.
	if token := r.URL.Query().Get("token"); token != "" {
		return token
	}
	if auth := r.Header.Get("Authorization"); strings.HasPrefix(auth, "Bearer ") {
		return strings.TrimPrefix(auth, "Bearer ")
	}
	return ""
}

func sameOrigin(r *http.Request, origin string) bool {
	u, err := url.Parse(origin)
	if err != nil || u.Host == "" {
		return false
	}
	return strings.EqualFold(u.Host, r.Host)
}

// harden sets baseline response headers: frame-ancestors/X-Frame-Options
// prevent clickjacking, nosniff prevents response MIME confusion.
func (s *Server) harden(w http.ResponseWriter) {
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("X-Frame-Options", "DENY")
	w.Header().Set("Content-Security-Policy", "frame-ancestors 'none'")
}