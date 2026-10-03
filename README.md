# ai-assisted-setup-demo

This is a sample repo you can use to get a feel for how you can use AI to
set up Everscribe for a codebase, either using the
[Everscribe hosted AI agent](https://everscribe.io/docs/quickstart/hosted-ui)
or the [BYOK flow](https://everscribe.io/docs/quickstart/skill).

## Deliberately uninstrumented

This repo's entire job is to be a clean target for that setup flow. On
`main`, no Everscribe SDK appears in any manifest or lockfile, no
`@everscribe/components-*` appears in any frontend, and no audit event is
recorded anywhere. Each app carries the seams instrumentation belongs at (a
store interface, a plumbed context) and nothing more, so the AI agent has
somewhere to attach without the demo pre-empting its work. Agent runs land
on their own branches; `main` stays clean.

This is four apps, one per language, so the setup flow can be exercised
against a target that is identical in every dimension except language
and framework. All four implement one shared HTTP contract
(`contract/openapi.yaml`), and a conformance harness compares them to
each other over HTTP.

## App matrix

| App | Backend | Frontend | Status |
|---|---|---|---|
| `apps/go` | Gin | React | available |
| `apps/node` | Express | Svelte | available |
| `apps/rust` | Axum | vanilla TS | available |
| `apps/python` | FastAPI | Vue | available |

## Running an app locally

Requires Go 1.25+, Node 18+ with `pnpm` (via `corepack enable`), Rust
1.96+, and Python 3.12+.

```bash
make dev-go      # build the SPA and run the Go server on :9001
make dev-node    # build the SPA and run the Node server on :9002
make dev-rust    # build the SPA and run the Rust server on :9003
make dev-python  # build the SPA and run the Python server on :9004
```

Open <http://localhost:9001> (Go), <http://localhost:9002> (Node),
<http://localhost:9003> (Rust) or <http://localhost:9004> (Python) and
sign in as one of the seeded users. For hot reload, run the Vite dev
server against the API instead:

```bash
cd apps/go && go run .                # Go API on :9001
cd apps/go/web && pnpm run dev        # Vite dev server, /api proxied

cd apps/node && pnpm run dev          # Node API on :9002
cd apps/node/web && pnpm run dev      # Vite dev server, /api proxied

cd apps/rust && cargo run             # Rust API on :9003
cd apps/rust/web && pnpm run dev      # Vite dev server, /api proxied

cd apps/python && .venv/bin/python -m app.main   # Python API on :9004
cd apps/python/web && pnpm run dev               # Vite dev server, /api proxied
```

### With Docker

No Go, Node, Rust or Python toolchain needed, just Docker:

```bash
make up      # docker compose up --build -d
make down    # docker compose down
make verify  # bring all four up, run the conformance suite, tear down
```

Then open <http://localhost:9001> (Go), <http://localhost:9002> (Node),
<http://localhost:9003> (Rust) or <http://localhost:9004> (Python).
