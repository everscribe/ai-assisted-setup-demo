// Everscribe wiring: the actor context the audit middleware resolves
// from, and the embed-token endpoint the frontend's audit-trail
// component POSTs for a short-lived credential.
//
// event.ActorResolver takes a context.Context, not a *http.Request, so it
// can only see what an upstream middleware has already put in the
// context. This app has no session middleware - requireActor runs inside
// each handler, reading X-User-ID and hitting the store - so there was
// nothing in the context to resolve from and every event would have
// recorded as an anonymous actor. ActorContext below is that missing
// upstream step, and main.go mounts it OUTSIDE the audit middleware so
// it has run by the time the resolver is called.
package handlers

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/everscribe/sdk-go/pkg/event"
	"github.com/everscribe/sdk-go/pkg/minter"

	"github.com/everscribe/ai-assisted-setup-demo/apps/go/models"
	"github.com/everscribe/ai-assisted-setup-demo/apps/go/store"
)

// actorKey is the context key ActorContext stores the resolved user
// under. Unexported struct key so nothing outside this package can
// collide with it.
type actorKey struct{}

// ActorContext resolves X-User-ID once per request and stashes the user
// in the request context for ActorResolver to read.
//
// An absent or unresolvable id is not an error here: this middleware
// never rejects. Authorization stays where it already is, in each
// handler's requireActor, so the contract's check order is untouched and
// POST /api/login (which legitimately arrives with no X-User-ID) still
// reaches its handler.
func ActorContext(s store.Store) gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.GetHeader("X-User-ID")
		if id == "" {
			c.Next()
			return
		}
		u, err := s.GetUser(c.Request.Context(), id)
		if err != nil {
			c.Next()
			return
		}
		ctx := context.WithValue(c.Request.Context(), actorKey{}, u)
		c.Request = c.Request.WithContext(ctx)
		c.Next()
	}
}

// ActorFromContext returns the user ActorContext resolved, if any.
func ActorFromContext(ctx context.Context) (*models.User, bool) {
	u, ok := ctx.Value(actorKey{}).(*models.User)
	return u, ok && u != nil
}

// ActorResolver derives the audit Actor from the context ActorContext
// populated. An unresolved request records as an anonymous actor rather
// than failing - a failed sign-in is still an event worth keeping.
//
// Actor.Type uses the SDK's conventional values: "admin" for this app's
// admin role, "user" for everyone else.
func ActorResolver(ctx context.Context) event.Actor {
	u, ok := ActorFromContext(ctx)
	if !ok {
		return event.Actor{Type: "anonymous"}
	}
	typ := "user"
	if u.Role == models.RoleAdmin {
		typ = "admin"
	}
	return event.Actor{
		Type:        typ,
		ID:          u.ID,
		DisplayName: u.Name,
		Email:       u.Email,
	}
}

// EmbedToken mints the short-lived credential the embeddable audit-trail
// component needs, scoped to the caller's organization.
//
// POST, not GET: the component POSTs with no body, so a GET-only route
// would answer 405 and the embed would never load. Minting also issues a
// bearer credential and consumes quota, so it is not a safe idempotent
// read. Cache-Control: no-store on top, as defense in depth - a POST
// response is not cached without an explicit opt-in, but a bearer token
// sitting in a shared cache is the whole risk this endpoint carries.
//
// This route is mounted on the root mux, not the gin API router, so
// ActorContext has not run and it reads X-User-ID itself.
func EmbedToken(m *minter.Client, s store.Store) http.HandlerFunc {
	writeErr := func(w http.ResponseWriter, status int, msg string) {
		w.WriteHeader(status)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": msg})
	}

	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.Header().Set("Cache-Control", "no-store")

		if r.Method != http.MethodPost {
			w.Header().Set("Allow", "POST")
			writeErr(w, http.StatusMethodNotAllowed, "method not allowed")
			return
		}

		id := r.Header.Get("X-User-ID")
		if id == "" {
			writeErr(w, http.StatusUnauthorized, "unauthorized")
			return
		}
		u, err := s.GetUser(r.Context(), id)
		if err != nil {
			writeErr(w, http.StatusUnauthorized, "unauthorized")
			return
		}

		// Scoped to the caller's own organization: the token must not let
		// the component read another tenant's trail.
		token, err := m.MintToken(r.Context(), minter.TokenOptions{
			TenantID: u.OrganizationID,
		})
		if err != nil {
			writeErr(w, http.StatusBadGateway, "could not mint embed token")
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]string{"token": token})
	}
}
