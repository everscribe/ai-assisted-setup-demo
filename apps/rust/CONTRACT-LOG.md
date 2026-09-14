# Contract log

I did not open `apps/go/handlers/`, `apps/go/store/memory/`,
`apps/node/src/handlers.ts`, or `apps/node/src/memory.ts` while doing this
task, nor `apps/node/dist/`, which holds the compiled form of the same two
Node files. Everything below comes from `contract/openapi.yaml`,
`contract/seed.json`, the task brief, and `apps/go/store/store.go`, which
the brief permits and which describes no HTTP behaviour. I also read
`apps/node/CONTRACT-LOG.md` (prose about the contract, not an
implementation) and `apps/node/package.json` for the crate naming
convention.

The complete list of files I read is in the task report.

## Questions the contract could not answer

### 1. Which user does a duplicate email resolve to?

`POST /api/members` explicitly does not enforce email uniqueness: "a
duplicate email, including one already belonging to a seeded user, creates
a second user and returns 201." `POST /api/login` then "looks up a seeded
user by email." The contract never says which of two users sharing an
email that lookup returns, and after one duplicate invite the question is
live.

What I did: `find_user_by_email` collects every match and applies the same
total order the list endpoints use, created_at ascending then id ascending,
and returns the first. So a seeded user always beats a later duplicate, and
the answer never depends on storage order. Confidence: high that this is
sane, low that it is what the reference app does. It is unreachable by any
conformance check I can see unless the harness invites a duplicate of
`ada@acme.test` and then logs in as her, so this is a genuinely open
question rather than a divergence I can test my way out of.

### 2. Does removing a member cascade to their requests and comments?

`DELETE /api/members/{id}` declares 401, 403, 404 and 405 and no 409, and
nothing in the contract says what becomes of the rows that point at the
removed user. `req_acme_1` through `req_acme_5` all carry
`requested_by: u_linus`, so removing Linus is a reachable case.

What I did: no cascade. The user row goes, the requests and comments stay
and keep pointing at an id that no longer resolves. Reasoning: every
`*_by` field in the response schemas is a plain string with no stated
referential guarantee, and deleting audit history to tidy up a user
removal would be the surprising behaviour in an access-request app.
Confidence: medium. A port that cascaded would serve a shorter
`GET /api/resources/r_acme_pg/requests` after the same DELETE and nothing
in the contract would call either of us wrong.

### 3. Can an admin remove themselves, or the last admin of an org?

Contract silent. `DELETE /api/members/{id}` lists no 409, and 403 is
documented as "the actor is authenticated but is not an admin", so there
is no status code left for "you may not remove this particular member".

What I did: allowed both. An org can be left with no admin, and an admin
can delete their own record, after which their `X-User-ID` stops resolving
and every later call is a 401. `StoreError::Forbidden` exists but no store
method constructs it, for the same reason: no store method takes an actor.
Confidence: high, because the absent 409 is close to dispositive.

### 4. Is a duplicate access request a conflict?

`POST /api/resources/{id}/requests` declares 409 and the description gives
exactly one cause: "Fails with 409 if the resource has been
decommissioned." It does not say whether a member with a request already
pending on the same resource may open a second one.

What I did: allowed it. The one documented cause reads as exhaustive, and
`POST /api/members` sets the precedent that this API does not invent
uniqueness rules. Confidence: high.

### 5. Can you comment on a decommissioned or already-decided request?

`POST /api/requests/{id}/comments` declares 400, 401, 404 and 405 and no
409 at all, unlike approve, deny, revoke and request-access, which all
declare one.

What I did: no state check. A comment lands on a request in any status,
including one whose resource is decommissioned. Confidence: high, since
the route has no 409 to return.

### 6. What does revoke do to `decided_by` and `decided_at`?

The prose says only that revoke fails with 409 if the request is not
currently approved. It does not say whether the approval's decision fields
survive.

