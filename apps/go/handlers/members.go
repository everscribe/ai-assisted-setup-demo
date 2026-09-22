package handlers

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/everscribe/ai-assisted-setup-demo/apps/go/models"
	"github.com/everscribe/ai-assisted-setup-demo/apps/go/store"
)

// ListMembers returns every user in the caller's organization.
//
// GET /api/members → []User
func ListMembers(s store.Store) gin.HandlerFunc {
	return func(c *gin.Context) {
		me, ok := requireActor(c, s)
		if !ok {
			return
		}
		members, err := s.ListOrgMembers(c.Request.Context(), me.OrganizationID)
		if err != nil {
			failStore(c, err)
			return
		}
		c.JSON(http.StatusOK, members)
	}
}

// InviteMember adds a new user to the caller's organization. Admin only.
//
// POST /api/members   {"name", "email", "role"} → User
func InviteMember(s store.Store) gin.HandlerFunc {
	return func(c *gin.Context) {
		me, ok := requireActor(c, s)
		if !ok {
			return
		}
		if !requireAdmin(c, me) {
			return
		}
		var in struct {
			Name  string `json:"name"`
			Email string `json:"email"`
			Role  string `json:"role"`
		}
		if err := bindJSON(c, &in); err != nil {
			fail(c, http.StatusBadRequest, "invalid body: "+err.Error())
			return
		}
		in.Name = strings.TrimSpace(in.Name)
		in.Email = strings.TrimSpace(in.Email)
		if in.Name == "" || in.Email == "" {
			fail(c, http.StatusBadRequest, "name and email required")
			return
		}
		if in.Role != models.RoleAdmin && in.Role != models.RoleMember {
			in.Role = models.RoleMember
		}
		u, err := s.InviteMember(c.Request.Context(), me.OrganizationID, in.Name, in.Email, in.Role)
		if err != nil {
			failStore(c, err)
			return
		}
		c.JSON(http.StatusCreated, u)
	}
}

// RemoveMember deletes a user from the caller's organization. Admin only.
//
// DELETE /api/members/{id} → 204
func RemoveMember(s store.Store) gin.HandlerFunc {
	return func(c *gin.Context) {
		me, ok := requireActor(c, s)
		if !ok {
			return
		}
		if !requireAdmin(c, me) {
			return
		}
		if err := s.RemoveMember(c.Request.Context(), me.OrganizationID, c.Param("id")); err != nil {
			failStore(c, err)
			return
		}
		c.Status(http.StatusNoContent)
	}
}

// ChangeMemberRole flips a user's role between admin and member. Admin
// only.
//
// PUT /api/members/{id}/role   {"role": "admin"} → User
func ChangeMemberRole(s store.Store) gin.HandlerFunc {
	return func(c *gin.Context) {
		me, ok := requireActor(c, s)
		if !ok {
			return
		}
		if !requireAdmin(c, me) {
			return
		}
		var in struct {
			Role string `json:"role"`
		}
		if err := bindJSON(c, &in); err != nil {
			fail(c, http.StatusBadRequest, "invalid body: "+err.Error())
			return
		}
		if in.Role != models.RoleAdmin && in.Role != models.RoleMember {
			fail(c, http.StatusBadRequest, "role must be admin or member")
			return
		}
		u, err := s.UpdateMemberRole(c.Request.Context(), me.OrganizationID, c.Param("id"), in.Role)
		if err != nil {
			failStore(c, err)
			return
		}
		c.JSON(http.StatusOK, u)
	}
}
