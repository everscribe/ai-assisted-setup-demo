package conformance

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"
)

// The two rules below were silent in the contract until this task: all
// three apps agreed, and nothing written down said what they agreed on,
// so a fourth port could differ and be within its rights. openapi.yaml
// now states both.
//
// The no-cascade rule is fully pinned here. The duplicate-email rule is
// only half pinned here, for the reason its own comment gives: this layer
// can see that the apps agree, not that they agree for the stated reason.
// The stores carry the other half.

// TestRemovingAMemberLeavesTheirHistory pins the no-cascade rule on
// DELETE /api/members/{id}: the user row goes, the rows that point at it
// stay, and requested_by/author_id keep naming an id that no longer
// resolves.
//
// This can't be a lifecycle step: every seeded user is load-bearing
// elsewhere (u_linus, u_alan, u_grace), so removing one would break other
// tests and the seed comparison. The probe mints its own member instead,
// which also keeps the check re-runnable against an already-exercised app.
// Rows are read before the removal as well as after, since "still there"
// asserted only afterwards would pass against an app where they never were.
func TestRemovingAMemberLeavesTheirHistory(t *testing.T) {
	for _, app := range apps(t) {
		app := app
		t.Run(app.Name, func(t *testing.T) {
			c := NewClient(app)

			member := inviteMember(t, c, "Cascade Probe", "cascade-probe@acme.test")

			// Minted against r_acme_stripe, never r_acme_pg, for the
			// reason mintStripeRequest documents. Posted as the member
			// rather than through that helper, because the point is that
			// these rows belong to the user about to be removed.
			reqResp := validated(t, c, http.MethodPost,
				"/api/resources/r_acme_stripe/requests", member, `{"reason":"cascade probe"}`)
			require.Equal(t, http.StatusCreated, reqResp.Status)
			request := requestID(t, reqResp.Body)

			commentResp := validated(t, c, http.MethodPost,
				"/api/requests/"+request+"/comments", member, `{"body":"cascade probe comment"}`)
			require.Equal(t, http.StatusCreated, commentResp.Status)
			comment := requestID(t, commentResp.Body)

			require.NotNil(t, rowByID(t, c, "/api/resources/r_acme_stripe/requests", request),
				"the request must be served before the removal, or surviving it proves nothing")
			require.NotNil(t, rowByID(t, c, "/api/requests/"+request+"/comments", comment),
				"the comment must be served before the removal, or surviving it proves nothing")

			removed := validated(t, c, http.MethodDelete, "/api/members/"+member, "u_ada", "")
			require.Equal(t, http.StatusNoContent, removed.Status)

			// The removal has to have taken effect, or nothing below says
			// anything about cascading.
			gone := validated(t, c, http.MethodGet, "/api/me", member, "")
			require.Equal(t, http.StatusUnauthorized, gone.Status,
				"the removed member must stop authenticating")

			survivingRequest := rowByID(t, c, "/api/resources/r_acme_stripe/requests", request)
			require.NotNil(t, survivingRequest, "the request must survive its requester's removal")
			require.Equal(t, member, survivingRequest["requested_by"],
				"requested_by must still name the removed member")

			survivingComment := rowByID(t, c, "/api/requests/"+request+"/comments", comment)
			require.NotNil(t, survivingComment, "the comment must survive its author's removal")
			require.Equal(t, member, survivingComment["author_id"],
				"author_id must still name the removed member")
		})
	}
}

// TestAppsAgreeWhichUserADuplicateEmailResolvesTo pins that every app
// answers POST /api/login with the same user (u_ada, in the seeded
// fixture) as duplicate emails accumulate.
//
// It cannot pin the created_at-then-id ordering openapi.yaml requires:
// over HTTP a duplicate is always minted after the user it duplicates, so
// the seeded row is earliest by both fields and a store with no ordering
// passes too (confirmed: deleting the sort from apps/node's
// findUserByEmail stays green here). That rule is pinned instead per
// store, by planting a duplicate older than the seeded user:
//   - apps/go/store/memory/memory_internal_test.go, TestDuplicateEmailPrefersTheOlderRow
//   - apps/node/src/memory.test.ts, "resolves a duplicate email to the earliest user"
//   - apps/rust/src/memory.rs, duplicate_email_resolves_to_the_earliest_user
//
// Still worth having: it also catches a port resolving duplicates differently.
func TestAppsAgreeWhichUserADuplicateEmailResolvesTo(t *testing.T) {
	for _, app := range apps(t) {
		app := app
		t.Run(app.Name, func(t *testing.T) {
			c := NewClient(app)

			// Baseline: the lookup answers at all, so a failure below is
			// about which user won rather than about logging in.
			before := validated(t, c, http.MethodPost, "/api/login", "", `{"email":"ada@acme.test"}`)
			require.Equal(t, http.StatusOK, before.Status)
			require.Equal(t, "u_ada", jsonField(t, before.Body, "id"))

			// Two duplicates, checked after each: one alone leaves "the
			// last match wins" indistinguishable from bad luck in a map
			// walk, and the count keeps rising because the lifecycle has
			// already invited duplicates of this email by now. Two also
			// keeps the check meaningful on a re-run against an app that
			// has already been exercised.
			for i, name := range []string{"Ada Impostor One", "Ada Impostor Two"} {
				dup := inviteMember(t, c, name, "ada@acme.test")
				require.NotEqual(t, "u_ada", dup, "duplicate %d must be a new user", i)

				resp := validated(t, c, http.MethodPost, "/api/login", "", `{"email":"ada@acme.test"}`)
				require.Equal(t, http.StatusOK, resp.Status)
				require.Equal(t, "u_ada", jsonField(t, resp.Body, "id"),
					"after duplicate %d (%s), every app must still resolve the email to u_ada", i, dup)
			}
		})
	}
}

// rowByID fetches a list as u_ada and returns the row with that id, or
// nil. Used for presence checks where the row's position does not
// matter and ordering_test.go already owns the order.
func rowByID(t *testing.T, c *Client, path, id string) map[string]any {
	t.Helper()
	for _, row := range listRows(t, c, path, "u_ada") {
		if row["id"] == id {
			return row
		}
	}
	return nil
}
