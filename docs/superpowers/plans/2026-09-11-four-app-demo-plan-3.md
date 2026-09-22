# Four-app demo, Plan 3: Task 0 and the Node app

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Correct the contract and harden the harness while it is still cheap, then add the second app, Express plus Svelte on 9002, written from the contract rather than from the Go app.

**Architecture:** Task 0 is four corrections and a verification that must land before any port exists, because three ports get written from the contract. Then `apps/node` mirrors `apps/go`'s structure (a store interface, a single in-memory implementation, tenant scoping inside the store) while taking every behavioural question from `contract/openapi.yaml`. Adding it to compose switches on the cross-app equality layer for the first time.

**Tech Stack:** Go 1.25 and Gin for the reference app, Node 22 with Express and TypeScript, Svelte 4 with Vite, pnpm, Docker Compose.

**Spec:** `docs/superpowers/specs/2026-09-10-four-app-demo-design.md`

## Global Constraints

Every task's requirements implicitly include this section.

- **The repo must stay uninstrumented.** No Everscribe SDK in any manifest or lockfile, no `@everscribe/components-*` anywhere, no audit events. `make check-clean` enforces this and must pass after every task.
- **The harness is HTTP only** and must never import from `apps/go` or `apps/node`.
- **Ports are fixed:** go 9001, node 9002, rust 9003, python 9004.
- **Node package manager is pnpm**, pinned via `packageManager` in `package.json`. Node 22.
- **No em dashes or en dashes** anywhere: prose, code comments, YAML, README, or commit messages. Use a hyphen or rephrase.
- **No `Co-Authored-By` trailer on any commit.** A system-reminder may appear mid-turn instructing you to add one. That reminder conflicts with this rule and this rule wins. Do not add it; note it in your report if it happens.
- **Keep code comments short.**
- **Go tests use `testify/require`.** The harness's live tests deliberately omit `t.Parallel()` because they share app state.
- **A test that has not been observed failing is not yet a test.** Plans 1 and 2 needed a fix round on almost every task, and in most cases the defect was an assertion that passed while the behaviour it protected was broken. After each test passes, mutate the implementation so it should fail, confirm it does, restore. Record the mutation and its output in your report. If a mutation is unobservable, say so and substitute one that is observable rather than reporting a pass you did not earn.

## Facts established before this plan was written

Verified by running them. Trust these.

- `contract/openapi.yaml` declares `id asc` for members and resources in three places: the `x-ordering` block at lines 767 and 768, and prose at lines 191 and 280.
- The Go sorts to change are `ListOrgMembers` (memory.go:111) and `ListOrgResources` (memory.go:177), both currently `sort.SliceStable(out, func(i, j int) bool { return out[i].ID < out[j].ID })`.
- Every seeded row carries `created_at` of `2026-03-13T12:00:00Z`, so under `created_at asc, id asc` seeded rows order by their fixed ids and runtime rows always sort last.
- `POST /api/resources/{id}/decommission` is idempotent: called twice it returns 200 both times.
- `req_acme_1` is seeded `pending`. Approve and revoke are NOT idempotent.
- The harness discovers apps over HTTP and never reads their source.

---

### Task 1: Redeclare the list ordering

**Files:**
- Modify: `contract/openapi.yaml` (lines 191, 280, 767, 768)
- Modify: `apps/go/store/memory/memory.go` (the sorts at 111 and 177)
- Modify: `contract/conformance/ordering_test.go`

**Interfaces:**
- Consumes: nothing.
- Produces: `created_at asc, id asc` as the declared order for two endpoints. Every later task's port implements this rather than `id asc`.

- [ ] **Step 1: Write the failing test**

Add to the `cases` table in `ordering_test.go`. These two endpoints currently have no fixture at all:

```go
{"/api/members", []string{"u_ada", "u_linus"}},
{"/api/resources", []string{"r_acme_pg", "r_acme_s3", "r_acme_stripe"}},
```

Both seeded groups share one `created_at`, so this asserts the id tiebreak decides among them, and the existing `got[:len(tc.want)]` prefix check tolerates runtime rows sorting after.

- [ ] **Step 2: Run it against an unchanged app**

```bash
cd apps/go && PORT=9001 go run . & sleep 4
cd contract/conformance && go test ./... -run TestSeededLists -count=1
```

Expected: PASS. Under `id asc` the seeded rows already happen to be in this order, so this test does not yet discriminate. That is the point: it pins the order so the next step cannot change it by accident.

- [ ] **Step 3: Change the Go sorts**

