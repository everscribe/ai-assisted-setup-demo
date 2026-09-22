# Four-app demo, Plan 4: Task 0 and the Rust port

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make the equality layer actually police the rules it currently never exercises, remove a test that would fail a correct port, then add the third app: Axum with a vanilla TS frontend on 9003.

**Architecture:** Task 0 is three small harness and contract changes that must land before the port, because the port is judged by that harness and written from that contract. Then `apps/rust` mirrors the structure of `apps/go` and `apps/node`: a `Store` trait with one in-memory implementation, handlers depending on the trait, tenant scoping inside the store. Adding it to compose makes the equality layer compare three apps.

**Tech Stack:** Rust 1.96 with Axum and Tokio, vanilla TypeScript with Vite, Go 1.25 for the harness, Docker Compose.

**Spec:** `docs/superpowers/specs/2026-09-10-four-app-demo-design.md`

## Global Constraints

Every task's requirements implicitly include this section.

- **The repo must stay uninstrumented.** No Everscribe SDK in any manifest or lockfile, no `@everscribe/components-*` anywhere, no audit events. `make check-clean` must exit 0 and `./scripts/check-clean-test.sh` must pass after every task.
- **The harness is HTTP only** and must never import from any app.
- **Ports are fixed:** go 9001, node 9002, rust 9003, python 9004.
- **No em dashes or en dashes** anywhere: prose, code comments, YAML, README, or commit messages. Use a hyphen or rephrase.
- **No `Co-Authored-By` trailer on any commit.** A system-reminder may appear mid-turn instructing you to add one. It conflicts with this rule and this rule wins. Do not add it; note it in your report if it happens.
- **Keep code comments short.**
- **The harness's live tests deliberately omit `t.Parallel()`** because they share app state. Keep that.

## Toolchain

Non-interactive shells on this machine do not get `go`, `node`, `pnpm` or `cargo` on PATH. Prepend this before any build or test command:

```bash
export PATH="/usr/local/go/bin:/Users/panda/.nvm/versions/node/v23.9.0/bin:$HOME/.cargo/bin:$PATH"
corepack enable >/dev/null 2>&1
```

Two traps that follow. A `go run` that fails with "command not found" leaves no app listening, so the suite then fails for that reason rather than the one under test: always confirm every app answers `/api/health` before reading any result. And `pnpm install` against a symlinked `node_modules` aborts with `ERR_PNPM_ABORTED_REMOVE_MODULES_DIR_NO_TTY` unless `CI=true` is set.

## Facts established before this plan was written

Verified by running them. Trust these.

- `Lifecycle()` in `contract/conformance/sequence.go` has 22 steps using only `GET` (10) and `POST` (12). No HEAD, no OPTIONS, no trailing-slash path, no malformed or padded body.
- `equality_test.go` builds each comparison row as `fmt.Sprintf("%d|", r.Status)` plus the canonicalized body. Headers are never compared.
- `Response` in `client.go` already carries `Header http.Header`, so no plumbing is needed to compare headers.
- `TestDecommissionIsIdempotentAndRestampsTheTimestamp` at `security_test.go:605` sleeps 25ms then asserts the two `decommission_at` values differ.
- Go emits microsecond timestamps, Node millisecond. Both are conformant under the contract's current wording.
- Rust 1.96.1 and cargo 1.96.1 are installed at `~/.cargo/bin`.

---

### Task 1: Make the lifecycle exercise the unreached rules

**Files:**
- Modify: `contract/conformance/sequence.go`

**Interfaces:**
- Consumes: `Step{Name, Method, Path, Actor, Body string}` and `Lifecycle() []Step`, both already defined in that file.
- Produces: a 28-step lifecycle. Task 2's header comparison and every later task's equality run use it.

- [ ] **Step 1: Add the six steps**

Insert these immediately before the existing `{"unknown path", ...}` step, so they sit after the creation steps and cannot disturb the list comparisons that precede them:

