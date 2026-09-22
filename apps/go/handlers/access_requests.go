package handlers

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/everscribe/ai-assisted-setup-demo/apps/go/store"
)

// ListResourceRequests returns every access request for a resource,
// newest first. Workspace-scoped via the store.
//
// GET /api/resources/{id}/requests → []AccessRequest
func ListResourceRequests(s store.Store) gin.HandlerFunc {
	return func(c *gin.Context) {
		me, ok := requireActor(c, s)
		if !ok {
			return
		}
		reqs, err := s.ListResourceRequests(c.Request.Context(), me.OrganizationID, c.Param("id"))
		if err != nil {
			failStore(c, err)
			return
		}
		c.JSON(http.StatusOK, reqs)
	}
}

// RequestAccess submits a new pending access request against a resource.
// Anyone in the organization can request - approval is gated separately.
//
// POST /api/resources/{id}/requests   {"reason"} → AccessRequest
func RequestAccess(s store.Store) gin.HandlerFunc {
	return func(c *gin.Context) {
		me, ok := requireActor(c, s)
		if !ok {
			return
		}
		var in struct {
			Reason string `json:"reason"`
		}
		if err := bindJSON(c, &in); err != nil {
			fail(c, http.StatusBadRequest, "invalid body: "+err.Error())
			return
		}
		in.Reason = strings.TrimSpace(in.Reason)
		if in.Reason == "" {
			fail(c, http.StatusBadRequest, "reason required")
			return
		}
		req, err := s.RequestAccess(c.Request.Context(), me.OrganizationID, c.Param("id"), me.ID, in.Reason)
		if err != nil {
			failStore(c, err)
			return
		}
		c.JSON(http.StatusCreated, req)
	}
}

// ApproveRequest grants a pending request. Admin only - approving is
// the privileged decision in this domain.
//
// POST /api/requests/{id}/approve → AccessRequest
func ApproveRequest(s store.Store) gin.HandlerFunc {
	return func(c *gin.Context) {
		me, ok := requireActor(c, s)
		if !ok {
			return
		}
		if !requireAdmin(c, me) {
			return
		}
		req, err := s.ApproveRequest(c.Request.Context(), me.OrganizationID, c.Param("id"), me.ID)
		if err != nil {
			failStore(c, err)
			return
		}
		c.JSON(http.StatusOK, req)
	}
}

// DenyRequest refuses a pending request. Admin only. Denial is terminal -
// the requester would need to file a fresh request to try again.
//
// POST /api/requests/{id}/deny → AccessRequest
func DenyRequest(s store.Store) gin.HandlerFunc {
	return func(c *gin.Context) {
		me, ok := requireActor(c, s)
		if !ok {
			return
		}
		if !requireAdmin(c, me) {
			return
		}
		req, err := s.DenyRequest(c.Request.Context(), me.OrganizationID, c.Param("id"), me.ID)
		if err != nil {
			failStore(c, err)
			return
		}
		c.JSON(http.StatusOK, req)
	}
}

// RevokeRequest pulls a previously-approved request back. Admin only.
// This is the security-ops action - "I let them in, now I'm pulling
// them back out." Audit-critical: who revoked, when, why.
//
// POST /api/requests/{id}/revoke → AccessRequest
func RevokeRequest(s store.Store) gin.HandlerFunc {
	return func(c *gin.Context) {
		me, ok := requireActor(c, s)
		if !ok {
			return
		}
		if !requireAdmin(c, me) {
			return
		}
		req, err := s.RevokeRequest(c.Request.Context(), me.OrganizationID, c.Param("id"), me.ID)
		if err != nil {
			failStore(c, err)
			return
		}
		c.JSON(http.StatusOK, req)
	}
}

// ListRequestComments returns every comment on an access request.
//
// GET /api/requests/{id}/comments → []Comment
func ListRequestComments(s store.Store) gin.HandlerFunc {
	return func(c *gin.Context) {
		me, ok := requireActor(c, s)
		if !ok {
			return
		}
		comments, err := s.ListRequestComments(c.Request.Context(), me.OrganizationID, c.Param("id"))
		if err != nil {
			failStore(c, err)
			return
		}
		c.JSON(http.StatusOK, comments)
	}
}

// AddComment posts a review note on an access request. Used by approvers
// to record context ("approved for 24h", "denied - over-broad scope")
// and by requesters to add follow-up info.
//
// POST /api/requests/{id}/comments   {"body"} → Comment
func AddComment(s store.Store) gin.HandlerFunc {
	return func(c *gin.Context) {
		me, ok := requireActor(c, s)
		if !ok {
			return
		}
		var in struct {
			Body string `json:"body"`
		}
		if err := bindJSON(c, &in); err != nil {
			fail(c, http.StatusBadRequest, "invalid body: "+err.Error())
			return
		}
		in.Body = strings.TrimSpace(in.Body)
		if in.Body == "" {
			fail(c, http.StatusBadRequest, "body required")
			return
		}
		cm, err := s.AddComment(c.Request.Context(), me.OrganizationID, c.Param("id"), me.ID, in.Body)
		if err != nil {
			failStore(c, err)
			return
		}
		c.JSON(http.StatusCreated, cm)
	}
}