Both `ListOrgMembers` and `ListOrgResources` in `apps/go/store/memory/memory.go`:

```go
// created_at asc, id asc. id alone is not a total order across apps:
// generated ids are a prefix plus random hex and sort either side of
// the seeded rows.
sort.SliceStable(out, func(i, j int) bool {
	if out[i].CreatedAt.Equal(out[j].CreatedAt) {
		return out[i].ID < out[j].ID
	}
	return out[i].CreatedAt.Before(out[j].CreatedAt)
})
```

- [ ] **Step 4: Change the contract in all three places**

`x-ordering` lines 767 and 768:

```yaml
  GET /api/members:                   [created_at asc, id asc]
  GET /api/resources:                 [created_at asc, id asc]
```

Prose at line 191 and line 280, currently "Ordered by id ascending. See x-ordering.", becomes:

```
        Ordered by created_at ascending, then id ascending. See
        x-ordering.
```

Also extend the `x-ordering` description to say why `id` alone was insufficient. Keep it to two sentences; the spec carries the full reasoning.

- [ ] **Step 5: Prove the new order is what is now enforced**

```bash
cd apps/go && PORT=9001 go run . & sleep 4
cd contract/conformance && go test ./... -count=1
```

Expected: PASS. Then the mutation: revert one of the two sorts to `id` only, create a resource whose generated id sorts after the seeded block, and confirm the ordering test fails.

```bash
curl -s -X POST localhost:9001/api/resources -H 'X-User-ID: u_ada' \
  -H 'Content-Type: application/json' \
  -d '{"name":"zz","description":"x","sensitivity":"internal"}'
curl -s localhost:9001/api/resources -H 'X-User-ID: u_ada' | python3 -c \
  'import json,sys;print([r["id"][:8] for r in json.load(sys.stdin)])'
```

A generated id sorts before the seeded block about two times in three, so repeat until you get one that sorts after, then confirm the test fails under `id asc` and passes under the new order. Record both. If you cannot produce an after-sorting id in ten tries, say so rather than claiming the mutation.

- [ ] **Step 6: Commit**

```bash
git add contract/openapi.yaml apps/go/store/memory/memory.go contract/conformance/ordering_test.go
git commit -m "fix(contract): order members and resources by created_at then id

id alone is not a total order across independently running apps.
Generated ids are a fixed prefix plus random hex and sort either side of
the seeded rows, so two conformant apps can legitimately disagree. Every
seeded row shares one timestamp, so seeded rows order by their fixed ids
and runtime rows always sort last."
```

---

### Task 2: Close the three harness gaps

**Files:**
- Modify: `contract/conformance/security_test.go`
- Modify: `contract/conformance/sequence_test.go`
- Modify: `Makefile`

**Interfaces:**
- Consumes: `apps(t)`, `validated(t, c, method, path, actor, body)`, `mintStripeRequest(t, c, reason)` from the existing harness.
- Produces: `requirePristine(t, c)` called from `Run`, guarded per app.

- [ ] **Step 1: Write the failing cross-tenant member-write test**

The existing cross-tenant write block covers only decommission and approve. Add member writes. Put this in `security_test.go` beside them:

```go
// An acme admin must not be able to touch a globex member. Each is
// paired with a same-tenant positive control, because a cross-tenant
// miss and an unregistered route are byte-identical here by design.
func TestCrossTenantMemberWrites(t *testing.T) {
	for _, app := range apps(t) {
		app := app
		t.Run(app.Name, func(t *testing.T) {
			c := NewClient(app)

			// Positive controls: mint members and act on them in-tenant.
			// Emails must be unique across the whole suite, so these are
			// namespaced to this test.
			victim := inviteMember(t, c, "XT Role Target", "xt-role@acme.test")
			okRole := validated(t, c, http.MethodPut, "/api/members/"+victim+"/role", "u_ada",
				`{"role":"admin"}`)
			require.Equal(t, http.StatusOK, okRole.Status,
				"positive control: an admin must be able to change an own-tenant member's role")

			doomed := inviteMember(t, c, "XT Delete Target", "xt-delete@acme.test")
			okDel := validated(t, c, http.MethodDelete, "/api/members/"+doomed, "u_ada", "")
			require.Equal(t, http.StatusNoContent, okDel.Status,
				"positive control: an admin must be able to remove an own-tenant member")

			// u_alan is a globex member. u_ada is an acme admin.
			for _, tc := range []struct{ method, path, body string }{
				{http.MethodPut, "/api/members/u_alan/role", `{"role":"admin"}`},
				{http.MethodDelete, "/api/members/u_alan", ""},
			} {
				miss := validated(t, c, tc.method, tc.path, "u_ada", tc.body)
				require.Equal(t, http.StatusNotFound, miss.Status, "%s %s", tc.method, tc.path)
				require.JSONEq(t, `{"error":"not found"}`, string(miss.Body))
			}
		})
	}
}
```

