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
