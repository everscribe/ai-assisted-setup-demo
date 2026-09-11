// One function per route, plus the shared helpers every handler builds
// on. Every status code, body shape, trimming rule, and precedence rule
// here comes from contract/openapi.yaml, not from the Go app.
import type { Request, Response } from "express";
import type { User } from "./models.js";
import { ErrConflict, ErrForbidden, ErrNotFound, type Store } from "./store.js";

export function fail(res: Response, status: number, message: string): void {
  res.status(status).json({ error: message });
}

// Maps a rejected Store call to its contract status. Anything else is a
// bug in this app, not a modeled error, so it is a 500.
export function failStore(res: Response, err: unknown): void {
  if (err === ErrNotFound) return fail(res, 404, "not found");
  if (err === ErrForbidden) return fail(res, 403, "forbidden");
  if (err === ErrConflict) return fail(res, 409, "conflict");
  fail(res, 500, "internal error");
}

export async function requireActor(req: Request, res: Response, store: Store): Promise<User | null> {
  const id = req.header("X-User-ID");
  if (!id) {
    fail(res, 401, "unauthorized");
    return null;
  }
  try {
    return await store.getUser(id);
  } catch {
    fail(res, 401, "unauthorized");
    return null;
  }
}

export function requireAdmin(res: Response, user: User): boolean {
  if (user.role !== "admin") {
    fail(res, 403, "admin role required");
    return false;
  }
  return true;
}

// req.body is raw text (see router.ts): body parsing never runs ahead
// of auth/role checks, so a malformed body cannot preempt a 403.
function parseJSON(raw: unknown): unknown {
  if (typeof raw !== "string" || raw === "") return undefined;
  try {
    return JSON.parse(raw);
  } catch {
    return undefined;
  }
}

function isObject(v: unknown): v is Record<string, unknown> {
  return typeof v === "object" && v !== null && !Array.isArray(v);
}

// Express 5 types a route param as string | string[] to allow for
// repeated segments; none of our patterns repeat one, so this is a
// type-level accommodation only.
function idParam(req: Request): string {
  const id = req.params.id;
  return Array.isArray(id) ? id[0] : id;
}

type FieldResult = { ok: true; value: string } | { ok: false; kind: "invalid" | "required" };

// A required field, absent or wrong-typed distinguished from present
// but blank (after an optional trim), since the contract gives those
// two failures different messages on most routes.
function requiredStringField(body: Record<string, unknown>, key: string, trim: boolean): FieldResult {
  const raw = body[key];
  if (raw === undefined) return { ok: false, kind: "required" };
  if (typeof raw !== "string") return { ok: false, kind: "invalid" };
  const value = trim ? raw.trim() : raw;
  if (value === "") return { ok: false, kind: "required" };
  return { ok: true, value };
}

type OptionalFieldResult = { ok: true; value: string } | { ok: false };

// An optional, trimmed field: absent becomes "", present must be a
// string.
function optionalStringField(body: Record<string, unknown>, key: string): OptionalFieldResult {
  const raw = body[key];
  if (raw === undefined) return { ok: true, value: "" };
  if (typeof raw !== "string") return { ok: false };
  return { ok: true, value: raw.trim() };
}

// POST /login - anonymous.
export async function handleLogin(req: Request, res: Response, store: Store): Promise<void> {
  const body = parseJSON(req.body);
  if (!isObject(body)) return fail(res, 400, "invalid body");
  // Email is matched exactly: not trimmed, per the contract.
  const field = requiredStringField(body, "email", false);
  if (!field.ok && field.kind === "invalid") return fail(res, 400, "invalid body");
  if (!field.ok) return fail(res, 400, "email required");
  try {
    const user = await store.findUserByEmail(field.value);
    res.status(200).json(user);
  } catch {
    fail(res, 401, "no user with that email");
  }
}

// POST /logout
export async function handleLogout(req: Request, res: Response, store: Store): Promise<void> {
  const actor = await requireActor(req, res, store);
  if (!actor) return;
  res.status(204).end();
}

