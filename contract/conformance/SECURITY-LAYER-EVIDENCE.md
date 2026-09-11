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
| 6 | Made `DecommissionResource` a no-op | `TestRequestingAccessToDecommissionedResourceIs409` (added after this row was first an open hole; see "Update" below) | `Not equal: expected: 409, actual: 201` |
| 7 | Removed `requireActor` from `POST /api/requests/{id}/revoke` | `TestMissingActorIs401` | `Not equal: expected: 401, actual: 403` (`POST /api/requests/req_acme_1/revoke`) |
| 8 | Removed `requireAdmin` from `POST /api/resources/{id}/decommission` | `TestMemberCannotUseAdminRoutes` | `Not equal: expected: 403, actual: 200` (`u_linus: POST /api/resources/{id}/decommission`) |
| 9 | Seeded `req_acme_1` as `approved` | `TestLifecycleRunsCleanAgainstEveryApp` (the `requirePristine` guard it runs first) | `Not equal: expected: "pending", actual: "approved"` ("req_acme_1 must start pending.") |
| 10 | Seeded `r_acme_s3` with a decommission timestamp | `TestLifecycleRunsCleanAgainstEveryApp` (the `requirePristine` guard it runs first) | `Expected nil, but got: "2026-03-14T10:00:00Z"` ("r_acme_s3 must start live.") |

10 of 10 planted defects are now caught. Row 6 was the exception on first
run; see "Update" below for how it was closed.

## Update: the row 6 open hole is closed

The first version of this file recorded that making `DecommissionResource`
a no-op (the resource is found, ownership is checked, but
`decommission_at` is never set and the unchanged row is returned with
200) passed the entire suite with zero failures. It was reported as an
open hole rather than silently dropped, naming the missing test as one
that requests access to a resource immediately after decommissioning it
and asserts 409.

That test now exists: `TestRequestingAccessToDecommissionedResourceIs409`
registers a resource, requests access to it while live (positive
control, expects 201), decommissions it, then requests access again and
asserts 409. Re-running the same `DecommissionResource` no-op mutation
against the app with this test in place fails exactly there:

```
security_test.go:498:
    Error: Not equal:
           expected: 409
           actual  : 201
Test: TestRequestingAccessToDecommissionedResourceIs409/go
```

This also closed the matching gap in `COVERAGE-MATRIX.md`: `POST
/api/resources/{id}/requests` x "409 for an invalid state transition" was
NOT COVERED, and it was the same missing check that let the no-op through.

## Update: six more NOT COVERED cells closed

A reviewer separately found four routes with a role-gate test but no
tenant-boundary test: `POST /requests/{id}/deny`, `POST
/requests/{id}/revoke`, `POST /resources/{id}/requests`, and `POST
/requests/{id}/comments`. Combined with the two missing 409 tests for
deny and revoke, this made seven NOT COVERED cells in the coverage
matrix (the resource-requests 409 cell above already counted as the
eighth, now first, gap closed).

Each new or extended test was proven able to fail before being trusted,
the same way as the ten defects above: planted the matching mutation on
a scratch run of `apps/go`, ran the harness, recorded the failure,
restored, confirmed clean.

| Gap closed | New/extended test | Mutation planted to prove it fails | First line of failure |
|---|---|---|---|
| `POST /requests/{id}/deny`, 404 cross-tenant | `TestCrossTenantWritesAre404WithPositiveControls` (extended) | Dropped the org filter in `DenyRequest` | `Not equal: expected: 404, actual: 200` |
| `POST /requests/{id}/revoke`, 404 cross-tenant | `TestCrossTenantWritesAre404WithPositiveControls` (extended) | Dropped the org filter in `RevokeRequest` | `Not equal: expected: 404, actual: 200` |
| `POST /resources/{id}/requests`, 404 cross-tenant | `TestCrossTenantMemberInitiatedWritesAre404WithPositiveControls` (new) | Dropped the org filter in `RequestAccess` | `Not equal: expected: 404, actual: 201` |
| `POST /requests/{id}/comments`, 404 cross-tenant | `TestCrossTenantMemberInitiatedWritesAre404WithPositiveControls` (new) | Dropped the org filter in `AddComment` | `Not equal: expected: 404, actual: 201` (`POST /api/requests/req_globex_1/comments`) |
| `POST /requests/{id}/deny`, 409 invalid transition | `TestDenyingNonPendingRequestIs409` (new) | Made `DenyRequest` a no-op returning the unchanged row | `Not equal: expected: 409, actual: 200` |
| `POST /requests/{id}/revoke`, 409 invalid transition | `TestRevokingNonApprovedRequestIs409` (new) | Made `RevokeRequest` a no-op returning the unchanged row | `Not equal: expected: 409, actual: 200` |

