import { randomBytes } from "node:crypto";
import type { AccessRequest, Comment, Organization, Resource, User } from "./models.js";
import { seedAccessRequests, seedComments, seedOrganizations, seedResources, seedUsers } from "./seed.js";
import { ErrConflict, ErrNotFound, type Store } from "./store.js";

function genId(prefix: string): string {
  return `${prefix}${randomBytes(4).toString("hex")}`;
}

function now(): string {
  return new Date().toISOString();
}

// Total order: domain key first, id ascending breaks ties. Comparing
// timestamps by parsed value (not string) keeps runtime-minted,
// fractional-second timestamps ordered correctly against seeded,
// whole-second ones.
function byCreatedAt<T extends { created_at: string; id: string }>(direction: "asc" | "desc") {
  const sign = direction === "asc" ? 1 : -1;
  return (a: T, b: T): number => {
    const diff = Date.parse(a.created_at) - Date.parse(b.created_at);
    if (diff !== 0) return sign * diff;
    return a.id < b.id ? -1 : a.id > b.id ? 1 : 0;
  };
}

// MemoryStore is the in-memory Store implementation. Maps are keyed by
// id; every row read out is a shallow copy so a caller mutating a
// returned object can never corrupt stored state or the seed fixture.
export class MemoryStore implements Store {
  private organizations = new Map<string, Organization>();
  private users = new Map<string, User>();
  private resources = new Map<string, Resource>();
  private requests = new Map<string, AccessRequest>();
  private comments = new Map<string, Comment>();

  async seed(): Promise<void> {
    this.organizations.clear();
    this.users.clear();
    this.resources.clear();
    this.requests.clear();
    this.comments.clear();
    for (const o of seedOrganizations) this.organizations.set(o.id, { ...o });
    for (const u of seedUsers) this.users.set(u.id, { ...u });
    for (const r of seedResources) this.resources.set(r.id, { ...r });
    for (const r of seedAccessRequests) this.requests.set(r.id, { ...r });
    for (const c of seedComments) this.comments.set(c.id, { ...c });
  }

  async listOrganizations(): Promise<Organization[]> {
    return Array.from(this.organizations.values()).map((o) => ({ ...o }));
  }

  async getOrganization(id: string): Promise<Organization> {
    const org = this.organizations.get(id);
    if (!org) throw ErrNotFound;
    return { ...org };
  }

  async getUser(id: string): Promise<User> {
    const user = this.users.get(id);
    if (!user) throw ErrNotFound;
    return { ...user };
  }

  // Emails are not unique, so every match is collected and the same
  // total order the list endpoints use decides: created_at asc, then
  // id asc. A Map iterates in insertion order, which happens to agree
  // today, but the rule is what the contract states so the rule is what
  // is applied.
  async findUserByEmail(email: string): Promise<User> {
    const hits = Array.from(this.users.values())
      .filter((u) => u.email === email)
      .sort(byCreatedAt("asc"));
    if (hits.length === 0) throw ErrNotFound;
    return { ...hits[0] };
  }

  async listOrgMembers(orgId: string): Promise<User[]> {
    return Array.from(this.users.values())
      .filter((u) => u.organization_id === orgId)
      .sort(byCreatedAt("asc"))
      .map((u) => ({ ...u }));
  }

  async inviteMember(orgId: string, name: string, email: string, role: string): Promise<User> {
    const user: User = {
      id: genId("u_"),
      name,
      email,
      role,
      organization_id: orgId,
      created_at: now(),
    };
    this.users.set(user.id, user);
    return { ...user };
  }

  async removeMember(orgId: string, userId: string): Promise<void> {
    const user = this.users.get(userId);
    if (!user || user.organization_id !== orgId) throw ErrNotFound;
    this.users.delete(userId);
  }

  async updateMemberRole(orgId: string, userId: string, role: string): Promise<User> {
    const user = this.users.get(userId);
    if (!user || user.organization_id !== orgId) throw ErrNotFound;
    const updated: User = { ...user, role };
    this.users.set(userId, updated);
    return { ...updated };
  }

  async listOrgResources(orgId: string): Promise<Resource[]> {
    return Array.from(this.resources.values())
      .filter((r) => r.organization_id === orgId)
      .sort(byCreatedAt("asc"))
      .map((r) => ({ ...r }));
  }

