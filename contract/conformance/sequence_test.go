package conformance

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"
)

var pristineOnce sync.Map // app name -> *sync.Once

// requirePristineExplanation is appended to every failure below so a
// developer reading the output knows what to do next, not just that
// something disagrees.
const requirePristineExplanation = "\nIf this app has already run the conformance suite, its state is " +
	"no longer pristine and this check cannot pass: restart the app, " +
	"or use `make verify`, which recreates containers on every run. " +
	"If this is a fresh app, its seed genuinely disagrees with contract/seed.json."

// requirePristine asserts the two seeded rows the lifecycle mutates start
// in the state seed.json claims. The seed test cannot check them, because
// it exempts exactly these fields so the suite stays re-runnable. Running
// once per app, before any replay, is what closes that hole.
//
// Trade-off, on purpose: this guard needs a pristine app, and the
// lifecycle it guards mutates these two rows on every run. Both cannot
// hold across repeated runs against one unrestarted app, so a second
// `go test` against a `make up` you have already exercised will now fail
// here. That is expected, not a bug to fix by deleting the guard: `make
// verify` always starts fresh, so CI and the real acceptance path never
// hit this. Only the manual "make up, then go test twice" loop does, and
// the failure message says why.
func requirePristine(t *testing.T, c *Client) {
	t.Helper()
	v, _ := pristineOnce.LoadOrStore(c.App.Name, &sync.Once{})
	v.(*sync.Once).Do(func() {
		reqs := validated(t, c, http.MethodGet, "/api/resources/r_acme_pg/requests", "u_ada", "")
		var rows []map[string]any
		require.NoError(t, json.Unmarshal(reqs.Body, &rows))
		found := false
		for _, r := range rows {
			if r["id"] == "req_acme_1" {
				found = true
				require.Equal(t, "pending", r["status"],
					"req_acme_1 must start pending."+requirePristineExplanation)
				require.Nil(t, r["decided_by"],
					"req_acme_1 must start undecided."+requirePristineExplanation)
				require.Nil(t, r["revoked_by"],
					"req_acme_1 must start unrevoked."+requirePristineExplanation)
			}
		}
		require.True(t, found, "req_acme_1 missing from r_acme_pg's requests")

		res := validated(t, c, http.MethodGet, "/api/resources", "u_ada", "")
		var resources []map[string]any
		require.NoError(t, json.Unmarshal(res.Body, &resources))
		for _, r := range resources {
			if r["id"] == "r_acme_s3" {
				require.Nil(t, r["decommission_at"],
					"r_acme_s3 must start live."+requirePristineExplanation)
			}
		}
	})
}

// Run replays the lifecycle against one app, schema-checking every
// response, and returns them in order.
func Run(t *testing.T, c *Client) []*Response {
	t.Helper()
	requirePristine(t, c)
	steps := Lifecycle()
	out := make([]*Response, 0, len(steps))
	for i, s := range steps {
		out = append(out, validated(t, c, s.Method, s.Path, s.Actor, s.Body,
			fmt.Sprintf("step %d (%s)", i, s.Name)))
	}
	return out
}

func TestLifecycleRunsCleanAgainstEveryApp(t *testing.T) {
	for _, app := range apps(t) {
		app := app
		t.Run(app.Name, func(t *testing.T) {
			responses := Run(t, NewClient(app))
			require.Len(t, responses, len(Lifecycle()))
			for i, r := range responses {
				require.NotEqual(t, 0, r.Status, "step %d (%s) produced no status", i, Lifecycle()[i].Name)
				require.Less(t, r.Status, 500, "step %d (%s) returned %d", i, Lifecycle()[i].Name, r.Status)
			}
		})
	}
}
