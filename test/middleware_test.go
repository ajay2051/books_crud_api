package test

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"golang_crud/auth"
	"golang_crud/utils"
)

// Cross-cutting behaviour: the auth middleware, routing errors and request IDs.

func TestBooks_RequireAuth(t *testing.T) {
	r := newTestRouter()
	routesToCheck := []struct{ method, path, body string }{
		{http.MethodPost, "/api/v1/books", `{"title":"A","author":"B"}`},
		{http.MethodGet, "/api/v1/books", ""},
		{http.MethodGet, "/api/v1/books/1", ""},
		{http.MethodPut, "/api/v1/books/1", `{"title":"A","author":"B"}`},
		{http.MethodDelete, "/api/v1/books/1", ""},
	}

	for _, rt := range routesToCheck {
		w := sendJSON(r, rt.method, rt.path, rt.body)
		expectError(t, w, http.StatusUnauthorized, utils.CodeUnauthorized)
		if w.Header().Get("WWW-Authenticate") != "Bearer" {
			t.Errorf("%s %s: missing WWW-Authenticate header", rt.method, rt.path)
		}
	}
}

func TestBooks_RejectBadTokens(t *testing.T) {
	r := newTestRouter()

	otherSecret := auth.NewTokenManager("another-secret-another-secret-another-1", testIssuer, time.Hour)
	otherIssuer := auth.NewTokenManager(testSecret, "someone-else", time.Hour)
	expired := newTokens(-time.Hour)

	wrongSecretToken, _, _ := otherSecret.Issue(1, "x@example.com")
	wrongIssuerToken, _, _ := otherIssuer.Issue(1, "x@example.com")
	expiredToken, _, _ := expired.Issue(1, "x@example.com")

	cases := []struct {
		name, header, code string
	}{
		{"basic scheme", "Basic abc", utils.CodeUnauthorized},
		{"no token after Bearer", "Bearer", utils.CodeUnauthorized},
		{"garbage token", "Bearer not.a.jwt", utils.CodeUnauthorized},
		{"wrong signing secret", "Bearer " + wrongSecretToken, utils.CodeUnauthorized},
		{"wrong issuer", "Bearer " + wrongIssuerToken, utils.CodeUnauthorized},
		{"expired token", "Bearer " + expiredToken, utils.CodeTokenExpired},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			w := send(r, http.MethodGet, "/api/v1/books", "", "application/json", tc.header)
			expectError(t, w, http.StatusUnauthorized, tc.code)
		})
	}
}

func TestHealth_IsPublic(t *testing.T) {
	if w := sendJSON(newTestRouter(), http.MethodGet, "/health", ""); w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d", w.Code)
	}
}

func TestUnknownRoute(t *testing.T) {
	expectError(t, sendJSON(newTestRouter(), http.MethodGet, "/nope", ""),
		http.StatusNotFound, utils.CodeNotFound)
}

func TestMethodNotAllowed(t *testing.T) {
	expectError(t, sendJSON(newTestRouter(), http.MethodPatch, "/api/v1/auth/login", ""),
		http.StatusMethodNotAllowed, utils.CodeMethodNotAllowed)
}

func TestRequestID_GeneratedAndEchoed(t *testing.T) {
	r := newTestRouter()

	w := sendAuthed(t, r, http.MethodGet, "/api/v1/books/999", "")
	id := w.Header().Get("X-Request-ID")
	if id == "" {
		t.Fatal("missing X-Request-ID header")
	}
	if decode(t, w).RequestID != id {
		t.Fatal("request_id in body must match the X-Request-ID header")
	}

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	req.Header.Set("X-Request-ID", "trace-123")
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Header().Get("X-Request-ID") != "trace-123" {
		t.Fatalf("a valid incoming request ID should be reused, got %q", w.Header().Get("X-Request-ID"))
	}
}
