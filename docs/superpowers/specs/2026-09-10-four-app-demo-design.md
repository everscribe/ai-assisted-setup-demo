# Four-app demo: design

Date: 2026-09-10
Status: approved, not implemented

## Purpose

Extend `ai-assisted-setup-demo` from one Go app into four apps in four
languages with four frontend frameworks, so that Everscribe's AI-assisted
setup can be tested across every SDK we ship, repeatedly, against a target
that is identical in every dimension except language.

The value is comparability. If the four apps behave the same and the agent
produces materially different instrumentation for each, that difference is
attributable to the skill rather than to the app.

## The uninstrumented invariant

This repo's entire job is to be a clean target. On `main`:

- No Everscribe SDK appears in `go.mod`, `pyproject.toml`, `package.json`,
  or `Cargo.toml`, or in any lockfile.
- No `@everscribe/components-*` appears in any frontend, whether as a
  dependency or as a CDN `<script>` tag.
- No audit event is recorded anywhere.

The apps carry *seams* where instrumentation belongs (a store interface, a
plumbed context) and nothing more. Agent runs land on `everscribe/setup-*`
branches. `main` stays clean, always.

This invariant is enforced mechanically, not remembered. See "The clean
guard" below.

## App matrix

| App | Backend | SDK adapter it should attract | Frontend | Component it should attract |
|---|---|---|---|---|
| `apps/go` | Gin | `pkg/event/adapter_gin.go` | React | `@everscribe/components-react` |
| `apps/node` | Express | `src/express.ts` | Svelte | `@everscribe/components-element` |
| `apps/rust` | Axum | `src/axum.rs` | vanilla TS | `@everscribe/components-element` |
| `apps/python` | FastAPI | `everscribe/asgi.py` | Vue | `@everscribe/components-element` |

The right-hand columns describe what a successful instrument run should
produce. Nothing in them ships on `main`.

Three consequences worth stating plainly:

1. The Go app moves from stdlib `http.ServeMux` to Gin. This is a decision
   to test the most popular framework per language. It costs us coverage of
   the stdlib adapter, which is the hardest instrumentation case because
   `ServeMux` offers no middleware convention.
2. The first three frontends mirror what the docs already publish. Under
   `monorepo/internal/ui/content/docs/`, the `web-components/` directory
   ships `react.md`, `svelte.md`, and `vanilla.md`, and `examples/` ships
   twelve pages covering those three against all four backends. React,
   Svelte, and vanilla TS are the paths a customer can actually follow, so
   testing them is testing the documented product.
3. Vue is the deliberate stretch case: no wrapper package, no doc page, no
   example. `everscribe/components/packages/` holds only `core`, `element`,
   `react`, and `styles`, so Vue has nothing to reach for but the
   framework-agnostic custom element. Whether the agent works that out
   unaided is the most interesting question this repo can answer, and a
   failure there is a finding rather than a defect in the demo.

## Domain

Ported verbatim from today's Go app. It is already a security domain: an
internal access portal where organizations manage access to databases,
production environments, customer data, and third-party SaaS. Members
request access with a reason; admins approve, deny, or revoke.

Entities: `Organization` (the tenant), `User` (role admin or member),
`Resource` (with a `Sensitivity` of public, internal, confidential, or
restricted), `AccessRequest` (pending, approved, denied, revoked),
`Comment`.

Every mutating route is an audit-worthy security event, which is the point.

### Routes

Eighteen registrations per app, identical paths and methods across all four.

| Group | Method and path |
|---|---|
| Auth | `POST /api/login` |
| Auth | `POST /api/logout` |
| Auth | `GET /api/me` |
| Members | `GET /api/members` |
| Members | `POST /api/members` |
| Members | `DELETE /api/members/{id}` |
| Members | `PUT /api/members/{id}/role` |
| Resources | `GET /api/resources` |
| Resources | `POST /api/resources` |
| Resources | `POST /api/resources/{id}/decommission` |
| Access requests | `GET /api/resources/{id}/requests` |
| Access requests | `POST /api/resources/{id}/requests` |
| Access requests | `POST /api/requests/{id}/approve` |
| Access requests | `POST /api/requests/{id}/deny` |
| Access requests | `POST /api/requests/{id}/revoke` |
| Comments | `GET /api/requests/{id}/comments` |
| Comments | `POST /api/requests/{id}/comments` |
| Health | `GET /api/health` |

