import type { AccessRequest, Comment, Organization, Resource, User } from "./models.js";

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