  async getResource(orgId: string, resourceId: string): Promise<Resource> {
    const resource = this.resources.get(resourceId);
    if (!resource || resource.organization_id !== orgId) throw ErrNotFound;
    return { ...resource };
  }

  async registerResource(orgId: string, name: string, description: string, sensitivity: string, registeredBy: string): Promise<Resource> {
    const resource: Resource = {
      id: genId("r_"),
      name,
      description,
      sensitivity,
      organization_id: orgId,
      registered_by: registeredBy,
      created_at: now(),
    };
    this.resources.set(resource.id, resource);
    return { ...resource };
  }

  async decommissionResource(orgId: string, resourceId: string): Promise<Resource> {
    const resource = this.resources.get(resourceId);
    if (!resource || resource.organization_id !== orgId) throw ErrNotFound;
    // Idempotent per the contract: an already-decommissioned resource
    // returns 200 again with decommission_at re-stamped, not 409.
    const updated: Resource = { ...resource, decommission_at: now() };
    this.resources.set(resourceId, updated);
    return { ...updated };
  }

  async listResourceRequests(orgId: string, resourceId: string): Promise<AccessRequest[]> {
    await this.getResource(orgId, resourceId);
    return Array.from(this.requests.values())
      .filter((r) => r.resource_id === resourceId && r.organization_id === orgId)
      .sort(byCreatedAt("desc"))
      .map((r) => ({ ...r }));
  }

  async getRequest(orgId: string, requestId: string): Promise<AccessRequest> {
    const request = this.requests.get(requestId);
    if (!request || request.organization_id !== orgId) throw ErrNotFound;
    return { ...request };
  }

  async requestAccess(orgId: string, resourceId: string, requestedBy: string, reason: string): Promise<AccessRequest> {
    const resource = await this.getResource(orgId, resourceId);
    if (resource.decommission_at) throw ErrConflict;
    const request: AccessRequest = {
      id: genId("req_"),
      resource_id: resourceId,
      organization_id: orgId,
      requested_by: requestedBy,
      reason,
      status: "pending",
      created_at: now(),
    };
    this.requests.set(request.id, request);
    return { ...request };
  }

  async approveRequest(orgId: string, requestId: string, approvedBy: string): Promise<AccessRequest> {
    const request = this.requests.get(requestId);
    if (!request || request.organization_id !== orgId) throw ErrNotFound;
    if (request.status !== "pending") throw ErrConflict;
    const updated: AccessRequest = { ...request, status: "approved", decided_by: approvedBy, decided_at: now() };
    this.requests.set(requestId, updated);
    return { ...updated };
  }

  async denyRequest(orgId: string, requestId: string, deniedBy: string): Promise<AccessRequest> {
    const request = this.requests.get(requestId);
    if (!request || request.organization_id !== orgId) throw ErrNotFound;
    if (request.status !== "pending") throw ErrConflict;
    const updated: AccessRequest = { ...request, status: "denied", decided_by: deniedBy, decided_at: now() };
    this.requests.set(requestId, updated);
    return { ...updated };
  }

  async revokeRequest(orgId: string, requestId: string, revokedBy: string): Promise<AccessRequest> {
    const request = this.requests.get(requestId);
    if (!request || request.organization_id !== orgId) throw ErrNotFound;
    if (request.status !== "approved") throw ErrConflict;
    const updated: AccessRequest = { ...request, status: "revoked", revoked_by: revokedBy, revoked_at: now() };
    this.requests.set(requestId, updated);
    return { ...updated };
  }

  async listRequestComments(orgId: string, requestId: string): Promise<Comment[]> {
    await this.getRequest(orgId, requestId);
    return Array.from(this.comments.values())
      .filter((c) => c.request_id === requestId && c.organization_id === orgId)
      .sort(byCreatedAt("asc"))
      .map((c) => ({ ...c }));
  }

  async addComment(orgId: string, requestId: string, authorId: string, body: string): Promise<Comment> {
    await this.getRequest(orgId, requestId);
    const comment: Comment = {
      id: genId("c_"),
      request_id: requestId,
      organization_id: orgId,
      author_id: authorId,
      body,
      created_at: now(),
    };
    this.comments.set(comment.id, comment);
    return { ...comment };
  }
}