Two different 404s exist and the contract distinguishes them, because the
security layer would otherwise read a routing miss as a tenant-boundary
refusal:

- **Unknown path.** Any `/api/*` that matches no route returns
  `404 {"error":"not found"}`.
- **Cross-tenant or absent resource.** A route that matched but resolved
  nothing returns `404 {"error":"not found"}` with the same body, on
  purpose, so the boundary does not leak existence. The two are told apart
  by the request path, not the response.

Both are JSON. An empty body would not validate against any schema, which
would force a carve-out in layer 1 of the harness. Pinning this matters
because the frameworks disagree by default: FastAPI returns
`{"detail":"Not Found"}`, Axum returns an empty body, Express returns an
HTML error page. FastAPI's default is the dangerous one, since it looks
plausible enough to leave in place by accident.

Auth is deliberately minimal: the SPA posts an email to `/api/login`, then
attaches `X-User-ID` on every subsequent request. Real apps use sessions or
JWTs. The agent should work with either shape, and the simpler one keeps the
four ports honest.

## Repo layout

```
ai-assisted-setup-demo/
  apps/
    go/       go.mod, main.go, handlers/, store/, store/memory/, models/,
              web/, Dockerfile, README.md
    node/     package.json, src/, web/, Dockerfile, README.md
    rust/     Cargo.toml, src/, web/, Dockerfile, README.md
    python/   pyproject.toml, app/, web/, Dockerfile, README.md
  contract/
    openapi.yaml
    seed.json
    conformance/     go.mod, the runner
  docs/superpowers/specs/
  .github/workflows/ci.yml
  docker-compose.yml
  Makefile
  README.md
```

Each app is self-contained. `cd apps/rust && docker build .` works with no
reference to anything outside that directory, because that is what we point
the hosted instrument flow at.

Ports are fixed and distinct so all four run at once: Go 9001, Node 9002,
Rust 9003, Python 9004. Each server reads `PORT` and falls back to its
default, as today's `main.go` already does, since compose sets it.

### The Go module moves

Today `go.mod` sits at the repo root with module path
`github.com/everscribe/ai-assisted-setup-demo`. It becomes `apps/go/go.mod`
with module path `github.com/everscribe/ai-assisted-setup-demo/apps/go`, and
the repo root carries no `go.mod`.

The `go` directive moves from 1.22 to 1.25, matching `sdk-go`'s minimum.
Below that, an agent that adds the SDK gets a toolchain resolution error
rather than an instrumented app, which would read as a skill failure when it
is a repo defect.

Every import inside the Go app changes. This is mechanical, but it touches
every file and it strands the existing `everscribe/setup-*` branches (13 at
time of writing), which are disposable agent output.

## Store abstraction

Handlers depend on a `Store` interface, never on a concrete implementation.
The only implementation is in-memory, but the shape is the shape a real
database-backed app would have, so that the agent sees a realistic target.

There is no database. Not Postgres, not SQLite. Four in-memory stores seeded
from the same declared fixture start identical every time, conformance runs
get a clean slate on every restart, and compose stays four services rather
than eight. A database would add a variable that has nothing to do with what
is being tested.

Two rules give the interface its DB-shaped character:

**Every method takes a context and returns an error.** Real repository calls
carry a context, and it matters beyond cosmetics: the SDK recorders take a
context, so a handler that already has one plumbed is what a real
instrumentation target looks like.

**Mutating methods return the updated entity.** This is the `RETURNING`
pattern the Everscribe monorepo already prescribes, so the demo matches
house style, and it gives an audit event something to describe.

```go
type Store interface {
	GetUser(ctx context.Context, id string) (*models.User, error)
	ListOrgResources(ctx context.Context, orgID string) ([]*models.Resource, error)
	ApproveRequest(ctx context.Context, orgID, reqID, approvedBy string) (*models.AccessRequest, error)
	// ...
}
```