// GET /me
export async function handleMe(req: Request, res: Response, store: Store): Promise<void> {
  const actor = await requireActor(req, res, store);
  if (!actor) return;
  res.status(200).json(actor);
}

// GET /members
export async function handleListMembers(req: Request, res: Response, store: Store): Promise<void> {
  const actor = await requireActor(req, res, store);
  if (!actor) return;
  const members = await store.listOrgMembers(actor.organization_id);
  res.status(200).json(members);
}

// POST /members - admin only.
export async function handleInviteMember(req: Request, res: Response, store: Store): Promise<void> {
  const actor = await requireActor(req, res, store);
  if (!actor) return;
  if (!requireAdmin(res, actor)) return;

  const body = parseJSON(req.body);
  if (!isObject(body)) return fail(res, 400, "invalid body");

  const name = requiredStringField(body, "name", true);
  const email = requiredStringField(body, "email", true);
  if ((!name.ok && name.kind === "invalid") || (!email.ok && email.kind === "invalid")) {
    return fail(res, 400, "invalid body");
  }
  if (!name.ok || !email.ok) return fail(res, 400, "name and email required");

  const rawRole = body.role;
  const role = rawRole === "admin" || rawRole === "member" ? rawRole : "member";

  try {
    const member = await store.inviteMember(actor.organization_id, name.value, email.value, role);
    res.status(201).json(member);
  } catch (err) {
    failStore(res, err);
  }
}

// DELETE /members/{id} - admin only.
export async function handleRemoveMember(req: Request, res: Response, store: Store): Promise<void> {
  const actor = await requireActor(req, res, store);
  if (!actor) return;
  if (!requireAdmin(res, actor)) return;
  try {
    await store.removeMember(actor.organization_id, idParam(req));
    res.status(204).end();
  } catch (err) {
    failStore(res, err);
  }
}

// PUT /members/{id}/role - admin only.
export async function handleUpdateMemberRole(req: Request, res: Response, store: Store): Promise<void> {
  const actor = await requireActor(req, res, store);
  if (!actor) return;
  if (!requireAdmin(res, actor)) return;

  const body = parseJSON(req.body);
  if (!isObject(body)) return fail(res, 400, "invalid body");

  // Compared exactly, not trimmed: any value other than the two
  // literals fails the same way, whether absent, mistyped, or padded.
  const role = body.role;
  if (role !== "admin" && role !== "member") return fail(res, 400, "role must be admin or member");

  try {
    const member = await store.updateMemberRole(actor.organization_id, idParam(req), role);
    res.status(200).json(member);
  } catch (err) {
    failStore(res, err);
  }
}

// GET /resources
export async function handleListResources(req: Request, res: Response, store: Store): Promise<void> {
  const actor = await requireActor(req, res, store);
  if (!actor) return;
  const resources = await store.listOrgResources(actor.organization_id);
  res.status(200).json(resources);
}

// POST /resources - admin only.
export async function handleRegisterResource(req: Request, res: Response, store: Store): Promise<void> {
  const actor = await requireActor(req, res, store);
  if (!actor) return;
  if (!requireAdmin(res, actor)) return;

  const body = parseJSON(req.body);
  if (!isObject(body)) return fail(res, 400, "invalid body");

  const name = requiredStringField(body, "name", true);
  if (!name.ok && name.kind === "invalid") return fail(res, 400, "invalid body");
  if (!name.ok) return fail(res, 400, "name required");

  const description = optionalStringField(body, "description");
  if (!description.ok) return fail(res, 400, "invalid body");

  const sensitivityField = optionalStringField(body, "sensitivity");
  if (!sensitivityField.ok) return fail(res, 400, "invalid body");
  const sensitivity = sensitivityField.value === "" ? "internal" : sensitivityField.value;

  try {
    const resource = await store.registerResource(
      actor.organization_id,
      name.value,
      description.value,
      sensitivity,
      actor.id,
    );
    res.status(201).json(resource);
  } catch (err) {
    failStore(res, err);
  }
}

