# Security layer evidence

This file is a checked-in record that the conformance harness's security
checks actually fail when the security they check for is broken. A green
suite proves nothing on its own: it could be green because the app is
correct, or green because the check never ran, never asserted the right
thing, or was never wired up. The equality layer (layer 2) cannot catch
this either, because it only compares the four apps to each other; a hole
present in all four the same way sails through equality untouched.

For each row below, a real defect was planted in a scratch run of the Go
app's source (never committed), the harness was run against the mutated
app, the result was recorded, and the app was restored to its pristine
state before the next row. `apps/go` is unmodified in this commit; `git
status --porcelain apps/go` is empty.

## Results

| # | Defect planted | Failing test | First line of failure |
|---|---|---|---|
| 1 | Dropped the org filter in `RemoveMember` | `TestCrossTenantMemberWrites` | `Not equal: expected: 404, actual: 204` (`DELETE /api/members/u_alan`) |
| 2 | Dropped the org filter in `UpdateMemberRole` | `TestCrossTenantMemberWrites` | `Not equal: expected: 404, actual: 200` (`PUT /api/members/u_alan/role`) |
| 3 | Dropped the org filter in `ListOrgResources` | `TestListRoutesServeOnlyTheActorsOrganization` | `Not equal: expected: "org_acme", actual: "org_globex"` |
| 4 | Dropped the org filter in `ListOrgMembers` | `TestListRoutesServeOnlyTheActorsOrganization` | `Not equal: expected: "org_acme", actual: "org_globex"` |
| 5 | Made `ApproveRequest` a no-op returning the unchanged row | `TestMemberCannotUseAdminRoutes` | `Not equal: expected: 200, actual: 409` ("admin must be able to revoke an approved request") |
| 6 | Made `DecommissionResource` a no-op | **none. See "Open hole" below.** | n/a |
| 7 | Removed `requireActor` from `POST /api/requests/{id}/revoke` | `TestMissingActorIs401` | `Not equal: expected: 401, actual: 403` (`POST /api/requests/req_acme_1/revoke`) |
| 8 | Removed `requireAdmin` from `POST /api/resources/{id}/decommission` | `TestMemberCannotUseAdminRoutes` | `Not equal: expected: 403, actual: 200` (`u_linus: POST /api/resources/{id}/decommission`) |
| 9 | Seeded `req_acme_1` as `approved` | `TestLifecycleRunsCleanAgainstEveryApp` (the `requirePristine` guard it runs first) | `Not equal: expected: "pending", actual: "approved"` ("req_acme_1 must start pending.") |
| 10 | Seeded `r_acme_s3` with a decommission timestamp | `TestLifecycleRunsCleanAgainstEveryApp` (the `requirePristine` guard it runs first) | `Expected nil, but got: "2026-03-14T10:00:00Z"` ("r_acme_s3 must start live.") |

9 of 10 planted defects were caught. Row 6 was not.

## Open hole: defect 6 produced no failure

Making `DecommissionResource` a no-op (the resource is found, ownership is
checked, but `decommission_at` is never set and the unchanged row is
returned with 200) passed the entire suite: `go test ./... -count=1`
reported `ok`, zero failures, on both the mutated run and the restored
pristine re-check.

Every existing test that touches decommission only checks the HTTP status
code of the decommission call itself (200 for an admin, 403 for a member,
404 cross-tenant), never that the resource was actually marked
decommissioned. Nothing then re-reads the resource to check
`decommission_at`, and nothing requests access to a freshly-decommissioned
resource to confirm the 409 the contract promises
("`POST /api/resources/{id}/requests` ... Fails with 409 if the resource
has been decommissioned"). That second gap is not incidental: the
coverage matrix (`contract/conformance/COVERAGE-MATRIX.md`) independently
finds `POST /api/resources/{id}/requests` x "409 for an invalid state
transition" as NOT COVERED, and this is exactly the defect that gap lets
through.

The test that should have caught this does not exist yet. The fix is not
a new assertion added quietly here; it belongs to closing the matrix's
NOT COVERED cell for `POST /api/resources/{id}/requests` under the 409
column, most directly by requesting access to a resource immediately
after decommissioning it and asserting 409, and secondarily by asserting
`decommission_at` is non-null on the decommission response itself. Per
this task's scope, no such test is added here; it is reported as an open
hole for a deliberate decision, not silently patched.

## Run metadata

- Date: 2026-09-11
- Run against commit: `c13e5cc29ca6b1b4b0af26666e73f556fad9fd00`
- Harness: `cd contract/conformance && go test ./... -count=1`, against
  the Go app alone on `:9001` (`CONFORMANCE_APPS` unset; node, rust, and
  python do not exist yet, so the harness only discovers `go` and skips
  `TestAppsAgreeWithEachOther`, which needs at least two apps).