Translation across the four is two separate things, and conflating them is
the mistake to avoid. The store interface translates directly. The *context*
does not: only Go threads it as a parameter, and the other three SDKs use an
ambient mechanism that an implementer must not fake with an invented `ctx`
argument.

| App | Store interface | How the SDK actually threads context |
|---|---|---|
| Go | `interface` | `context.Context` as first argument |
| Python | `Protocol` | `contextvars`, set by the ASGI middleware (`sdk-python/src/everscribe/event/context.py`) |
| Node | `interface` | `AsyncLocalStorage` |
| Rust | `#[async_trait] trait Store: Send + Sync`, held as `Arc<dyn Store>` | request extensions (`sdk-rust/src/event/context.rs`) |

So the Go app plumbs `ctx` explicitly; the other three plumb the framework's
own request-scoped carrier (FastAPI's request, Express's `req`, axum's
`Extension`) and let the ambient mechanism do the rest.

Rust needs `#[async_trait]`. An `async fn` in a plain trait is not
dyn-compatible, so `Arc<dyn Store>` will not compile without it or a
hand-rolled `Pin<Box<dyn Future>>`.

### What the interface deliberately does not do

Domain methods are not split into `Get` plus `Update` to force a
read-before-write. That would look more database-like, but it drags tenant
scoping out of the store and into handlers, where it would stop being
enforced consistently. The conformance suite's cross-tenant checks are
exactly what would quietly weaken.

Tenant scoping stays in the store. Every method that touches a tenant-owned
row takes `orgID` and returns `ErrNotFound` rather than `ErrForbidden` on a
cross-tenant miss, so the boundary does not leak existence.

## The contract

`contract/openapi.yaml` is the written source of truth for all eighteen
routes: request bodies, response shapes, status codes.

`contract/seed.json` declares the expected seeded state: two organizations,
four users (two admins, two members), five resources, and access requests
spanning all four states, with fixed IDs so the frontends' quick-pick login
is stable, and with fixed absolute timestamps.

Absolute matters. Today's `store/seed.go` builds every timestamp from
`time.Now()` offsets, which would leave the harness unable to assert seeded
`created_at` values at all: a hole at exactly the field most likely to
differ across ports. Pinning them also makes seeded ordering deterministic,
so only rows created during a run need the tiebreak described under
Normalization.

**Seed data is duplicated on purpose.** Each app hardcodes its own seed
idiomatically, the way `store/seed.go` does today, rather than reading
`seed.json` at runtime. Reading a shared fixture would guarantee parity but
would look nothing like a real application, and looking like a real
application is this repo's only job. The conformance runner closes the gap
by asserting that what each app serves matches `seed.json`.

## Conformance harness

Written in Go, in `contract/conformance/` with its own `go.mod`. It is a
pure HTTP client with no handlers of its own, so it should be invisible to
the instrument agent.

`make verify` brings up whichever apps are present in `docker-compose.yml`,
runs the harness against them, and tears them down. Cross-app equality is
skipped when fewer than two apps are up, which is the state Plans 1 and 2
leave the repo in.

Three layers of assertion:

1. **Schema conformance.** Every response validates against its declared
   schema in `openapi.yaml`. Catches a missing field or a wrong type in one
   app.
2. **Cross-app equality.** One scripted sequence of roughly twenty requests
   covering the full lifecycle (login, list resources, request access,
   comment, approve, revoke) is replayed against all four apps. Normalized
   responses must match across all four. The comparison is symmetric: the
   four are compared to each other rather than to a designated reference, so
   no app's runtime output is privileged and a single app's drift is caught
   wherever it appears.

   Go does still shape the contract, since `openapi.yaml` is written from
   the Go app's behaviour and the three ports are written by reading Go. The
   equality layer catches divergence between apps; it cannot catch a mistake
   the contract itself encodes. Layers 1 and 3 are the defence against
   that.
3. **Security invariants**, asserted explicitly rather than by comparison,
   because these must be right rather than merely consistent:
   - cross-tenant reads return 404, each one paired with a positive control:
     the same route and method with a same-tenant id must return 200 first
   - member writes to admin-only routes return 403
   - a missing `X-User-ID` returns 401 on every route except
     `POST /api/login` and `GET /api/health`, which are anonymous by
     design
   - a denied request cannot transition to approved

