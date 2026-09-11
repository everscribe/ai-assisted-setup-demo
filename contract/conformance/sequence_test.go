package conformance

import (
	"encoding/json"
	"net/http"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"
)

var pristineOnce sync.Map // app name -> *sync.Once

// requirePristine asserts the two seeded rows the lifecycle mutates start
// in the state seed.json claims. The seed test cannot check them, because
// it exempts exactly these fields so the suite stays re-runnable. Running
// once per app, before any replay, is what closes that hole.
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
					"req_acme_1 must start pending; the seed test exempts this field")
				require.Nil(t, r["decided_by"], "req_acme_1 must start undecided")
				require.Nil(t, r["revoked_by"], "req_acme_1 must start unrevoked")
			}
		}
		require.True(t, found, "req_acme_1 missing from r_acme_pg's requests")

		res := validated(t, c, http.MethodGet, "/api/resources", "u_ada", "")
		var resources []map[string]any
		require.NoError(t, json.Unmarshal(res.Body, &resources))
		for _, r := range resources {
			if r["id"] == "r_acme_s3" {
				require.Nil(t, r["decommission_at"],
					"r_acme_s3 must start live; the seed test exempts this field")
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
	for _, s := range steps {
		resp := validated(t, c, s.Method, s.Path, s.Actor, s.Body)
		out = append(out, resp)
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
