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

Task 5, the handlers and the router, kept the same restriction and added
questions 11 to 18 and the notes below them. Its review lifted the
restriction far enough to run the other two apps, which settled question
13; nothing in the four forbidden paths was opened even then. Its own read list is in
`task-5-report.md`; it opened none of those four paths either, and read
nothing under `apps/go/` or `apps/node/` at all.

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

Settled in task 7, and only halfway by the harness. `contract/openapi.yaml`
now states the rule, and the lifecycle invites a duplicate and logs in with
it, so the equality layer catches a port that answers differently from the
others. It does not catch a port that keeps no order at all: over HTTP a
duplicate is always minted after the row it duplicates, so created_at order
and a store's own iteration order always agree. Confirmed by deleting the
sort from `apps/node`'s store, which left both the harness test and
equality green. The ordering itself is pinned in each store's own suite,
here by `duplicate_email_resolves_to_the_earliest_user`, which plants a
duplicate older than the seeded user, something no route can do.

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

### 11. Does routing run before authentication?

The check-order paragraph starts at 401, which reads as if authentication
is the first thing that happens. It is not: the 405 example in the same
document is "GET on a POST-only route such as /api/login", with no actor
mentioned, and the brief's own router test sends GET /api/login with no
X-User-ID and expects a 405.

What I did: method dispatch happens in the router, before any handler
runs, so a method mismatch is a 405 whether or not the caller is
authenticated. The contract's ordered list starts once a handler has the
request. Confidence: high, but the paragraph would be clearer if it said
so, since a port that answered 401 there would be reading the same words.

### 12. Does a 405 Allow header list HEAD?

The contract says HEAD and OPTIONS are registered on no route, and that
Allow lists "the methods that are registered for the path". Put together
that excludes both, but nothing states it outright, and the framework
default disagrees loudly.

Axum attaches an Allow header of its own to any method-mismatch response
whose handler did not set one, built from the methods in that path's
MethodRouter. Since a HEAD handler has to be registered to stop axum
routing HEAD to the GET handler (see below), axum's own header reads
"POST,HEAD" on /api/login. The handler sets Allow explicitly, which axum
then leaves alone, so the wire carries exactly one Allow listing only the
contract's methods.

This also means the brief's `method_mismatch_is_405_with_allow` cannot
fail: it asserts the header is present, and axum supplies one for free
with the wrong value. Review confirmed it and overrode the brief, so that
test is deleted rather than kept as coverage it does not provide.
`allow_lists_the_contract_methods_and_nothing_else` replaces it: it
compares the methods as a sorted set against every one of the fourteen
paths, and reads through `get_all` so a handler that appended rather than
replaced is caught too.

### 13. Does a bodiless HEAD 405 carry a Content-Type? Resolved: yes

The contract said every response that has a body is
`application/json; charset=utf-8`, and that the two 204s, which have no
body, send no Content-Type at all. A HEAD 405 also has no body, and the
contract did not say which of those two rules it followed.

What I did first: followed the 204 rule, and sent the status and Allow and
nothing else.

That was wrong, and review caught it by running all three apps: Go and
Node both send `application/json; charset=utf-8` on a HEAD 405, and
Content-Type is one of the two headers the cross-app equality check
compares, with `head health` a lifecycle step. So it was a live
divergence, the only one in twenty-eight steps.

Resolved in favour of Go and Node. RFC 9110 4.3.2 forbids content in a
HEAD response, but 9.3.2 keeps the header fields the equivalent GET would
have sent, and learning about a representation without transferring it is
the whole point of HEAD, so Content-Type stays and only the body goes. The
handler now builds the ordinary JSON 405, headers and all, and the HTTP
layer drops the body for HEAD. `contract/openapi.yaml` is amended to say
so in the prose block that already covers HEAD, so the next port does not
have to guess.

Two tests hold it: one in process, which pins the status, Allow and
Content-Type, and one over a real socket, which is the only place the
dropped body is observable, since the handler no longer drops it.

### 14. Which 400 message does a wrong-typed field get?

The contract pins a stable message for a required field that is missing,
null or trims to empty: "name required", "name and email required",
"reason required", "body required", "email required",
"role must be admin or member". It also says any other wrong type is a
400 and that the message there is not normative, because Go's is its JSON
decoder's own text.

What I did: value errors get the route's stable message, type errors get a
generic "invalid body". So `{"role": 7}` on PUT /api/members/{id}/role is
400 "invalid body" while `{"role": null}` is 400
"role must be admin or member", which is the split a struct-binding
decoder produces: null leaves the zero value and reaches the app's own
check, a number fails the decode. The text is "invalid body" with no
colon, so it cannot collide with the Go message the contract bars a check
from matching. Confidence: high on the status, which is all that binds.

### 15. Is X-User-ID trimmed?

The trimming rules are stated per route and per body field, and the two
exceptions are body fields too. The header is not mentioned anywhere.

What I did: matched exactly, like the login email, so nothing strips
whitespace from the header value before the lookup.

