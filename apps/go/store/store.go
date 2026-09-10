// Package store is the persistence boundary. memory.Store is the only
// implementation today.
package store

import (
	"context"
	"errors"

	"github.com/everscribe/ai-assisted-setup-demo/apps/go/models"
)

// ErrNotFound covers both a missing resource and one that lives in a
// different organization, so the tenant boundary does not leak existence.
var ErrNotFound = errors.New("not found")

// ErrForbidden is a structurally disallowed action, such as a non-admin
// managing members. Handlers map it to 403.
var ErrForbidden = errors.New("forbidden")

// ErrConflict is a state transition that does not apply, such as
// approving an already-denied request. Handlers map it to 409.
var ErrConflict = errors.New("conflict")

// Store is the persistence boundary. Every method takes a context and
// returns an error, and every mutation returns the updated entity.
// Tenant scoping lives here rather than in handlers: methods that touch a
// tenant-owned row take orgID and return ErrNotFound on a cross-tenant miss.
type Store interface {
	Seed(ctx context.Context) error

	ListOrganizations(ctx context.Context) ([]*models.Organization, error)
	GetOrganization(ctx context.Context, id string) (*models.Organization, error)

	GetUser(ctx context.Context, id string) (*models.User, error)
	FindUserByEmail(ctx context.Context, email string) (*models.User, error)
	ListOrgMembers(ctx context.Context, orgID string) ([]*models.User, error)
	InviteMember(ctx context.Context, orgID, name, email, role string) (*models.User, error)
	RemoveMember(ctx context.Context, orgID, userID string) error
	UpdateMemberRole(ctx context.Context, orgID, userID, role string) (*models.User, error)

	ListOrgResources(ctx context.Context, orgID string) ([]*models.Resource, error)
	GetResource(ctx context.Context, orgID, resourceID string) (*models.Resource, error)
	RegisterResource(ctx context.Context, orgID, name, description, sensitivity, registeredBy string) (*models.Resource, error)
	DecommissionResource(ctx context.Context, orgID, resourceID string) (*models.Resource, error)

	ListResourceRequests(ctx context.Context, orgID, resourceID string) ([]*models.AccessRequest, error)
	GetRequest(ctx context.Context, orgID, requestID string) (*models.AccessRequest, error)
	RequestAccess(ctx context.Context, orgID, resourceID, requestedBy, reason string) (*models.AccessRequest, error)
	ApproveRequest(ctx context.Context, orgID, requestID, approvedBy string) (*models.AccessRequest, error)
	DenyRequest(ctx context.Context, orgID, requestID, deniedBy string) (*models.AccessRequest, error)
	RevokeRequest(ctx context.Context, orgID, requestID, revokedBy string) (*models.AccessRequest, error)

	ListRequestComments(ctx context.Context, orgID, requestID string) ([]*models.Comment, error)
	AddComment(ctx context.Context, orgID, requestID, authorID, body string) (*models.Comment, error)
}