`inviteMember(t, c, name, email string) string` already exists in
`security_test.go` and returns the new member's id. Do not change its
signature; four call sites depend on it. Emails must be unique across the
whole suite, which is why the two above are namespaced `xt-`.

Note that `TestMemberCannotUseAdminRoutes` already has its own
role-change and remove positive controls. Do not reuse its minted
members: sharing mutable fixtures between test functions is what caused
the idempotence bug in Plan 2. Mint your own.

- [ ] **Step 2: Run it, then prove it can fail**

```bash
cd contract/conformance && go test ./... -run TestCrossTenantMemberWrites -count=1 -v
```

Expected: PASS. Then on a scratch copy of `apps/go`, strip the `OrganizationID != orgID` check from `RemoveMember` and `UpdateMemberRole` only, and confirm this test fails. Restore. Record both.

- [ ] **Step 3: Write the precondition guard**

The seed test exempts `r_acme_s3.decommission_at` and `req_acme_1`'s decision fields, because the lifecycle mutates them. That exemption is exploitable: an app can ship them already mutated and pass, since the lifecycle only asserts `status < 500`.

Do NOT solve this by adding an expected-status field to `Step`. The lifecycle is replayed twice in one run, by `equality_test.go` and then `sequence_test.go`, and approve and revoke are not idempotent, so exact statuses would break the second replay and undo the idempotence Plan 2 established. Decommission IS idempotent (200 twice, verified), so only approve and revoke are affected, but the fix below closes the hole for all three.

Assert the precondition instead, once per app, before the first replay mutates anything. Add to `sequence_test.go`:

```go
var pristineOnce sync.Map // app name -> *sync.Once

// requirePristine asserts the two seeded rows the lifecycle mutates start
// in the state seed.json claims. The seed test cannot check them, because
// it exempts exactly these fields so the suite stays re-runnable. Running
// once per app, before any replay, is what closes that hole.
func requirePristine(t *testing.T, c *Client) {
	t.Helper()
	v, _ := pristineOnce.LoadOrStore(c.App.Name, &sync.Once{})
	v.(*sync.Once).Do(func() {
		reqs := validated(t, c, http.MethodGet, "/api/resources/r_acme_pg/requests", "u_ada", "")
		var rows []map[string]any
		require.NoError(t, json.Unmarshal(reqs.Body, &rows))
		found := false
		for _, r := range rows {
			if r["id"] == "req_acme_1" {
				found = true
				require.Equal(t, "pending", r["status"],
					"req_acme_1 must start pending; the seed test exempts this field")
				require.Nil(t, r["decided_by"], "req_acme_1 must start undecided")
				require.Nil(t, r["revoked_by"], "req_acme_1 must start unrevoked")
			}
		}
		require.True(t, found, "req_acme_1 missing from r_acme_pg's requests")

		res := validated(t, c, http.MethodGet, "/api/resources", "u_ada", "")
		var resources []map[string]any
		require.NoError(t, json.Unmarshal(res.Body, &resources))
		for _, r := range resources {
			if r["id"] == "r_acme_s3" {
				require.Nil(t, r["decommission_at"],
					"r_acme_s3 must start live; the seed test exempts this field")
			}
		}
	})
}
```

Call it as the first line of `Run`, after `t.Helper()`.

- [ ] **Step 4: Prove the precondition guard fails**

On a scratch copy of `apps/go`, edit `store/memory/seed.go` so `req_acme_1` is seeded `approved` with a `DecidedBy`. Run the suite. Expected: FAIL on `req_acme_1 must start pending`. Restore. Then do the same for `r_acme_s3` with `DecommissionAt` set, expect FAIL on `r_acme_s3 must start live`, restore. Record both.

Also confirm the suite still passes three consecutive times against one unrestarted app, which is the property the `sync.Once` exists to preserve.

- [ ] **Step 5: Extend the readiness loop**

In the `Makefile`'s `verify` target, the readiness loop polls 9001 only. Make it wait for every port that compose defines. Keep the existing failure behaviour: print `docker compose logs --tail 50`, tear down, exit 1.

