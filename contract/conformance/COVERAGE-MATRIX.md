# Security invariant coverage matrix

Four reviews in a row found a security-test gap by looking at one corner
of the contract at a time: an admin-gate check covering four of eight
admin routes, a tenant-scoping check covering two of four list routes,
a whole task (Task 2 of this plan) that existed only because member
writes had no test at all, and a reviewer finding four more routes with
a role-gate test but no tenant-boundary test. Each was found by someone
who happened to look. Nobody had enumerated the full route x invariant
grid and checked every cell.

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
`contract/conformance/*_test.go`, not inferred from test names or
comments.

## Update: the seven gaps are closed

The first version of this file found seven NOT COVERED cells and closed
none of them on purpose, so the decision to close them could be made in
one piece rather than found one at a time. That decision was made: all
seven are now covered by new or extended tests in `security_test.go`,
and each new test was proven able to fail (see
`SECURITY-LAYER-EVIDENCE.md`) before being trusted. The grid below
reflects that closed state.

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
| 12 | `POST /resources/{id}/requests` | Covered - `TestMissingActorIs401` | N/A - "any organization member can request access" per spec | Covered - `TestCrossTenantMemberInitiatedWritesAre404WithPositiveControls` | Covered - `TestRequestingAccessToDecommissionedResourceIs409` |
| 13 | `POST /requests/{id}/approve` | Covered - `TestMissingActorIs401` | Covered - `TestMemberCannotUseAdminRoutes` | Covered - `TestCrossTenantWritesAre404WithPositiveControls` | Covered - `TestDeniedRequestCannotBeApproved` (approving a denied request) and `TestApprovingNonPendingRequestIs409` (approving an already-approved request and approving a revoked request); all three non-pending states a request can be in are now asserted |
| 14 | `POST /requests/{id}/deny` | Covered - `TestMissingActorIs401` | Covered - `TestMemberCannotUseAdminRoutes` | Covered - `TestCrossTenantWritesAre404WithPositiveControls` | Covered - `TestDenyingNonPendingRequestIs409` (denying an already-denied request, denying an approved request, and denying a revoked request); all three non-pending states are now asserted |
| 15 | `POST /requests/{id}/revoke` | Covered - `TestMissingActorIs401` | Covered - `TestMemberCannotUseAdminRoutes` | Covered - `TestCrossTenantWritesAre404WithPositiveControls` | Covered - `TestRevokingNonApprovedRequestIs409` (revoking an already-revoked request, revoking a request that was never approved, and revoking a denied request); all three non-approved states are now asserted |
| 16 | `GET /requests/{id}/comments` | Covered - `TestMissingActorIs401` | N/A - any member can list | Covered - `TestCrossTenantReadsAre404WithPositiveControls` | N/A - no state transition |
| 17 | `POST /requests/{id}/comments` | Covered - `TestMissingActorIs401` | N/A - no "admin only" in spec, any member can comment | Covered - `TestCrossTenantMemberInitiatedWritesAre404WithPositiveControls` | N/A - plain insert, no prior state to conflict with |
| 18 | `GET /health` | N/A - anonymous route (`security: []`) | N/A - no admin gate | N/A - no target id | N/A - no state transition |

## Caveat on row 10 (decommission) and 409

Decommission's own response codes in the contract are 200/401/403/404/405;
there is no 409 for decommissioning something already decommissioned, and
the store has no status check to violate, so N/A on row 10's 409 cell is
correct against the contract as written. The 409 the contract actually
promises for decommissioning sits one hop over, on row 12: "existing
access requests are preserved; new access requests against the resource
fail with 409" is `POST /api/resources/{id}/requests`'s conflict case.
That cell is now covered by `TestRequestingAccessToDecommissionedResourceIs409`,
which requests access to a live resource (positive control), decommissions
it, and requests access again expecting 409. `SECURITY-LAYER-EVIDENCE.md`
records that this exact test now catches the no-op `DecommissionResource`
defect that previously passed the whole suite silently.

## What closed each gap

- Row 12, 404: `TestCrossTenantMemberInitiatedWritesAre404WithPositiveControls`
  (new). Requests access to, and comments on, an own-tenant fixture as a
  positive control, then probes `POST /api/resources/r_globex_k8s/requests`
  and `POST /api/requests/req_globex_1/comments` as an acme member,
  expecting 404 on both.
- Row 12, 409: `TestRequestingAccessToDecommissionedResourceIs409` (new,
  described above).
- Row 14, 404 and row 15, 404: `TestCrossTenantWritesAre404WithPositiveControls`
  (extended). Deny and revoke positive controls were added alongside the
  existing decommission and approve ones, and `req_globex_1/deny` and
  `req_globex_1/revoke` were added to the cross-tenant probe list. The
  org filter runs before the store's status check on every one of these
  mutations, so the probe 404s regardless of req_globex_1's real
  (seeded-approved) status; nothing about it is mutated.