Answered by the fixture rather than the prose: `req_globex_2` in
`contract/seed.json` is `"status": "revoked"` and carries
`decided_by`/`decided_at` from the approval alongside
`revoked_by`/`revoked_at`. So `revoke_request` sets the revocation fields
and leaves the decision fields alone. Worth recording because a port
author reading only the operation descriptions would not learn this, and
because the seed is the only place it is written down.

### 7. Which layer trims and defaults?

The contract states the trimming and defaulting rules ("an omitted or
empty sensitivity becomes internal", role "defaults to member") in its
top-level prose about request validation, which is handler territory, but
the store is what actually writes the row.

What I did, and the asymmetry is deliberate:

- `register_resource` treats an empty `sensitivity` as `internal`. The
  handler will do the same; doing it twice costs nothing and means no
  row can be stored with an empty sensitivity.
- `invite_member` coerces anything that is not `admin` to `member`,
  because the contract documents that as a default, not an error.
- `update_member_role` does not coerce, because the contract documents
  that route as rejecting anything but `admin` or `member` with a 400.
  Coercing there would swallow a 400 the handler owes the caller.

The store does not trim. Trimming is stated per-route and per-field, so it
belongs to the handler that knows which route it is.

### 8. Does `seed()` reset or merge?

Contract silent; it is not an HTTP surface. Chose reset, so calling it
twice leaves exactly the fixture rather than duplicating it.

### 9. Id shape.

The brief fixes the prefixes (`u_`, `r_`, `req_`, `c_`) and says "random
hex". Neither it nor the contract fixes a length or a case. `x-ordering`
notes only that generated ids "are a random hex suffix that can sort
either side of the seeded rows", which is true of any length.

What I did: sixteen lowercase hex digits from a `u64`. Ids are opaque to
every conformance check, so this is free.

### 10. Timestamp precision and formatting.

The amended `x-ordering.timestamp-precision` sets a floor of at least
millisecond precision on runtime-minted timestamps and accepts anything at
or above it, while every seeded timestamp must serialize with whole-second
precision and a literal `Z`.

What I did: one serializer, `to_rfc3339_opts(SecondsFormat::AutoSi, true)`.
AutoSi emits no fractional part when the instant has none, which is what
the seed rows need, and emits every non-zero sub-second digit otherwise,
which on this platform is microseconds from `Utc::now()`. Nothing is
truncated. Two tests pin both halves.

One honest edge: if a runtime `Utc::now()` ever landed on exactly zero
nanoseconds, AutoSi would print that instant without a fraction. The
instant is still correct and the contract says comparison is by parsed
instant with the digit count unspecified, so this conforms; I am noting it
because it is the one input that makes the output look second-precise.

## Decisions that are not contract holes

### The crate cannot be called `everscribe-demo-rust`

The brief's Step 1 says `cargo init --name everscribe-demo-rust`. That name
fails this repo's own uninstrumented guard: `make check-clean` scans
`apps/*/Cargo.toml` with, among other patterns, `"everscribe[^"]*"`, and
`name = "everscribe-demo-rust"` matches it. I confirmed this by running the
guard's grep against that literal line before choosing a name.

Used `ai-assisted-setup-demo-rust`, which also matches `apps/node`'s
`ai-assisted-setup-demo-node` and `apps/go`'s module path. Consequence for
a later task: the default binary is `target/release/ai-assisted-setup-demo-rust`,
which is the name the Dockerfile will need.

### `cargo init` wrote no `.gitignore`

It skips VCS scaffolding inside an existing repository, so the
`apps/rust/.gitignore` the plan expected did not appear. Wrote it by hand
with `/target`. `Cargo.lock` is committed, as a binary crate's should be.

### Vec, not HashMap, behind the lock

Each collection is a `Vec` rather than a `HashMap`. A `HashMap` gives the
sort an arbitrary iteration order, so a port that dropped the id tiebreak
would emit a different order run to run instead of a reproducibly wrong
one, and no test could pin it. A `Vec` keeps insertion order defined, which
is what makes the ordering mutation below deterministic. Lookups are linear
over a fixture of a few rows.

A `BTreeMap` keyed by id would have been worse than either: it iterates
id-ascending, so a sort with no tiebreak would produce the right answer for
free and the tiebreak would be untestable.

### The brief's ordering test did not test ordering

Recording this because it is the most useful thing this task found. The
test the brief supplies asserts
`["r_acme_pg", "r_acme_s3", "r_acme_stripe"]`. All three share
`created_at: 2026-03-13T12:00:00Z` and the seed lists them id-ascending, so
with `sort_by` being stable, a comparator with no tiebreak at all produces
that exact sequence.

I verified this rather than asserting it. With the id tiebreak removed from
`list_org_resources` and the probe rows stripped back out, the brief's
three-id assertion passes. The test as given cannot fail for the reason it
exists.

The committed test therefore also pushes two rows sharing a later
`created_at` in reverse id order (`r_acme_zz` then `r_acme_aa`) before
listing, and asserts the full five-id sequence. That is the assertion the
mutation breaks. The full mutation record is in the task report.

Then I made the identical mistake in the test I added to cover the two
`x-ordering.examples` sequences, and review caught it. `r_acme_pg`'s
requests are already in file order `[req_acme_1, req_acme_4, req_acme_5]`
and `req_acme_1`'s comments are already `[c_acme_1, c_acme_2, c_acme_3]`,
so both worked examples reproduce themselves with no sort at all. Diagnosing
the trap in someone else's test is evidently not the same as avoiding it in
your own. Both lists now get a probe pair of their own, sharing an instant
later than every seeded row in reverse id order, so insertion order and
sorted order differ on the domain key and on the tiebreak at once. Four
mutation runs are recorded in the task report.

### Tenant scoping needed a test of its own

Also from review: eleven of the twelve org-scoped methods had no scoping
test. Only `resource_in_org` was covered, by the brief's
`cross_tenant_resource_is_not_found`. Dropping the `organization_id` check
from `request_in_org`, `request_in_org_mut`, `remove_member` and
`update_member_role` left every test green, which is a poor state for the
one invariant this repo exists to demonstrate.

`every_org_scoped_method_refuses_a_cross_tenant_target` is table-driven over
all twelve. Each row runs the call twice against a fresh seeded store: once
with the owning org, which must succeed, and once with the other org, which
must fail with exactly `NotFound`. The positive control is what stops a
method that refuses everything from passing, and matching the variant rather
than merely "it errored" is what stops a `Forbidden` from passing, since
`Forbidden` would leak the existence of the row the 404 rule exists to hide.
Failures are collected rather than asserted one at a time, so one run names
every method that leaked.

### `#![allow(dead_code)]` in `main.rs`

The store is the whole crate today and `main` is still the `cargo init`
stub, so every public item is unreachable and the build emitted 29 dead
code warnings. The allow suppresses them. Task 5 wires the store into the
server and should delete the attribute then.

### Models derive `Serialize` only

Nothing deserializes a domain record. Request bodies are separate shapes
owned by the handlers, so adding `Deserialize` would mean writing a
matching RFC 3339 deserializer for a path nothing takes. `role`, `status`
and `sensitivity` stay plain `String`, matching the trait's own parameter
types, and `sensitivity` is documented as free-form rather than an enum.

### No store-level role enforcement

`StoreError::Forbidden` is exported for the handlers. No store method takes
an actor or a role, and the contract requires the admin check to run ahead
of body parsing, which is a handler ordering concern the store cannot see.

## Contract versus brief

No disagreement about any field name, type or optionality. Everything in
`models.rs` comes from the response schemas, including `decommission_at`
(not `decommissioned_at`) and the four optional fields on `AccessRequest`.
The only brief-versus-repo conflict is the crate name above, and it is with
the clean guard rather than with the contract.

Of the three recent amendments, only `timestamp-precision` reaches the
store, and it costs nothing here. The HEAD-has-no-body clarification and
the unspecified `Allow` order are handler and router concerns for Task 5.