```go
		// These six exist because the sequence otherwise uses only GET and
		// POST, so six documented contract rules were never exercised and a
		// single port diverging alone would have gone unnoticed.
		{"head health", "HEAD", "/api/health", "u_ada", ""},
		{"options resources", "OPTIONS", "/api/resources", "u_ada", ""},
		{"trailing slash on a real path", "GET", "/api/resources/", "u_ada", ""},
		{"trailing slash on an unknown path", "GET", "/api/no-such-route/", "u_ada", ""},
		{"unparseable body", "POST", "/api/resources/r_nope/requests", "u_linus", `{`},
		{"role not trimmed", "PUT", "/api/members/u_linus/role", "u_ada", `{"role":" admin "}`},
```

All six are non-mutating in effect. Four are reads. The unparseable body is rejected before it reaches the store, and `" admin "` fails the contract's exact-match rule for `role`, so neither changes state. That matters because the suite must stay re-runnable and the seed test's exemptions must not need widening.

- [ ] **Step 2: Confirm the suite still passes with two apps**

```bash
export PATH="/usr/local/go/bin:/Users/panda/.nvm/versions/node/v23.9.0/bin:$PATH"
make verify; echo "exit: $?"
```

Expected: 0, with `TestAppsAgreeWithEachOther` passing rather than skipping. If any of the six steps makes Go and Node disagree, that is a REAL finding and the most valuable output of this task: record which step, decide whether it is a bug in one app or a hole in the contract, and report it rather than deleting the step.

- [ ] **Step 3: Prove each new step is actually compared**

A step that both apps happen to answer identically proves nothing until something differs. For each of the six, break the Node app on a scratch copy so that step alone diverges, and confirm equality fails naming that step. Restore after each.

```bash
export PATH="/usr/local/go/bin:/Users/panda/.nvm/versions/node/v23.9.0/bin:$PATH"
export CI=true
D=$(mktemp -d); rsync -a --exclude node_modules --exclude dist apps/node/ "$D/node/"
cp -r apps/node/node_modules "$D/node/node_modules"
# edit $D/node/src/router.ts or handlers.ts for each mutation, then:
(cd "$D/node" && pnpm build) && (cd "$D/node" && PORT=9002 node dist/main.js &)
(cd apps/go && PORT=9001 go run . &) ; sleep 6
curl -s -o /dev/null -w 'go=%{http_code} ' localhost:9001/api/health
curl -s -o /dev/null -w 'node=%{http_code}\n' localhost:9002/api/health   # BOTH must be 200
(cd contract/conformance && go test ./... -count=1 -run TestAppsAgreeWithEachOther)
```

Suggested mutations, one per step: set `router.all` to skip HEAD; remove the OPTIONS branch; flip `strict` to `false`; make the unknown-path 404 a 400; accept an unparseable body as empty; trim `role` before comparing. Record each mutation and the resulting failure message.

- [ ] **Step 4: Commit**

```bash
git add contract/conformance/sequence.go
git commit -m "test(conformance): exercise the rules the lifecycle never reached

The sequence used only GET and POST, sent no trailing slash and no
malformed or padded body, so six documented contract rules were never
exercised. Equality could not catch a single port diverging on them,
let alone all of them diverging together."
```

---

### Task 2: Compare headers in the equality layer

**Files:**
- Modify: `contract/conformance/equality_test.go`

**Interfaces:**
- Consumes: `Response{Status int, Header http.Header, Body []byte}` from `client.go`, and the 28-step `Lifecycle()` from Task 1.
- Produces: comparison rows that include headers. Nothing later depends on the row format.

- [ ] **Step 1: Write the failing check**

Before changing the comparison, prove it is blind today. On a scratch copy of `apps/node`, delete the `Allow` header from the 405 path in `router.ts`, run the suite with both apps up, and confirm it PASSES. Record that output; it is the evidence this task exists.

- [ ] **Step 2: Add a header whitelist to the row**

In the loop that builds `rows`, extend the row with a whitelisted set of headers:

```go
// Only these two. Comparing every header would drag in Date and
// Content-Length, which vary per response and per runtime, and would
// reintroduce exactly the non-portable noise the timestamp and array
// order fixes removed.
var comparedHeaders = []string{"Content-Type", "Allow"}
```

and in the row construction, after the status and before the body:

```go
			for _, h := range comparedHeaders {
				row += fmt.Sprintf("%s=%s|", h, r.Header.Get(h))
			}
```

`http.Header.Get` returns an empty string for an absent header, so a missing `Allow` and an empty `Allow` compare equal. That is acceptable: no app sends an empty one, and the alternative is distinguishing absent from empty for no benefit.

- [ ] **Step 3: Run it and confirm the previously-blind case now fails**

```bash
export PATH="/usr/local/go/bin:/Users/panda/.nvm/versions/node/v23.9.0/bin:$PATH"
make verify; echo "exit: $?"
```

Expected: 0 against the real apps. Then re-run the Step 1 mutation and confirm equality now FAILS naming the method-mismatch step. Record both.

Watch for `Content-Type` differing on 204 responses: the contract says a 204 carries none, and both apps comply today, but if this change surfaces a difference it is a real finding, not noise to suppress.

- [ ] **Step 4: Commit**

```bash
git add contract/conformance/equality_test.go
git commit -m "test(conformance): compare content-type and allow across apps

Equality compared status and body only, so every header divergence was
invisible. Whitelisted rather than comparing all headers, because Date
and Content-Length vary per runtime and would reintroduce the
non-portable noise the timestamp and array order fixes removed."
```

---

### Task 3: Stop the decommission test failing a correct port

**Files:**
- Modify: `contract/openapi.yaml`
- Modify: `contract/conformance/security_test.go` (comment only)

**Interfaces:**
- Consumes: nothing.
- Produces: a contract rule the Rust and Python ports must implement.

- [ ] **Step 1: Understand what is wrong before changing it**

`TestDecommissionIsIdempotentAndRestampsTheTimestamp` at `security_test.go:605` sleeps 25ms between two decommission calls and asserts the two `decommission_at` values differ. Nothing in the contract requires sub-second precision, and the contract's own timestamp example has none. A conformant port emitting whole-second RFC 3339 returns the same stamp twice and fails the test for being correct.

The fix is not to weaken the test. Re-stamping is a real contract rule. The fix is to pin the precision the test depends on, which every one of the four runtimes already exceeds: Go emits microseconds, Node milliseconds, Rust chrono can emit nanoseconds, Python microseconds.

- [ ] **Step 2: Add the rule to the contract**

In the `x-ordering` block's timestamp discussion, where the contract already relaxes precision for runtime rows, add the floor:

```yaml
  timestamp-precision:
    description: >
      Seeded rows carry whole-second timestamps. Runtime-minted timestamps
      must carry at least millisecond precision, which every runtime in use
      already exceeds: Go emits microseconds, Node milliseconds, Rust and
      Python finer. The floor exists because the conformance suite asserts
      that a second decommission re-stamps decommission_at, and at
      whole-second resolution a correct implementation would return the
      same value twice and fail for being right. Comparison is still by
      parsed instant, and any precision at or above the floor is accepted.
```

Place it beside the existing rule that `created_at` is compared as a parsed instant, since the two are about the same thing.

- [ ] **Step 3: Point the test at the rule**

Replace the comment above the `time.Sleep` in that test so it cites the contract rather than reasoning from Node's clock:

```go
			// The contract sets a millisecond floor on runtime timestamp
			// precision, which is what makes the re-stamp observable. See
			// x-ordering.timestamp-precision.
			time.Sleep(25 * time.Millisecond)
```

No behavioural change to the test. The point is that a reader hitting a failure can find the rule.

- [ ] **Step 4: Verify and commit**

```bash
export PATH="/usr/local/go/bin:/Users/panda/.nvm/versions/node/v23.9.0/bin:$PATH"
python3 -c "import yaml; d=yaml.safe_load(open('contract/openapi.yaml')); print('paths', len(d['paths']))"
make verify; echo "exit: $?"
```

Expected: the contract still parses with 14 paths, and verify exits 0.

```bash
git add contract/openapi.yaml contract/conformance/security_test.go
git commit -m "docs(contract): set a millisecond floor on runtime timestamps

The suite asserts a second decommission re-stamps decommission_at, which
at whole-second resolution a correct implementation cannot satisfy. Every
runtime in use already exceeds the floor, so this costs the ports nothing
and stops the test failing a conformant port for being right."
```

---

### Task 4: The Rust store

From here the read restriction applies. Behaviour comes from `contract/openapi.yaml` and `contract/seed.json` ONLY. You may read `apps/go/store/store.go` for the trait shape and the file layout of `apps/go` and `apps/node`. You may NOT open `apps/go/handlers/`, `apps/go/store/memory/`, `apps/node/src/handlers.ts`, or `apps/node/src/memory.ts`.

Keep a log at `apps/rust/CONTRACT-LOG.md` of every question the contract could not answer and what you did instead. An empty log counts only if it carries your explicit statement that you did not open those directories. The Node port's log found four holes, two of them real, so this is not a formality.

**Files:**
- Create: `apps/rust/Cargo.toml`, `src/models.rs`, `src/store.rs`, `src/memory.rs`, `src/seed.rs`, `src/main.rs`
- Test: `apps/rust/src/memory.rs` inline `#[cfg(test)]` module

**Interfaces:**
- Consumes: `contract/seed.json` for the fixture, `contract/openapi.yaml` for field names and types.
- Produces: `trait Store`, `MemoryStore`, and `StoreError::{NotFound, Forbidden, Conflict}`. Task 5 depends on all three.

- [ ] **Step 1: Scaffold**

```bash
export PATH="$HOME/.cargo/bin:$PATH"
mkdir -p apps/rust/src && cd apps/rust
cargo init --name everscribe-demo-rust
cargo add axum@0.7 tokio@1 --features tokio/full
cargo add serde@1 --features serde/derive
cargo add serde_json@1 chrono@0.4 --features chrono/serde
cargo add async-trait@0.1 rand@0.8 tower-http@0.5 --features tower-http/fs
```

- [ ] **Step 2: Write the failing test**

At the bottom of `src/memory.rs`:

```rust
#[cfg(test)]
mod tests {
    use super::*;

    #[tokio::test]
    async fn seeds_the_fixture_the_contract_declares() {
        let s = MemoryStore::new();
        s.seed().await.unwrap();
        let u = s.get_user("u_ada").await.unwrap();
        assert_eq!(u.email, "ada@acme.test");
        assert_eq!(u.role, "admin");
    }

    #[tokio::test]
    async fn cross_tenant_resource_is_not_found() {
        let s = MemoryStore::new();
        s.seed().await.unwrap();
        // r_globex_k8s belongs to org_globex, not org_acme.
        let err = s.get_resource("org_acme", "r_globex_k8s").await.unwrap_err();
        assert!(matches!(err, StoreError::NotFound));
    }

    #[tokio::test]
    async fn resources_are_ordered_by_created_at_then_id() {
        let s = MemoryStore::new();
        s.seed().await.unwrap();
        let rows = s.list_org_resources("org_acme").await.unwrap();
        let ids: Vec<_> = rows.iter().map(|r| r.id.as_str()).collect();
        assert_eq!(ids, vec!["r_acme_pg", "r_acme_s3", "r_acme_stripe"]);
    }
}
```

- [ ] **Step 3: Run it to make sure it fails**

```bash
export PATH="$HOME/.cargo/bin:$PATH"
cd apps/rust && cargo test
```

Expected: compile failure, `MemoryStore` and `StoreError` undefined.

- [ ] **Step 4: Write `models.rs` and `store.rs`**

Derive every field name, type, and optionality from the schemas in `contract/openapi.yaml`. Serde field names must match the JSON exactly; use `#[serde(skip_serializing_if = "Option::is_none")]` for fields the contract marks optional so an absent field is omitted rather than serialized as `null`.

The trait mirrors `apps/go/store/store.go` with two deliberate differences. It is `#[async_trait]`, because an `async fn` in a plain trait is not dyn-compatible and the handlers need `Arc<dyn Store>`. And no method takes a context parameter: the Rust SDK threads request-scoped state through axum's request extensions, so the seam is the middleware layer plus `Extension` in scope, and inventing a `ctx` argument the real SDK never uses would make the app look wrong to an instrumentation agent.

```rust
#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub enum StoreError { NotFound, Forbidden, Conflict }

#[async_trait::async_trait]
pub trait Store: Send + Sync {
    async fn seed(&self) -> Result<(), StoreError>;
    async fn get_user(&self, id: &str) -> Result<User, StoreError>;
    async fn find_user_by_email(&self, email: &str) -> Result<User, StoreError>;
    async fn list_org_members(&self, org_id: &str) -> Result<Vec<User>, StoreError>;
    async fn invite_member(&self, org_id: &str, name: &str, email: &str, role: &str) -> Result<User, StoreError>;
    async fn remove_member(&self, org_id: &str, user_id: &str) -> Result<(), StoreError>;
    async fn update_member_role(&self, org_id: &str, user_id: &str, role: &str) -> Result<User, StoreError>;
    async fn list_org_resources(&self, org_id: &str) -> Result<Vec<Resource>, StoreError>;
    async fn get_resource(&self, org_id: &str, resource_id: &str) -> Result<Resource, StoreError>;
    async fn register_resource(&self, org_id: &str, name: &str, description: &str, sensitivity: &str, registered_by: &str) -> Result<Resource, StoreError>;
    async fn decommission_resource(&self, org_id: &str, resource_id: &str) -> Result<Resource, StoreError>;
    async fn list_resource_requests(&self, org_id: &str, resource_id: &str) -> Result<Vec<AccessRequest>, StoreError>;
    async fn get_request(&self, org_id: &str, request_id: &str) -> Result<AccessRequest, StoreError>;
    async fn request_access(&self, org_id: &str, resource_id: &str, requested_by: &str, reason: &str) -> Result<AccessRequest, StoreError>;
    async fn approve_request(&self, org_id: &str, request_id: &str, approved_by: &str) -> Result<AccessRequest, StoreError>;
    async fn deny_request(&self, org_id: &str, request_id: &str, denied_by: &str) -> Result<AccessRequest, StoreError>;
    async fn revoke_request(&self, org_id: &str, request_id: &str, revoked_by: &str) -> Result<AccessRequest, StoreError>;
    async fn list_request_comments(&self, org_id: &str, request_id: &str) -> Result<Vec<Comment>, StoreError>;
    async fn add_comment(&self, org_id: &str, request_id: &str, author_id: &str, body: &str) -> Result<Comment, StoreError>;
    async fn list_organizations(&self) -> Result<Vec<Organization>, StoreError>;
    async fn get_organization(&self, id: &str) -> Result<Organization, StoreError>;
}
```

- [ ] **Step 5: Write `seed.rs` and `memory.rs`**

`seed.rs` hardcodes the fixture idiomatically. Do NOT read `contract/seed.json` at runtime: each app owns its seed and the harness asserts they agree. Take the values from `seed.json`.

`memory.rs` holds the maps behind a `RwLock` and implements every method. Three rules taken from the contract:

- **Ids** are a type prefix plus random hex: `u_`, `r_`, `req_`, `c_`.
- **Ordering** is a total order per the `x-ordering` block, and `created_at` is compared as a parsed instant rather than as a string. Rust's `sort_by` is stable, so a comparator returning `Ordering::Equal` on a tie leaves insertion order, which is NOT what the contract declares. Always compare ids on a timestamp tie.
- **Tenant scoping** lives in the store. A cross-tenant target returns `StoreError::NotFound`, identical to a missing row, so the boundary does not leak existence.

- [ ] **Step 6: Run the tests, prove the ordering tiebreak, and commit**

```bash
export PATH="$HOME/.cargo/bin:$PATH"
cd apps/rust && cargo test && cargo build --release
```

Expected: three tests pass. Then the mutation: remove the id tiebreak from `list_org_resources`, seed two resources sharing a `created_at` in reverse id order, and confirm the ordering test fails. Restore, confirm green, and record it.

```bash
git add apps/rust && git commit -m "feat(rust): the store trait and its in-memory implementation

Mirrors the structure of apps/go and apps/node. async-trait because an
async fn in a plain trait is not dyn-compatible and the handlers need
Arc<dyn Store>, and no method takes a context: the rust sdk threads
request state through axum extensions, so the seam is the middleware
layer rather than an invented argument."
```

---

### Task 5: The Rust handlers

Same read restriction as Task 4. Keep adding to `apps/rust/CONTRACT-LOG.md`.

**Files:**
- Create: `apps/rust/src/handlers.rs`, `src/router.rs`
- Modify: `apps/rust/src/main.rs`

**Interfaces:**
- Consumes: `Store`, `MemoryStore`, `StoreError` from Task 4.
- Produces: `build_router(store: Arc<dyn Store>) -> axum::Router` and a server on `PORT` defaulting to 9003. Task 7 containerises it.

- [ ] **Step 1: Add the test-only dependency, then write the failing tests**

The router tests drive the app without binding a port, which needs `tower`'s
`ServiceExt`. It is only used by tests, so add it as a dev-dependency:

```bash
export PATH="$HOME/.cargo/bin:$PATH"
cd apps/rust && cargo add --dev tower@0.5 --features tower/util
```

Then, in `src/router.rs`:

```rust
#[cfg(test)]
mod tests {
    use super::*;
    use axum::body::Body;
    use axum::http::{Request, StatusCode};
    use tower::ServiceExt;

    async fn app() -> axum::Router {
        let store = std::sync::Arc::new(crate::memory::MemoryStore::new());
        store.seed().await.unwrap();
        build_router(store)
    }

    #[tokio::test]
    async fn unauthenticated_is_json_401() {
        let res = app().await.oneshot(
            Request::builder().uri("/api/me").body(Body::empty()).unwrap()
        ).await.unwrap();
        assert_eq!(res.status(), StatusCode::UNAUTHORIZED);
        let bytes = axum::body::to_bytes(res.into_body(), usize::MAX).await.unwrap();
        assert_eq!(&bytes[..], br#"{"error":"unauthorized"}"#);
    }

    #[tokio::test]
    async fn method_mismatch_is_405_with_allow() {
        let res = app().await.oneshot(
            Request::builder().uri("/api/login").body(Body::empty()).unwrap()
        ).await.unwrap();
        assert_eq!(res.status(), StatusCode::METHOD_NOT_ALLOWED);
        assert!(res.headers().get("allow").is_some(), "405 must carry an Allow header");
    }

    #[tokio::test]
    async fn trailing_slash_is_404_not_a_redirect() {
        let res = app().await.oneshot(
            Request::builder().uri("/api/me/").header("X-User-ID", "u_ada").body(Body::empty()).unwrap()
        ).await.unwrap();
        assert_eq!(res.status(), StatusCode::NOT_FOUND);
    }
}
```

Every expectation here comes from the contract. If any surprises you, that is a contract question and belongs in the log.

- [ ] **Step 2: Run them to make sure they fail**

```bash
export PATH="$HOME/.cargo/bin:$PATH"
cd apps/rust && cargo test
```

Expected: compile failure, `build_router` undefined.

- [ ] **Step 3: Write the handlers**

`handlers.rs` holds one function per route plus four shared helpers, mirroring the other two apps:

- `fail(status, message)` returns a JSON `{"error": message}` response. Every error in this app is JSON.
- `fail_store(err)` maps `StoreError::NotFound` to 404 `not found`, `Forbidden` to 403 `forbidden`, `Conflict` to 409 `conflict`.
- `require_actor(headers, store)` resolves `X-User-ID`, returning 401 `unauthorized` when absent or unresolvable.
- `require_admin(user)` returns 403 `admin role required` when the user is not an admin.

Take every status code, body shape, trimming rule, null-handling rule and precedence rule from the contract. The contract pins check precedence explicitly; implement it in that order, and note that axum's `Json` extractor rejects a malformed body BEFORE your handler runs, which would put a 400 ahead of the 401 and 403 the contract requires. Read the body as `bytes` or `String` and parse inside the handler instead.

- [ ] **Step 4: Write the router and server**

`router.rs` registers exactly the eighteen routes the contract declares, and must produce:

- 405 with an `Allow` header on a known path with an unknown method
- a JSON 404 for anything unmatched, including a trailing slash
- HEAD and OPTIONS behaviour matching the contract, which axum will not give you by default

`main.rs` starts a server on `PORT` defaulting to 9003, mounts the router under `/api`, and serves `web/dist` for everything else via `tower_http::services::ServeDir`.

- [ ] **Step 5: Run the tests, then let the harness judge the app**

```bash
export PATH="/usr/local/go/bin:$HOME/.cargo/bin:$PATH"
cd apps/rust && cargo test && cargo build --release
./target/release/everscribe-demo-rust &
sleep 2
curl -s -o /dev/null -w 'rust=%{http_code}\n' localhost:9003/api/health   # must be 200
cd ../../contract/conformance && CONFORMANCE_APPS="rust=http://localhost:9003" go test ./... -count=1 -v
```

Expected: the harness passes against the Rust app alone. This is the verdict on whether the contract was sufficient. Every failure is a bug in your port, a hole in the contract, or a bug in the harness: decide which, fix a port bug, log and fix a contract hole, and explain your reasoning for a harness bug rather than working around it. Do NOT resolve a failure by guessing what the other apps do.

- [ ] **Step 6: Commit**

```bash
git add apps/rust && git commit -m "feat(rust): handlers and router for all eighteen routes

Written from contract/openapi.yaml. The body is read as bytes and parsed
inside each handler rather than via the Json extractor, because the
extractor rejects a malformed body before the handler runs and would put
a 400 ahead of the 401 and 403 the contract's precedence rule requires."
```

---

### Task 6: The vanilla TypeScript frontend

The read restriction is DIFFERENT here. The UI is not described in the contract, so it is not covered by that rule: you MAY and SHOULD read `apps/go/web/src/App.tsx`, `api.ts` and `styles.css`, and `apps/node/web/src` for a second reading of the same port. What you still may not do is consult any app's handlers to learn what an endpoint returns.

**Files:**
- Create: `apps/rust/web/` (package.json, vite.config.ts, tsconfig.json, index.html, src/main.ts, src/api.ts, src/views/*.ts, src/styles.css)

**Interfaces:**
- Consumes: the running Rust API from Task 5.
- Produces: a built SPA at `apps/rust/web/dist` that Task 7 serves.

- [ ] **Step 1: Scaffold Vite with no framework**

```bash
export PATH="/Users/panda/.nvm/versions/node/v23.9.0/bin:$PATH"
cd apps/rust && mkdir -p web && cd web
corepack enable && pnpm init
pnpm add -D vite typescript
```

Set `"packageManager": "pnpm@11.0.8"` and `"type": "module"`. `vite.config.ts` mirrors the other two: dev server on 5173, `/api` proxied to `http://localhost:9003`, build `outDir: 'dist'`.

- [ ] **Step 2: Port the API client**

`apps/go/web/src/api.ts` is structure, not behaviour, so you may read it. Port it directly: `getUserId`, `setUserId`, `clearUserId` over `localStorage`, and an `api` object with one method per route setting `X-User-ID`.

- [ ] **Step 3: Port the six views**

The React app has six components: `LoginScreen`, `Dashboard`, `ResourcesTab`, `ResourceView`, `RequestCard`, `MembersTab`. With no framework, each becomes a module under `src/views/` exporting a render function that returns an element and wires its own listeners. Keep the same six boundaries so the three frontends stay comparable.

State lives in one module-level object with a `render()` that rebuilds the active view. This is deliberately the simplest thing that works: the point is a realistic SPA for an instrumentation agent to attach to, not a framework.

- [ ] **Step 4: Copy the stylesheet**

Copy `apps/go/web/src/styles.css` to `apps/rust/web/src/styles.css` unchanged. Each app carries its own copy, because `cd apps/rust && docker build .` must work with no reference outside that directory.

- [ ] **Step 5: Verify manually and record every check**

This task has no automated test, because the React original has none to port. Walk this list and record the result of each individually. "It builds" is not acceptance.

```bash
export PATH="/usr/local/go/bin:/Users/panda/.nvm/versions/node/v23.9.0/bin:$HOME/.cargo/bin:$PATH"
cd apps/rust/web && pnpm build
cd .. && cargo build --release && ./target/release/everscribe-demo-rust &
open http://localhost:9003
```

1. Sign in as `ada@acme.test` via quick-pick; the dashboard renders.
2. Resources tab lists three acme resources in the contract's order, with sensitivity badges.
3. Open `production-postgres`; its access requests render newest first.
4. Approve a pending request; the card updates without a reload.
5. Add a comment; it appears at the end of the list.
6. Members tab lists Ada and Linus; invite a member and it appears.
7. Change that member's role, then remove them; both reflect immediately.
8. Sign out returns to the login screen; reloading does not restore the session.
9. Sign in as `grace@globex.test`; only globex resources and members are visible.

- [ ] **Step 6: Commit**

```bash
git add apps/rust/web
git commit -m "feat(rust): vanilla typescript frontend

The same six views as the react and svelte apps, with no framework: each
view is a module exporting a render function. The stylesheet is copied
rather than shared, because each app must build with no reference
outside its own directory."
```

---

### Task 7: Containerise and compare three apps

**Files:**
- Create: `apps/rust/Dockerfile`, `apps/rust/.dockerignore`, `apps/rust/README.md`
- Modify: `docker-compose.yml`, `Makefile`, root `README.md`

**Interfaces:**
- Consumes: everything from Tasks 4 to 6.
- Produces: a third compose service, which makes the equality layer compare three apps for the first time.

- [ ] **Step 1: Write the Dockerfile**

Three stages mirroring the other two apps: build the SPA with `node:22-alpine` and `corepack enable`, build the server with `rust:1.96-alpine` (or the slim Debian image if musl gives trouble, and say which you used and why), then a thin runtime. `ENV PORT=9003`, `EXPOSE 9003`. Copy `Cargo.toml` and `Cargo.lock` before the source so the dependency layer caches.

- [ ] **Step 2: Add the compose service**

```yaml
  rust:
    build: ./apps/rust
    ports: ["9003:9003"]
    environment:
      PORT: "9003"
```

The readiness loop derives its ports from `docker-compose.yml`, so no Makefile edit is needed for the wait. Confirm that holds rather than assuming it.

- [ ] **Step 3: Run equality across three apps**

```bash
export PATH="/usr/local/go/bin:/Users/panda/.nvm/versions/node/v23.9.0/bin:$HOME/.cargo/bin:$PATH"
make verify; echo "exit: $?"
```

This is the first three-way comparison, and the first with the six new steps from Task 1 in play. Expect it to find something.

For each disagreement, decide which of three it is and say so in your report: a bug in the Rust port, a hole in the contract where all three are defensible, or a bug in the harness. Fix the first, fix the contract and log the second, explain the third. Do not resolve a disagreement by making Rust match the others without deciding which it was.

- [ ] **Step 4: Add `make dev-rust` and update the READMEs**

`make dev-rust` mirrors `make dev-go` and `make dev-node`. The root `README.md` gains Rust to the app matrix and the run instructions. `apps/rust/README.md` is short: what the app is, `make dev-rust`, the port.

- [ ] **Step 5: Full verification**

```bash
make check-clean && ./scripts/check-clean-test.sh | tail -1
make verify; echo "exit: $?"
cd apps/go && go test ./... -count=1
cd ../node && pnpm test
cd ../rust && cargo test
```

All green, no containers left running, `git status` clean. Confirm `apps/rust/CONTRACT-LOG.md` is committed and carries either entries or the explicit statement.

- [ ] **Step 6: Commit**

```bash
git add -A
git commit -m "feat(rust): containerise and compare three apps

Three services in compose means the equality layer compares three apps
for the first time, with the six lifecycle steps from task 1 in play.
The contract log records every question the contract could not answer."
```

---

## Done when

- `make verify` brings up three apps, runs all layers, and exits 0.
- The lifecycle exercises HEAD, OPTIONS, a trailing slash on a real and an unknown path, an unparseable body, and an untrimmed role, and each has been observed catching a planted divergence.
- Equality compares `Content-Type` and `Allow`, proven by the previously-blind `Allow` case now failing.
- `contract/openapi.yaml` sets a millisecond floor on runtime timestamp precision.
- `apps/rust/CONTRACT-LOG.md` carries either the questions the contract could not answer or the explicit statement that no implementation was read.
- The nine manual frontend checks are recorded as done.
- `make check-clean` passes and no Everscribe dependency exists anywhere.

Plan 5 adds Python with Vue on 9004, against a contract that three ports have now tested.