All six new tests, plus `TestRequestingAccessToDecommissionedResourceIs409`,
also pass clean against the restored, pristine app: the full suite ends
`ok`, zero failures, `apps/go` unmodified.

## Update: the deny and revoke 409 tests were too narrow

A reviewer found that `TestDenyingNonPendingRequestIs409` and
`TestRevokingNonApprovedRequestIs409`, as written above, only ever
re-entered the state a request was already in: mint, transition once,
repeat the same transition. Neither test exercised a transition starting
from a different wrong state, and the reviewer proved this was
exploitable with two planted defects, both of which passed the entire
suite including the test named for that invariant:

- `RevokeRequest`'s guard changed from `req.Status != StatusApproved` to
  `req.Status == StatusRevoked`. Revoking a freshly minted pending
  request returned 200.
- `DenyRequest`'s guard changed from `req.Status != StatusPending` to
  `req.Status == StatusDenied`. Denying an already-approved request
  returned 200.

Both tests were widened with an additional case rather than replaced, so
the original same-state coverage stays in place: the revoke test now
also revokes a fresh, still-pending request with no approve step at all,
and the deny test now also approves a fresh request then denies it.
Reproducing the reviewer's exact two defects against the widened suite:

| Defect (reviewer's exact mutation) | Failing test | First line of failure |
|---|---|---|
| `RevokeRequest`: `req.Status != StatusApproved` to `req.Status == StatusRevoked` | `TestRevokingNonApprovedRequestIs409` | `Not equal: expected: 409, actual: 200` ("revoking a request that was never approved") |
| `DenyRequest`: `req.Status != StatusPending` to `req.Status == StatusDenied` | `TestDenyingNonPendingRequestIs409` | `Not equal: expected: 409, actual: 200` ("denying an approved request") |

Both original same-state mutations (making `DenyRequest` and
`RevokeRequest` no-ops, the ones this file first recorded for rows 5-6 of
the ten-defect table's neighbors) were re-run against the widened tests
to confirm the new cases had not displaced the old coverage:

| Original mutation | Failing test | First line of failure |
|---|---|---|
| `DenyRequest` made a no-op | `TestDenyingNonPendingRequestIs409` | `Not equal: expected: 409, actual: 200` ("denying an already-denied request") |
| `RevokeRequest` made a no-op | `TestRevokingNonApprovedRequestIs409` | `Not equal: expected: 409, actual: 200` ("revoking an already-revoked request") |

After each of the four mutations above, `apps/go` was restored and a
pristine run confirmed the full suite passes clean before moving to the
next one.

## Run metadata

- Date: 2026-09-11
- Original ten-defect run against commit: `c13e5cc29ca6b1b4b0af26666e73f556fad9fd00`
- Seven-gap-closing update run against commit: `4683280d7bddd50cbc369c6e9a87f9f6e4424d17`
- Wrong-state-widening update run against commit: `94a39b5d2d40a135dc3a75495865a7b2789570c9`
- Harness: `cd contract/conformance && go test ./... -count=1`, against
  the Go app alone on `:9001` (`CONFORMANCE_APPS` unset; node, rust, and
  python do not exist yet, so the harness only discovers `go` and skips
  `TestAppsAgreeWithEachOther`, which needs at least two apps).
