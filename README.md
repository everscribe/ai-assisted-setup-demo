# ai-assisted-setup-demo

A deliberately **un-instrumented** multi-tenant SaaS demo. The point of
this repo is to be a clean target for Everscribe's auto-instrument
feature: clone it, run the hosted instrument flow against it, and watch
the AI agent:

- walk the Go handlers and identify the auditable ones,
- propose an event name + tenant source + rationale for each,
- open a pull request that wires up the Everscribe SDK (recorder +
  middleware + per-handler `e.Action` calls), and
- drop the `<AuditTrail />` React component into the SPA so you can see
  the events in your own UI.

Nothing in this repo references Everscribe today. That's the contract -
the auto-instrument is responsible for adding all of it.

## Run it locally

You need Go 1.22+ and Node 18+ for the SPA build.

```bash
# install web deps, build the SPA, run the Go server (port 9001)
make dev
```

Then open <http://localhost:9001>. Pick one of the four seeded users
from the login screen and click around - request access to a resource,
approve as an admin, leave review notes, revoke previously-granted
access, register new resources, manage members.

Domain: internal access portal. Organizations (tenants) manage who has
access to internal resources - databases, production environments,
customer-data tables, third-party SaaS consoles. Members file an access
request with a reason; admins approve, deny, or revoke. Every mutating
action is a real-world security event that belongs in an audit log.

The API responds at `/api/*`. A health probe:

```bash
curl http://localhost:9001/api/health
# {"status":"ok"}
```

For a faster inner loop, run the Vite dev server (port 5173) against
the Go API in another terminal:

```bash
# terminal 1 - Go API
go run .

# terminal 2 - Vite with hot reload, /api proxied to the Go server
cd web && npm run dev
```

## What the auto-instrument should produce

When you point Everscribe's auto-instrument feature at this repo, expect it to:

1. Add `github.com/everscribe/sdk-go` to `go.mod`.
2. Initialize a recorder + register the audit middleware in `main.go`.
3. Add an `e.Action = "…"` + `e.TenantID = me.OrganizationID` call in
   each mutating handler - covering at least:
   - `user.login`, `user.logout`
   - `member.invited`, `member.removed`, `member.role.changed`
   - `resource.registered`, `resource.decommissioned`
   - `access.requested`, `access.approved`, `access.denied`, `access.revoked`
   - `comment.added`
4. Add `@everscribe/components-react` to `web/package.json`.
5. Drop an `<AuditTrail />` block into the SPA with a `tokenEndpoint`
   pointing at a new `/api/embed-token/*` route the PR also adds.

Each candidate's tenant source should resolve to `me.OrganizationID`
(the caller's organization) for everything except `user.login`, where
the actor isn't bound to a tenant yet.

## What it should NOT do

- It must not touch the in-memory store or domain types.
- It must not change the routing table beyond adding new endpoints
  (e.g. `/api/embed-token/*`).
- It must not modify the React frontend's existing components - only add
  the audit-trail block.

## License

MIT
