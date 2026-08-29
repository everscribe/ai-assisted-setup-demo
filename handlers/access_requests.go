package handlers

import (
	"net/http"
	"strings"

	"github.com/everscribe/sdk-go/pkg/event"

	"github.com/everscribe/ai-assisted-setup-demo/store"
)

// ListResourceRequests returns every access request for a resource,
// newest first. Workspace-scoped via the store.
//
// GET /api/resources/{id}/requests → []AccessRequest
func ListResourceRequests(s *store.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		me, ok := actorFromRequest(r, s)
		if !ok {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		reqs, err := s.ListResourceRequests(me.OrganizationID, r.PathValue("id"))
		if err != nil {
			httpError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, reqs)
	}
}

// RequestAccess submits a new pending access request against a resource.
// Anyone in the organization can request - approval is gated separately.
//
// POST /api/resources/{id}/requests   {"reason"} → AccessRequest
func RequestAccess(s *store.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		me, ok := actorFromRequest(r, s)
		if !ok {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		var in struct {
			Reason string `json:"reason"`
		}
		if !decodeJSON(w, r, &in) {
			return
		}
		in.Reason = strings.TrimSpace(in.Reason)
		if in.Reason == "" {
			http.Error(w, "reason required", http.StatusBadRequest)
			return
		}

		e := event.Current(r.Context())
		e.TenantID = me.OrganizationID
		e.Action = "access.request.created"
		e.Target = event.Target{Type: "access_request"}

		req, err := s.RequestAccess(me.OrganizationID, r.PathValue("id"), me.ID, in.Reason)
		if err != nil {
			httpError(w, err)
			return
		}
		e.Target.ID = req.ID
		writeJSON(w, http.StatusCreated, req)
	}
}

// ApproveRequest grants a pending request. Admin only - approving is
// the privileged decision in this domain.
//
// POST /api/requests/{id}/approve → AccessRequest
func ApproveRequest(s *store.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		me, ok := actorFromRequest(r, s)
		if !ok {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}

		e := event.Current(r.Context())
		e.TenantID = me.OrganizationID
		e.Action = "access.request.approved"
		e.Target = event.Target{Type: "access_request", ID: r.PathValue("id")}

		if !requireAdmin(w, me) {
			return
		}
		req, err := s.ApproveRequest(me.OrganizationID, r.PathValue("id"), me.ID)
		if err != nil {
			httpError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, req)
	}
}

// DenyRequest refuses a pending request. Admin only. Denial is terminal -
// the requester would need to file a fresh request to try again.
//
// POST /api/requests/{id}/deny → AccessRequest
func DenyRequest(s *store.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		me, ok := actorFromRequest(r, s)
		if !ok {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}

		e := event.Current(r.Context())
		e.TenantID = me.OrganizationID
		e.Action = "access.request.denied"
		e.Target = event.Target{Type: "access_request", ID: r.PathValue("id")}

		if !requireAdmin(w, me) {
			return
		}
		req, err := s.DenyRequest(me.OrganizationID, r.PathValue("id"), me.ID)
		if err != nil {
			httpError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, req)
	}
}

// RevokeRequest pulls a previously-approved request back. Admin only.
// This is the security-ops action - "I let them in, now I'm pulling
// them back out." Audit-critical: who revoked, when, why.
//
// POST /api/requests/{id}/revoke → AccessRequest
func RevokeRequest(s *store.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		me, ok := actorFromRequest(r, s)
		if !ok {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}

		e := event.Current(r.Context())
		e.TenantID = me.OrganizationID
		e.Action = "access.request.revoked"
		e.Target = event.Target{Type: "access_request", ID: r.PathValue("id")}

		if !requireAdmin(w, me) {
			return
		}
		req, err := s.RevokeRequest(me.OrganizationID, r.PathValue("id"), me.ID)
		if err != nil {
			httpError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, req)
	}
}

// ListRequestComments returns every comment on an access request.
//
// GET /api/requests/{id}/comments → []Comment
func ListRequestComments(s *store.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		me, ok := actorFromRequest(r, s)
		if !ok {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		comments, err := s.ListRequestComments(me.OrganizationID, r.PathValue("id"))
		if err != nil {
			httpError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, comments)
	}
}

// AddComment posts a review note on an access request. Used by approvers
// to record context ("approved for 24h", "denied - over-broad scope")
// and by requesters to add follow-up info.
//
// POST /api/requests/{id}/comments   {"body"} → Comment
func AddComment(s *store.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		me, ok := actorFromRequest(r, s)
		if !ok {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		var in struct {
			Body string `json:"body"`
		}
		if !decodeJSON(w, r, &in) {
			return
		}
		in.Body = strings.TrimSpace(in.Body)
		if in.Body == "" {
			http.Error(w, "body required", http.StatusBadRequest)
			return
		}

		e := event.Current(r.Context())
		e.TenantID = me.OrganizationID
		e.Action = "access.request.comment.created"
		e.Target = event.Target{Type: "comment"}

		c, err := s.AddComment(me.OrganizationID, r.PathValue("id"), me.ID, in.Body)
		if err != nil {
			httpError(w, err)
			return
		}
		e.Target.ID = c.ID
		writeJSON(w, http.StatusCreated, c)
	}
}