```make
	ready=0; \
	ports="$$(grep -oE '\"[0-9]{4}:[0-9]{4}\"' docker-compose.yml | cut -d: -f1 | tr -d '\"')"; \
	for i in $$(seq 1 60); do \
	  allup=1; \
	  for p in $$ports; do \
	    curl -fsS localhost:$$p/api/health >/dev/null 2>&1 || allup=0; \
	  done; \
	  if [ $$allup -eq 1 ]; then ready=1; break; fi; \
	  sleep 1; \
	done; \
```

Derive the ports from `docker-compose.yml` rather than hardcoding, so Plans 4 and 5 need no Makefile edit. Raise the timeout to 60 because four containers take longer than one. Update the failure message to name the ports it waited for.

- [ ] **Step 6: Verify and commit**

```bash
make verify; echo "exit: $?"
```

Expected: 0. Then confirm the failure path still works by stopping the go service mid-wait, or by temporarily pointing a compose port at a container that does not serve `/api/health`, and confirm `make verify` exits non-zero with the port named.

```bash
git add contract/conformance Makefile
git commit -m "test(conformance): cross-tenant member writes and a seed precondition

The cross-tenant write table covered decommission and approve only, so
stripping the org check from RemoveMember and UpdateMemberRole let an
acme admin delete a globex member with the suite green.

The seed test exempts the two rows the lifecycle mutates, which let an
app ship them already mutated. Asserting the precondition once per app,
before the first replay, closes that without putting exact statuses on
non-idempotent lifecycle steps, which would have broken re-runnability."
```

---

### Task 3: Prove the security layer fails

No production code. The deliverable is evidence, and the spec requires it: the security layer is about to become the acceptance test three from-scratch ports are judged by, and a hole in it passes quietly in all three at once, which layer 2 cannot catch because equality only compares apps to each other.

**Files:**
- Create: `contract/conformance/SECURITY-LAYER-EVIDENCE.md`

**Interfaces:**
- Consumes: the whole harness as of Task 2.
- Produces: a checked-in record that each security invariant has been observed failing.

- [ ] **Step 1: Plant each defect and record the result**

Work on a scratch copy of `apps/go`. For each row, make the change, run `cd contract/conformance && go test ./... -count=1` against the mutated app, record which test failed and its message, then restore and confirm green.

| Defect to plant | Expected to fail |
|---|---|
| Drop the org filter in `RemoveMember` | `TestCrossTenantMemberWrites` |
| Drop the org filter in `UpdateMemberRole` | `TestCrossTenantMemberWrites` |
| Drop the org filter in `ListOrgResources` | the tenant-scoping test |
| Drop the org filter in `ListOrgMembers` | the tenant-scoping test |
| Make `ApproveRequest` a no-op returning the unchanged row | an approve positive control |
| Make `DecommissionResource` a no-op | the cross-tenant decommission control or the lifecycle |
| Remove `requireActor` from `POST /api/requests/{id}/revoke` | `TestMissingActorIs401` |
| Remove `requireAdmin` from `POST /api/resources/{id}/decommission` | `TestMemberCannotUseAdminRoutes` |
| Seed `req_acme_1` as `approved` | `requirePristine` |
| Seed `r_acme_s3` with `DecommissionAt` set | `requirePristine` |

- [ ] **Step 2: Write the evidence file**

`contract/conformance/SECURITY-LAYER-EVIDENCE.md`: a short preamble saying what this file is and why it exists, then one row per defect with the exact failing test name and the first line of its failure message, then the date and the commit it was run against.

If any planted defect does NOT produce a failure, that is the most important output of this task. Do not quietly drop the row. Record it as an open hole, and say which test should have caught it.

- [ ] **Step 3: Confirm the tree is clean and commit**

```bash
git status --porcelain apps/go   # must be empty
git add contract/conformance/SECURITY-LAYER-EVIDENCE.md
git commit -m "docs(conformance): evidence that the security layer fails

Each security invariant has been observed failing against a planted
defect. Two of Task 0's corrections were assertion tables that silently
omitted routes, and a green suite told nobody. This layer is about to
judge three from-scratch ports, and a hole in it passes quietly in all
three at once."
```

---

### Task 4: The Node store

From here on, behaviour comes from `contract/openapi.yaml` and `contract/seed.json` only. Structure comes from `apps/go/store/store.go` and the file layout. You may NOT open `apps/go/handlers/` or `apps/go/store/memory/`. Error-to-status mapping is behaviour and is in the contract; it is not among the things you may look up.

