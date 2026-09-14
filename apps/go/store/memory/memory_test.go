package memory_test

import (
	"fmt"
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

// TestDuplicateEmailResolvesToTheEarliestUser pins the rule openapi.yaml
// now states for POST /api/login: an email matching more than one user
// resolves to the earliest by created_at, then by id ascending.
//
// This lookup used to range the users map and return the first hit, and
// Go randomizes map iteration, so the answer changed run to run once a
// duplicate existed. Five duplicates plus 500 lookups is what makes the
// test able to fail: the old code picks the seeded user roughly one time
// in six, so surviving 500 consecutive lookups by luck has probability
// about (1/6)^500. Observed: the old code fails this on the first few
// iterations, the sorted one never does.
func TestDuplicateEmailResolvesToTheEarliestUser(t *testing.T) {
	t.Parallel()
	s := memory.New()
	require.NoError(t, s.Seed(t.Context()))

	// Positive control: with no duplicate the lookup already answers, so
	// a later failure is about which user won, not about finding one.
	first, err := s.FindUserByEmail(t.Context(), "ada@acme.test")
	require.NoError(t, err)
	require.Equal(t, "u_ada", first.ID)

	for i := 0; i < 5; i++ {
		dup, err := s.InviteMember(t.Context(), "org_acme",
			fmt.Sprintf("Ada Impostor %d", i), "ada@acme.test", "member")
		require.NoError(t, err)
		require.NotEqual(t, "u_ada", dup.ID)
		require.True(t, dup.CreatedAt.After(first.CreatedAt),
			"the duplicate must be minted after the seeded user for created_at to decide")
	}

	for i := 0; i < 500; i++ {
		u, err := s.FindUserByEmail(t.Context(), "ada@acme.test")
		require.NoError(t, err)
		require.Equal(t, "u_ada", u.ID, "lookup %d resolved the duplicate email to %s", i, u.ID)
	}

	_, err = s.FindUserByEmail(t.Context(), "nobody@acme.test")
	require.ErrorIs(t, err, store.ErrNotFound)
}

// TestRemovingAMemberLeavesTheirHistory pins the no-cascade rule
// openapi.yaml now states for DELETE /api/members/{id}. u_linus owns
// every seeded acme request, so removing him is the reachable case.
func TestRemovingAMemberLeavesTheirHistory(t *testing.T) {
	t.Parallel()
	s := memory.New()
	require.NoError(t, s.Seed(t.Context()))

	before, err := s.ListResourceRequests(t.Context(), "org_acme", "r_acme_pg")
	require.NoError(t, err)
	require.NotEmpty(t, before)
	comments, err := s.ListRequestComments(t.Context(), "org_acme", "req_acme_1")
	require.NoError(t, err)
	require.NotEmpty(t, comments)

	require.NoError(t, s.RemoveMember(t.Context(), "org_acme", "u_linus"))
	_, err = s.GetUser(t.Context(), "u_linus")
	require.ErrorIs(t, err, store.ErrNotFound, "the removal itself must have happened")

	after, err := s.ListResourceRequests(t.Context(), "org_acme", "r_acme_pg")
	require.NoError(t, err)
	require.Equal(t, before, after, "requests must survive their requester's removal")

	stillThere, err := s.ListRequestComments(t.Context(), "org_acme", "req_acme_1")
	require.NoError(t, err)
	require.Equal(t, comments, stillThere, "comments must survive their author's removal")
}

// TestRevokeKeepsTheDecisionFields pins the rule openapi.yaml now states
// for POST /api/requests/{id}/revoke, which until now was documented
// only by req_globex_2's shape in contract/seed.json.
func TestRevokeKeepsTheDecisionFields(t *testing.T) {
	t.Parallel()
	s := memory.New()
	require.NoError(t, s.Seed(t.Context()))

	approved, err := s.ApproveRequest(t.Context(), "org_acme", "req_acme_1", "u_ada")
	require.NoError(t, err)
	require.Equal(t, "u_ada", approved.DecidedBy)
	require.NotNil(t, approved.DecidedAt)
	decidedAt := *approved.DecidedAt

	revoked, err := s.RevokeRequest(t.Context(), "org_acme", "req_acme_1", "u_ada")
	require.NoError(t, err)
	require.Equal(t, "revoked", string(revoked.Status))
	require.Equal(t, "u_ada", revoked.DecidedBy, "revoke must keep decided_by")
	require.NotNil(t, revoked.DecidedAt)
	require.True(t, revoked.DecidedAt.Equal(decidedAt), "revoke must keep decided_at unchanged")
	require.Equal(t, "u_ada", revoked.RevokedBy)
	require.NotNil(t, revoked.RevokedAt)
}
