package conformance

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"
)

// The contract's x-ordering block declares a total order for every list
// endpoint: the domain key, then id ascending as tiebreak. These two
// fixtures each contain a pair tied on created_at, so they fail for any
// app that drops the tiebreak or serves insertion order.
//
// Go runs a package's test files in alphabetical order, so equality_test.go
// (added in Task 6, which replays this same lifecycle) runs before this
// file. The assertions below still hold against that run, but only for two
// non-obvious reasons, so a change to the lifecycle does not silently
// invalidate them and look like a port bug:
//
//  1. Seeded comments carry March 2026 timestamps while the lifecycle's
//     runtime comments use time.Now() (September 2026), so under the
//     declared ascending order the seeded three remain the prefix.
//  2. The lifecycle's new access request goes to r_acme_s3, not r_acme_pg,
//     and no test in this package mints requests against r_acme_pg either,
//     so the requests fixture on r_acme_pg stays untouched by any prior
//     test. Anything that starts minting there breaks this fixture on a
//     second run against the same app, since a fresh request sorts ahead
//     of the seeded March 2026 pair under the declared descending order.
func TestSeededListsMatchTheDeclaredOrder(t *testing.T) {
	for _, app := range apps(t) {
		app := app
		t.Run(app.Name, func(t *testing.T) {
			c := NewClient(app)
			cases := []struct {
				path string
				want []string
			}{
				{"/api/resources/r_acme_pg/requests", []string{"req_acme_1", "req_acme_4", "req_acme_5"}},
				{"/api/requests/req_acme_1/comments", []string{"c_acme_1", "c_acme_2", "c_acme_3"}},
			}
			for _, tc := range cases {
				resp := validated(t, c, http.MethodGet, tc.path, "u_ada", "")
				require.Equal(t, http.StatusOK, resp.Status)

				var rows []struct {
					ID string `json:"id"`
				}
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
			}
		})
	}
}
