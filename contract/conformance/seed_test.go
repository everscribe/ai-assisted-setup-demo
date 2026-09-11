package conformance

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

// Each app carries its own copy of the seed data, written by a port
// author from contract/seed.json with no access to any other app's
// source. Nothing else in the harness checks that the copy is faithful:
// normalization only needs the id set, and cross-app equality compares
// apps to each other, so a value wrong the same way in every app would
// sail through. This test closes that gap by comparing what an app
// serves to seed.json field for field.
//
// seed.json's organizations collection cannot be checked here. No route
// in the contract exposes organizations, so a conformance run cannot
// reach that collection at all; seed.json's own _notes says the same.
// The top-level _notes key is prose and is skipped.
//
// Extra rows are tolerated, seeded rows are strict. Other tests in this
// package mint members, resources, requests and comments before this one
// runs, and a developer running the suite twice against one `make up`
// sees the previous run's rows as well, so a test that demanded an exact
// row set would fail for reasons that say nothing about the app. Every
// id declared in seed.json must still be served, with every field equal.
//
// Two seeded rows are exempt on the fields the harness itself changes:
// the lifecycle sequence decommissions r_acme_s3 and approves then
// revokes req_acme_1. Only those fields on those two rows are skipped.
// Every other field on them, and every field on every other seeded row,
// is compared exactly.
var seedMutableFields = map[string][]string{
	"r_acme_s3":  {"decommission_at"},
	"req_acme_1": {"status", "decided_by", "decided_at", "revoked_by", "revoked_at"},
}

// seedCollections are the collections a conformance run can reach,
// in the order they are compared.
var seedCollections = []string{"users", "resources", "access_requests", "comments"}

func TestServedDataMatchesTheSeedFile(t *testing.T) {
	seed := loadSeedRows(t, filepath.Join("..", "seed.json"))
	for _, coll := range seedCollections {
		require.NotEmpty(t, seed[coll], "seed.json declares no %s", coll)
	}

	for _, app := range apps(t) {
		app := app
		t.Run(app.Name, func(t *testing.T) {
			served := collectServedRows(t, NewClient(app))
			for _, coll := range seedCollections {
				for _, want := range seed[coll] {
					id, _ := want["id"].(string)
					require.NotEmpty(t, id, "seed.json has a %s row with no id", coll)

					got, ok := served[coll][id]
					require.True(t, ok, "%s: seeded %s is not served by the app", coll, id)
					require.Equal(t, withoutMutableFields(want, id), withoutMutableFields(got, id),
						"%s: served %s does not match contract/seed.json", coll, id)
				}
			}
		})
	}
}

// withoutMutableFields copies a row minus the fields the harness itself mutates
// on that id.
func withoutMutableFields(row map[string]any, id string) map[string]any {
	out := make(map[string]any, len(row))
	for k, v := range row {
		out[k] = v
	}
	for _, k := range seedMutableFields[id] {
		delete(out, k)
	}
	return out
}

// loadSeedRows reads contract/seed.json as generic rows per collection.
func loadSeedRows(t *testing.T, path string) map[string][]map[string]any {
	t.Helper()
	raw, err := os.ReadFile(path)
	require.NoError(t, err)

	var doc map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(raw, &doc))

	out := map[string][]map[string]any{}
	for key, val := range doc {
		if key == "_notes" {
			continue
		}
		var rows []map[string]any
		if err := json.Unmarshal(val, &rows); err != nil {
			continue
		}
		out[key] = rows
	}
	return out
}

// collectServedRows fetches every reachable collection from both
// tenants and unions them, keyed by id. Both organizations are read,
// as u_ada and u_grace, because each tenant only ever sees its own
// rows: one actor alone would leave the other tenant's seed unchecked.
// Access requests and comments have no list-all route, so they are
// reached through the resources and requests already collected.
func collectServedRows(t *testing.T, c *Client) map[string]map[string]map[string]any {
	t.Helper()
	out := map[string]map[string]map[string]any{}
	for _, coll := range seedCollections {
		out[coll] = map[string]map[string]any{}
	}

	for _, actor := range []string{"u_ada", "u_grace"} {
		for _, row := range listRows(t, c, "/api/members", actor) {
			keepRow(out["users"], row)
		}
		for _, resource := range listRows(t, c, "/api/resources", actor) {
			keepRow(out["resources"], resource)
			resourceID, _ := resource["id"].(string)
			for _, req := range listRows(t, c, "/api/resources/"+resourceID+"/requests", actor) {
				keepRow(out["access_requests"], req)
				reqID, _ := req["id"].(string)
				for _, comment := range listRows(t, c, "/api/requests/"+reqID+"/comments", actor) {
					keepRow(out["comments"], comment)
				}
			}
		}
	}
	return out
}

func listRows(t *testing.T, c *Client, path, actor string) []map[string]any {
	t.Helper()
	resp := validated(t, c, http.MethodGet, path, actor, "")
	require.Equal(t, http.StatusOK, resp.Status, "%s as %s", path, actor)
	return jsonRows(t, resp.Body)
}

func keepRow(dst map[string]map[string]any, row map[string]any) {
	if id, ok := row["id"].(string); ok {
		dst[id] = row
	}
}