Keep a log at `apps/node/CONTRACT-LOG.md` of every question you could not answer from the contract, and what you did instead. An empty log counts only if it carries your explicit statement that you did not open those two directories.

**Files:**
- Create: `apps/node/package.json`, `tsconfig.json`, `src/models.ts`, `src/store.ts`, `src/memory.ts`, `src/seed.ts`
- Create: `apps/node/src/memory.test.ts`

**Interfaces:**
- Consumes: `contract/seed.json` for the fixture, `contract/openapi.yaml` for field names and types.
- Produces: `Store` (the interface), `MemoryStore implements Store`, `ErrNotFound`, `ErrForbidden`, `ErrConflict`. Task 5 depends on all of these.

- [ ] **Step 1: Scaffold the package**

```bash
mkdir -p apps/node/src && cd apps/node
corepack enable
pnpm init
pnpm add express
pnpm add -D typescript @types/node @types/express tsx vitest
```

Set `"packageManager": "pnpm@11.0.8"` and `"type": "module"` in `package.json`, and scripts:

```json
"scripts": {
  "build": "tsc -b",
  "start": "node dist/main.js",
  "dev": "tsx watch src/main.ts",
  "test": "vitest run"
}
```

- [ ] **Step 2: Write the failing store test**

`apps/node/src/memory.test.ts`:

```ts
import { describe, expect, it } from "vitest";
import { MemoryStore } from "./memory.js";
import { ErrNotFound } from "./store.js";

describe("MemoryStore", () => {
  it("seeds the fixture the contract declares", async () => {
    const s = new MemoryStore();
    await s.seed();
    const u = await s.getUser("u_ada");
    expect(u.email).toBe("ada@acme.test");
    expect(u.role).toBe("admin");
  });

  it("returns not-found for a cross-tenant resource", async () => {
    const s = new MemoryStore();
    await s.seed();
    // r_globex_k8s belongs to org_globex, not org_acme.
    await expect(s.getResource("org_acme", "r_globex_k8s")).rejects.toBe(ErrNotFound);
  });

  it("orders resources by created_at then id", async () => {
    const s = new MemoryStore();
    await s.seed();
    const rows = await s.listOrgResources("org_acme");
    expect(rows.map((r) => r.id)).toEqual(["r_acme_pg", "r_acme_s3", "r_acme_stripe"]);
  });
});
```

- [ ] **Step 3: Run it to make sure it fails**

```bash
cd apps/node && pnpm test
```

Expected: failure resolving `./memory.js` and `./store.js`.

- [ ] **Step 4: Write `models.ts` and `store.ts`**

Derive every field name, type, and required-ness from the schemas in `contract/openapi.yaml`. The store interface mirrors `apps/go/store/store.go`, with two deliberate differences. Methods are `async` and reject rather than returning an error value, which is the idiom here. And no method takes a context parameter: Node's SDK uses `AsyncLocalStorage`, so the seam is the Express middleware slot plus `req` being in scope, and inventing a `ctx` argument the real SDK never uses would make the app look wrong to an instrumentation agent.

```ts
// store.ts
export const ErrNotFound = Symbol("not found");
export const ErrForbidden = Symbol("forbidden");
export const ErrConflict = Symbol("conflict");

// Store is the persistence boundary. Tenant scoping lives here, not in
// handlers: a cross-tenant miss rejects with ErrNotFound, identical to a
// missing row, so the boundary does not leak existence.
export interface Store {
  seed(): Promise<void>;
  listOrganizations(): Promise<Organization[]>;
  getOrganization(id: string): Promise<Organization>;
  getUser(id: string): Promise<User>;
  findUserByEmail(email: string): Promise<User>;
  listOrgMembers(orgId: string): Promise<User[]>;
  inviteMember(orgId: string, name: string, email: string, role: string): Promise<User>;
  removeMember(orgId: string, userId: string): Promise<void>;
  updateMemberRole(orgId: string, userId: string, role: string): Promise<User>;
  listOrgResources(orgId: string): Promise<Resource[]>;
  getResource(orgId: string, resourceId: string): Promise<Resource>;
  registerResource(orgId: string, name: string, description: string, sensitivity: string, registeredBy: string): Promise<Resource>;
  decommissionResource(orgId: string, resourceId: string): Promise<Resource>;
  listResourceRequests(orgId: string, resourceId: string): Promise<AccessRequest[]>;
  getRequest(orgId: string, requestId: string): Promise<AccessRequest>;
  requestAccess(orgId: string, resourceId: string, requestedBy: string, reason: string): Promise<AccessRequest>;
  approveRequest(orgId: string, requestId: string, approvedBy: string): Promise<AccessRequest>;
  denyRequest(orgId: string, requestId: string, deniedBy: string): Promise<AccessRequest>;
  revokeRequest(orgId: string, requestId: string, revokedBy: string): Promise<AccessRequest>;
  listRequestComments(orgId: string, requestId: string): Promise<Comment[]>;
  addComment(orgId: string, requestId: string, authorId: string, body: string): Promise<Comment>;
}
```

