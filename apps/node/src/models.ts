// Field names, required-ness, and shapes come from the schemas in
// contract/openapi.yaml. Optional properties model fields the contract
// says are absent, not empty: for example decided_at is absent while a
// request is pending, not present as an empty string.

export interface Organization {
  id: string;
  name: string;
  created_at: string;
}

export interface User {
  id: string;
  name: string;
  email: string;
  // "admin" | "member" per the contract enum, kept as string to match
  // the Store interface, which takes role as a plain string too.
  role: string;
  organization_id: string;
  created_at: string;
}

export interface Resource {
  id: string;
  name: string;
  description: string;
  // Free-form label, not an enum. The contract recognises public,
  // internal, confidential, restricted, and defaults an omitted or
  // empty value to "internal", but stores whatever it is given.
  sensitivity: string;
  organization_id: string;
  registered_by: string;
  created_at: string;
  // Present only once the resource has been decommissioned.
  decommission_at?: string;
}

export interface AccessRequest {
  id: string;
  resource_id: string;
  organization_id: string;
  requested_by: string;
  reason: string;
  // "pending" | "approved" | "denied" | "revoked"
  status: string;
  // decided_by/decided_at are absent while pending.
  decided_by?: string;
  decided_at?: string;
  // revoked_by/revoked_at are present only once revoked.
  revoked_by?: string;
  revoked_at?: string;
  created_at: string;
}

export interface Comment {
  id: string;
  request_id: string;
  organization_id: string;
  author_id: string;
  body: string;
  created_at: string;
}
