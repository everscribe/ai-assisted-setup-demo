package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/everscribe/ai-assisted-setup-demo/apps/go/store"
)

// Login signs a user in by email. There's no password - this is a demo,
// the email picks one of the seeded users from the cast. The frontend
// stashes the returned user id in localStorage and sends it as
// X-User-ID on every subsequent request.
//
// POST /api/login   {"email": "ada@acme.test"} → User
func Login(s store.Store) gin.HandlerFunc {
	return func(c *gin.Context) {
		var in struct {
			Email string `json:"email"`
		}
		if err := c.ShouldBindJSON(&in); err != nil {
			fail(c, http.StatusBadRequest, "invalid body: "+err.Error())
			return
		}
		if in.Email == "" {
			fail(c, http.StatusBadRequest, "email required")
			return
		}
		u, err := s.FindUserByEmail(c.Request.Context(), in.Email)
		if err != nil {
			fail(c, http.StatusUnauthorized, "no user with that email")
			return
		}
		c.JSON(http.StatusOK, u)
	}
}

// Logout is a no-op on the server (there's no session state) but the
// endpoint exists so the SPA has a real call to make on sign-out. It's
// also the kind of auditable event a real auth system records.
//
// POST /api/logout → 204
func Logout(s store.Store) gin.HandlerFunc {
	return func(c *gin.Context) {
		_, ok := requireActor(c, s)
		if !ok {
			return
		}
		c.Status(http.StatusNoContent)
	}
}

// Me returns the currently authenticated user. The SPA calls this on
// load to rehydrate its session from localStorage.
//
// GET /api/me → User
func Me(s store.Store) gin.HandlerFunc {
	return func(c *gin.Context) {
		u, ok := requireActor(c, s)
		if !ok {
			return
		}
		c.JSON(http.StatusOK, u)
	}
}