// POST /resources/{id}/decommission - admin only.
export async function handleDecommissionResource(req: Request, res: Response, store: Store): Promise<void> {
  const actor = await requireActor(req, res, store);
  if (!actor) return;
  if (!requireAdmin(res, actor)) return;
  try {
    const resource = await store.decommissionResource(actor.organization_id, idParam(req));
    res.status(200).json(resource);
  } catch (err) {
    failStore(res, err);
  }
}

// GET /resources/{id}/requests
export async function handleListResourceRequests(req: Request, res: Response, store: Store): Promise<void> {
  const actor = await requireActor(req, res, store);
  if (!actor) return;
  try {
    const requests = await store.listResourceRequests(actor.organization_id, idParam(req));
    res.status(200).json(requests);
  } catch (err) {
    failStore(res, err);
  }
}

// POST /resources/{id}/requests - any member, no admin check. The body
// is read and validated before the store is touched, so an unknown
// resource id with a bad body is 400, not 404.
export async function handleRequestAccess(req: Request, res: Response, store: Store): Promise<void> {
  const actor = await requireActor(req, res, store);
  if (!actor) return;

  const body = parseJSON(req.body);
  if (!isObject(body)) return fail(res, 400, "invalid body");

  const reason = requiredStringField(body, "reason", true);
  if (!reason.ok && reason.kind === "invalid") return fail(res, 400, "invalid body");
  if (!reason.ok) return fail(res, 400, "reason required");

  try {
    const request = await store.requestAccess(actor.organization_id, idParam(req), actor.id, reason.value);
    res.status(201).json(request);
  } catch (err) {
    failStore(res, err);
  }
}

// POST /requests/{id}/approve - admin only.
export async function handleApproveRequest(req: Request, res: Response, store: Store): Promise<void> {
  const actor = await requireActor(req, res, store);
  if (!actor) return;
  if (!requireAdmin(res, actor)) return;
  try {
    const request = await store.approveRequest(actor.organization_id, idParam(req), actor.id);
    res.status(200).json(request);
  } catch (err) {
    failStore(res, err);
  }
}

// POST /requests/{id}/deny - admin only.
export async function handleDenyRequest(req: Request, res: Response, store: Store): Promise<void> {
  const actor = await requireActor(req, res, store);
  if (!actor) return;
  if (!requireAdmin(res, actor)) return;
  try {
    const request = await store.denyRequest(actor.organization_id, idParam(req), actor.id);
    res.status(200).json(request);
  } catch (err) {
    failStore(res, err);
  }
}

// POST /requests/{id}/revoke - admin only.
export async function handleRevokeRequest(req: Request, res: Response, store: Store): Promise<void> {
  const actor = await requireActor(req, res, store);
  if (!actor) return;
  if (!requireAdmin(res, actor)) return;
  try {
    const request = await store.revokeRequest(actor.organization_id, idParam(req), actor.id);
    res.status(200).json(request);
  } catch (err) {
    failStore(res, err);
  }
}

// GET /requests/{id}/comments
export async function handleListRequestComments(req: Request, res: Response, store: Store): Promise<void> {
  const actor = await requireActor(req, res, store);
  if (!actor) return;
  try {
    const comments = await store.listRequestComments(actor.organization_id, idParam(req));
    res.status(200).json(comments);
  } catch (err) {
    failStore(res, err);
  }
}

// POST /requests/{id}/comments - no admin check.
export async function handleAddComment(req: Request, res: Response, store: Store): Promise<void> {
  const actor = await requireActor(req, res, store);
  if (!actor) return;

  const body = parseJSON(req.body);
  if (!isObject(body)) return fail(res, 400, "invalid body");

  const commentBody = requiredStringField(body, "body", true);
  if (!commentBody.ok && commentBody.kind === "invalid") return fail(res, 400, "invalid body");
  if (!commentBody.ok) return fail(res, 400, "body required");

  try {
    const comment = await store.addComment(actor.organization_id, idParam(req), actor.id, commentBody.value);
    res.status(201).json(comment);
  } catch (err) {
    failStore(res, err);
  }
}

// GET /health - anonymous.
export function handleHealth(_req: Request, res: Response): void {
  res.status(200).json({ status: "ok" });
}
