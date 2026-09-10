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

func TestHeadAndOptionsAre405(t *testing.T) {
	t.Parallel()
	r := newRouter(t)

	// Neither method is registered anywhere. Starlette and Express both
	// route HEAD to the GET handler by default, so the contract pins
	// this rather than leaving it to the framework.
	for _, method := range []string{http.MethodHead, http.MethodOptions} {
		w := do(t, r, method, "/api/health", "", "")
		require.Equal(t, http.StatusMethodNotAllowed, w.Code, "%s /api/health", method)
		require.Equal(t, "GET", w.Header().Get("Allow"))
	}
}

func TestCheckPrecedence(t *testing.T) {
	t.Parallel()
	r := newRouter(t)

	badBody := `{"name":123,"reason":123}`

	// 401 wins over the admin check and the body check.
	w := do(t, r, http.MethodPost, "/api/resources", "", badBody)
	require.Equal(t, http.StatusUnauthorized, w.Code)

	// 403 wins over the body check. A port that binds a body model
	// before the role check answers 400 or 422 here instead.
	w = do(t, r, http.MethodPost, "/api/resources", "u_linus", badBody)
	require.Equal(t, http.StatusForbidden, w.Code)

	// 400 wins over the store's 404, on a route with no admin check.
	w = do(t, r, http.MethodPost, "/api/resources/r_nope/requests", "u_linus", badBody)
	require.Equal(t, http.StatusBadRequest, w.Code)

	// 403 wins over the store's 404 on an admin-only route.
	w = do(t, r, http.MethodPost, "/api/requests/req_nope/approve", "u_linus", "")
	require.Equal(t, http.StatusForbidden, w.Code)
}

func TestRequiredFieldsAreTrimmed(t *testing.T) {
	t.Parallel()
	r := newRouter(t)

	cases := []struct {
		name, method, path, actor, body, wantErr string
	}{
		{"resource name", http.MethodPost, "/api/resources", "u_ada", `{"name":"  "}`, "name required"},
		{"member name", http.MethodPost, "/api/members", "u_ada", `{"name":" ","email":"x@y.test"}`, "name and email required"},
		{"member email", http.MethodPost, "/api/members", "u_ada", `{"name":"n","email":"  "}`, "name and email required"},
		{"request reason", http.MethodPost, "/api/resources/r_acme_pg/requests", "u_linus", `{"reason":"  "}`, "reason required"},
		{"comment body", http.MethodPost, "/api/requests/req_acme_1/comments", "u_linus", `{"body":"  "}`, "body required"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			w := do(t, r, tc.method, tc.path, tc.actor, tc.body)
			require.Equal(t, http.StatusBadRequest, w.Code)
			require.JSONEq(t, `{"error":"`+tc.wantErr+`"}`, w.Body.String())
		})
	}

	// Login's email is matched exactly, so padding is a 401, not a login.
	w := do(t, r, http.MethodPost, "/api/login", "", `{"email":" ada@acme.test "}`)
	require.Equal(t, http.StatusUnauthorized, w.Code)

	// The role on a role change is compared exactly too.
	w = do(t, r, http.MethodPut, "/api/members/u_linus/role", "u_ada", `{"role":" admin "}`)
	require.Equal(t, http.StatusBadRequest, w.Code)
}
