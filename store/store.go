// Package store is the in-memory persistence layer for the demo. Everything
// is mutex-guarded maps — there is no database. The store is rebuilt with
// the seed cast on every server start so the demo is deterministic.
//
// Every read/write API takes an orgID and enforces it: cross-organization
// access returns ErrNotFound, never the actual resource. This is the
// multi-tenant boundary; handlers don't have to remember to filter.
package store

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"sort"
	"sync"
	"time"

	"github.com/everscribe/ai-setup-example/models"
)

// ErrNotFound is returned by lookups that don't find the resource or
// when the resource exists but lives in a different organization.
var ErrNotFound = errors.New("not found")

// ErrForbidden is returned when an action is structurally disallowed —
// e.g. a non-admin trying to manage members. Handlers translate this to
// HTTP 403.
var ErrForbidden = errors.New("forbidden")

// ErrConflict is returned by state transitions that don't apply to the
// resource's current state — e.g. approving a request that's already
// denied. Handlers translate this to HTTP 409.
var ErrConflict = errors.New("conflict")

// Store is the in-memory database.
type Store struct {
	mu sync.RWMutex

	orgs      map[string]*models.Organization
	users     map[string]*models.User
	resources map[string]*models.Resource
	requests  map[string]*models.AccessRequest
	comments  map[string]*models.Comment

	usersByOrg     map[string][]string // orgID -> userIDs
	resourcesByOrg map[string][]string // orgID -> resourceIDs
	requestsByRes  map[string][]string // resourceID -> requestIDs
	commentsByReq  map[string][]string // requestID -> commentIDs
}

// New returns an empty store. Call Seed to populate it.
func New() *Store {
	return &Store{
		orgs:           map[string]*models.Organization{},
		users:          map[string]*models.User{},
		resources:      map[string]*models.Resource{},
		requests:       map[string]*models.AccessRequest{},
		comments:       map[string]*models.Comment{},
		usersByOrg:     map[string][]string{},
		resourcesByOrg: map[string][]string{},
		requestsByRes:  map[string][]string{},
		commentsByReq:  map[string][]string{},
	}
}

// ============================================================
// Organizations
// ============================================================

