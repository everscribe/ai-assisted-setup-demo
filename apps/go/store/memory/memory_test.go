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
