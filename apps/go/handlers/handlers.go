// Package handlers exposes the HTTP handlers for the demo, one file per
// resource: auth, members, resources, requests, comments. Handlers depend
// on the store.Store interface and reach the authenticated user via
// requireActor, which reads the X-User-ID header set by the SPA's login.
//
// Every error response is JSON via fail/failStore. Mutating routes are
// organization-scoped: the store enforces that resources live in the
// actor's organization, returning ErrNotFound on cross-tenant access.
package handlers

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/everscribe/ai-assisted-setup-demo/apps/go/models"
	"github.com/everscribe/ai-assisted-setup-demo/apps/go/store"
)

// fail writes a JSON error body. Every error response in this app is
// JSON; the contract depends on it and http.Error writes plain text.
func fail(c *gin.Context, status int, msg string) {
	c.AbortWithStatusJSON(status, gin.H{"error": msg})
}

// failStore maps a store error to its status. ErrNotFound covers both a
// missing row and a cross-tenant miss, on purpose.
func failStore(c *gin.Context, err error) {
	switch {
	case errors.Is(err, store.ErrNotFound):
		fail(c, http.StatusNotFound, "not found")
	case errors.Is(err, store.ErrForbidden):
		fail(c, http.StatusForbidden, "forbidden")
	case errors.Is(err, store.ErrConflict):
		fail(c, http.StatusConflict, "conflict")
	default:
		fail(c, http.StatusInternalServerError, err.Error())
	}
}

// actor resolves X-User-ID. Handlers call requireActor instead.
func actor(c *gin.Context, s store.Store) (*models.User, bool) {
	id := c.GetHeader("X-User-ID")
	if id == "" {
		return nil, false
	}
	u, err := s.GetUser(c.Request.Context(), id)
	if err != nil {
		return nil, false
	}
	return u, true
}

func requireActor(c *gin.Context, s store.Store) (*models.User, bool) {
	u, ok := actor(c, s)
	if !ok {
		fail(c, http.StatusUnauthorized, "unauthorized")
		return nil, false
	}
	return u, true
}

func requireAdmin(c *gin.Context, u *models.User) bool {
	if u.Role != models.RoleAdmin {
		fail(c, http.StatusForbidden, "admin role required")
		return false
	}
	return true
}

// Router builds the full API. The eighteen registrations here are the
// contract; all four apps in this repo register the same set.
func Router(s store.Store) *gin.Engine {
	gin.SetMode(gin.ReleaseMode)
	r := gin.New()
	r.Use(gin.Recovery())

	// An unknown /api/* path returns the same JSON 404 as a cross-tenant
	// miss. The two are told apart by the request path, not the response.
	r.NoRoute(func(c *gin.Context) {
		fail(c, http.StatusNotFound, "not found")
	})

	api := r.Group("/api")
	{
		api.POST("/login", Login(s))
		api.POST("/logout", Logout(s))
		api.GET("/me", Me(s))

		api.GET("/members", ListMembers(s))
		api.POST("/members", InviteMember(s))
		api.DELETE("/members/:id", RemoveMember(s))
		api.PUT("/members/:id/role", ChangeMemberRole(s))

		api.GET("/resources", ListResources(s))
		api.POST("/resources", RegisterResource(s))
		api.POST("/resources/:id/decommission", DecommissionResource(s))

		api.GET("/resources/:id/requests", ListResourceRequests(s))
		api.POST("/resources/:id/requests", RequestAccess(s))
		api.POST("/requests/:id/approve", ApproveRequest(s))
		api.POST("/requests/:id/deny", DenyRequest(s))
		api.POST("/requests/:id/revoke", RevokeRequest(s))

		api.GET("/requests/:id/comments", ListRequestComments(s))
		api.POST("/requests/:id/comments", AddComment(s))

		api.GET("/health", func(c *gin.Context) {
			c.JSON(http.StatusOK, gin.H{"status": "ok"})
		})
	}
	return r
}
