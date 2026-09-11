# Security invariant coverage matrix

Four reviews in a row found a security-test gap by looking at one corner
of the contract at a time: an admin-gate check covering four of eight
admin routes, a tenant-scoping check covering two of four list routes,
a whole task (Task 2 of this plan) that existed only because member
writes had no test at all, and now a reviewer finding four more routes
with a role-gate test but no tenant-boundary test. Each was found by
someone who happened to look. Nobody had enumerated the full route x
invariant grid and checked every cell.

This file is that grid. Rows are the eighteen routes `contract/openapi.yaml`
declares (`servers: - url: /api`, so a path below is `/api` + the route
path). Columns are the four security invariants named for this task:

- **401**: the route requires an actor; a request with no `X-User-ID`
  (or an unknown one) must get 401.
- **403**: the route is admin-only; a non-admin actor must get 403.
- **404**: the route names a specific tenant-scoped object by id in its
  path; an actor from another organization naming that id must get 404,
  identical to an unknown path.
- **409**: the route performs a state transition the store can refuse;
  an invalid transition must get 409.

Every cell is marked one of:

- **Covered - `TestName`**: a named test in `contract/conformance/*_test.go`
  asserts this exact invariant for this exact route.
- **N/A - reason**: the invariant does not apply to this route, with why.
- **NOT COVERED**: the invariant applies and no test in the harness
  asserts it.

Route list and "admin only" / "any member" designations are read off
`contract/openapi.yaml` directly. Coverage is read off the test bodies in
`contract/conformance/*_test.go` as of this task, not inferred from test
names or comments. No new tests were written to close any gap found here;
see the note at the bottom.

## The grid

| # | Route | 401 no actor | 403 non-admin on admin route | 404 cross-tenant target | 409 invalid transition |
|---|---|---|---|---|---|
| 1 | `POST /login` | N/A - anonymous route (`security: []`) | N/A - no admin gate | N/A - no target id | N/A - no state transition |
| 2 | `POST /logout` | Covered - `TestMissingActorIs401` | N/A - no admin gate | N/A - no target id | N/A - no state transition |
| 3 | `GET /me` | Covered - `TestMissingActorIs401` | N/A - no admin gate | N/A - spec: only ever 200 or 401, no lookup that can miss | N/A - no state transition |
| 4 | `GET /members` | Covered - `TestMissingActorIs401` | N/A - any member can list | N/A - list route, no single-item id; org scoping is a row-filter, verified by `TestListRoutesServeOnlyTheActorsOrganization` | N/A - no state transition |
| 5 | `POST /members` | Covered - `TestMissingActorIs401` | Covered - `TestMemberCannotUseAdminRoutes` | N/A - create route, no target id | N/A - no state transition |
| 6 | `DELETE /members/{id}` | Covered - `TestMissingActorIs401` | Covered - `TestMemberCannotUseAdminRoutes` | Covered - `TestCrossTenantMemberWrites` | N/A - delete either succeeds or 404s, no other state |
| 7 | `PUT /members/{id}/role` | Covered - `TestMissingActorIs401` | Covered - `TestMemberCannotUseAdminRoutes` | Covered - `TestCrossTenantMemberWrites` | N/A - an invalid role value is 400 (validation), not a store-level conflict |
| 8 | `GET /resources` | Covered - `TestMissingActorIs401` | N/A - any member can list | N/A - list route, no single-item id; org scoping verified by `TestListRoutesServeOnlyTheActorsOrganization` | N/A - no state transition |
| 9 | `POST /resources` | Covered - `TestMissingActorIs401` | Covered - `TestMemberCannotUseAdminRoutes` | N/A - create route, no target id | N/A - no state transition |
| 10 | `POST /resources/{id}/decommission` | Covered - `TestMissingActorIs401` | Covered - `TestMemberCannotUseAdminRoutes` | Covered - `TestCrossTenantWritesAre404WithPositiveControls` | N/A - the contract defines no 409 response for this route, and the store has no precondition to violate (see caveat below) |
| 11 | `GET /resources/{id}/requests` | Covered - `TestMissingActorIs401` | N/A - any member can list | Covered - `TestCrossTenantReadsAre404WithPositiveControls` | N/A - no state transition |
| 12 | `POST /resources/{id}/requests` | Covered - `TestMissingActorIs401` | N/A - "any organization member can request access" per spec | **NOT COVERED** | **NOT COVERED** |
| 13 | `POST /requests/{id}/approve` | Covered - `TestMissingActorIs401` | Covered - `TestMemberCannotUseAdminRoutes` | Covered - `TestCrossTenantWritesAre404WithPositiveControls` | Covered - `TestDeniedRequestCannotBeApproved` |
| 14 | `POST /requests/{id}/deny` | Covered - `TestMissingActorIs401` | Covered - `TestMemberCannotUseAdminRoutes` | **NOT COVERED** | **NOT COVERED** |
| 15 | `POST /requests/{id}/revoke` | Covered - `TestMissingActorIs401` | Covered - `TestMemberCannotUseAdminRoutes` | **NOT COVERED** | **NOT COVERED** |
| 16 | `GET /requests/{id}/comments` | Covered - `TestMissingActorIs401` | N/A - any member can list | Covered - `TestCrossTenantReadsAre404WithPositiveControls` | N/A - no state transition |
| 17 | `POST /requests/{id}/comments` | Covered - `TestMissingActorIs401` | N/A - no "admin only" in spec, any member can comment | **NOT COVERED** | N/A - plain insert, no prior state to conflict with |
| 18 | `GET /health` | N/A - anonymous route (`security: []`) | N/A - no admin gate | N/A - no target id | N/A - no state transition |

