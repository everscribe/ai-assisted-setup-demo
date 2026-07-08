// Package models holds the JSON-serializable domain types shared between
// handlers, the store, and the React frontend.
//
// Every resource carries an OrganizationID - this is a multi-tenant app,
// and the organization is the tenant boundary. Handlers enforce that an
// actor can only see/mutate resources in their own organization.
//
// Domain: internal access portal. Organizations manage who has access to
// internal resources (databases, prod environments, customer data, third-
// party apps). Members request access; admins approve, deny, or revoke.
package models

import "time"

// Organization is a tenant. Each user belongs to exactly one organization;
// every resource (Resource, AccessRequest, Comment, Membership) is scoped
// to a single organization.
type Organization struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	CreatedAt time.Time `json:"created_at"`
}

// User is a person in the system. Role is "admin" or "member" - admins
// can manage memberships, register resources, and approve/deny/revoke
// access requests; members can request access and comment.
type User struct {
	ID             string    `json:"id"`
	Name           string    `json:"name"`
	Email          string    `json:"email"`
	Role           string    `json:"role"`
	OrganizationID string    `json:"organization_id"`
	CreatedAt      time.Time `json:"created_at"`
}

// Resource is something that can be requested access to - a database,
// a production environment, a customer-data table, a third-party SaaS,
// etc. Resources are registered by admins and scoped to an organization.
//
// Sensitivity is a free-form label ("public", "internal", "confidential",
// "restricted") that admins use to triage which requests need extra care.
type Resource struct {
	ID             string     `json:"id"`
	Name           string     `json:"name"`
	Description    string     `json:"description"`
	Sensitivity    string     `json:"sensitivity"`
	OrganizationID string     `json:"organization_id"`
	RegisteredBy   string     `json:"registered_by"`
	CreatedAt      time.Time  `json:"created_at"`
	DecommissionAt *time.Time `json:"decommission_at,omitempty"`
}

// AccessRequest is a member's request for access to a resource. Status
// transitions:
//
//	pending → approved   (admin approves)
//	pending → denied     (admin denies, terminal)
//	approved → revoked   (admin pulls access back)
//
// Approved + revoked carry the approver/revoker user id so the trail of
// "who decided this" is intact even after the request is closed.
type AccessRequest struct {
	ID             string     `json:"id"`
	ResourceID     string     `json:"resource_id"`
	OrganizationID string     `json:"organization_id"`
	RequestedBy    string     `json:"requested_by"`
	Reason         string     `json:"reason"`
	Status         string     `json:"status"`
	DecidedBy      string     `json:"decided_by,omitempty"`
	DecidedAt      *time.Time `json:"decided_at,omitempty"`
	RevokedBy      string     `json:"revoked_by,omitempty"`
	RevokedAt      *time.Time `json:"revoked_at,omitempty"`
	CreatedAt      time.Time  `json:"created_at"`
}

// Comment is a note on an access request. Reviewers use these to leave
// context: "why I approved", "what's the blast radius", etc. Comments
// share the request's organization so listing them by request already
// enforces tenant scoping.
type Comment struct {
	ID             string    `json:"id"`
	RequestID      string    `json:"request_id"`
	OrganizationID string    `json:"organization_id"`
	AuthorID       string    `json:"author_id"`
	Body           string    `json:"body"`
	CreatedAt      time.Time `json:"created_at"`
}

// Access-request status constants - mirrored in the frontend.
const (
	StatusPending  = "pending"
	StatusApproved = "approved"
	StatusDenied   = "denied"
	StatusRevoked  = "revoked"
)

// User role constants.
const (
	RoleAdmin  = "admin"
	RoleMember = "member"
)

// Sensitivity constants - free-form, but these four are seeded.
const (
	SensitivityPublic       = "public"
	SensitivityInternal     = "internal"
	SensitivityConfidential = "confidential"
	SensitivityRestricted   = "restricted"
)
