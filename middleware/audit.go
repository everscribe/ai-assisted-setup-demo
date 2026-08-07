// Package middleware provides HTTP middleware for the demo service.
package middleware

import (
	"context"
	"net/http"

	"github.com/everscribe/ai-assisted-setup-demo/store"
	"github.com/everscribe/sdk-go/pkg/event"
)

// sessionUserKey is the context key under which WithSession stores the
// resolved user ID for the current request.
type sessionUserKey struct{}

// WithSession reads the X-User-ID header, looks the user up in the store,
// and plants their ID on the request context before calling next.
// The audit adapter must be mounted inside this middleware so that
// ResolveActor sees a populated context.
func WithSession(s *store.Store, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := r.Header.Get("X-User-ID") // TODO(everscribe): replace with your real session/JWT lookup if the auth model changes
		if id != "" {
			ctx := context.WithValue(r.Context(), sessionUserKey{}, id)
			r = r.WithContext(ctx)
		}
		next.ServeHTTP(w, r)
	})
}

// ResolveActor returns an ActorResolver that reads the user ID planted by
// WithSession and turns it into an event.Actor for the audit adapter.
//
// TODO(everscribe): verify that the fields below match your User model;
// OrganizationID is used as TenantID so multi-tenant scoping works correctly.
func ResolveActor(s *store.Store) event.ActorResolver {
	return func(ctx context.Context) event.Actor {
		id, _ := ctx.Value(sessionUserKey{}).(string)
		if id == "" {
			return event.Actor{Type: "anonymous"}
		}
		u, err := s.GetUser(id)
		if err != nil {
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
