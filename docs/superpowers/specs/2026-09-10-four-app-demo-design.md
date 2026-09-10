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
2. The first three frontends mirror what the docs already publish. The
   monorepo ships `web-components/{react,svelte,vanilla}.md` and twelve
   example pages covering `{react,svelte,vanilla}` against all four
   backends, so React, Svelte, and vanilla TS are the paths a customer can
   actually follow. Testing them is testing the documented product.
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
as today's `main.go` already does, since compose sets it.

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
   - cross-tenant reads return 404
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
`docs/examples/svelte-node.md` publishes. SvelteKit with `adapter-static`
writes `build/` rather than `web/dist`, which would reintroduce exactly the
per-app static path this uniformity exists to avoid.

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
| Python | a quoted `everscribe...` in dependencies | `pyproject.toml`, `uv.lock` |
| Rust | `everscribe =` under `[dependencies]` | `Cargo.toml`, `Cargo.lock` |

Lock tooling is pinned so the lockfile names are knowable: `uv` for Python,
`pnpm` for Node, matching what `sdk-node` and `components` already use.

```make
check-clean:
	@files=""; \
	for f in apps/*/go.mod apps/*/go.sum apps/*/package.json apps/*/pnpm-lock.yaml \
	         apps/*/web/package.json apps/*/web/pnpm-lock.yaml \
	         apps/*/Cargo.toml apps/*/Cargo.lock apps/*/pyproject.toml apps/*/uv.lock \
	         apps/*/web/index.html apps/*/web/dist/index.html; do \
	  [ -f "$$f" ] && files="$$files $$f"; \
	done; \
	hits=$$(grep -nE '@everscribe/(sdk-node|components-)|everscribe/sdk-go|"everscribe[^"]*"|^[[:space:]]*everscribe[[:space:]]*=' $$files 2>/dev/null); \
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

This design is too large for one implementation plan. It splits into three,
each independently landable and each leaving the repo in a working state:

**Plan 1: restructure and reference app.** Move the Go app to `apps/go`,
port it from stdlib to Gin, introduce the `Store` interface and
`store/memory`, plumb context. Write `contract/openapi.yaml` and
`contract/seed.json` from the resulting behaviour. Add the root `Makefile`,
compose file, `make check-clean`, and `.github/workflows/ci.yml` running it.
At the end, one app works and the contract is written down.

`make check-clean` is not done when it is written. It is done when it has
been run against a tree with a planted violation in each of the five
ecosystems and observed to exit non-zero, and against a clean tree and
observed to exit zero. A guard that reports clean on a dirty tree is worse
than no guard, because it gets trusted.
contract is written down.

**Plan 2: conformance harness.** Build the runner and the normalizer against
the single Go app. It cannot yet compare across apps, so it runs the schema
and security layers only. This lands before the new backends on purpose:
building the harness first means each new app has an acceptance test waiting
for it, rather than being declared correct by inspection.

**Plan 3: the three new apps.** Python, Node, Rust, backend and frontend
each, one app per unit of work. Cross-app equality switches on as the second
app lands.

The ordering is deliberate. Writing the contract before the ports means the
ports have a target; building the harness before the ports means they have a
judge.

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
