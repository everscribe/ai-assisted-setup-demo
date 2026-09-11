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

// A 404 on a cross-tenant read only proves the reader is blocked. The
// write routes are where a missing tenant filter does damage, so an
// acme admin reaching into globex is asserted here too, each case
// paired with a same-tenant positive control on the same route: a
// cross-tenant miss and an unregistered route are byte-identical by
// design, so without the control the assertion passes on a route that
// does not exist at all.
//
// Both controls are minted here. Decommissioning or approving a seeded
// acme row would disturb fixtures other tests pin.
func TestCrossTenantWritesAre404WithPositiveControls(t *testing.T) {
	for _, app := range apps(t) {
		app := app
		t.Run(app.Name, func(t *testing.T) {
			c := NewClient(app)

			live := registerResource(t, c, "cross-tenant-control-resource")
			decommissioned := validated(t, c, http.MethodPost, "/api/resources/"+live+"/decommission", "u_ada", "")
			require.Equal(t, http.StatusOK, decommissioned.Status,
				"positive control: an admin must be able to decommission an own-tenant resource")

			pending := mintStripeRequest(t, c, "cross-tenant control for approve")
			approved := validated(t, c, http.MethodPost, "/api/requests/"+pending+"/approve", "u_ada", "")
			require.Equal(t, http.StatusOK, approved.Status,
				"positive control: an admin must be able to approve an own-tenant request")

			for _, path := range []string{
				"/api/resources/r_globex_k8s/decommission",
				"/api/requests/req_globex_1/approve",
			} {
				miss := validated(t, c, http.MethodPost, path, "u_ada", "")
				require.Equal(t, http.StatusNotFound, miss.Status, path)
				require.JSONEq(t, `{"error":"not found"}`, string(miss.Body))
			}
		})
	}
}

