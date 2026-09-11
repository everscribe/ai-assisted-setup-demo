package conformance

import (
	"fmt"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

// The four apps are compared to each other, not to a designated
// reference, so no app's runtime output is privileged and drift is
// caught wherever it appears. Skipped with fewer than two apps up,
// which is the state plans 1 and 2 leave behind.
func TestAppsAgreeWithEachOther(t *testing.T) {
	up := apps(t)
	if len(up) < 2 {
		t.Skipf("cross-app equality needs at least two apps, found %d (%s)", len(up), up[0].Name)
	}

	seedIDs, err := LoadSeedIDs(filepath.Join("..", "seed.json"))
	require.NoError(t, err)

	steps := Lifecycle()
	normalized := map[string][]string{}

	for _, app := range up {
		responses := Run(t, NewClient(app))
		n := NewNormalizer(seedIDs)
		rows := make([]string, 0, len(responses))
		for i, r := range responses {
			body, err := n.Normalize(r.Body)
			require.NoError(t, err, "%s: step %d (%s)", app.Name, i, steps[i].Name)
			rows = append(rows, fmt.Sprintf("%d|%s", r.Status, string(body)))
		}
		normalized[app.Name] = rows
	}

	base := up[0].Name
	for _, app := range up[1:] {
		for i := range steps {
			require.Equal(t, normalized[base][i], normalized[app.Name][i],
				"step %d (%s): %s and %s disagree", i, steps[i].Name, base, app.Name)
		}
	}
}
