package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/everscribe/sdk-go/pkg/event"

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
		if err := bindJSON(c, &in); err != nil {
			fail(c, http.StatusBadRequest, "invalid body: "+err.Error())
			return
		}
		if in.Email == "" {
			fail(c, http.StatusBadRequest, "email required")
			return
		}

		// Named here, after the body checks but before the lookup, so a
		// rejected sign-in still records: the adapter fills Result from
		// the response, so the 401 below lands as user.login with an
		// error result and no tenant. The two 400s above return without
		// an Action and record nothing, which is the intent - a
		// malformed body reached no account to speak of.
		e := event.Current(c.Request.Context())
		e.Action = "user.login"

		u, err := s.FindUserByEmail(c.Request.Context(), in.Email)
		if err != nil {
			fail(c, http.StatusUnauthorized, "no user with that email")
			return
		}

		// Only known once the email resolves. A failed sign-in has no
		// tenant to attribute, by construction.
		e.Target = event.Target{Type: "user", ID: u.ID}
		e.TenantID = u.OrganizationID

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
		u, ok := requireActor(c, s)
		if !ok {
			return
		}

		e := event.Current(c.Request.Context())
		e.Action = "user.logout"
		e.Target = event.Target{Type: "user", ID: u.ID}
		e.TenantID = u.OrganizationID

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
