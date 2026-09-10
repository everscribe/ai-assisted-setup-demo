package handlers_test

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestSeedMatchesContract asserts the running app serves exactly what
// contract/seed.json declares, across both tenants. Each app hardcodes
// its own seed, so this is what stops the four from drifting.
//
// Organizations have no dedicated route in this API, so they are not
// checked here. Users, resources, and access requests are compared
// field for field; comments are seeded empty, so that collection is
// checked by asserting the app actually serves none.
func TestSeedMatchesContract(t *testing.T) {
	t.Parallel()
	r := newRouter(t)

	raw, err := os.ReadFile(filepath.Join("..", "..", "..", "contract", "seed.json"))
	require.NoError(t, err)

	var want struct {
		Users          []map[string]any `json:"users"`
		Resources      []map[string]any `json:"resources"`
		AccessRequests []map[string]any `json:"access_requests"`
		Comments       []map[string]any `json:"comments"`
	}
	require.NoError(t, json.Unmarshal(raw, &want))

	// One admin actor per tenant, so every fetch below can pick the
	// right actor for the org it is reading.
	actorFor := func(orgID string) string {
		if orgID == "org_globex" {
			return "u_grace"
		}
		return "u_ada"
	}

	t.Run("users", func(t *testing.T) {
		var got []map[string]any
		for _, actor := range []string{"u_ada", "u_grace"} {
			w := do(t, r, http.MethodGet, "/api/members", actor, "")
			require.Equal(t, http.StatusOK, w.Code)
			var members []map[string]any
			require.NoError(t, json.Unmarshal(w.Body.Bytes(), &members))
			got = append(got, members...)
		}
		requireSameByID(t, want.Users, got)
	})

	var gotResources []map[string]any
	t.Run("resources", func(t *testing.T) {
		for _, actor := range []string{"u_ada", "u_grace"} {
			w := do(t, r, http.MethodGet, "/api/resources", actor, "")
			require.Equal(t, http.StatusOK, w.Code)
			var resources []map[string]any
			require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resources))
			gotResources = append(gotResources, resources...)
		}
		requireSameByID(t, want.Resources, gotResources)
	})

	// Access requests are only reachable per resource, so walk the
	// resources just fetched and union their requests.
	var gotRequests []map[string]any
	t.Run("access_requests", func(t *testing.T) {
		for _, res := range gotResources {
			resID, _ := res["id"].(string)
			orgID, _ := res["organization_id"].(string)
			w := do(t, r, http.MethodGet, "/api/resources/"+resID+"/requests", actorFor(orgID), "")
			require.Equal(t, http.StatusOK, w.Code)
			var reqs []map[string]any
			require.NoError(t, json.Unmarshal(w.Body.Bytes(), &reqs))
			gotRequests = append(gotRequests, reqs...)
		}
		requireSameByID(t, want.AccessRequests, gotRequests)
	})

	t.Run("comments", func(t *testing.T) {
		require.Empty(t, want.Comments, "seed.json declares comments seeded empty")
		for _, req := range gotRequests {
			reqID, _ := req["id"].(string)
			orgID, _ := req["organization_id"].(string)
			w := do(t, r, http.MethodGet, "/api/requests/"+reqID+"/comments", actorFor(orgID), "")
			require.Equal(t, http.StatusOK, w.Code)
			var comments []map[string]any
			require.NoError(t, json.Unmarshal(w.Body.Bytes(), &comments))
			require.Empty(t, comments, "seeded request %s must have no comments", reqID)
		}
	})
}

// requireSameByID compares two record sets regardless of the order
// each side returned them in.
func requireSameByID(t *testing.T, want, got []map[string]any) {
	t.Helper()
	sortByID(want)
	sortByID(got)
	require.Equal(t, want, got)
}

func sortByID(records []map[string]any) {
	sort.Slice(records, func(i, j int) bool {
		return records[i]["id"].(string) < records[j]["id"].(string)
	})
}
