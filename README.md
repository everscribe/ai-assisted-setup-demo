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

The plan is to grow this into four apps, one per language, so the setup
flow can be exercised against a target that is identical in every
dimension except language and framework. Today only the Go app exists.

## App matrix

| App | Backend | Frontend | Status |
|---|---|---|---|
| `apps/go` | Gin | React | available |
| `apps/node` | Express | Svelte | planned |
| `apps/rust` | Axum | vanilla TS | planned |
| `apps/python` | FastAPI | Vue | planned |

## Running an app locally

Requires Go 1.25+ and Node 18+ with `pnpm` (via `corepack enable`).

```bash
make dev-go   # build the SPA and run the Go server on :9001
```

Open <http://localhost:9001> and sign in as one of the seeded users. For
hot reload, run the Vite dev server against the API instead:

```bash
cd apps/go && go run .                # Go API on :9001
cd apps/go/web && pnpm run dev        # Vite dev server, /api proxied
```

### With Docker

No Go or Node toolchain needed, just Docker:

```bash
make up      # docker compose up --build -d
make down    # docker compose down
```

Then open <http://localhost:9001>.

## Pointing the setup flow at a single app

Both the hosted flow and the BYOK skill instrument one app at a time. Point
either one at the subdirectory for the app you want to instrument, for
example `apps/go`, rather than the repo root. Each app is a complete,
independently buildable project with its own manifest and lockfile.