### Normalization

Generated IDs and timestamps differ per app and per run, so cross-app
comparison needs normalization: generated IDs rewritten to `$id1`, `$id2`
in order of first appearance, timestamps replaced by a placeholder once
checked for plausible ordering.

Ordering is the other half, and it is where cross-language comparison
actually breaks. Every list endpoint declares a total order in
`openapi.yaml`: the domain sort key, then `id` ascending as tiebreak.

**The domain sort key must be stable across apps, and `id` alone is not.**
`GET /api/resources` and `GET /api/members` originally declared `id asc`.
That is not a total order across independently running apps, because
generated ids are a fixed prefix plus random hex: a new resource gets
`r_<hex>`, which sorts either side of the seeded `r_acme_*` rows depending
on its suffix. Two conformant apps each creating one resource were observed
returning `[r_17be, r_acme, r_acme, r_acme]` and
`[r_acme, r_acme, r_acme, r_ed5d]`.

The split is not even. A generated id's first hex character decides against
`acme`: `0` to `9` sorts before (10 in 16), `b` to `f` after (5 in 16), and
`a` resolves on the next character, which sorts before in 13 cases out of
16. So a new row sorts before the seeded block with probability about 0.676,
and all four apps agreeing is 0.676 to the fourth plus 0.324 to the fourth,
roughly one chance in five. The affected steps therefore fail about four
times in five. A generated id can never land BETWEEN the seeded rows, since
that would require the non-hex characters `m` and `_`, so the outcome really
is binary.

**Both endpoints must therefore declare `created_at asc, id asc`. This is
pending work, not current state: it is Task 0 of Plan 3.** The edit touches
three places, the `x-ordering` block in `contract/openapi.yaml` and the
prose describing each endpoint. Once made, every seeded row shares
`2026-03-13T12:00:00Z`, so seeded rows order by their fixed ids
deterministically, and every runtime row carries a later timestamp so it
sorts last, in creation order, identically in every app.

Without the tiebreak, equal timestamps order arbitrarily, and Go diverges
from the other three by construction: `sort.Slice` is unstable, while
JavaScript's `Array.prototype.sort`, Python's `sorted`, and Rust's `sort_by`
are all stable. Today's `store.go:265` sorts `CreatedAt` descending and
`store.go:381` ascending, neither with a tiebreak, over 6-byte random hex
IDs. Ties are likely rather than theoretical: the conformance sequence posts
comments back to back, and JavaScript's `Date.now()` has millisecond
resolution. Apps use a stable sort (`sort.SliceStable` in Go) as belt and
braces.

The normalizer is the one piece of real logic in the harness and the one
place where a bug would produce false confidence. It gets its own unit
tests.

### What the harness deliberately does not check

The harness cannot catch every app being wrong the same way. Only cross-app
equality polices behaviour, and equality by construction sees disagreement
rather than shared error.

For most of these rules the gap is wider than that, and worse. The lifecycle
sequence uses only GET and POST, sends no trailing slash, and sends no
malformed or padded body, so it never exercises them at all. That means even
a SINGLE port diverging on its own goes unnoticed: a from-scratch Rust or
Python app can get trailing-slash handling wrong, disagree with both shipped
apps, and still ship green. Of the seven rules below only the `Allow` header
is reachable today, through the one method-mismatch step.

Seven documented contract rules were each violated in a
scratch app with the whole suite passing: the `Allow` header, HEAD and
OPTIONS routing, trailing-slash strictness, input trimming, check precedence,
the stable 400 message text, and role coercion.

A machine-readable behaviour table in the contract, iterated by the harness,
would close that. It was designed and rejected. These are demo apps whose
only job is to be a realistic target for the hosted instrument flow and a
repo someone can play with; they never run in production and have no
consumer that depends on their API hygiene. If all four apps omit the `Allow`
header identically, nothing breaks and the instrument agent does not care.
Thirty table rows enforcing rules nobody exercises is machinery for a
correctness problem this repo does not have.

What matters here is only that the four apps AGREE, so that a divergence in
instrumentation results is attributable to the skill rather than to the apps.

