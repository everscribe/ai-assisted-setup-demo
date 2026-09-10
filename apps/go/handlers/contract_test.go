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
// checked here and nothing in this repo can verify them. Users,
// resources, access requests, and comments are compared field for
// field.
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
		var got []map[string]any
		for _, req := range gotRequests {
			reqID, _ := req["id"].(string)
			orgID, _ := req["organization_id"].(string)
			w := do(t, r, http.MethodGet, "/api/requests/"+reqID+"/comments", actorFor(orgID), "")
			require.Equal(t, http.StatusOK, w.Code)
			var comments []map[string]any
			require.NoError(t, json.Unmarshal(w.Body.Bytes(), &comments))
			got = append(got, comments...)
		}
		require.NotEmpty(t, want.Comments, "seed.json must fixture the comment ordering")
		requireSameByID(t, want.Comments, got)
	})
}

// TestSeedFixturesBothOrderings pins the served order of the two list
// endpoints that declare one. Each group holds a pair tied on created_at,
// so an implementation missing the id tiebreak fails here even though
// TestSeedMatchesContract, which compares order-insensitively, passes.
func TestSeedFixturesBothOrderings(t *testing.T) {
	t.Parallel()
	r := newRouter(t)

	t.Run("requests are created_at desc then id asc", func(t *testing.T) {
		w := do(t, r, http.MethodGet, "/api/resources/r_acme_pg/requests", "u_ada", "")
		require.Equal(t, http.StatusOK, w.Code)
		require.Equal(t, []string{"req_acme_1", "req_acme_4", "req_acme_5"}, idsOf(t, w.Body.Bytes()))
	})

	t.Run("comments are created_at asc then id asc", func(t *testing.T) {
		w := do(t, r, http.MethodGet, "/api/requests/req_acme_1/comments", "u_ada", "")
		require.Equal(t, http.StatusOK, w.Code)
		require.Equal(t, []string{"c_acme_1", "c_acme_2", "c_acme_3"}, idsOf(t, w.Body.Bytes()))
	})
}

// idsOf pulls the id field out of a JSON array, preserving order.
func idsOf(t *testing.T, body []byte) []string {
	t.Helper()
	var records []map[string]any
	require.NoError(t, json.Unmarshal(body, &records))
	ids := make([]string, 0, len(records))
	for _, rec := range records {
		ids = append(ids, rec["id"].(string))
	}
	return ids
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