## Caveat on row 10 (decommission) and 409

Decommission's own response codes in the contract are 200/401/403/404/405;
there is no 409 for decommissioning something already decommissioned, and
the store has no status check to violate, so N/A on row 10's 409 cell is
correct against the contract as written. But the contract does promise a
409 one hop over: "existing access requests are preserved; new access
requests against the resource fail with 409" is `POST
/api/resources/{id}/requests`'s conflict case (row 12), and that cell is
NOT COVERED. `SECURITY-LAYER-EVIDENCE.md` in this same directory records
that a no-op `DecommissionResource` (never actually sets
`decommission_at`) passed the entire suite silently; this row is exactly
where a test would have caught it.

## Totals

- 18 routes x 4 invariants = 72 cells.
- Covered: 31.
- N/A: 34.
- **NOT COVERED: 7.**

## NOT COVERED, listed plainly

1. `POST /api/resources/{id}/requests`, 404 cross-tenant target. No test
   requests access to a resource id belonging to another organization.
2. `POST /api/resources/{id}/requests`, 409 invalid transition. No test
   requests access to a decommissioned resource and checks for 409.
3. `POST /api/requests/{id}/deny`, 404 cross-tenant target. No test denies
   a request id belonging to another organization.
   `TestCrossTenantWritesAre404WithPositiveControls` covers decommission
   and approve but not deny.
4. `POST /api/requests/{id}/deny`, 409 invalid transition. The lifecycle
   script (`sequence.go`) runs `POST /api/requests/req_acme_3/deny` (a
   seeded-denied request) but `TestLifecycleRunsCleanAgainstEveryApp`
   only asserts the response is non-empty and under 500; it never asserts
   the status is exactly 409.
5. `POST /api/requests/{id}/revoke`, 404 cross-tenant target. Same gap as
   deny: `TestCrossTenantWritesAre404WithPositiveControls` does not
   include revoke.
6. `POST /api/requests/{id}/revoke`, 409 invalid transition. No test
   revokes a request that is pending, denied, or already revoked and
   checks for 409.
7. `POST /api/requests/{id}/comments`, 404 cross-tenant target. No test
   comments on a request id belonging to another organization.

Per this task's scope, none of these seven cells are closed here. The
point of this file is to make the full picture visible in one place so
closing them is one deliberate decision, not a fifth round of a reviewer
finding one at a time.