The cheap way to get that is steps, not headers. `Lifecycle()` is a literal
slice with no state threading and `Response` already carries `Header`, so six
added steps plus a header comparison make equality actually police six of the
seven: HEAD and OPTIONS routing, trailing-slash strictness on both a real and
an unknown path, check precedence and the stable 400 text via an unparseable
body, the not-trimmed `role` rule, and `Allow`. All six are non-mutating in
effect, since four are reads and the last two are rejected before reaching
the store, so no seed exemption is needed. Only accepted-input trimming is
left over, because exercising it creates a row and would need to sit after
the existing creation steps to keep the later list comparisons stable.

Six lines of table plus a header whitelist is cheaper than the thirty-row
behaviour table that was rejected, and closes more than the header change
alone would. That is Plan 4's Task 0.

An accepted known divergence, recorded rather than fixed: `/api/%6de`
returns a user record on Go and 404 on Node, because Gin reads `URL.Path`
with `UseRawPath` off while Express matches the raw path. This is NOT another
instance of the unchecked-rule argument above; it is a verified current
disagreement on the one property this section says matters. It is waived
because nobody will type that at a demo. Note it will grow rather than stay
at two apps, since Rust and Python will each land on one side unpredictably.

### The harness survives instrumentation

The harness asserts app behaviour and knows nothing about Everscribe, so it
passes identically before and after an instrument run. Pointed at an agent's
PR branch, `make verify` answers "did the agent change how this app
behaves?", which is a question we currently have no way to answer.

## Build and run

Per app: a multi-stage Dockerfile (build the SPA, build the server, thin
runtime) and a short README.

**Serving the SPA.** Every server serves its built frontend itself, so one
process on one port is the whole app. Go keeps `go:embed` on `web/dist` as
it does today, including the checked-in `web/dist/index.html` placeholder
that lets `go build` succeed before the SPA is ever built. The `.gitignore`
negation that preserves it moves with the app; if the paths are rewritten to
`apps/go/web/dist/*` without the matching `!apps/go/web/dist/index.html`,
`apps/go` stops compiling on a fresh clone, which breaks the harness and
every agent run.

The other three serve the build output from disk, which the
Dockerfile stages into the runtime image.

All four frontends are Vite-based, so every one of them builds to
`web/dist` and the four Dockerfiles stay structurally identical. This is a
reason Angular was not chosen: the Angular CLI writes
`web/dist/<project>/browser`, which would have made one app's static path a
special case for no gain in what is being measured.

Svelte here means Svelte 4 plus Vite, not SvelteKit, matching what
`monorepo/internal/ui/content/docs/examples/svelte-node.md` publishes.
SvelteKit with `adapter-static` writes `build/` rather than `web/dist`,
which would reintroduce exactly the per-app static path this uniformity
exists to avoid.

Root `Makefile`:

| Target | Effect |
|---|---|
| `make up` | compose up whichever apps are defined |
| `make down` | compose down |
| `make verify` | up, run conformance, down |
| `make check-clean` | fail if any Everscribe dependency is present |
| `make dev-go`, `make dev-python`, `make dev-node`, `make dev-rust` | hot reload one app |

Compose exists to run the four app processes. It introduces no databases and
no other services. It also means trying the demo needs Docker alone, not four
language toolchains, which matters if the repo is ever handed to a prospect.

## The clean guard

`make check-clean` fails if an Everscribe *dependency* appears in any
manifest or lockfile. It matches exact dependency identifiers, never a bare
`everscribe`, because the Go module path is `github.com/everscribe/...` and a
fuzzy grep would fail on a clean `main` forever.

| Ecosystem | Banned identifier | Files scanned |
|---|---|---|
| Go | `github.com/everscribe/sdk-go` | `go.mod`, `go.sum` |
| Node | `@everscribe/sdk-node` | `package.json`, `pnpm-lock.yaml` |
| Frontends | `@everscribe/components-` (core, element, react, styles) | `web/package.json` and lockfile, `web/index.html`, `web/dist/index.html`, `web/src/**` |
| Python | a quoted `everscribe...` in dependencies | `requirements.txt`, `requirements-dev.txt` |
| Rust | `everscribe =` under `[dependencies]` | `Cargo.toml`, `Cargo.lock` |

