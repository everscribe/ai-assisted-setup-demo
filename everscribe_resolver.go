package main

import (
	"context"

	"github.com/everscribe/sdk-go/pkg/event"
)

// resolveActor is the Everscribe ActorResolver for this service.
// It reads the *models.User that withSession planted on the request
// context and maps it to an event.Actor.
//
// TODO(everscribe): verify that sessionUserKey{} matches the key used in
// withSession (main.go). If your auth stack changes, update here too.
func resolveActor(ctx context.Context) event.Actor {
	u, ok := ctx.Value(sessionUserKey{}).(*userSession)
	if !ok || u == nil {
		return event.Actor{Type: "anonymous"}
	}
	return event.Actor{
		Type:        "user",
		ID:          u.ID,          // TODO(everscribe): confirm field name on your User model
		DisplayName: u.Name,        // TODO(everscribe): confirm field name on your User model
		Email:       u.Email,       // TODO(everscribe): confirm field name on your User model
	}
}
