# Contract log

I did not open `apps/go/handlers/` or `apps/go/store/memory/` while doing this
task. Everything below comes from `contract/openapi.yaml`,
`contract/seed.json`, and the `Store` interface shape and file layout given
directly in the task brief (which itself is derived from
`apps/go/store/store.go`, which I did read, as permitted).

## Questions the contract did not answer

1. **RESOLVED. Does decommissioning an already-decommissioned resource
   conflict?** Originally a hole: the contract stated 409 for requesting
   access to a decommissioned resource, and for approve/deny/revoke on a
   request already past the relevant state, but said nothing about
   calling decommission twice on the same resource.

   What I did at the time: `MemoryStore.decommissionResource` rejected
   with `ErrConflict` if `decommission_at` was already set, reasoning
   from the pattern of the other terminal transitions. Confidence:
   medium, flagged as an inference, not a stated rule.

   Resolution: checked against the reference app, which returns 200 on
   both the first and second call. The contract now states this directly
   on `POST /resources/{id}/decommission` in `openapi.yaml`: idempotent,
   200, `decommission_at` re-stamped. `MemoryStore.decommissionResource`
   was changed to match, and a test
   (`memory.test.ts`, "decommissions idempotently, re-stamping
   decommission_at") asserts it, proven by mutation (reintroducing the
   `ErrConflict` throw fails the test, `Unknown Error: Symbol(conflict)`).
   My original 409 guess was a reasonable reading that happened to
   diverge from the reference; recording that here as the reason two
   competent implementations can disagree on a document that is truly
   silent.

2. **RESOLVED. Does `inviteMember` reject a duplicate email?** Originally
   a hole: nothing in the contract stated whether inviting a member with
   an email that already belongs to another user was allowed.

   What I did at the time: no uniqueness check, always create a new
   user. Confidence: low, flagged as unconfirmed.

   Resolution: checked against the reference app, which returns 201 for
   a duplicate email, including one already belonging to a seeded user.
   The contract now states this directly on `POST /members` in
   `openapi.yaml`. My implementation already matched; no code change was
   needed here, only the contract text and this log entry.

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

   ADOPTED INTO THE CONTRACT. This was independently confirmed as a real
   question an earlier plan had parked from the harness side and never
   resolved. `x-ordering` in `openapi.yaml` now states explicitly that
   `created_at` is compared as a parsed instant, never as a string, and
   why. No code change was needed; `MemoryStore` already did this.

5. **Task 5. Does a missing `role` on `PUT /members/{id}/role` get the
   same message as a mismatched one?** The contract states role there
   is "compared exactly," gives one worked example (`" admin "` is 400
   `{"error": "role must be admin or member"}`), but never separately
   addresses a request body that omits `role` entirely. Every other
   required-field route in the contract gets a distinct "X required"
   message for an absent field, which could argue for a
   `{"error": "role required"}` message here instead when the key is
   missing outright.

   What I did: treated "compared exactly" as the whole rule, not just
   the trimming half of it. `PUT /members/{id}/role` does one equality
   check (`role !== "admin" && role !== "member"`), so a missing key,
   a wrong type, and a mismatched string all fail it identically and
   get `{"error": "role must be admin or member"}`. I did not read
   `apps/go/handlers/members.go` to check the Go app's actual message
   for this case, per the read restriction for this task. Confidence:
   medium. This reads more naturally to me as one check, not two, but
   it is an inference from the framing of a single worked example, not
   a rule the contract states for the missing-key case specifically.

## Read restriction for Task 5

Same as Task 4: I did not open `apps/go/handlers/` or
`apps/go/store/memory/` while writing `handlers.ts`, `router.ts`, or
`main.ts`. Every status code, body shape, trimming rule, and precedence
rule in those three files comes from `contract/openapi.yaml` and
`contract/seed.json`. The one exception, permitted by the task brief,
is `apps/go/store/store.go` (read in Task 4) and the file layout of
`apps/go`, neither of which describes HTTP behaviour.

The full conformance harness (`CONFORMANCE_APPS="node=http://localhost:9002"
go test ./... -count=1 -v` from `contract/conformance`) passed against
this app on the first run against a freshly started process, with no
handler or contract changes needed beyond the revoke-description fix
below. That is the strongest evidence the contract held up: eighteen
routes, written blind to the Go implementation, matched its observable
behaviour on every check the harness makes.

## Contract text fix (not a hole)

`POST /api/requests/{id}/revoke`'s description ended with "for example
approving an already-denied request is 409 the same way," an example
about approving sitting in the revoke operation's own text. Changed the
example to be about revoking (an already-revoked request), since this
sentence, not the Go source, is what a port author reads. The rule
itself (409 if the request is not currently approved) was not touched.

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
- Task 5: did not use `express.json()` as global body-parsing
  middleware, even though that is idiomatic Express. The contract is
  explicit that a malformed body on an admin route must lose to the 403
  admin check, and Express's own body parsers throw before a route
  handler ever runs, which would answer 400 first. Used
  `express.text({ type: () => true })` instead to buffer the raw body
  as a string with no validation, then parse and validate it by hand
  inside each handler, strictly after `requireActor`/`requireAdmin`.
- Task 5: registered every path with `router.all(path, ...)` and
  dispatched on `req.method` by hand, rather than `router.get`/`.post`.
  Express's `Route` type auto-routes HEAD to a GET handler, and Express
  auto-answers OPTIONS with a 200 Allow list, when nothing else claims
  the method; the contract requires 405 for both on every path. `.all`
  claims every method itself, including HEAD and OPTIONS, so neither of
  those framework defaults gets a chance to run. Verified by hand with
  curl against a running instance (`HEAD` and `OPTIONS
  /api/health` both 405, not 200) before trusting it to the harness.