Lock tooling is pinned so the lockfile names are knowable: `pnpm` for Node,
matching `sdk-node` and `components`, which already use it. `pip` with
`requirements.txt` for Python.

The only requirement the guard places on dependency tooling is that the
filename be knowable before the app exists, and `requirements.txt` satisfies
that as well as a lockfile would. This spec originally picked `uv` on the
grounds that `sdk-python` has no lockfile today and builds with hatchling, so
there was no existing convention to match. That argument decided nothing:
with no convention either way, both options met the guard's requirement, and
the choice came down to whether the demo should carry a tool the machine does
not already have. It should not. The owner chose `pip`, and the tradeoff
being accepted is that `requirements.txt` pins direct dependencies rather
than locking the full transitive graph with hashes. For a demo that is
cloned and run rather than deployed, that costs nothing.

The shipped guard already scans `requirements.txt` and `requirements-dev.txt`
alongside the lockfile names, so this decision needs no Makefile change. The guard scans npm and yarn
lockfiles anyway, because the demo's frontend is npm today and a stale
`package-lock.json` surviving the move is the likeliest way a dependency
slips past.

```make
check-clean:
	@files=""; \
	for f in apps/*/go.mod apps/*/go.sum \
	         apps/*/package.json apps/*/pnpm-lock.yaml apps/*/package-lock.json \
	         apps/*/web/package.json apps/*/web/pnpm-lock.yaml \
	         apps/*/web/package-lock.json apps/*/web/yarn.lock \
	         apps/*/Cargo.toml apps/*/Cargo.lock apps/*/pyproject.toml apps/*/uv.lock \
	         apps/*/requirements.txt apps/*/requirements-dev.txt \
	         apps/*/web/index.html apps/*/web/dist/index.html; do \
	  [ -f "$$f" ] && files="$$files $$f"; \
	done; \
	hits=""; [ -n "$$files" ] && hits=$$(grep -nE '@everscribe/(sdk-node|components-)|everscribe/sdk-go|"everscribe[^"]*"|^[[:space:]]*everscribe[[:space:]]*=' $$files 2>/dev/null); \
	src=$$(grep -rnE '@everscribe/components-' apps/*/web/src 2>/dev/null); \
	if [ -n "$$hits$$src" ]; then \
	  echo "everscribe dependency on main:"; echo "$$hits"; echo "$$src"; exit 1; \
	fi
```

**Build the file list before matching.** This is not style. `grep` exits 2
when any file argument is unreadable, and `! grep` inverts 2 to 0, so a
guard written as `! grep ... || exit 1` prints the violation and then reports
clean. Missing files are the normal state rather than an edge case: there is
no `uv.lock` until Python lands, no `Cargo.lock` until Rust lands, and no
`apps/*/go.sum` at all until the Go app takes its first dependency. During
Plans 1 and 2, when only `apps/go` exists, nearly every glob is unmatched.

**Two caveats.** The `"everscribe[^"]*"` pattern would also match an app
literally named `everscribe-something`, so the four apps must not be named
that way. And the frontend pattern is `@everscribe/components-`, matching the
stated rule, not `components-element`: the Go app's React frontend is meant
to attract `components-react`, which a narrower pattern would wave through.

Scanning `web/dist/index.html` matters because that file is checked into
source control by design, so a CDN `<script>` tag can reach `main` there
without touching any manifest.

It runs in CI on `main`, in a workflow this repo does not have yet. See
Plan 1.

About a dozen lines of shell, and it makes the repo's core invariant
mechanical.
Without it the failure mode is silent: a stray dependency merges to `main`,
and from then on every instrument test starts from a contaminated baseline
with nobody aware of it.

## Implementation shape

This design is too large for one implementation plan. It splits into five,
each independently landable and each leaving the repo in a working state:

**Plan 1: restructure and reference app.** Move the Go app to `apps/go`,
port it from stdlib to Gin, introduce the `Store` interface and
`store/memory`, plumb context. Write `contract/openapi.yaml` and
`contract/seed.json` from the resulting behaviour. Add the root `Makefile`,
compose file, `make check-clean`, and `.github/workflows/ci.yml` running it.
At the end, one app works and the contract is written down.