// Every row a list route serves must carry the actor's organization_id.
// All four list routes are asserted, for an acme actor and a globex
// actor, so a filter hardcoded to one organization fails here as
// loudly as no filter at all.
func TestListRoutesServeOnlyTheActorsOrganization(t *testing.T) {
	for _, app := range apps(t) {
		app := app
		t.Run(app.Name, func(t *testing.T) {
			c := NewClient(app)
			for _, tenant := range []struct {
				actor, org, resource, request string
			}{
				{"u_ada", "org_acme", "r_acme_pg", "req_acme_1"},
				{"u_grace", "org_globex", "r_globex_k8s", "req_globex_2"},
			} {
				for _, path := range []string{
					"/api/members",
					"/api/resources",
					"/api/resources/" + tenant.resource + "/requests",
					"/api/requests/" + tenant.request + "/comments",
				} {
					resp := validated(t, c, http.MethodGet, path, tenant.actor, "")
					require.Equal(t, http.StatusOK, resp.Status, "%s as %s", path, tenant.actor)

					rows := jsonRows(t, resp.Body)
					require.NotEmpty(t, rows,
						"%s as %s served nothing, which makes the scoping assertion vacuous", path, tenant.actor)
					for _, row := range rows {
						require.Equal(t, tenant.org, row["organization_id"],
							"%s as %s served a row from another organization: %v", path, tenant.actor, row)
					}
				}
			}
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

// jsonRows decodes a list response into generic rows, so a test can
// read fields the harness has no typed model for.
func jsonRows(t *testing.T, body []byte) []map[string]any {
	t.Helper()
	var rows []map[string]any
	require.NoError(t, json.Unmarshal(body, &rows))
	return rows
}

// registerResource mints an acme resource and returns its id.
func registerResource(t *testing.T, c *Client, name string) string {
	t.Helper()
	resp := validated(t, c, http.MethodPost, "/api/resources", "u_ada",
		`{"name":"`+name+`","description":"minted by the conformance harness","sensitivity":"internal"}`)
	require.Equal(t, http.StatusCreated, resp.Status, "minting resource %q", name)
	return requestID(t, resp.Body)
}

// inviteMember mints an acme member and returns its id.
func inviteMember(t *testing.T, c *Client, name, email string) string {
	t.Helper()
	resp := validated(t, c, http.MethodPost, "/api/members", "u_ada",
		`{"name":"`+name+`","email":"`+email+`","role":"member"}`)
	require.Equal(t, http.StatusCreated, resp.Status, "inviting %q", email)
	return requestID(t, resp.Body)
}

// mintStripeRequest mints a pending access request and returns its id.
// Always against r_acme_stripe, never r_acme_pg: ordering_test.go pins
// the served order of r_acme_pg's requests, and a minted request there
// would pollute that fixture on a second run against the same app.
func mintStripeRequest(t *testing.T, c *Client, reason string) string {
	t.Helper()
	resp := validated(t, c, http.MethodPost, "/api/resources/r_acme_stripe/requests", "u_linus",
		`{"reason":"`+reason+`"}`)
	require.Equal(t, http.StatusCreated, resp.Status, "minting request %q", reason)
	return requestID(t, resp.Body)
}

// All eight admin-gated routes are listed, not a sample, the same way
// TestMissingActorIs401 lists all sixteen authenticated routes. The four
// destructive ones - revoke, remove member, change role, decommission -
// are where a neutered gate does the most damage: a member who can
// change roles promotes himself to admin and the rest of the gate stops
// mattering.
//
// Each route is paired with a positive control: an admin must succeed on
// the same route first. Without that, refusing u_linus only proves he is
// refused, not that the refusal is about role - a gate keyed on his user
// id instead of his role would pass this test too. The refusal is also
// checked against a second, freshly-invited member who is not u_linus,
// so a gate that denies "u_linus" by name has nowhere to hide either.
//
// Every fixture the controls consume is minted here. No seeded member is
// removed or promoted, no seeded resource is decommissioned, and nothing
// is minted against r_acme_pg, whose request list ordering_test.go pins.
func TestMemberCannotUseAdminRoutes(t *testing.T) {
	for _, app := range apps(t) {
		app := app
		t.Run(app.Name, func(t *testing.T) {
			c := NewClient(app)

			// Positive controls: an admin doing each of the eight things
			// a member is refused below.
			registerResource(t, c, "positive-control-resource")
			secondMember := inviteMember(t, c, "Positive Control", "positive-control@acme.test")

			toApprove := mintStripeRequest(t, c, "positive control for approve")
			approved := validated(t, c, http.MethodPost, "/api/requests/"+toApprove+"/approve", "u_ada", "")
			require.Equal(t, http.StatusOK, approved.Status, "admin must be able to approve a pending request")

			toDeny := mintStripeRequest(t, c, "positive control for deny")
			denied := validated(t, c, http.MethodPost, "/api/requests/"+toDeny+"/deny", "u_ada", "")
			require.Equal(t, http.StatusOK, denied.Status, "admin must be able to deny a pending request")

			toRevoke := mintStripeRequest(t, c, "positive control for revoke")
			okApprove := validated(t, c, http.MethodPost, "/api/requests/"+toRevoke+"/approve", "u_ada", "")
			require.Equal(t, http.StatusOK, okApprove.Status)
			revoked := validated(t, c, http.MethodPost, "/api/requests/"+toRevoke+"/revoke", "u_ada", "")
			require.Equal(t, http.StatusOK, revoked.Status, "admin must be able to revoke an approved request")

			removable := inviteMember(t, c, "Removable Control", "removable-control@acme.test")
			removedResp := validated(t, c, http.MethodDelete, "/api/members/"+removable, "u_ada", "")
			require.Equal(t, http.StatusNoContent, removedResp.Status, "admin must be able to remove a member")

			promotable := inviteMember(t, c, "Promotable Control", "promotable-control@acme.test")
			promoted := validated(t, c, http.MethodPut, "/api/members/"+promotable+"/role", "u_ada", `{"role":"admin"}`)
			require.Equal(t, http.StatusOK, promoted.Status, "admin must be able to change a member's role")

			decommissionable := registerResource(t, c, "decommission-control-resource")
			gone := validated(t, c, http.MethodPost,
				"/api/resources/"+decommissionable+"/decommission", "u_ada", "")
			require.Equal(t, http.StatusOK, gone.Status, "admin must be able to decommission a resource")

			// Targets for the refusals. These must still exist when the
			// member tries, so a 403 cannot be confused with a 404.
			gateMember := inviteMember(t, c, "Gate Target", "gate-target@acme.test")
			gateResource := registerResource(t, c, "gate-target-resource")

			for _, actor := range []string{"u_linus", secondMember} {
				for _, r := range []struct {
					method, path, body string
				}{
					{http.MethodPost, "/api/resources", `{"name":"x","description":"y","sensitivity":"internal"}`},
					{http.MethodPost, "/api/members", `{"name":"X","email":"x@acme.test","role":"member"}`},
					{http.MethodPost, "/api/requests/req_acme_1/approve", ""},
					{http.MethodPost, "/api/requests/req_acme_1/deny", ""},
					{http.MethodPost, "/api/requests/req_acme_1/revoke", ""},
					{http.MethodDelete, "/api/members/" + gateMember, ""},
					{http.MethodPut, "/api/members/" + gateMember + "/role", `{"role":"admin"}`},
					{http.MethodPost, "/api/resources/" + gateResource + "/decommission", ""},
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
func TestDeniedRequestCannotBeApproved(t *testing.T) {
	for _, app := range apps(t) {
		app := app
		t.Run(app.Name, func(t *testing.T) {
			c := NewClient(app)

			pending := mintStripeRequest(t, c, "positive control for denied-is-terminal")
			approved := validated(t, c, http.MethodPost, "/api/requests/"+pending+"/approve", "u_ada", "")
			require.Equal(t, http.StatusOK, approved.Status,
				"positive control: approving a pending request must work before a 409 on a denied one means anything")

			resp := validated(t, c, http.MethodPost, "/api/requests/req_acme_3/approve", "u_ada", "")
			require.Equal(t, http.StatusConflict, resp.Status)
		})
	}
}
