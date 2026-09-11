# contract/conformance

An HTTP-only test harness that checks whether a running demo app conforms
to `contract/openapi.yaml`. It imports nothing from `apps/`; it only makes
requests and inspects responses, so it works the same way against a Go
app today and against Node, Rust, and Python apps once they exist.

## The three layers

1. **Schema validation.** Every request the harness makes goes through
   `validated()`, which checks the response against the OpenAPI spec.
   This runs for every test in the package, not just the ones in this
   file, so layer 1 covers the whole surface.
2. **Cross-app equality** (`equality_test.go`). Replays the same scripted
   lifecycle (`sequence.go`) against every running app, normalizes each
   app's responses (seeded ids kept, generated ids and timestamps
   rewritten to placeholders in order of first appearance), and asserts
   the apps produced the same normalized output at every step. The apps
   are compared to each other, not to a designated reference, so no
   app's behavior is privileged. With fewer than two apps running, this
   test skips rather than passing or failing, which is the state Plans
   1 and 2 leave the repo in.
3. **Security invariants** (`security_test.go`). Missing-actor 401s,
   cross-tenant 404s, admin-only routes, and terminal-state conflicts
   are asserted directly against each app rather than compared between
   apps. Equality can't catch a security defect that every app shares:
   if all four apps leaked the same cross-tenant row, layer 2 would
   call that agreement, not a bug. Layer 3 checks each app against a
   fixed rule instead. `seed_test.go` belongs to this layer too: each
   app carries its own hand-written copy of `contract/seed.json`, and
   a value wrong the same way in every app would pass layer 2, so the
   served rows are compared to the seed file field for field.

## Running it

```bash
make verify
```

Brings the app(s) up with `docker compose up --build -d`, waits for
`/api/health`, runs `go test ./...` in this directory, tears compose
down, and exits with the test run's status, not the teardown's. A
failing test still produces a non-zero exit even though `docker compose
down` itself succeeds.

While iterating, skip the rebuild-and-teardown cycle:

```bash
make up
cd contract/conformance && go test ./... -count=1 -v
cd ../.. && make down
```

These tests share mutable app state (the lifecycle mutates data), so
they don't use `t.Parallel()`, and a fresh `make up` is what makes a
run reproducible; running the suite twice against the same container
without restarting it can pick up state left over from the first run.

## `CONFORMANCE_APPS`

The default roster is the four ports in `docker-compose.yml` (`go` on
`9001`, `node` on `9002`, `rust` on `9003`, `python` on `9004`).
Override it to point at a different set:

```bash
CONFORMANCE_APPS="go=http://localhost:9001,node=http://localhost:9002" go test ./...
```

Only apps that answer `/api/health` are included; anything else in the
roster is skipped silently, not failed.

## No apps running

Every test that needs a live app calls the shared `apps(t)` helper,
which skips with a clear message when nothing answers `/api/health`.
`go test ./...` is green on a machine with nothing running; it is not
proof that anything conforms, only that nothing was checked.

## `CONFORMANCE_REQUIRE_APPS`

Set it (`make verify` sets it to `1`) and `apps(t)` fails instead of
skipping when nothing answers `/api/health`. That is the belt to the
readiness loop's braces: in CI an app that builds but crashes at boot
would otherwise skip every live test and exit 0, which is a guard
reporting clean having checked nothing. Leave it unset locally so
`go test ./...` stays green with nothing running.

## What this harness does not know

It has no idea what Everscribe is. It reads the OpenAPI contract and the
seed fixtures, sends HTTP requests, and compares responses. That
ignorance is the point: because it is entirely orthogonal to
instrumentation, it passes identically before and after an AI agent
wires up the Everscribe SDK on one of these apps. That is what makes it
useful for answering "did the agent change how this app behaves", not
just "did the agent add the SDK".