The illustration I first wrote for this, that `" u_ada "` is a 401, was
wrong and review caught it: a field value's leading and trailing
whitespace is optional whitespace that the transport strips, so a padded
header never arrives padded and all three apps answer 200. The decision is
therefore unobservable at the HTTP boundary for outer whitespace, and the
only reachable case is inner whitespace such as `"u_ ada"`, which is a 401
under any reading. Recorded because the code still makes a choice, not
because a conformance check could tell.

### 16. What answers a path outside /api?

The contract pins that the mount must not redirect and must not reach the
API for `/api` or `/API/me`, but not what those requests get instead. The
frontend mount is the only thing there, and this app's is a ServeDir that
the next task fills in.

What I did: `/api`, `/api/` and anything under `/api` reach the API's own
JSON 404. Outside `/api`, a request for a file that exists is served, and
anything else is the same `{"error": "not found"}` with the same
Content-Type. So `GET /API/me` is byte-identical to
`GET /api/no-such-route` rather than an empty static 404, which is the
strongest reading of the rule that the leak the contract closes stays
closed at the mount too. Confidence: medium. A port that let its static
server answer its own empty 404 there is not contradicted by anything
written down, and the two would differ on the wire.

Update: confirmed, and the contract has been amended to say so. The four apps
were measured above the mount and disagree (`GET /%2e` is 301, 301, 307 and
404), so the path rules are now scoped to paths under `/api` and only the
security half, that such a path must not reach the API, is pinned above it.

### 17. May a conformance run mutate the seeded fixture?

Not a hole in the app's behaviour but a hole in what the contract says
about running the suite. `Lifecycle()` approves and then revokes
`req_acme_1`, and asserts it "must start pending". Nothing resets the
store between runs, so a second run against the same process fails on that
assertion. I hit this: my first harness run passed, the second failed with
`expected: "pending", actual: "revoked"`.

What I did: no reset, and no reseed route. `seed()` runs once at startup,
because the contract declares eighteen routes and none of them is a reset.
Every harness run needs a freshly started app. Confidence: high on the
behaviour, and this is worth writing down because a port author who sees
that failure will reach for a state bug that is not there.

### 18. Does the duplicate-email question reach the handler?

Question 1 above is live in the login handler, which is the only caller of
`find_user_by_email`. I did not invent a second policy: the handler passes
the email through untrimmed and takes whichever user the store's total
order returns. The resolution stays in one place, which is also why the
store test added in task 7 is enough to cover it.

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

### The brief's `cargo add` line does not parse

`cargo add --dev tower@0.5 --features tower/util` fails on cargo 1.96.1
with "feature `util` activated for crate `tower` but the crate wasn't
specified". The `crate/feature` form names a different dependency's
feature, and there is no `tower` dependency for it to qualify. Used
`cargo add --dev tower@0.5 --features util`, which produces the intended
`tower = { version = "0.5", features = ["util"] }`.

### HEAD needs a handler, not just an absent one

Axum 0.7 dispatches HEAD to the `head` slot and then, if that is empty,
to the `get` slot: `call!(HEAD, head); call!(HEAD, get);` in
`method_routing.rs`. So `on(MethodFilter::GET, handler)` still answers a
HEAD with a 200 from the GET handler, which the contract forbids. Every
route registers HEAD explicitly against the 405 handler so the `head` slot
wins. Verified by mutation: deleting that registration turns
`head_on_a_get_route_is_405_with_the_headers_the_get_would_have_sent` red
and nothing else.

### The crate-wide `#![allow(dead_code)]` is gone

Task 4's blanket allow is deleted. It was hiding three warnings, all in
the store layer and all structural rather than accidental:
`StoreError::Forbidden` is never constructed, four trait methods
(`get_resource`, `get_request`, `list_organizations`, `get_organization`)
are never called by a handler, and `Data::organizations` is therefore
never read.

None of them is removable. The handlers must map `Forbidden`, the tenant
scoping test drives `get_resource` and `get_request` through all twelve
org-scoped methods, and the contract itself says no route returns
organizations while the seed declares them anyway. So they carry five
allows scoped to the item each one is about, one on the variant and one on
each of the four methods no route reaches, rather than a blanket allow at
the crate root or on the whole trait. A handler that stopped calling any
of the other thirteen store methods still warns.
`cargo build --release` is warning-free.

### Where the SPA lives

The brief says to serve `apps/rust/web/dist`, which is a repo-relative
path, and ServeDir resolves against the working directory. The binary is
normally started from the crate root, so `web/dist` is the default;
`apps/rust/web/dist` is used when that is the directory that exists, and
`STATIC_DIR` overrides both for a container that lays the files out
somewhere else. Nothing is created: the directory is the next task's.

## Contract versus brief

No disagreement about any field name, type or optionality. Everything in
`models.rs` comes from the response schemas, including `decommission_at`
(not `decommissioned_at`) and the four optional fields on `AccessRequest`.
The only brief-versus-repo conflict is the crate name above, and it is with
the clean guard rather than with the contract.

Of the three recent amendments, only `timestamp-precision` reaches the
store, and it costs nothing here. The HEAD-has-no-body clarification and
the unspecified `Allow` order are handler and router concerns for Task 5.
