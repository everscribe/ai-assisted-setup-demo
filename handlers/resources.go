package handlers

import (
	"net/http"
	"strings"

	"github.com/everscribe/ai-assisted-setup-demo/models"
	"github.com/everscribe/ai-assisted-setup-demo/store"
	"github.com/everscribe/sdk-go/pkg/event"
)

// ListResources returns every resource in the caller's organization
// (including decommissioned ones - the UI grays them out instead of
// hiding so historical context stays visible).
//
// GET /api/resources → []Resource
func ListResources(s *store.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		me, ok := actorFromRequest(r, s)
		if !ok {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		writeJSON(w, http.StatusOK, s.ListOrgResources(me.OrganizationID))
	}
}

// RegisterResource adds a new resource to the caller's organization.
// Admin only - registering a new resource defines what can be requested,
// which is privileged.
//
// POST /api/resources   {"name", "description", "sensitivity"} → Resource
func RegisterResource(s *store.Store) http.HandlerFunc {
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
			Name        string `json:"name"`
			Description string `json:"description"`
			Sensitivity string `json:"sensitivity"`
		}
		if !decodeJSON(w, r, &in) {
			return
		}
		in.Name = strings.TrimSpace(in.Name)
		in.Description = strings.TrimSpace(in.Description)
		in.Sensitivity = strings.TrimSpace(in.Sensitivity)
		if in.Name == "" {
			http.Error(w, "name required", http.StatusBadRequest)
			return
		}
		if in.Sensitivity == "" {
			in.Sensitivity = models.SensitivityInternal
		}

		e := event.Current(r.Context())
		e.TenantID = me.OrganizationID
		e.Action = "resource.registered"
		e.Target = event.Target{Type: "resource"}

		r2, err := s.RegisterResource(me.OrganizationID, in.Name, in.Description, in.Sensitivity, me.ID)
		if err != nil {
			httpError(w, err)
			return
		}
		e.Target.ID = r2.ID
		writeJSON(w, http.StatusCreated, r2)
	}
}

// DecommissionResource marks a resource as no longer available for new
// access requests. Existing approvals are preserved (so the audit trail
// is intact) but RequestAccess refuses new ones. Admin only.
//
// POST /api/resources/{id}/decommission → Resource
func DecommissionResource(s *store.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		me, ok := actorFromRequest(r, s)
		if !ok {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		if !requireAdmin(w, me) {
			return
		}

		e := event.Current(r.Context())
		e.TenantID = me.OrganizationID
		e.Action = "resource.decommissioned"
		e.Target = event.Target{Type: "resource", ID: r.PathValue("id")}

		res, err := s.DecommissionResource(me.OrganizationID, r.PathValue("id"))
		if err != nil {
			httpError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, res)
	}
}
