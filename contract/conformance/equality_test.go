package conformance

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// invalidBodyPrefix is the language-specific decoder text openapi.yaml
// (around lines 113-119) says is not normative: only the 400 status is
// binding, and "no conformance check may match on a message that begins
// 'invalid body:'". Collapsed to invalidBodyCanonical before two apps'
// error bodies are compared, so this comparator does not itself become
// such a check.
const (
	invalidBodyPrefix    = "invalid body:"
	invalidBodyCanonical = "invalid body"
)

// tsPlaceholder matches one of Normalizer's $ts1, $ts2, ... values exactly.
// Collapsed to one constant only for cross-app comparison: Go emits
// microsecond timestamps, Node millisecond, so writes landing in the same
// millisecond are one distinct value in Node's output and two in Go's,
// though both are conformant (any fractional precision is allowed). That
// distinctness is a clock-resolution artifact, not app behavior. Normalizer
// itself still numbers timestamps per app, since normalize_test.go pins a
// repeated timestamp mapping to one placeholder within a single app's output.
var tsPlaceholder = regexp.MustCompile(`^\$ts\d+$`)

// Only these two. Comparing every header would drag in Date and
// Content-Length, which vary per response and per runtime, and would
// reintroduce exactly the non-portable noise the timestamp and array
// order fixes removed.
var comparedHeaders = []string{"Content-Type", "Allow"}

// canonicalizeAllow sorts an Allow header's methods before comparison.
// openapi.yaml:122-124 says the Allow header on a 405 lists the
// registered methods in an unspecified order, so a raw string compare
// would be a conformance check matching on something the contract
// disowns. Sorting keeps the method set exact: a missing or invented
// method still diverges.
func canonicalizeAllow(v string) string {
	if v == "" {
		return ""
	}
	methods := strings.Split(v, ",")
	for i, m := range methods {
		methods[i] = strings.TrimSpace(m)
	}
	sort.Strings(methods)
	return strings.Join(methods, ",")
}

// canonicalize collapses timestamp placeholders (see tsPlaceholder) and sorts arrays
// into a stable order: Node's millisecond clock ties rows Go's microsecond clock
// never does, so Node applies the contract's id tiebreak where Go never needs to.
// Both follow the ordering rule correctly, but a tied pair's relative order isn't
// portable, only the set is. Object keys are already deterministic (Normalizer sorts
// map keys), so only arrays need this. It recurses into nested arrays; no schema has
// one yet, so a bug there is latent, since this only does cross-app set comparison -
// ordering_test.go's per-app check is what catches an ordering bug within one list.
func canonicalize(v any) any {
	switch t := v.(type) {
	case map[string]any:
		out := make(map[string]any, len(t))
		for k, val := range t {
			if k == "error" {
				if s, ok := val.(string); ok && strings.HasPrefix(s, invalidBodyPrefix) {
					out[k] = invalidBodyCanonical
					continue
				}
			}
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

// TestCanonicalizeCollapsesInvalidBodyPrefix pins the invalid body:
// exemption without needing either app up. Two decoder-specific
// messages must collapse to the same value; a genuinely different
// message must not, or the collapse would be swallowing every error
// rather than just this one documented case.
func TestCanonicalizeCollapsesInvalidBodyPrefix(t *testing.T) {
	t.Parallel()
	goBody := map[string]any{"error": "invalid body: json: cannot unmarshal number into Go struct field .email of type string"}
	nodeBody := map[string]any{"error": "invalid body"}
	require.Equal(t, canonicalize(goBody), canonicalize(nodeBody))

	other := map[string]any{"error": "role must be admin or member"}
	require.NotEqual(t, canonicalize(goBody), canonicalize(other),
		"a genuinely different error message must not collapse to the same value")
}

// TestCanonicalizeAllowIgnoresOrder pins the unspecified-order exemption
// (openapi.yaml:122-124) without needing either app up. Two orderings of
// the same set must canonicalize equal; a genuinely different set must
// not, or the sort would be swallowing a missing or invented method
// instead of just reordering. A repeated method must also not collapse
// to its deduplicated form: sorting is correct, deduplicating is not,
// and a repeat is a real divergence in what a port registered.
func TestCanonicalizeAllowIgnoresOrder(t *testing.T) {
	t.Parallel()
	require.Equal(t, canonicalizeAllow("POST, GET"), canonicalizeAllow("GET, POST"))

	require.NotEqual(t, canonicalizeAllow("GET, POST"), canonicalizeAllow("GET, POST, DELETE"),
		"a missing or invented method must not canonicalize equal")

	require.NotEqual(t, canonicalizeAllow("GET, GET, POST"), canonicalizeAllow("GET, POST"),
		"a repeated method must not canonicalize equal to its deduplicated form")
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
			for _, h := range comparedHeaders {
				v := r.Header.Get(h)
				if h == "Allow" {
					v = canonicalizeAllow(v)
				}
				row += fmt.Sprintf("%s=%s|", h, v)
			}
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
