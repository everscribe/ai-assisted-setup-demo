package conformance

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func seedSet() map[string]bool {
	return map[string]bool{"u_ada": true, "r_acme_pg": true, "req_acme_1": true, "org_acme": true}
}

func TestNormalizeLeavesSeededIDsAlone(t *testing.T) {
	t.Parallel()
	n := NewNormalizer(seedSet())
	got, err := n.Normalize([]byte(`{"id":"u_ada","organization_id":"org_acme"}`))
	require.NoError(t, err)
	require.JSONEq(t, `{"id":"u_ada","organization_id":"org_acme"}`, string(got))
}

func TestNormalizeRewritesGeneratedIDsInOrderOfAppearance(t *testing.T) {
	t.Parallel()
	n := NewNormalizer(seedSet())
	got, err := n.Normalize([]byte(`[{"id":"9f2a10bc4d5e"},{"id":"11bb22cc33dd"},{"id":"9f2a10bc4d5e"}]`))
	require.NoError(t, err)
	require.JSONEq(t, `[{"id":"$id1"},{"id":"$id2"},{"id":"$id1"}]`, string(got),
		"the same generated id must map to the same placeholder every time")
}

func TestNormalizeReplacesTimestamps(t *testing.T) {
	t.Parallel()
	n := NewNormalizer(seedSet())
	got, err := n.Normalize([]byte(`{"created_at":"2026-03-14T12:00:00Z","decided_at":"2026-09-10T17:35:21.226491Z"}`))
	require.NoError(t, err)
	require.JSONEq(t, `{"created_at":"$ts1","decided_at":"$ts2"}`, string(got))
}

func TestNormalizeRejectsNonUTCTimestamps(t *testing.T) {
	t.Parallel()
	n := NewNormalizer(seedSet())
	for _, bad := range []string{
		`{"created_at":"2026-03-14T12:00:00+00:00"}`,
		`{"created_at":"2026-03-14T12:00:00"}`,
		`{"created_at":"2026-03-14 12:00:00Z"}`,
	} {
		_, err := n.Normalize([]byte(bad))
		require.Error(t, err, "must reject %s", bad)
		require.Contains(t, err.Error(), "timestamp")
	}
}

func TestNormalizeAcceptsAnyFractionalPrecision(t *testing.T) {
	t.Parallel()
	n := NewNormalizer(seedSet())
	// Go trims trailing zeros, Node emits 3 digits, Python 6, Rust 9.
	// All four are conforming; only the Z suffix and UTC are pinned.
	for _, ok := range []string{
		`{"created_at":"2026-03-14T12:00:00Z"}`,
		`{"created_at":"2026-03-14T12:00:00.123Z"}`,
		`{"created_at":"2026-03-14T12:00:00.123456Z"}`,
		`{"created_at":"2026-03-14T12:00:00.123456789Z"}`,
	} {
		got, err := n.Normalize([]byte(ok))
		require.NoError(t, err, "must accept %s", ok)
		require.JSONEq(t, `{"created_at":"$ts1"}`, string(got))
	}
}

func TestNormalizeIsStableAcrossKeyOrder(t *testing.T) {
	t.Parallel()
	n1, n2 := NewNormalizer(seedSet()), NewNormalizer(seedSet())
	a, err := n1.Normalize([]byte(`{"b":1,"a":2}`))
	require.NoError(t, err)
	b, err := n2.Normalize([]byte(`{"a":2,"b":1}`))
	require.NoError(t, err)
	require.Equal(t, string(a), string(b), "output must not depend on input key order")
}
