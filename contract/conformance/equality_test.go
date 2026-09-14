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
