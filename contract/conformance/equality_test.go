package conformance

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"regexp"
	"sort"
	"testing"

	"github.com/stretchr/testify/require"
)

// tsPlaceholder matches one of Normalizer's $ts1, $ts2, ... values
// exactly. Collapsed to a single constant before cross-app comparison
// only: Go emits microsecond timestamps, Node emits millisecond, so two
// runtime writes that land in the same millisecond are one distinct
// value in Node's response and two in Go's, even though both apps are
// conformant (the contract allows any fractional precision).
// Distinctness of a placeholder number is a clock-resolution artifact,
// not app behaviour, so it must not be compared across apps. Normalizer
// itself keeps numbering timestamps, since normalize_test.go pins that a
// repeated timestamp within one app's own output maps to one
// placeholder.
var tsPlaceholder = regexp.MustCompile(`^\$ts\d+$`)

// canonicalize collapses timestamp placeholders (see tsPlaceholder) and
// sorts array elements into a stable order, so two apps' responses
// compare equal regardless of a clock-resolution-driven ordering tie.
// A tie in created_at is app-specific: Node's millisecond clock ties two
// rows Go's microsecond clock never does, so Node applies the contract's
// id tiebreak (a random generated string) where Go never needs to. Both
// apps correctly follow the ordering rule against their own data, but
// the resulting relative order of the tied rows is then not portable
// across apps, only the set of rows is. Object key order was already
// deterministic (Normalizer sorts map keys), so only arrays need this.
//
// This recurses into nested arrays unconditionally, so a sort here
// hides order for any array at any depth, not just the top-level list
// endpoints. No schema in this contract has a nested array today, so
// that is latent, not live, but a later port adding one would have its
// order silently unchecked by this test. ordering_test.go's per-app
// property check is what still catches an ordering bug in a single
// list; this function's job is only to make two apps' sets comparable.
func canonicalize(v any) any {
	switch t := v.(type) {
	case map[string]any:
		out := make(map[string]any, len(t))
		for k, val := range t {
			out[k] = canonicalize(val)
		}
		return out
	case []any:
		out := make([]any, len(t))
		for i, item := range t {
			out[i] = canonicalize(item)
		}
		sort.Slice(out, func(i, j int) bool {
			bi, _ := json.Marshal(out[i])
			bj, _ := json.Marshal(out[j])
			return string(bi) < string(bj)
		})
		return out
	case string:
		if tsPlaceholder.MatchString(t) {
			return "$ts"
		}
		return t
	default:
		return t
	}
}

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
			row := fmt.Sprintf("%d|", r.Status)
			if len(body) > 0 {
				var parsed any
				require.NoError(t, json.Unmarshal(body, &parsed), "%s: step %d (%s)", app.Name, i, steps[i].Name)
				canon, err := json.Marshal(canonicalize(parsed))
				require.NoError(t, err)
				row += string(canon)
			}
			rows = append(rows, row)
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
