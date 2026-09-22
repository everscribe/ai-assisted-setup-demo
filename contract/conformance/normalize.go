package conformance

import (
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strings"
)

// utcTimestamp is the only accepted shape: RFC 3339, UTC, literal Z.
// The fractional part is optional and of any length, because Go trims
// trailing zeros while Node emits 3 digits, Python 6, and Rust 9.
var utcTimestamp = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}(\.\d+)?Z$`)

// looksLikeTimestamp catches near misses so they fail loudly rather than
// being treated as an ordinary string that happens to differ.
var looksLikeTimestamp = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}[T ]\d{2}:\d{2}:\d{2}`)

// Normalizer rewrites runtime-varying values so two apps' responses can be
// compared. Seeded ids survive; generated ids become $id1, $id2 in order of
// first appearance; every timestamp becomes $ts1, $ts2 the same way.
type Normalizer struct {
	seedIDs map[string]bool
	ids     map[string]string
	stamps  map[string]string
}

func NewNormalizer(seedIDs map[string]bool) *Normalizer {
	return &Normalizer{seedIDs: seedIDs, ids: map[string]string{}, stamps: map[string]string{}}
}

// Normalize parses body as JSON, rewrites it, and re-encodes with sorted
// keys so the output does not depend on the app's key order.
func (n *Normalizer) Normalize(body []byte) ([]byte, error) {
	if len(body) == 0 {
		return body, nil
	}
	var v any
	if err := json.Unmarshal(body, &v); err != nil {
		return nil, fmt.Errorf("normalize: %w", err)
	}
	out, err := n.walk(v, "")
	if err != nil {
		return nil, err
	}
	return json.Marshal(out)
}

func (n *Normalizer) walk(v any, key string) (any, error) {
	switch t := v.(type) {
	case map[string]any:
		out := make(map[string]any, len(t))
		for _, k := range sortedKeys(t) {
			w, err := n.walk(t[k], k)
			if err != nil {
				return nil, err
			}
			out[k] = w
		}
		return out, nil
	case []any:
		out := make([]any, len(t))
		for i, item := range t {
			w, err := n.walk(item, key)
			if err != nil {
				return nil, err
			}
			out[i] = w
		}
		return out, nil
	case string:
		return n.scalar(t, key)
	default:
		return v, nil
	}
}

func (n *Normalizer) scalar(s, key string) (any, error) {
	if isTimestampKey(key) || looksLikeTimestamp.MatchString(s) {
		if !utcTimestamp.MatchString(s) {
			return nil, fmt.Errorf("normalize: timestamp %q is not RFC 3339 UTC with a Z suffix", s)
		}
		return n.placeholder(n.stamps, s, "$ts"), nil
	}
	if isIDKey(key) && !n.seedIDs[s] && s != "" {
		return n.placeholder(n.ids, s, "$id"), nil
	}
	return s, nil
}

func (n *Normalizer) placeholder(table map[string]string, value, prefix string) string {
	if existing, ok := table[value]; ok {
		return existing
	}
	next := fmt.Sprintf("%s%d", prefix, len(table)+1)
	table[value] = next
	return next
}

func isTimestampKey(k string) bool {
	return strings.HasSuffix(k, "_at")
}

func isIDKey(k string) bool {
	return k == "id" || strings.HasSuffix(k, "_id") || strings.HasSuffix(k, "_by")
}

func sortedKeys(m map[string]any) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