Moving `web/` to `apps/go/web/` includes migrating it from npm to pnpm and
deleting `package-lock.json`. Today the demo runs `npm install` and checks in
a 58KB `web/package-lock.json`, so without this step the pin above is true
of the spec and false of the tree.

`make check-clean` is not done when it is written. It is done when it has
been run against a tree with a planted violation in each of the five
ecosystems and observed to exit non-zero, and against a clean tree and
observed to exit zero. A guard that reports clean on a dirty tree is worse
than no guard, because it gets trusted.

**Plan 2: conformance harness.** Build the runner and the normalizer against
the single Go app. It cannot yet compare across apps, so it runs the schema
and security layers only. This lands before the new backends on purpose:
building the harness first means each new app has an acceptance test waiting
for it, rather than being declared correct by inspection.

**Plan 3: Task 0 plus the Node app.** Task 0 is four corrections and a
verification, all of which must land before any port is written, because
three ports get written from the contract and a contract change is cheap
only until then: the ordering
redeclaration above; cross-tenant controls for `DELETE /api/members/{id}`
and `PUT /api/members/{id}/role`, without which an acme admin can delete a
globex member and promote them with the suite green; a precondition guard
asserting that the two rows the seed test exempts start unmutated, whose
absence makes those exemptions exploitable (exact statuses on the lifecycle
steps themselves would not work, because the lifecycle is replayed twice in
one run and approve and revoke are not idempotent); the readiness
loop gaining ports 9002 to 9004; and, fifth, proving the security layer
fails.

That fifth item generalises the rule this spec already applies to
`make check-clean`. Two of the four corrections above are the same bug: an
assertion table that silently omits routes, where a green suite told nobody
the other routes were missing. The security layer has had no
demonstrate-it-fails requirement, and it is about to become the acceptance
test three from-scratch ports are judged by. A hole in it does not fail
loudly, it passes quietly in all three ports at once, and layer 2 cannot
catch that because equality only compares apps to each other. So: for each
security invariant, plant the corresponding defect in `apps/go` and confirm
the suite goes red. Drop the org filter from `RemoveMember` and from
`ChangeMemberRole`, make `ApproveRequest` a no-op, make
`DecommissionResource` a no-op, and remove the `X-User-ID` check from one
route. Revert each after observing the failure. This check would have found
Task 0 items 2 and 3 during Plan 2 rather than during Plan 3 triage.

Then Express plus Svelte on 9002, which switches on the cross-app equality
layer for the first time.

**Plan 4: a three-item Task 0, then Rust and vanilla TS** on 9003. Task 0
adds six steps to `Lifecycle()` exercising HEAD, OPTIONS, a trailing slash on
both a real and an unknown path, an unparseable body, and an untrimmed
`role`, because the sequence uses only GET and POST today and so never
reaches six of the seven rules named above; it adds `Content-Type` and
`Allow` to the equality comparison, which sees status and body only, so every
header divergence is currently invisible; and it fixes
`TestDecommissionIsIdempotentAndRestampsTheTimestamp`, which requires two
timestamps 25ms apart to differ even though nothing pins sub-second
precision, so a conformant second-resolution port fails it for being correct.

The six steps come first in that list because they are what turns "the four
apps agree" from an assertion into something the suite checks.

**Plan 5: Python on 9004, paired
with Vue**, the undocumented frontend and the stretch case, last on purpose.
FastAPI itself is documented; `examples/{react,svelte,vanilla}-python.md`
all exist. Vue is the uncharted half, and saying so keeps a Plan 5 failure
from being ambiguous between "the agent cannot do Vue" and "the agent cannot
do Python".

One app per plan, not one app per task. Each app is roughly 1800 lines of
backend and 1600 of frontend: 1000 lines of TypeScript plus a 611-line
stylesheet that each app carries its own copy of, since the
self-containment rule above forbids sharing it. A 3400-line task has no
useful review granularity, and this design's first two plans needed a fix
round on almost every task.

