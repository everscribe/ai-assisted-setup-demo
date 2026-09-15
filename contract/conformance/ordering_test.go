package conformance

import (
	"encoding/json"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// orderedRow is enough of any list row to check the declared order.
type orderedRow struct {
	ID        string `json:"id"`
	CreatedAt string `json:"created_at"`
}

// assertDeclaredOrder walks every adjacent pair of the full served list,
// not a fixed prefix, checking x-ordering's rule: the domain key
// (created_at, ascending or descending per endpoint) with id ascending
// as the tiebreak. This is what catches a bug confined to the
// runtime-appended tail, which a fixed-prefix check never inspects.
// created_at is parsed as an instant, never compared as a string, per
// x-ordering in openapi.yaml: a runtime row's fractional seconds must
// still sort correctly against a seed row's whole-second precision.
func assertDeclaredOrder(t *testing.T, path string, rows []orderedRow, descending bool) {
	t.Helper()
	for i := 1; i < len(rows); i++ {
		prev, cur := rows[i-1], rows[i]
		pt, err := time.Parse(time.RFC3339Nano, prev.CreatedAt)
		require.NoError(t, err, "%s: row %d created_at %q", path, i-1, prev.CreatedAt)
		ct, err := time.Parse(time.RFC3339Nano, cur.CreatedAt)
		require.NoError(t, err, "%s: row %d created_at %q", path, i, cur.CreatedAt)

		switch {
		case pt.Equal(ct):
			require.True(t, prev.ID <= cur.ID,
				"%s: rows %d (%s) and %d (%s) tie on created_at, id must be ascending",
				path, i-1, prev.ID, i, cur.ID)
		case descending:
			require.True(t, pt.After(ct),
				"%s: row %d (%s, %s) must come after row %d (%s, %s) under descending created_at",
				path, i-1, prev.ID, prev.CreatedAt, i, cur.ID, cur.CreatedAt)
		default:
			require.True(t, pt.Before(ct),
				"%s: row %d (%s, %s) must come before row %d (%s, %s) under ascending created_at",
				path, i-1, prev.ID, prev.CreatedAt, i, cur.ID, cur.CreatedAt)
		}
	}
}

// The contract's x-ordering block declares a total order for every list
// endpoint: domain key, then id ascending as tiebreak. These fixtures each
// hold a pair tied on created_at, so they fail for any app that drops the
// tiebreak or serves insertion order.
//
// Go runs test files alphabetically, so equality_test.go (which replays
// this lifecycle) runs first. These assertions still hold only because:
//  1. seeded comments carry March 2026 timestamps vs. the lifecycle's
//     time.Now(), keeping the seeded three as the ascending prefix; and
//  2. the lifecycle's new request goes to r_acme_s3, and nothing else here
//     mints against r_acme_pg, so a fresh request there would sort ahead
//     of the seeded pair under descending order and break this fixture.
func TestSeededListsMatchTheDeclaredOrder(t *testing.T) {
	for _, app := range apps(t) {
		app := app
		t.Run(app.Name, func(t *testing.T) {
			c := NewClient(app)
			cases := []struct {
				path string
				want []string
				desc bool
			}{
				{"/api/resources/r_acme_pg/requests", []string{"req_acme_1", "req_acme_4", "req_acme_5"}, true},
				{"/api/requests/req_acme_1/comments", []string{"c_acme_1", "c_acme_2", "c_acme_3"}, false},
				{"/api/members", []string{"u_ada", "u_linus"}, false},
				{"/api/resources", []string{"r_acme_pg", "r_acme_s3", "r_acme_stripe"}, false},
			}
			for _, tc := range cases {
				resp := validated(t, c, http.MethodGet, tc.path, "u_ada", "")
				require.Equal(t, http.StatusOK, resp.Status)

				var rows []orderedRow
				require.NoError(t, json.Unmarshal(resp.Body, &rows))

				var got []string
				for _, r := range rows {
					got = append(got, r.ID)
				}
				// Guard the slice: an app serving fewer rows than the
				// fixture is an app defect, not a harness panic.
				require.GreaterOrEqual(t, len(got), len(tc.want),
					"%s: served %d rows, fixture needs %d", tc.path, len(got), len(tc.want))
				require.Equal(t, tc.want, got[:len(tc.want)], "%s: declared order violated", tc.path)

				// The prefix check above only ever looks at the seeded
				// rows. This walks the full list, including whatever
				// runtime tail exists, so a bug confined to generated
				// rows cannot hide behind a fixed prefix.
				assertDeclaredOrder(t, tc.path, rows, tc.desc)
			}

			// r_acme_pg deliberately never receives a minted request
			// (see the file header), so its property check above never
			// sees a runtime row. Mint a few here against r_acme_stripe
			// instead, which is never used as a prefix fixture, so this
			// endpoint's runtime tail is exercised on every run rather
			// than only when this file happens to execute after
			// equality_test.go's lifecycle has minted rows elsewhere.
			for i := 0; i < 6; i++ {
				validated(t, c, http.MethodPost, "/api/resources/r_acme_stripe/requests", "u_linus",
					fmt.Sprintf(`{"reason":"ordering probe %d"}`, i))
			}
			stripeResp := validated(t, c, http.MethodGet, "/api/resources/r_acme_stripe/requests", "u_ada", "")
			require.Equal(t, http.StatusOK, stripeResp.Status)
			var stripeRows []orderedRow
			require.NoError(t, json.Unmarshal(stripeResp.Body, &stripeRows))
			assertDeclaredOrder(t, "/api/resources/r_acme_stripe/requests", stripeRows, true)
		})
	}
}
