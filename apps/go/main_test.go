package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/everscribe/ai-assisted-setup-demo/apps/go/handlers"
	"github.com/everscribe/ai-assisted-setup-demo/apps/go/store/memory"
)

// A bare GET /api must reach the API's own JSON 404. Registering only
// "/api/" on the mux made it a 301 to /api/ with an HTML body, which
// leaks that the prefix exists and breaks the all-JSON rule.
func TestBareAPIPathIsJSON404NotRedirect(t *testing.T) {
	t.Parallel()

	s := memory.New()
	require.NoError(t, s.Seed(context.Background()))
	static := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	root := newRootMux(handlers.Router(s), static)

	for _, path := range []string{"/api", "/api/"} {
		w := httptest.NewRecorder()
		root.ServeHTTP(w, httptest.NewRequest(http.MethodGet, path, nil))

		require.Equal(t, http.StatusNotFound, w.Code, "GET %s", path)
		require.Equal(t, "application/json; charset=utf-8", w.Header().Get("Content-Type"))
		require.JSONEq(t, `{"error":"not found"}`, w.Body.String())
	}
}

// A path with an empty segment is the API's own JSON 404. Left to
// ServeMux it is a 301 to the cleaned path with an HTML body, the same
// existence oracle the trailing-slash rule closes, and the three apps
// answered it three different ways.
func TestEmptySegmentIsJSON404NotRedirect(t *testing.T) {
	t.Parallel()

	s := memory.New()
	require.NoError(t, s.Seed(context.Background()))
	static := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	root := newRootMux(handlers.Router(s), static)

	for _, path := range []string{"/api//me", "/api//resources", "/api/resources//"} {
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, path, nil)
		req.Header.Set("X-User-ID", "u_ada")
		root.ServeHTTP(w, req)

		require.Equal(t, http.StatusNotFound, w.Code, "GET %s", path)
		require.Empty(t, w.Header().Get("Location"), "GET %s must not redirect", path)
		require.Equal(t, "application/json; charset=utf-8", w.Header().Get("Content-Type"), "GET %s", path)
		require.JSONEq(t, `{"error":"not found"}`, w.Body.String(), "GET %s", path)
	}

	// Positive controls: the guard must not swallow the real path, its
	// trailing-slash sibling, or the SPA's root.
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/me", nil)
	req.Header.Set("X-User-ID", "u_ada")
	root.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code)

	w = httptest.NewRecorder()
	root.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/", nil))
	require.Equal(t, http.StatusOK, w.Code, "the SPA root is one slash, not an empty segment")
}

// A path with a "." or ".." segment is the API's own JSON 404. Left to
// ServeMux it is a 301 to the cleaned path with an HTML body, the same
// existence oracle the empty-segment rule closes one line above: Node,
// Rust and Python all answered the JSON 404 while Go redirected.
func TestDotSegmentIsJSON404NotRedirect(t *testing.T) {
	t.Parallel()

	s := memory.New()
	require.NoError(t, s.Seed(context.Background()))
	static := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	root := newRootMux(handlers.Router(s), static)

	// The last two are the percent-encoded spellings: a %2e must not
	// smuggle a dot segment past the guard into the mux's redirect.
	for _, path := range []string{
		"/api/./me", "/api/resources/../me", "/api/me/.", "/api/../api/me",
		"/api/%2e/me", "/api/resources/%2e%2e/me",
	} {
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, path, nil)
		req.Header.Set("X-User-ID", "u_ada")
		root.ServeHTTP(w, req)

		require.Equal(t, http.StatusNotFound, w.Code, "GET %s", path)
		require.Empty(t, w.Header().Get("Location"), "GET %s must not redirect", path)
		require.Equal(t, "application/json; charset=utf-8", w.Header().Get("Content-Type"), "GET %s", path)
		require.JSONEq(t, `{"error":"not found"}`, w.Body.String(), "GET %s", path)
	}

	// Positive controls: a dot inside a segment is not a dot segment, so
	// neither a real API path nor a hashed SPA asset may be swallowed.
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/me", nil)
	req.Header.Set("X-User-ID", "u_ada")
	root.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code)

	w = httptest.NewRecorder()
	root.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/assets/index-a1b2c3.js", nil))
	require.Equal(t, http.StatusOK, w.Code, "a dot inside a segment is not a dot segment")
}
