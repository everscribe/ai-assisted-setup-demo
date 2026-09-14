package memory

// Internal, because the case it needs cannot be reached from outside the
// package. Every route mints a user with created_at = now, so a duplicate
// is always the later row and the ordering rule is invisible from any
// seam the store exports.

import (
	"testing"
	"time"

	"github.com/everscribe/ai-assisted-setup-demo/apps/go/models"
	"github.com/stretchr/testify/require"
)

// TestDuplicateEmailPrefersTheOlderRow pins the ordering half of the
// login rule: created_at ascending, then id ascending.
//
// The external TestDuplicateEmailResolvesToTheEarliestUser pins the other
// half, that the answer is stable. It can only mint duplicates through
// InviteMember, which always stamps a later created_at and never a tied
// one, so it never reaches the id tiebreak. This one plants a row older
// than the seeded user and a second tied with it, straight into the map,
// which is the only way either half of the comparator is exercised.
// Observed: dropping the id tiebreak leaves the external test green and
// turns this one red on every one of 20 runs.
func TestDuplicateEmailPrefersTheOlderRow(t *testing.T) {
	t.Parallel()
	s := New()
	require.NoError(t, s.Seed(t.Context()))

	// Positive control: the lookup answers before anything is planted.
	seeded, err := s.FindUserByEmail(t.Context(), "ada@acme.test")
	require.NoError(t, err)
	require.Equal(t, "u_ada", seeded.ID)

	older := &models.User{
		ID:             "u_older",
		Name:           "Ada Elder",
		Email:          "ada@acme.test",
		Role:           "member",
		OrganizationID: "org_acme",
		CreatedAt:      time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC),
	}
	s.users[older.ID] = older
	got, err := s.FindUserByEmail(t.Context(), "ada@acme.test")
	require.NoError(t, err)
	require.Equal(t, "u_older", got.ID, "the older duplicate must win on created_at")

	// Tied created_at, so only the id tiebreak can win it. Asserted over
	// many lookups, not one: with the tiebreak gone the sort keeps the
	// order the two rows came out of the map in, which is random per
	// lookup, so a single check would let that bug through about half the
	// time. Correct code answers the same row every time, so the loop is
	// free.
	tied := *older
	tied.ID = "u_aaa"
	tied.Name = "Ada Tie"
	s.users[tied.ID] = &tied
	for i := 0; i < tieLookups; i++ {
		got, err = s.FindUserByEmail(t.Context(), "ada@acme.test")
		require.NoError(t, err)
		require.Equal(t, "u_aaa", got.ID,
			"lookup %d: a tie on created_at must break on id ascending", i)
	}
}

// tieLookups is how many times the tied pair is looked up. Each lookup is
// close to a coin flip once the tiebreak is dropped, so 200 puts a false
// negative somewhere past one in 10^60.
const tieLookups = 200
