package conformance

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"
)

// Every route except the two anonymous ones must 401 without an actor.
func TestMissingActorIs401(t *testing.T) {
	for _, app := range apps(t) {
		app := app
		t.Run(app.Name, func(t *testing.T) {
			c := NewClient(app)
			for _, r := range []struct{ method, path string }{
				{http.MethodGet, "/api/me"},
				{http.MethodPost, "/api/logout"},
				{http.MethodGet, "/api/members"},
				{http.MethodPost, "/api/members"},
				{http.MethodGet, "/api/resources"},
				{http.MethodPost, "/api/resources"},
				{http.MethodGet, "/api/resources/r_acme_pg/requests"},
				{http.MethodGet, "/api/requests/req_acme_1/comments"},
			} {
				resp := validated(t, c, r.method, r.path, "", "")
				require.Equal(t, http.StatusUnauthorized, resp.Status, "%s %s", r.method, r.path)
			}
			// The two anonymous routes.
			require.Equal(t, http.StatusOK, validated(t, c, http.MethodGet, "/api/health", "", "").Status)
			require.Equal(t, http.StatusOK,
				validated(t, c, http.MethodPost, "/api/login", "", `{"email":"ada@acme.test"}`).Status)
		})
	}
}

// A cross-tenant read is 404, and each one is paired with a positive
// control. Without the control this test also passes when the route was
// never registered, because an unknown path returns the identical body.
func TestCrossTenantReadsAre404WithPositiveControls(t *testing.T) {
	for _, app := range apps(t) {
		app := app
		t.Run(app.Name, func(t *testing.T) {
			c := NewClient(app)
			cases := []struct {
				name, method, sameTenant, crossTenant string
			}{
				{"resource requests", http.MethodGet,
					"/api/resources/r_acme_pg/requests", "/api/resources/r_globex_k8s/requests"},
				{"request comments", http.MethodGet,
					"/api/requests/req_acme_1/comments", "/api/requests/req_globex_1/comments"},
			}
			for _, tc := range cases {
				t.Run(tc.name, func(t *testing.T) {
					ok := validated(t, c, tc.method, tc.sameTenant, "u_ada", "")
					require.Equal(t, http.StatusOK, ok.Status,
						"positive control: the route must work same-tenant before its 404 means anything")

					miss := validated(t, c, tc.method, tc.crossTenant, "u_ada", "")
					require.Equal(t, http.StatusNotFound, miss.Status)
					require.JSONEq(t, `{"error":"not found"}`, string(miss.Body))
				})
			}
		})
	}
}

// A cross-tenant miss and an unknown path must be byte-identical, so the
// boundary does not leak existence.
func TestCrossTenantAndUnknownPathAreIndistinguishable(t *testing.T) {
	for _, app := range apps(t) {
		app := app
		t.Run(app.Name, func(t *testing.T) {
			c := NewClient(app)
			cross := validated(t, c, http.MethodGet, "/api/resources/r_globex_k8s/requests", "u_ada", "")
			unknown, err := c.Do(t.Context(), http.MethodGet, "/api/no-such-route", "u_ada", "")
			require.NoError(t, err)
			require.Equal(t, cross.Status, unknown.Status)
			require.Equal(t, string(cross.Body), string(unknown.Body))
		})
	}
}

func TestMemberCannotUseAdminRoutes(t *testing.T) {
	for _, app := range apps(t) {
		app := app
		t.Run(app.Name, func(t *testing.T) {
			c := NewClient(app)
			for _, r := range []struct {
				method, path, body string
			}{
				{http.MethodPost, "/api/resources", `{"name":"x","description":"y","sensitivity":"internal"}`},
				{http.MethodPost, "/api/members", `{"name":"X","email":"x@acme.test","role":"member"}`},
				{http.MethodPost, "/api/requests/req_acme_1/approve", ""},
				{http.MethodPost, "/api/requests/req_acme_1/deny", ""},
			} {
				resp := validated(t, c, r.method, r.path, "u_linus", r.body)
				require.Equal(t, http.StatusForbidden, resp.Status, "%s %s", r.method, r.path)
				require.JSONEq(t, `{"error":"admin role required"}`, string(resp.Body))
			}
		})
	}
}

// req_acme_3 is seeded denied. Denied is terminal.
func TestDeniedRequestCannotBeApproved(t *testing.T) {
	for _, app := range apps(t) {
		app := app
		t.Run(app.Name, func(t *testing.T) {
			c := NewClient(app)
			resp := validated(t, c, http.MethodPost, "/api/requests/req_acme_3/approve", "u_ada", "")
			require.Equal(t, http.StatusConflict, resp.Status)
		})
	}
}