- Row 14, 409: `TestDenyingNonPendingRequestIs409` (new, later widened; see
  "Update: narrower than the name claimed" below). Denies a fresh pending
  request (positive control), then denies the same, now-denied request
  again, expecting 409, and separately approves a second fresh request
  then denies it, also expecting 409.
- Row 15, 409: `TestRevokingNonApprovedRequestIs409` (new, later widened;
  see below). Approves then revokes a fresh request (both positive
  controls), then revokes the same, now-revoked request again, expecting
  409, and separately revokes a second fresh, still-pending request with
  no approve step at all, also expecting 409.
- Row 17, 404: covered by the same
  `TestCrossTenantMemberInitiatedWritesAre404WithPositiveControls` as
  row 12.
- Row 13, 409: `TestDeniedRequestCannotBeApproved` (existing, unchanged)
  plus `TestApprovingNonPendingRequestIs409` (new; see "Update: the
  approve guard's blind spot" below).

## Update: rows 14 and 15's 409 cells were narrower than the grid claimed

Marking a cell Covered names a test, not the full precondition the
contract states. A reviewer found that both `TestDenyingNonPendingRequestIs409`
and `TestRevokingNonApprovedRequestIs409`, as first written, only ever
re-entered the state the request was already in (deny-a-denied,
revoke-a-revoked) and never exercised a transition from a different
wrong state. Two planted defects proved this was exploitable, not
theoretical: a `DenyRequest` guard of `== StatusDenied` instead of
`!= StatusPending` (lets an approved request be denied) and a
`RevokeRequest` guard of `== StatusRevoked` instead of `!= StatusApproved`
(lets a pending request be revoked directly) both passed the entire
suite, including the test named for that exact invariant.

Both tests were widened with an additional case each rather than
replaced, so the original same-state coverage was not displaced: deny
now also covers approved-then-denied, and revoke now also covers
pending-revoked-with-no-approval. Both original mutations were re-run
against the widened tests and still fail (see `SECURITY-LAYER-EVIDENCE.md`
for both the reviewer's two new defects and this re-check). Rows 14 and
15's 409 cells in the grid above are annotated with what they now assert.

The general lesson carried forward: a cell marked Covered by a test whose
name matches the invariant can still be a test that only proves the
mutation it was written against. Widening an existing assertion, not just
adding new rows, is part of what "covered" has to mean here.

## Update: the approve guard's blind spot, and two more found the same way

Row 13's 409 cell relied entirely on `TestDeniedRequestCannotBeApproved`,
which only ever approves the seeded denied request. A guard written as
"status is denied" instead of "status is not pending" refuses exactly
that one wrong state and lets the other two (approved, revoked) through.
This was found by an accidental mutation during verification (a
find-and-replace landed on `ApproveRequest`'s guard instead of the
intended target) and proved deliberately by changing `ApproveRequest`'s
guard at its known line to `req.Status == models.StatusDenied`: the
mutation passed the entire suite, `TestDeniedRequestCannotBeApproved`
included, since that test never asks approve to touch an approved or
revoked request.

`TestApprovingNonPendingRequestIs409` (new) closes this: approving an
already-approved request, and approving a revoked request (approve, then
revoke, then approve again). `TestDeniedRequestCannotBeApproved` is
unchanged and still covers the denied path.

Once this pattern was named, the same question was asked of deny and
revoke, which had already been widened once (see the update above) to
cover two of their three wrong states each. Both had a second blind spot
left: `TestDenyingNonPendingRequestIs409` never asked deny to touch a
revoked request, and `TestRevokingNonApprovedRequestIs409` never asked
revoke to touch a denied request. Both are now closed the same way, with
a third case added to each test. All three transition routes now assert
all three of their wrong starting states, not two.

`SECURITY-LAYER-EVIDENCE.md` carries the mutation and failure line for
the approve defect and for both self-found gaps.

## Totals

- 18 routes x 4 invariants = 72 cells.
- Covered: 38.
- N/A: 34.
- NOT COVERED: 0.

No new tests were written to close any gap that was not on this list, and
no gap on this list was left open.

## What this grid is not

"NOT COVERED: 0" is a claim about four security invariants on eighteen
routes, and nothing else. The grid says nothing about 400-class request
validation (required fields, trimming, wrong types, the null rule, and
unknown fields, which `validation_test.go` covers), nothing about
protocol-level rules (trailing slashes, case sensitivity, HEAD and
OPTIONS, Content-Type, the Allow header, the 401-before-403-before-400
check order), and nothing about ordering or the idempotence of a repeated
decommission. Those live in other tests and in cross-app equality. A full
grid across every rule in the contract would be a different and much
larger document; read this one's totals as "the security columns are
closed", not as "the contract is fully covered".
