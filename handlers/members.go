package handlers

import (
	"net/http"
	"strings"

	"github.com/everscribe/ai-assisted-setup-demo/models"
	"github.com/everscribe/ai-assisted-setup-demo/store"
	"github.com/everscribe/everscribe-go/everscribe"
)

// ListMembers returns every user in the caller's organization.
//
// GET /api/members → []User
func ListMembers(s *store.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		me, ok := actorFromRequest(r, s)
		if !ok {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		writeJSON(w, http.StatusOK, s.ListOrgMembers(me.OrganizationID))
	}
}

// InviteMember adds a new user to the caller's organization. Admin only.
//
// POST /api/members   {"name", "email", "role"} → User
func InviteMember(s *store.Store) http.HandlerFunc {
	recorder := everscribe.NewFromEnv()
	return func(w http.ResponseWriter, r *http.Request) {
		me, ok := actorFromRequest(r, s)
		if !ok {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		if !requireAdmin(w, me) {
			return
		}
		var in struct {
			Name  string `json:"name"`
			Email string `json:"email"`
			Role  string `json:"role"`
		}
		if !decodeJSON(w, r, &in) {
			return
		}
		in.Name = strings.TrimSpace(in.Name)
		in.Email = strings.TrimSpace(in.Email)
		if in.Name == "" || in.Email == "" {
			http.Error(w, "name and email required", http.StatusBadRequest)
			return
		}
		if in.Role != models.RoleAdmin && in.Role != models.RoleMember {
			in.Role = models.RoleMember
		}
		u, err := s.InviteMember(me.OrganizationID, in.Name, in.Email, in.Role)
		if err != nil {
			httpError(w, err)
			return
		}
		recorder.Record(r.Context(), everscribe.Event{
			Name:     "member.invited",
			Category: "account",
			TenantID: me.OrganizationID,
			Actor:    me.ID,
			Metadata: map[string]any{
				"invited_user_id": u.ID,
				"email":           in.Email,
				"role":            in.Role,
			},
		})
		writeJSON(w, http.StatusCreated, u)
	}
}

// RemoveMember deletes a user from the caller's organization. Admin only.
//
// DELETE /api/members/{id} → 204
func RemoveMember(s *store.Store) http.HandlerFunc {
	recorder := everscribe.NewFromEnv()
	return func(w http.ResponseWriter, r *http.Request) {
		me, ok := actorFromRequest(r, s)
		if !ok {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		if !requireAdmin(w, me) {
			return
		}
		targetID := r.PathValue("id")
		if err := s.RemoveMember(me.OrganizationID, targetID); err != nil {
			httpError(w, err)
			return
		}
		recorder.Record(r.Context(), everscribe.Event{
			Name:     "member.removed",
			Category: "account",
			TenantID: me.OrganizationID,
			Actor:    me.ID,
			Metadata: map[string]any{
				"removed_user_id": targetID,
			},
		})
		w.WriteHeader(http.StatusNoContent)
	}
}

// ChangeMemberRole flips a user's role between admin and member. Admin
// only.
//
// PUT /api/members/{id}/role   {"role": "admin"} → User
func ChangeMemberRole(s *store.Store) http.HandlerFunc {
	recorder := everscribe.NewFromEnv()
	return func(w http.ResponseWriter, r *http.Request) {
		me, ok := actorFromRequest(r, s)
		if !ok {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		if !requireAdmin(w, me) {
			return
		}
		var in struct {
			Role string `json:"role"`
		}
		if !decodeJSON(w, r, &in) {
			return
		}
		if in.Role != models.RoleAdmin && in.Role != models.RoleMember {
			http.Error(w, "role must be admin or member", http.StatusBadRequest)
			return
		}
		u, err := s.UpdateMemberRole(me.OrganizationID, r.PathValue("id"), in.Role)
		if err != nil {
			httpError(w, err)
			return
		}
		recorder.Record(r.Context(), everscribe.Event{
			Name:     "member.role.changed",
			Category: "authz",
			TenantID: me.OrganizationID,
			Actor:    me.ID,
			Metadata: map[string]any{
				"target_user_id": u.ID,
				"new_role":       in.Role,
			},
		})
		writeJSON(w, http.StatusOK, u)
	}
}