**The ports are written from the contract, not from the Go app.** An
implementer is given `apps/go`'s STRUCTURE, meaning the store interface in
`store/store.go` and the file layout, and takes every BEHAVIOURAL question
from `contract/openapi.yaml` and `contract/seed.json`. Note that
error-to-status mapping is behaviour, not structure, and lives in the
contract; it is not among the things an implementer may look up.

This is the only real test of whether the contract is sufficient to build
against. Letting an implementer read the Go handlers guarantees a
conforming app and proves nothing; a fix round caused by a hole in the
contract is the most valuable output of Plan 3, and it is far cheaper to
find with one port written than with three.

**The rule needs a mechanism or it will not survive the first ambiguity**,
since the implementer has `apps/go/handlers/` in the same checkout and the
tempting moment to read it is exactly the moment a contract hole would
otherwise surface. Removing those directories does not work: the equality
layer needs the Go app running on 9001 to compare against, and deleting
`handlers/` and `store/memory/` breaks its build, which was verified rather
than assumed. So the mechanism is a log. Every question the implementer
could not answer from `openapi.yaml` and `seed.json` is recorded, with what
it did instead,
and that log is a deliverable of Plans 3, 4 and 5 alike. A non-empty Plan 3
log is the list of contract fixes that must land before Plan 4; Plans 4 and
5 are where a corrected contract either holds or does not, so their logs are
the evidence that it did. Discarding the requirement after Plan 3 would
throw away the confirmation at the moment it becomes available.

**An empty log is only meaningful as an explicit claim**, because an absence
is produced equally by a sufficient contract, by an implementer who read the
Go handlers at the ambiguous moment, and by an implementer who simply did
not keep the log. Those are the three outcomes the mechanism most needs to
tell apart. So the implementer states, as part of the deliverable, that it
did not open `apps/go/handlers/` or `apps/go/store/memory/`. An empty log
with no such statement is treated as a log that was not kept.

The statement is the load-bearing half; do not ask for a provenance table
covering every behaviour. Across eighteen routes, their shapes, statuses,
ordering and error mapping, that runs to a hundred entries, which is large
enough to be done perfunctorily, and a perfunctory table cannot be told
from a real one. That is the same weakness this paragraph exists to close.
The one targeted exception is error-to-status mapping, which the structure
rule above singles out as the behaviour an implementer is most tempted to
look up: record where that one came from.

There is a harder variant if that ever proves insufficient. Build the Go
image once while the source is present (`docker compose build go`), then run
the port work in a worktree whose compose file says `image: demo-go:frozen`
rather than `build: ./apps/go`, with `handlers/` and `store/memory/` absent.
The harness discovers apps over HTTP and never reads their source, so a
prebuilt image serves 9001 perfectly well; the objection above is to
deleting those directories in place, not to the approach. It costs a compose
variant and a stale-image hazard, which is probably not worth it while the
log plus an explicit statement gets most of the value.

The ordering is deliberate. Writing the contract before the ports means the
ports have a target; building the harness before the ports means they have a
judge; and correcting the contract before the first port means the
correction costs one edit rather than four.

## Out of scope

**Event-diff harness (phase 2).** Once the apps are instrumented, pointing
all four at a recording stub inside the harness turns the same request
sequence into a cross-language diff of emitted audit events. That is the
thing we ultimately want to learn from an n>1 test. It is deliberately not
built now: designing it against events that do not exist yet would be
guessing.

When it is built, note that `contract/conformance/go.mod` will then require
`sdk-go`. The clean guard scopes to `apps/` deliberately, so this is already
fine; do not widen it to the repo root in response.

**A second app per language.** Full adapter coverage per language would mean
eight backends. The mainstream framework per language covers the common case,
which is the case that matters at launch. Revisit if the skill proves solid
on the common path.

**Anything resembling real auth.** The `X-User-ID` header stays.

## Known follow-ups

- The git remote still points at `github.com/everscribe/ai-setup-example.git`
  while the module and directory are `ai-assisted-setup-demo`. GitHub
  redirects, so it works, but the remote should be updated.
- The `everscribe/setup-*` branches (13 at time of writing) predate this
  restructure and will
  not merge cleanly afterwards. They are disposable agent output; delete
  them when the restructure lands.
