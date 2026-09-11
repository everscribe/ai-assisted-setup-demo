package conformance

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"
)

// Every authenticated route must 401 without an actor. All sixteen are
// listed, not a sample, so a route that drops requireActor cannot hide
// behind the ones that still have it.
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
				{http.MethodDelete, "/api/members/u_linus"},
				{http.MethodPut, "/api/members/u_linus/role"},
				{http.MethodGet, "/api/resources"},
				{http.MethodPost, "/api/resources"},
				{http.MethodPost, "/api/resources/r_acme_pg/decommission"},
				{http.MethodGet, "/api/resources/r_acme_pg/requests"},
				{http.MethodPost, "/api/resources/r_acme_pg/requests"},
				{http.MethodPost, "/api/requests/req_acme_1/approve"},
				{http.MethodPost, "/api/requests/req_acme_1/deny"},
				{http.MethodPost, "/api/requests/req_acme_1/revoke"},
				{http.MethodGet, "/api/requests/req_acme_1/comments"},
				{http.MethodPost, "/api/requests/req_acme_1/comments"},
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

// requestID pulls the "id" field out of a JSON body, for tests that mint
// their own fixtures instead of relying on seeded ones.
func requestID(t *testing.T, body []byte) string {
	t.Helper()
	var v struct {
		ID string `json:"id"`
	}
	require.NoError(t, json.Unmarshal(body, &v))
	require.NotEmpty(t, v.ID)
	return v.ID
}

// Each admin route is paired with a positive control: an admin actor
// must succeed on the same route first. Without that, refusing u_linus
// only proves he is refused, not that the refusal is about role - a gate
// keyed on his user ID instead of his role would pass this test too.
// The refusal is also checked against a second, freshly-invited member
// who is not u_linus, so a gate that denies "u_linus" by name rather
// than by role has nowhere to hide either.
func TestMemberCannotUseAdminRoutes(t *testing.T) {
	for _, app := range apps(t) {
		app := app
		t.Run(app.Name, func(t *testing.T) {
			c := NewClient(app)

			created := validated(t, c, http.MethodPost, "/api/resources",
				"u_ada", `{"name":"positive-control-resource","description":"d","sensitivity":"internal"}`)
			require.Equal(t, http.StatusCreated, created.Status, "admin must be able to register a resource")

			invited := validated(t, c, http.MethodPost, "/api/members",
				"u_ada", `{"name":"Positive Control","email":"positive-control@acme.test","role":"member"}`)
			require.Equal(t, http.StatusCreated, invited.Status, "admin must be able to invite a member")
			secondMember := requestID(t, invited.Body)

			// Fresh pending requests, minted here rather than reused, so
			// approving/denying them for the positive controls below
			// cannot collide with a lifecycle test elsewhere that also
			// mutates req_acme_1. Minted against r_acme_stripe, not
			// r_acme_pg: ordering_test.go pins the served order of
			// r_acme_pg's requests, and a minted request here would
			// pollute that fixture on a second run against the same app.
			toApprove := validated(t, c, http.MethodPost, "/api/resources/r_acme_stripe/requests",
				"u_linus", `{"reason":"positive control for approve"}`)
			require.Equal(t, http.StatusCreated, toApprove.Status)
			toDeny := validated(t, c, http.MethodPost, "/api/resources/r_acme_stripe/requests",
				"u_linus", `{"reason":"positive control for deny"}`)
			require.Equal(t, http.StatusCreated, toDeny.Status)

			approved := validated(t, c, http.MethodPost,
				"/api/requests/"+requestID(t, toApprove.Body)+"/approve", "u_ada", "")
			require.Equal(t, http.StatusOK, approved.Status, "admin must be able to approve a pending request")

			denied := validated(t, c, http.MethodPost,
				"/api/requests/"+requestID(t, toDeny.Body)+"/deny", "u_ada", "")
			require.Equal(t, http.StatusOK, denied.Status, "admin must be able to deny a pending request")

			for _, actor := range []string{"u_linus", secondMember} {
				for _, r := range []struct {
					method, path, body string
				}{
					{http.MethodPost, "/api/resources", `{"name":"x","description":"y","sensitivity":"internal"}`},
					{http.MethodPost, "/api/members", `{"name":"X","email":"x@acme.test","role":"member"}`},
					{http.MethodPost, "/api/requests/req_acme_1/approve", ""},
					{http.MethodPost, "/api/requests/req_acme_1/deny", ""},
				} {
					resp := validated(t, c, r.method, r.path, actor, r.body)
					require.Equal(t, http.StatusForbidden, resp.Status, "%s: %s %s", actor, r.method, r.path)
					require.JSONEq(t, `{"error":"admin role required"}`, string(resp.Body))
				}
			}
		})
	}
}

// req_acme_3 is seeded denied. Denied is terminal, which only means
// something next to a positive control: approving a fresh pending
// request must still succeed, or the 409 below could just as well mean
// approve is broken outright. The fresh request is minted here rather
// than reusing req_acme_1, which a later task's lifecycle sequence also
// mutates and would not reliably be pending by the time this test runs.
// Minted against r_acme_stripe: see the note in TestMemberCannotUseAdminRoutes
// on why r_acme_pg is off limits for minting.
func TestDeniedRequestCannotBeApproved(t *testing.T) {
	for _, app := range apps(t) {
		app := app
		t.Run(app.Name, func(t *testing.T) {
			c := NewClient(app)

			pending := validated(t, c, http.MethodPost, "/api/resources/r_acme_stripe/requests",
				"u_linus", `{"reason":"positive control for denied-is-terminal"}`)
			require.Equal(t, http.StatusCreated, pending.Status)

			approved := validated(t, c, http.MethodPost,
				"/api/requests/"+requestID(t, pending.Body)+"/approve", "u_ada", "")
			require.Equal(t, http.StatusOK, approved.Status,
				"positive control: approving a pending request must work before a 409 on a denied one means anything")

			resp := validated(t, c, http.MethodPost, "/api/requests/req_acme_3/approve", "u_ada", "")
			require.Equal(t, http.StatusConflict, resp.Status)
		})
	}
}
