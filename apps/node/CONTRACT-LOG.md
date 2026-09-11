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

6. **RESOLVED. Does a non-string `role` get rejected, or coerced to
   "member"?** Originally a divergence, caught by review, not by the
   harness (see the note on that below). `InviteMemberRequest.role`
   carried `type: string, enum: [admin, member]` with description
   "Defaults to member if omitted or not one of admin, member." I read
   "not one of admin, member" as covering any value that fails the enum
   check, including a value of the wrong type, so `{"role": 123}` on
   `POST /api/members` fell through to the "member" default and
   returned 201.

   What I did at the time: `handleInviteMember` compared `rawRole` to
   the two string literals with no type guard, unlike every other field
   in this app, which rejects a wrong type as 400 `invalid body` before
   ever reaching a default or a trim. Confidence at the time: not
   flagged, since I read the description as settling it.

   Resolution: the reference app returns 400 for `{"role": 123}`, not
   201. The description's "not one of admin, member" governs a string
   outside the enum, like `"wizard"`, since the field is declared
   `type: string` first; a non-string value fails the schema before the
   enum is ever consulted. `InviteMemberRequest.role`'s description in
   `openapi.yaml` now reads: "Optional. If omitted, or if the string
   value is not admin or member, defaults to member. A non-string value
   violates this schema and is rejected with 400." `handleInviteMember`
   now type-checks `role` the same way `requiredStringField` and
   `optionalStringField` check every other field: present and
   non-string is 400 `invalid body`; omitted or a string outside the
   enum still defaults to "member". `handleUpdateMemberRole` got the
   same type check for the same reason (`ChangeMemberRoleRequest.role`
   is also `type: string`), ahead of the existing exact-match check from
   item 5 above, which still governs a present, correctly-typed, but
   wrong string value (or the missing-key case item 5 covers, since
   `rawRole !== undefined` guards the new type check and leaves a
   missing key to fall into the existing equality branch unchanged).
   Both routes have a `router.test.ts` case sending `role: 123` and
   asserting 400 `invalid body`, proven by mutation: restoring the
   old untyped coercion made both new tests fail (201 and
   `"role must be admin or member"` respectively, instead of 400
   `invalid body`), and restoring the fix made them pass again.

   Worth recording plainly: my reading of the enum description was
   defensible on its own text, which is exactly why review caught it
   and the description needed rewording, not just the code.

   Amended again by the final review: the type guard this item added
   also classified a JSON `null` as a non-string, which made
   `{"role": null}` a 400 here while the reference app returned 201.
   See item 7.

7. **RESOLVED. Is a JSON `null` a wrong type, or an omitted field?** A
   divergence this app created. Item 6's type guards rejected `null`
   along with every other non-string, so `{"role": null}` on
   `POST /api/members` and `{"description": null}` or
   `{"sensitivity": null}` on `POST /api/resources` were 400 here and
   201 in the reference app. The same guard turned `{"name": null}`
   into `{"error": "invalid body"}` where the reference app says
   `{"error": "name and email required"}`.

   Why the reference behaves that way: Go's `encoding/json` treats a
   `null` into a string field as a no-op, so the field keeps its zero
   value and the documented default applies. Nothing in the contract
   contradicted that, and item 6's amendment ("a non-string value
   violates this schema") had accidentally endorsed this app against it.

   Resolution: `null` is equivalent to omitting the field, everywhere,
   optional and required alike. `requiredStringField` and
   `optionalStringField` now treat `null` exactly as `undefined`, and
   both role guards read `body.role ?? undefined` so a `null` falls
   through to the omitted path. Any other wrong type is still a 400.
   `openapi.yaml`'s top-level description now states the rule outright
   and `InviteMemberRequest.role`'s description was corrected to
   "A non-string value other than null". `validation_test.go` in the
   harness asserts it against every app, and `router.test.ts` has three
   cases of its own.

8. **RESOLVED. Are paths case sensitive?** Another divergence this app
   created, and a security one. `express.Router({ caseSensitive: false })`
   plus Express's case-insensitive mount meant `GET /api/ME` returned
   Ada's record and `GET /API/me` reached the API at all, where the
   reference app 404s both. Axum and FastAPI are case sensitive by
   default, so this app was the outlier of four.

   It also partly defeated the contract's own existence-nondisclosure
   rule: with no actor, a mixed-case spelling of a registered path
   answered 401 while an unregistered one answered 404, which is exactly
   the oracle the trailing-slash rule exists to close.

   Resolution: `caseSensitive: true` on the router, and
   `app.set("case sensitive routing", true)` on the app so the `/api`
   mount is case sensitive too. The contract now states the rule beside
   the trailing-slash rule, since both exist for the same reason.
   `validation_test.go` asserts a mixed-case path returns the same JSON
   404 as an unknown path, with a lowercase positive control and a
   no-actor probe.

9. **RESOLVED. Does `additionalProperties: false` on a request schema
   mean anything?** No, and the contract now says so. Every request
   schema declared it, but neither this app nor the reference app
   enforced it: `{"name": "x", "bogus": 1}` was a 201 in both. A Rust
   port using `serde(deny_unknown_fields)` or a Python port using
   Pydantic `extra="forbid"` would have mirrored the schema faithfully
   and returned 400 where both existing apps return 201, and nothing in
   the harness would have caught it.

   Resolution: the schemas were relaxed rather than the apps tightened,
   since ignoring unknown fields is what all four ecosystems do by
   default. The six request schemas now set
   `additionalProperties: true`, the top-level description states that
   unknown request fields are ignored and warns against mirroring a
   strict schema, and `validation_test.go` asserts it. Response schemas
   stay strict; those are accurate and the harness enforces them.

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
