package server

import (
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"testing/fstest"
)

const testToken = "secret-token"

func newAuthTestServer(t *testing.T) (*Server, http.Handler) {
	t.Helper()
	tempDir, store, _ := setupTestRepo(t)
	t.Cleanup(func() { os.RemoveAll(tempDir) })

	srv := NewServer(store, fstest.MapFS{
		"index.html": &fstest.MapFile{Data: []byte("<html><body>Branch Office</body></html>")},
	})
	return srv, srv.Routes()
}

func doRequest(handler http.Handler, req *http.Request) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	return rec
}

func TestTokenAuthRequired(t *testing.T) {
	srv, handler := newAuthTestServer(t)
	srv.SetAuthToken(testToken)

	// No token -> 401
	rec := doRequest(handler, httptest.NewRequest("GET", "/api/repos", nil))
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 without token, got %d", rec.Code)
	}

	// Wrong token -> 401
	req := httptest.NewRequest("GET", "/api/repos", nil)
	req.Header.Set(tokenHeader, "wrong-token")
	rec = doRequest(handler, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 with wrong token, got %d", rec.Code)
	}

	// Token header -> 200
	req = httptest.NewRequest("GET", "/api/repos", nil)
	req.Header.Set(tokenHeader, testToken)
	rec = doRequest(handler, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 with token header, got %d", rec.Code)
	}

	// Token query param (EventSource path) -> 200
	rec = doRequest(handler, httptest.NewRequest("GET", "/api/repos?token="+testToken, nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 with token query param, got %d", rec.Code)
	}

	// Bearer token -> 200
	req = httptest.NewRequest("GET", "/api/repos", nil)
	req.Header.Set("Authorization", "Bearer "+testToken)
	rec = doRequest(handler, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 with bearer token, got %d", rec.Code)
	}

	// Static SPA shell stays public and hardened
	rec = doRequest(handler, httptest.NewRequest("GET", "/", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 for static index without token, got %d", rec.Code)
	}
	if rec.Header().Get("X-Frame-Options") != "DENY" {
		t.Fatalf("expected X-Frame-Options DENY, got %q", rec.Header().Get("X-Frame-Options"))
	}
	if rec.Header().Get("Content-Security-Policy") != "frame-ancestors 'none'" {
		t.Fatalf("expected CSP frame-ancestors, got %q", rec.Header().Get("Content-Security-Policy"))
	}

	// 401 responses must not leak CORS headers
	req = httptest.NewRequest("GET", "/api/repos", nil)
	rec = doRequest(handler, req)
	if rec.Header().Get("Access-Control-Allow-Origin") != "" {
		t.Fatal("expected no Access-Control-Allow-Origin header")
	}
}

func TestAuthDisabled(t *testing.T) {
	_, handler := newAuthTestServer(t)

	// No token configured -> API is open
	rec := doRequest(handler, httptest.NewRequest("GET", "/api/repos", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 without token when auth disabled, got %d", rec.Code)
	}
}

func TestContentTypeEnforcement(t *testing.T) {
	_, handler := newAuthTestServer(t)

	// Non-JSON Content-Type on a body-bearing method -> 415
	req := httptest.NewRequest("POST", "/api/repos", strings.NewReader(`{"path":"/no/such/dir"}`))
	req.Header.Set("Content-Type", "text/plain")
	rec := doRequest(handler, req)
	if rec.Code != http.StatusUnsupportedMediaType {
		t.Fatalf("expected 415 for text/plain body, got %d", rec.Code)
	}

	// JSON Content-Type passes the middleware and reaches the handler
	// (which rejects the nonexistent path with 400)
	req = httptest.NewRequest("POST", "/api/repos", strings.NewReader(`{"path":"/no/such/dir"}`))
	req.Header.Set("Content-Type", "application/json")
	rec = doRequest(handler, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for JSON body with bad path, got %d", rec.Code)
	}

	// Missing Content-Type passes (curl without -H still works)
	req = httptest.NewRequest("POST", "/api/repos", strings.NewReader(`{"path":"/no/such/dir"}`))
	rec = doRequest(handler, req)
	if rec.Code == http.StatusUnsupportedMediaType {
		t.Fatal("unexpected 415 for missing Content-Type")
	}

	// GET requests are exempt from the Content-Type check
	req = httptest.NewRequest("GET", "/api/repos", nil)
	req.Header.Set("Content-Type", "text/plain")
	rec = doRequest(handler, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 for GET regardless of Content-Type, got %d", rec.Code)
	}
}

func TestOriginCheckWithoutAuth(t *testing.T) {
	_, handler := newAuthTestServer(t)

	// Cross-origin mutation with auth disabled -> 403
	req := httptest.NewRequest("POST", "http://127.0.0.1:8080/api/repos", strings.NewReader(`{"path":"/no/such/dir"}`))
	req.Header.Set("Origin", "http://evil.example")
	req.Header.Set("Content-Type", "application/json")
	rec := doRequest(handler, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for cross-origin POST, got %d", rec.Code)
	}

	// Same-origin mutation passes the gate
	req = httptest.NewRequest("POST", "http://127.0.0.1:8080/api/repos", strings.NewReader(`{"path":"/no/such/dir"}`))
	req.Header.Set("Origin", "http://127.0.0.1:8080")
	req.Header.Set("Content-Type", "application/json")
	rec = doRequest(handler, req)
	if rec.Code == http.StatusForbidden {
		t.Fatal("unexpected 403 for same-origin POST")
	}
}