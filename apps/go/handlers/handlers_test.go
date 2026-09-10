package handlers_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/everscribe/ai-assisted-setup-demo/apps/go/handlers"
	"github.com/everscribe/ai-assisted-setup-demo/apps/go/store/memory"
	"github.com/stretchr/testify/require"
)

func newRouter(t *testing.T) http.Handler {
	t.Helper()
	s := memory.New()
	require.NoError(t, s.Seed(t.Context()))
	return handlers.Router(s)
}

func do(t *testing.T, r http.Handler, method, path, actor, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	if actor != "" {
		req.Header.Set("X-User-ID", actor)
	}
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

func TestErrorsAreJSON(t *testing.T) {
	t.Parallel()
	r := newRouter(t)

	w := do(t, r, http.MethodGet, "/api/me", "", "")
	require.Equal(t, http.StatusUnauthorized, w.Code)
	require.Equal(t, "application/json; charset=utf-8", w.Header().Get("Content-Type"))

	var body map[string]string
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	require.Equal(t, "unauthorized", body["error"])
}

func TestUnknownAPIPathIsJSON404(t *testing.T) {
	t.Parallel()
	r := newRouter(t)

	w := do(t, r, http.MethodGet, "/api/nope", "u_ada", "")
	require.Equal(t, http.StatusNotFound, w.Code)
	require.JSONEq(t, `{"error":"not found"}`, w.Body.String())
}

func TestCrossTenantReadIs404WithAPositiveControl(t *testing.T) {
	t.Parallel()
	r := newRouter(t)

	// Positive control first. Without it this test also passes when the
	// route was never registered, since an unknown path returns the same
	// body by design.
	ok := do(t, r, http.MethodGet, "/api/resources/r_acme_pg/requests", "u_ada", "")
	require.Equal(t, http.StatusOK, ok.Code, "positive control: same-tenant read must work")

	// u_ada is in org_acme; r_globex_k8s is in org_globex.
	w := do(t, r, http.MethodGet, "/api/resources/r_globex_k8s/requests", "u_ada", "")
	require.Equal(t, http.StatusNotFound, w.Code)
	require.JSONEq(t, `{"error":"not found"}`, w.Body.String())
}

func TestMemberCannotWriteAdminRoute(t *testing.T) {
	t.Parallel()
	r := newRouter(t)

	// u_linus is a member of org_acme, not an admin.
	w := do(t, r, http.MethodPost, "/api/resources", "u_linus",
		`{"name":"x","description":"y","sensitivity":"internal"}`)
	require.Equal(t, http.StatusForbidden, w.Code)
	require.JSONEq(t, `{"error":"admin role required"}`, w.Body.String())
}

func TestAnonymousRoutes(t *testing.T) {
	t.Parallel()
	r := newRouter(t)

	require.Equal(t, http.StatusOK, do(t, r, http.MethodGet, "/api/health", "", "").Code)
	require.Equal(t, http.StatusOK,
		do(t, r, http.MethodPost, "/api/login", "", `{"email":"ada@acme.test"}`).Code)
}

func TestMethodMismatchIs405(t *testing.T) {
	t.Parallel()
	r := newRouter(t)

	// /api/login exists as POST; GET on the same path is a method
	// mismatch, not a missing route.
	w := do(t, r, http.MethodGet, "/api/login", "", "")
	require.Equal(t, http.StatusMethodNotAllowed, w.Code)
	require.JSONEq(t, `{"error":"method not allowed"}`, w.Body.String())
}

func TestTrailingSlashIs404NotRedirect(t *testing.T) {
	t.Parallel()
	r := newRouter(t)

	// A registered path with a trailing slash must not redirect - that
	// would leak which paths exist. It falls through to the same 404 as
	// an unknown path.
	w := do(t, r, http.MethodGet, "/api/me/", "u_ada", "")
	require.Equal(t, http.StatusNotFound, w.Code)
	require.JSONEq(t, `{"error":"not found"}`, w.Body.String())
}