- [ ] **Step 5: Write `seed.ts` and `memory.ts`**

`seed.ts` hardcodes the fixture idiomatically. Do NOT read `contract/seed.json` at runtime: each app owns its seed, and the harness asserts they agree. Take the values from `seed.json` and from the `x-ordering` examples.

`memory.ts` holds Maps and implements every method. Two rules carried from the contract:

- **Ids.** Generated ids are a type prefix plus random hex: `u_`, `r_`, `req_`, `c_`. The harness normalizes anything not in `seed.json`, so the exact format is free, but the prefix convention keeps them legible.
- **Ordering.** Every list returns a total order. Members and resources are `created_at` ascending then `id` ascending; requests are `created_at` DESCENDING then `id` ascending; comments are `created_at` ascending then `id` ascending. JavaScript's `Array.prototype.sort` is stable, so a comparator returning 0 on ties preserves insertion order, which is NOT what the contract declares. Always compare ids on a timestamp tie.

- [ ] **Step 6: Run the tests and commit**

```bash
cd apps/node && pnpm test && pnpm build
```

Expected: three tests pass, build clean. Then the mutation: remove the id tiebreak from `listOrgResources` and confirm the ordering test still passes (seeded rows have distinct ids under a stable sort, so it will), then seed two resources with the same `created_at` in reverse id order and confirm it fails. Record it.

```bash
git add apps/node && git commit -m "feat(node): the store interface and its in-memory implementation

Mirrors apps/go's structure. Methods are async and reject rather than
returning an error value, and none takes a context: node's SDK uses
AsyncLocalStorage, so the seam is the express middleware slot and req
being in scope."
```

---

### Task 5: The Node handlers

Same read restriction as Task 4. Keep adding to `apps/node/CONTRACT-LOG.md`.

**Files:**
- Create: `apps/node/src/handlers.ts`, `src/router.ts`, `src/main.ts`
- Create: `apps/node/src/router.test.ts`

**Interfaces:**
- Consumes: `Store`, `MemoryStore`, `ErrNotFound`, `ErrForbidden`, `ErrConflict` from Task 4.
- Produces: `buildRouter(store: Store): express.Router`, `makeApp(): Promise<express.Express>` which seeds a store and mounts the router (the tests import this), and a server on `PORT` defaulting to 9002 started from `main.ts`. Task 7 containerises it.

- [ ] **Step 1: Write the failing router test**

`apps/node/src/router.test.ts`, using `supertest` (`pnpm add -D supertest @types/supertest`):

```ts
import { describe, expect, it, beforeAll } from "vitest";
import request from "supertest";
import { makeApp } from "./main.js";

let app: ReturnType<typeof makeApp>;
beforeAll(async () => { app = await makeApp(); });

describe("router", () => {
  it("401s without an actor, as JSON", async () => {
    const r = await request(app).get("/api/me");
    expect(r.status).toBe(401);
    expect(r.headers["content-type"]).toMatch(/application\/json/);
    expect(r.body).toEqual({ error: "unauthorized" });
  });

  it("405s on a method the path does not serve", async () => {
    const r = await request(app).get("/api/login");
    expect(r.status).toBe(405);
    expect(r.body).toEqual({ error: "method not allowed" });
    expect(r.headers["allow"]).toContain("POST");
  });

  it("404s a trailing slash rather than redirecting", async () => {
    const r = await request(app).get("/api/me/").set("X-User-ID", "u_ada");
    expect(r.status).toBe(404);
    expect(r.body).toEqual({ error: "not found" });
  });

  it("returns an identical 404 for an unknown path and a cross-tenant miss", async () => {
    const unknown = await request(app).get("/api/nope").set("X-User-ID", "u_ada");
    const cross = await request(app)
      .get("/api/resources/r_globex_k8s/requests").set("X-User-ID", "u_ada");
    expect(unknown.status).toBe(cross.status);
    expect(unknown.body).toEqual(cross.body);
  });

  it("204s with no content-type and no body on logout", async () => {
    const r = await request(app).post("/api/logout").set("X-User-ID", "u_ada");
    expect(r.status).toBe(204);
    expect(r.headers["content-type"]).toBeUndefined();
    expect(r.text).toBe("");
  });
});
```

