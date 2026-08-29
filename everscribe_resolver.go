package main

import (
	"context"
	"net/http"

	"github.com/everscribe/ai-assisted-setup-demo/models"
	"github.com/everscribe/ai-assisted-setup-demo/store"
	"github.com/everscribe/sdk-go/pkg/event"
)

// sessionUserKey is the context key under which withSession plants the
// authenticated *models.User.
type sessionUserKey struct{}

// withSession reads the X-User-ID header, resolves it to a *models.User
// via the store, and plants it on the request context before calling the
// next handler. Requests with a missing or unresolvable header proceed
// without a user — handlers that require authentication check for the
// user themselves (returning 401 on the false case), so withSession does
// not short-circuit here.
func withSession(s *store.Store, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := r.Header.Get("X-User-ID")
		if id != "" {
			if u, err := s.GetUser(id); err == nil {
				ctx := context.WithValue(r.Context(), sessionUserKey{}, u)
				r = r.WithContext(ctx)
			}
		}
		next.ServeHTTP(w, r)
	})
}

// resolveActor reads the *models.User that withSession planted on the
// context and converts it into an Everscribe Actor.
//
// TODO(everscribe): verify that sessionUserKey{} matches the key used in
// withSession, and that the User fields below (ID, Name, Email,
// OrganizationID) map correctly to your audit Actor model.
func resolveActor(s *store.Store) event.ActorResolver {
	return func(ctx context.Context) event.Actor {
		u, ok := ctx.Value(sessionUserKey{}).(*models.User)
		if !ok || u == nil {
			return event.Actor{Type: "anonymous"}
		}
		return event.Actor{
			Type:        "user",
			ID:          u.ID,
			DisplayName: u.Name,
			Email:       u.Email,
		}
	}
}
