package memory_test

import (
	"testing"

	"github.com/everscribe/ai-assisted-setup-demo/apps/go/store"
	"github.com/everscribe/ai-assisted-setup-demo/apps/go/store/memory"
	"github.com/stretchr/testify/require"
)

// memory.Store must satisfy the interface handlers depend on.
var _ store.Store = (*memory.Store)(nil)

func TestGetUserAcceptsContext(t *testing.T) {
	t.Parallel()
	s := memory.New()
	require.NoError(t, s.Seed(t.Context()))

	u, err := s.GetUser(t.Context(), "u_ada")
	require.NoError(t, err)
	require.Equal(t, "ada@acme.test", u.Email)
}

func TestCrossOrgAccessIsNotFound(t *testing.T) {
	t.Parallel()
	s := memory.New()
	require.NoError(t, s.Seed(t.Context()))

	// r_globex_k8s belongs to org_globex, not org_acme.
	_, err := s.GetResource(t.Context(), "org_acme", "r_globex_k8s")
	require.ErrorIs(t, err, store.ErrNotFound)
}

func TestListsReturnAnError(t *testing.T) {
	t.Parallel()
	s := memory.New()
	require.NoError(t, s.Seed(t.Context()))

	members, err := s.ListOrgMembers(t.Context(), "org_acme")
	require.NoError(t, err)
	require.Len(t, members, 2)
}

func TestSeedTimestampsAreFixed(t *testing.T) {
	t.Parallel()
	a, b := memory.New(), memory.New()
	require.NoError(t, a.Seed(t.Context()))
	require.NoError(t, b.Seed(t.Context()))

	ua, err := a.GetUser(t.Context(), "u_ada")
	require.NoError(t, err)
	ub, err := b.GetUser(t.Context(), "u_ada")
	require.NoError(t, err)

	// Two independently seeded stores must agree exactly. With now-relative
	// offsets these differ by the time between the two Seed calls.
	require.True(t, ua.CreatedAt.Equal(ub.CreatedAt),
		"seed timestamps must be absolute, got %s and %s", ua.CreatedAt, ub.CreatedAt)
	require.Equal(t, 2026, ua.CreatedAt.Year())
}

func TestCommentOrderBreaksTiesOnID(t *testing.T) {
	t.Parallel()
	s := memory.New()
	require.NoError(t, s.Seed(t.Context()))

	// Post three comments fast enough to collide on the timestamp.
	for i := 0; i < 3; i++ {
		_, err := s.AddComment(t.Context(), "org_acme", "req_acme_1", "u_ada", "note")
		require.NoError(t, err)
	}

	got, err := s.ListRequestComments(t.Context(), "org_acme", "req_acme_1")
	require.NoError(t, err)
	require.GreaterOrEqual(t, len(got), 3)

	for i := 1; i < len(got); i++ {
		prev, cur := got[i-1], got[i]
		if prev.CreatedAt.Equal(cur.CreatedAt) {
			require.Less(t, prev.ID, cur.ID,
				"comments with equal timestamps must order by id ascending")
		} else {
			require.True(t, prev.CreatedAt.Before(cur.CreatedAt))
		}
	}
}