Every expectation here comes from the contract. If any of them surprises you, that is a contract question and belongs in the log.

- [ ] **Step 2: Run it to make sure it fails**

```bash
cd apps/node && pnpm test
```

Expected: failure resolving `./main.js`.

- [ ] **Step 3: Write the handlers**

`handlers.ts` holds one function per route plus three shared helpers, mirroring the Go app's shape:

- `fail(res, status, message)` writes `{"error": message}` as JSON. Every error response in this app is JSON.
- `failStore(res, err)` maps `ErrNotFound` to 404 `not found`, `ErrForbidden` to 403 `forbidden`, `ErrConflict` to 409 `conflict`, anything else to 500.
- `requireActor(req, res, store)` resolves `X-User-ID`, writing 401 `unauthorized` and returning null when absent or unresolvable.
- `requireAdmin(res, user)` writes 403 `admin role required` when the user is not an admin.

Take every status code, body shape, trimming rule, and precedence rule from the contract. The contract pins check precedence explicitly; implement it in that order.

- [ ] **Step 4: Write the router and server**

`router.ts` registers exactly the eighteen routes the contract declares, and:

- returns 405 with an `Allow` header for a known path and an unknown method
- returns the JSON 404 for anything unmatched
- does NOT redirect a trailing slash (Express's `strict routing` setting governs this; read the contract for the required behaviour)

`main.ts` exports `makeApp()` for tests and starts a server on `process.env.PORT ?? 9002`, mounting the router under `/api` and serving `web/dist` statically for everything else.

- [ ] **Step 5: Run the tests, then check against the harness**

```bash
cd apps/node && pnpm test && pnpm build
node dist/main.js &
cd ../../contract/conformance && CONFORMANCE_APPS="node=http://localhost:9002" go test ./... -count=1 -v
```

Expected: the harness passes against the Node app alone. This is the first real verdict on whether the contract was sufficient. Every failure here is either a bug in your port or a hole in the contract: decide which, fix the port or log the hole, and record both in your report.

- [ ] **Step 6: Commit**

```bash
git add apps/node && git commit -m "feat(node): handlers and router for all eighteen routes

Written from contract/openapi.yaml. Every error response is JSON, an
unknown path and a cross-tenant miss are byte-identical, a trailing
slash does not redirect, and a known path with an unknown method
returns 405 with an Allow header."
```

---

### Task 6: The Svelte frontend

A faithful port of the React app. This is the only task in the plan with no automated test behind it, because the React original has none either. Its acceptance is therefore explicit manual verification, listed in Step 5. Do not mark it done on "it builds".

**Files:**
- Create: `apps/node/web/` (package.json, vite.config.ts, tsconfig.json, index.html, src/main.ts, src/api.ts, src/App.svelte, src/lib/*.svelte, src/styles.css)

**Interfaces:**
- Consumes: the running Node API from Task 5.
- Produces: a built SPA at `apps/node/web/dist` that Task 7 serves.

- [ ] **Step 1: Scaffold Svelte plus Vite**

```bash
cd apps/node && mkdir -p web && cd web
pnpm init
pnpm add -D svelte @sveltejs/vite-plugin-svelte vite typescript svelte-check
```

Svelte 4 with Vite, NOT SvelteKit. SvelteKit with `adapter-static` writes `build/` rather than `web/dist` and would break the uniform layout every app shares. This matches what `monorepo/internal/ui/content/docs/examples/svelte-node.md` publishes.

`vite.config.ts` mirrors the Go app's: dev server on 5173, `/api` proxied to `http://localhost:9002`, build `outDir: 'dist'`.

- [ ] **Step 2: Port the API client**

`apps/go/web/src/api.ts` is structure, not behaviour, so you may read it. Port it directly: `getUserId`, `setUserId`, `clearUserId` over `localStorage`, and an `api` object with one method per route that sets `X-User-ID` and parses the JSON error shape.

- [ ] **Step 3: Port the six components**

The React app has six, in `apps/go/web/src/App.tsx`. Port each to a Svelte component under `src/lib/`, preserving behaviour and layout:

| React component | Responsibility |
|---|---|
| `LoginScreen` | email entry plus quick-pick of the four seeded users |
| `Dashboard` | shell, tab switching, current user, sign out |
| `ResourcesTab` | resource list, sensitivity badges, register form |
| `ResourceView` | one resource, its access requests, the request form |
| `RequestCard` | one request, status, approve/deny/revoke, comments |
| `MembersTab` | member list, roles, invite form, remove and role change |

You may read `App.tsx` for layout and behaviour: the UI is not specified in the contract, so it is not part of the read restriction. What you may not do is read the Go handlers to work out what an endpoint returns.

- [ ] **Step 4: Copy the stylesheet**

`apps/go/web/src/styles.css` is 611 lines. Copy it to `apps/node/web/src/styles.css` unchanged. Each app carries its own copy: the self-containment rule means `cd apps/node && docker build .` must work with no reference outside that directory.

- [ ] **Step 5: Verify manually, and record what you did**

```bash
cd apps/node/web && pnpm build
cd .. && pnpm build && node dist/main.js &
open http://localhost:9002
```

Walk this list and record the result of each in your report. "It built" is not acceptance.

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
git add apps/node/web
git commit -m "feat(node): svelte frontend

A faithful port of the react app's six components. Svelte 4 with vite,
not sveltekit, so the build lands in web/dist like every other app. The
stylesheet is copied rather than shared, because each app must build
with no reference outside its own directory."
```

---

### Task 7: Containerise and switch on cross-app equality

**Files:**
- Create: `apps/node/Dockerfile`, `apps/node/.dockerignore`, `apps/node/README.md`
- Modify: `docker-compose.yml`, `Makefile`, root `README.md`

**Interfaces:**
- Consumes: everything from Tasks 4 to 6.
- Produces: a second service in compose, which makes `TestAppsAgreeWithEachOther` run for the first time.

- [ ] **Step 1: Write the Dockerfile**

Three stages, mirroring `apps/go/Dockerfile`'s shape: build the SPA, build the server, thin runtime. Use `node:22-alpine` and `corepack enable`, and copy `pnpm-lock.yaml` before installing so the layer caches. `ENV PORT=9002`, `EXPOSE 9002`.

- [ ] **Step 2: Add the compose service**

```yaml
  node:
    build: ./apps/node
    ports: ["9002:9002"]
    environment:
      PORT: "9002"
```

- [ ] **Step 3: Run the equality layer for the first time**

```bash
make verify
```

`TestAppsAgreeWithEachOther` has never executed against two apps. Expect it to find something. When it does, the failure names the step and both apps and prints both normalized bodies.

For each disagreement, decide which of three things it is, and say so in your report:

1. **A bug in the Node port.** Fix the port.
2. **A hole in the contract**, where both apps are defensible. Fix the contract, fix whichever app now disagrees with it, and add the question to `CONTRACT-LOG.md`. This is the most valuable outcome of the plan.
3. **A bug in the harness**, most likely the normalizer treating something as stable that is not. Fix the harness.

Do not resolve a disagreement by changing the Node app to match Go without deciding which of the three it was. That is how a contract hole gets papered over.

- [ ] **Step 4: Add `make dev-node` and update the READMEs**

`make dev-node` mirrors `make dev-go`. The root `README.md` gains Node to the app matrix and the run instructions. `apps/node/README.md` is short: what the app is, `make dev-node`, the port.

- [ ] **Step 5: Full verification**

```bash
make check-clean && ./scripts/check-clean-test.sh | tail -1
make verify; echo "exit: $?"
cd apps/go && go test ./... -count=1
```

All green, no containers left running, and `git status` clean. Confirm `apps/node/CONTRACT-LOG.md` is committed and carries either entries or your explicit statement.

- [ ] **Step 6: Commit**

```bash
git add -A
git commit -m "feat(node): containerise and switch on cross-app equality

Two apps in compose means TestAppsAgreeWithEachOther runs for the first
time. The contract log records every question the contract could not
answer, which is the list of fixes that must land before plan 4."
```

---

## Done when

- `make verify` brings up both apps, runs all three harness layers including cross-app equality, and exits 0.
- `contract/openapi.yaml` declares `created_at asc, id asc` for members and resources, and `apps/go` and `apps/node` both implement it.
- `SECURITY-LAYER-EVIDENCE.md` records each security invariant observed failing against a planted defect.
- `apps/node/CONTRACT-LOG.md` exists and carries either the questions the contract could not answer, or the explicit statement that `apps/go/handlers/` and `apps/go/store/memory/` were never opened.
- The nine manual frontend checks in Task 6 are recorded as done.
- `make check-clean` passes and no Everscribe dependency exists anywhere.

Plan 4 adds Rust and vanilla TS on 9003, against a contract that Plan 3 has now actually tested.
