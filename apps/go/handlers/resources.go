package handlers

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/everscribe/sdk-go/pkg/event"

	"github.com/everscribe/ai-assisted-setup-demo/apps/go/models"
	"github.com/everscribe/ai-assisted-setup-demo/apps/go/store"
)

// ListResources returns every resource in the caller's organization
// (including decommissioned ones - the UI grays them out instead of
// hiding so historical context stays visible).
//
// GET /api/resources → []Resource
func ListResources(s store.Store) gin.HandlerFunc {
	return func(c *gin.Context) {
		me, ok := requireActor(c, s)
		if !ok {
			return
		}
		resources, err := s.ListOrgResources(c.Request.Context(), me.OrganizationID)
		if err != nil {
			failStore(c, err)
			return
		}
		c.JSON(http.StatusOK, resources)
	}
}

// RegisterResource adds a new resource to the caller's organization.
// Admin only - registering a new resource defines what can be requested,
// which is privileged.
//
// POST /api/resources   {"name", "description", "sensitivity"} → Resource
func RegisterResource(s store.Store) gin.HandlerFunc {
	return func(c *gin.Context) {
		me, ok := requireActor(c, s)
		if !ok {
			return
		}
		if !requireAdmin(c, me) {
			return
		}
		var in struct {
			Name        string `json:"name"`
			Description string `json:"description"`
			Sensitivity string `json:"sensitivity"`
		}
		if err := bindJSON(c, &in); err != nil {
			fail(c, http.StatusBadRequest, "invalid body: "+err.Error())
			return
		}
		in.Name = strings.TrimSpace(in.Name)
		in.Description = strings.TrimSpace(in.Description)
		in.Sensitivity = strings.TrimSpace(in.Sensitivity)
		if in.Name == "" {
			fail(c, http.StatusBadRequest, "name required")
			return
		}
		if in.Sensitivity == "" {
			in.Sensitivity = models.SensitivityInternal
		}
		e := event.Current(c.Request.Context())
		e.Action = "resource.registered"
		e.TenantID = me.OrganizationID
		e.WithFields("name", in.Name, "sensitivity", in.Sensitivity)

		r2, err := s.RegisterResource(c.Request.Context(), me.OrganizationID, in.Name, in.Description, in.Sensitivity, me.ID)
		if err != nil {
			failStore(c, err)
			return
		}

		e.Target = event.Target{Type: "resource", ID: r2.ID}

		c.JSON(http.StatusCreated, r2)
	}
}

// DecommissionResource marks a resource as no longer available for new
// access requests. Existing approvals are preserved (so the audit trail
// is intact) but RequestAccess refuses new ones. Admin only.
//
// POST /api/resources/{id}/decommission → Resource
func DecommissionResource(s store.Store) gin.HandlerFunc {
	return func(c *gin.Context) {
		me, ok := requireActor(c, s)
		if !ok {
			return
		}
		if !requireAdmin(c, me) {
			return
		}
		e := event.Current(c.Request.Context())
		e.Action = "resource.decommissioned"
		e.Target = event.Target{Type: "resource", ID: c.Param("id")}
		e.TenantID = me.OrganizationID

		res, err := s.DecommissionResource(c.Request.Context(), me.OrganizationID, c.Param("id"))
		if err != nil {
			failStore(c, err)
			return
		}
		c.JSON(http.StatusOK, res)
	}
}
