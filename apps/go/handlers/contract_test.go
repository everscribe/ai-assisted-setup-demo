package handlers_test

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestSeedMatchesContract asserts the running app serves exactly what
// contract/seed.json declares. Each app hardcodes its own seed, so this
// is what stops the four from drifting.
func TestSeedMatchesContract(t *testing.T) {
	t.Parallel()
	r := newRouter(t)

	raw, err := os.ReadFile(filepath.Join("..", "..", "..", "contract", "seed.json"))
	require.NoError(t, err)

	var want struct {
		Users     []map[string]any `json:"users"`
		Resources []map[string]any `json:"resources"`
	}
	require.NoError(t, json.Unmarshal(raw, &want))

	w := do(t, r, http.MethodGet, "/api/members", "u_ada", "")
	require.Equal(t, http.StatusOK, w.Code)

	var got []map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &got))

	var wantAcme []map[string]any
	for _, u := range want.Users {
		if u["organization_id"] == "org_acme" {
			wantAcme = append(wantAcme, u)
		}
	}
	require.Equal(t, wantAcme, got, "served members must match contract/seed.json exactly")
}
