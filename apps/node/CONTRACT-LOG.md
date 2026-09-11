# Contract log

I did not open `apps/go/handlers/` or `apps/go/store/memory/` while doing this
task. Everything below comes from `contract/openapi.yaml`,
`contract/seed.json`, and the `Store` interface shape and file layout given
directly in the task brief (which itself is derived from
`apps/go/store/store.go`, which I did read, as permitted).

## Questions the contract did not answer

1. **Does decommissioning an already-decommissioned resource conflict?**
   The contract states 409 for requesting access to a decommissioned
   resource, and for approve/deny/revoke on a request already past the
   relevant state, but says nothing about calling decommission twice on
   the same resource. There is no route or fixture in `seed.json` that
   exercises this.

   What I did: `MemoryStore.decommissionResource` rejects with
   `ErrConflict` if `decommission_at` is already set, on the reasoning
   that this fits the same class of "state transition that does not
   apply" the contract defines for every other terminal transition
   (approve/deny/revoke). Confidence: medium. This is an inference from
   the pattern elsewhere in the contract, not a stated rule, and a
   reasonable port could instead make decommission idempotent (200,
   re-stamping or leaving `decommission_at` alone). Task 5's handler
   should confirm this against however the conformance harness actually
   exercises it, since the harness is what ultimately arbitrates.

2. **Does `inviteMember` reject a duplicate email?**
   Login (`findUserByEmail`) implies email is a lookup key, but nothing
   in the contract states whether inviting a member with an email that
   already belongs to another user (same org or a different one) is
   allowed, or what error it would produce if not.

   What I did: `MemoryStore.inviteMember` performs no uniqueness check
   and always creates a new user. If Task 5 needs one, the correct
   status code is still an open question (400? 409?) that isn't in the
   contract's status-code list for `POST /members`, which only documents
   400/401/403. Confidence: low. I did not implement a check I have no
   contract basis for.

3. **Timestamp precision for runtime-minted rows.**
   Not really a hole: the contract explicitly says only seed rows need
   whole-second precision, and runtime-minted timestamps may carry a
   fractional part of unspecified digit count. I used
   `new Date().toISOString()` (millisecond precision) for every
   `created_at`/`decided_at`/`revoked_at` the store mints. Noting it here
   only because it's the one place the contract explicitly relaxes its
   own timestamp rule, and I want the choice to be visible.

4. **Ordering comparator for timestamps: string vs. parsed compare.**
   The contract says compare by `created_at` then `id`, but doesn't say
   how to compare two RFC 3339 strings. Lexicographic string comparison
   works for same-format-length values (which is all `seed.json` has),
   but breaks down once a millisecond-precision runtime timestamp is
   compared against a whole-second seed timestamp with the same second
   value (the `.` in `.500Z` sorts before `Z` lexicographically, which
   happens to still put the fractional one first, i.e. earlier, which is
   correct only if it truly is earlier - not guaranteed).

   What I did: compare with `Date.parse(a) - Date.parse(b)` instead of
   string comparison, so runtime and seed timestamps compare correctly
   regardless of fractional-second formatting. Confidence: high, this is
   a strictly safer superset of what string comparison gives on the
   fixture data, not a deviation from it.

## What I deliberately did not do

- Did not add a `role`/status literal union type to `models.ts`; kept
  `role`, `status`, and `sensitivity` as plain `string` to match the
  `Store` interface's own parameter types (`role: string`), which the
  brief specifies verbatim. `sensitivity` is explicitly documented in
  the contract as free-form, not an enum, so this was never a question
  for that field.
- Did not implement any admin/role enforcement in the store. The task
  brief and the `Store` interface (also given verbatim) both make clear
  that `ErrForbidden` is exported for handlers (Task 5) to use directly
  around the role check that the contract requires happen before body
  parsing; no store method takes an actor or a role to check against.