func (s *Store) ListOrganizations() []*models.Organization {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]*models.Organization, 0, len(s.orgs))
	for _, o := range s.orgs {
		out = append(out, o)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

func (s *Store) GetOrganization(id string) (*models.Organization, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	o, ok := s.orgs[id]
	if !ok {
		return nil, ErrNotFound
	}
	return o, nil
}

// ============================================================
// Users
// ============================================================

func (s *Store) GetUser(id string) (*models.User, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	u, ok := s.users[id]
	if !ok {
		return nil, ErrNotFound
	}
	return u, nil
}

// FindUserByEmail returns the user with the given email, or ErrNotFound.
// Used by the login handler.
func (s *Store) FindUserByEmail(email string) (*models.User, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, u := range s.users {
		if u.Email == email {
			return u, nil
		}
	}
	return nil, ErrNotFound
}

// ListOrgMembers returns every user in an organization, sorted by ID.
func (s *Store) ListOrgMembers(orgID string) []*models.User {
	s.mu.RLock()
	defer s.mu.RUnlock()
	ids := s.usersByOrg[orgID]
	out := make([]*models.User, 0, len(ids))
	for _, id := range ids {
		if u, ok := s.users[id]; ok {
			out = append(out, u)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

// InviteMember creates a new user in an organization. Returns the new
// user. Email collisions across organizations are allowed for this demo —
// real apps would unique on email globally.
func (s *Store) InviteMember(orgID, name, email, role string) (*models.User, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.orgs[orgID]; !ok {
		return nil, ErrNotFound
	}
	u := &models.User{
		ID:             "u_" + randomID(),
		Name:           name,
		Email:          email,
		Role:           role,
		OrganizationID: orgID,
		CreatedAt:      time.Now().UTC(),
	}
	s.users[u.ID] = u
	s.usersByOrg[orgID] = append(s.usersByOrg[orgID], u.ID)
	return u, nil
}

// RemoveMember deletes a user from an organization. Returns ErrNotFound
// if the user doesn't exist or lives in a different organization.
func (s *Store) RemoveMember(orgID, userID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	u, ok := s.users[userID]
	if !ok || u.OrganizationID != orgID {
		return ErrNotFound
	}
	delete(s.users, userID)
	s.usersByOrg[orgID] = removeString(s.usersByOrg[orgID], userID)
	return nil
}

// UpdateMemberRole changes a user's role within an organization. Returns
// ErrNotFound for cross-organization access. The new role replaces the
// old one in place — same User pointer, mutated.
func (s *Store) UpdateMemberRole(orgID, userID, role string) (*models.User, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	u, ok := s.users[userID]
	if !ok || u.OrganizationID != orgID {
		return nil, ErrNotFound
	}
	u.Role = role
	return u, nil
}

// ============================================================
// Resources
// ============================================================

func (s *Store) ListOrgResources(orgID string) []*models.Resource {
	s.mu.RLock()
	defer s.mu.RUnlock()
	ids := s.resourcesByOrg[orgID]
	out := make([]*models.Resource, 0, len(ids))
	for _, id := range ids {
		if r, ok := s.resources[id]; ok {
			out = append(out, r)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

func (s *Store) GetResource(orgID, resourceID string) (*models.Resource, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	r, ok := s.resources[resourceID]
	if !ok || r.OrganizationID != orgID {
		return nil, ErrNotFound
	}
	return r, nil
}

func (s *Store) RegisterResource(orgID, name, description, sensitivity, registeredBy string) (*models.Resource, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.orgs[orgID]; !ok {
		return nil, ErrNotFound
	}
	r := &models.Resource{
		ID:             "r_" + randomID(),
		Name:           name,
		Description:    description,
		Sensitivity:    sensitivity,
		OrganizationID: orgID,
		RegisteredBy:   registeredBy,
		CreatedAt:      time.Now().UTC(),
	}
	s.resources[r.ID] = r
	s.resourcesByOrg[orgID] = append(s.resourcesByOrg[orgID], r.ID)
	return r, nil
}

// DecommissionResource marks a resource as decommissioned (timestamps
// only — the row stays so historical access requests still reference
// something). Future requests against decommissioned resources are
// rejected by RequestAccess.
func (s *Store) DecommissionResource(orgID, resourceID string) (*models.Resource, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	r, ok := s.resources[resourceID]
	if !ok || r.OrganizationID != orgID {
		return nil, ErrNotFound
	}
	now := time.Now().UTC()
	r.DecommissionAt = &now
	return r, nil
}

// ============================================================
// Access requests
// ============================================================

func (s *Store) ListResourceRequests(orgID, resourceID string) ([]*models.AccessRequest, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	r, ok := s.resources[resourceID]
	if !ok || r.OrganizationID != orgID {
		return nil, ErrNotFound
	}
	ids := s.requestsByRes[resourceID]
	out := make([]*models.AccessRequest, 0, len(ids))
	for _, id := range ids {
		if req, ok := s.requests[id]; ok {
			out = append(out, req)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].CreatedAt.After(out[j].CreatedAt) })
	return out, nil
}

func (s *Store) GetRequest(orgID, requestID string) (*models.AccessRequest, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	req, ok := s.requests[requestID]
	if !ok || req.OrganizationID != orgID {
		return nil, ErrNotFound
	}
	return req, nil
}

// RequestAccess creates a new pending request for a resource. The
// requester must be in the same organization as the resource. A
// decommissioned resource rejects new requests with ErrConflict.
func (s *Store) RequestAccess(orgID, resourceID, requestedBy, reason string) (*models.AccessRequest, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	r, ok := s.resources[resourceID]
	if !ok || r.OrganizationID != orgID {
		return nil, ErrNotFound
	}
	if r.DecommissionAt != nil {
		return nil, ErrConflict
	}
	req := &models.AccessRequest{
		ID:             "req_" + randomID(),
		ResourceID:     resourceID,
		OrganizationID: orgID,
		RequestedBy:    requestedBy,
		Reason:         reason,
		Status:         models.StatusPending,
		CreatedAt:      time.Now().UTC(),
	}
	s.requests[req.ID] = req
	s.requestsByRes[resourceID] = append(s.requestsByRes[resourceID], req.ID)
	return req, nil
}

// ApproveRequest flips a pending request to approved + stamps decider.
// Any other state is a conflict.
func (s *Store) ApproveRequest(orgID, requestID, approvedBy string) (*models.AccessRequest, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	req, ok := s.requests[requestID]
	if !ok || req.OrganizationID != orgID {
		return nil, ErrNotFound
	}
	if req.Status != models.StatusPending {
		return nil, ErrConflict
	}
	now := time.Now().UTC()
	req.Status = models.StatusApproved
	req.DecidedBy = approvedBy
	req.DecidedAt = &now
	return req, nil
}

// DenyRequest flips a pending request to denied + stamps decider. Denied
// is terminal; revoking a denied request is a conflict.
func (s *Store) DenyRequest(orgID, requestID, deniedBy string) (*models.AccessRequest, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	req, ok := s.requests[requestID]
	if !ok || req.OrganizationID != orgID {
		return nil, ErrNotFound
	}
	if req.Status != models.StatusPending {
		return nil, ErrConflict
	}
	now := time.Now().UTC()
	req.Status = models.StatusDenied
	req.DecidedBy = deniedBy
	req.DecidedAt = &now
	return req, nil
}

// RevokeRequest pulls previously-approved access back. Only approved
// requests can be revoked; pending/denied/already-revoked are conflicts.
func (s *Store) RevokeRequest(orgID, requestID, revokedBy string) (*models.AccessRequest, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	req, ok := s.requests[requestID]
	if !ok || req.OrganizationID != orgID {
		return nil, ErrNotFound
	}
	if req.Status != models.StatusApproved {
		return nil, ErrConflict
	}
	now := time.Now().UTC()
	req.Status = models.StatusRevoked
	req.RevokedBy = revokedBy
	req.RevokedAt = &now
	return req, nil
}

// ============================================================
// Comments
// ============================================================

func (s *Store) ListRequestComments(orgID, requestID string) ([]*models.Comment, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	req, ok := s.requests[requestID]
	if !ok || req.OrganizationID != orgID {
		return nil, ErrNotFound
	}
	ids := s.commentsByReq[requestID]
	out := make([]*models.Comment, 0, len(ids))
	for _, id := range ids {
		if c, ok := s.comments[id]; ok {
			out = append(out, c)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].CreatedAt.Before(out[j].CreatedAt) })
	return out, nil
}

func (s *Store) AddComment(orgID, requestID, authorID, body string) (*models.Comment, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	req, ok := s.requests[requestID]
	if !ok || req.OrganizationID != orgID {
		return nil, ErrNotFound
	}
	c := &models.Comment{
		ID:             "c_" + randomID(),
		RequestID:      requestID,
		OrganizationID: orgID,
		AuthorID:       authorID,
		Body:           body,
		CreatedAt:      time.Now().UTC(),
	}
	s.comments[c.ID] = c
	s.commentsByReq[requestID] = append(s.commentsByReq[requestID], c.ID)
	return c, nil
}

// ============================================================
// Helpers
// ============================================================

func removeString(s []string, target string) []string {
	out := s[:0]
	for _, v := range s {
		if v != target {
			out = append(out, v)
		}
	}
	return out
}

func randomID() string {
	var b [6]byte
	if _, err := rand.Read(b[:]); err != nil {
		// rand.Read on Linux/macOS doesn't fail in practice; if it does
		// here, the demo can't continue minting deterministic ids.
		panic(err)
	}
	return hex.EncodeToString(b[:])
}
