// Package audit provides the Everscribe ActorResolver and the WithSession
// middleware for the ai-assisted-setup-demo service.
package audit

import (
	"context"
	"net/http"

	"github.com/everscribe/ai-assisted-setup-demo/models"
	"github.com/everscribe/ai-assisted-setup-demo/store"
	"github.com/everscribe/sdk-go/pkg/event"
)

// sessionUserKey is the context key used by WithSession to plant the
// authenticated *models.User on each request context.
type sessionUserKey struct{}

// WithSession is a middleware that reads the X-User-ID header, resolves the
// user from the store, and plants the result on the request context.
// It must wrap the audit middleware so that ResolveActor can read identity.
//
// This mirrors the existing per-handler actorFromRequest logic, centralised
// here so the audit adapter always sees a populated context.
func WithSession(s *store.Store, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := r.Header.Get("X-User-ID") // TODO(everscribe): adjust if auth scheme changes (e.g. JWT cookie)
		if id != "" {
			if u, err := s.GetUser(id); err == nil {
				ctx := context.WithValue(r.Context(), sessionUserKey{}, u)
				r = r.WithContext(ctx)
			}
		}
		next.ServeHTTP(w, r)
	})
}

// ResolveActor reads the *models.User that WithSession planted on the context
// and maps it to an Everscribe Actor.
//
// TODO(everscribe): verify field mapping matches your User model; adjust
// Actor.Type if you distinguish service accounts from human users.
func ResolveActor(s *store.Store) event.ActorResolver {
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
