// Package audit holds the Everscribe ActorResolver for this service.
//
// Identity is planted on the request context by the withSession middleware
// in main.go, which reads the X-User-ID header and resolves the matching
// *models.User from the store before the audit adapter runs.
package audit

import (
	"context"

	"github.com/everscribe/ai-assisted-setup-demo/models"
	"github.com/everscribe/sdk-go/pkg/event"
)

// UserContextKey is the context key withSession uses to plant the resolved
// *models.User. It is exported so resolver.go and main.go share the same key
// without importing each other.
type UserContextKey struct{}

// ActorResolver reads the *models.User that withSession planted on the
// context and converts it into an Everscribe Actor.
//
// TODO(everscribe): verify that UserContextKey{} matches the key used in
// the withSession middleware in main.go. If the service migrates to JWTs
// or a real session cookie, update the lookup here to match.
func ActorResolver(ctx context.Context) event.Actor {
	u, ok := ctx.Value(UserContextKey{}).(*models.User)
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
