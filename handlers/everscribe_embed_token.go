package handlers

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/everscribe/sdk-go/pkg/minter"
	"github.com/everscribe/ai-assisted-setup-demo/store"
)

// EverscribeEmbedToken mints a short-lived embed token for the
// signed-in user and returns it as JSON. The token is scoped to the
// user's organization so the Everscribe Events component only shows
// events belonging to that tenant.
//
// POST /everscribe/embed-token → {"token": "<jwt>"}
func EverscribeEmbedToken(s *store.Store) http.HandlerFunc {
	// Construct the minter once at handler-creation time using
	// credentials from the environment (EVERSCRIBE_PROJECT_ID and
	// EVERSCRIBE_API_KEY). This mirrors how the recorder is
	// initialised elsewhere in the app and keeps credentials out of
	// the binary.
	es, err := everscribe.NewFromEnv()
	if err != nil {
		// Surface the misconfiguration immediately at startup rather
		// than silently returning 500 on every real request.
		panic("everscribe: " + err.Error())
	}
	m := es.NewMinter()

	return func(w http.ResponseWriter, r *http.Request) {
		me, ok := actorFromRequest(r, s)
		if !ok {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}

		token, err := m.MintToken(r.Context(), minter.TokenOptions{
			// Scope the token to the signed-in user's organization so
			// they only see their own tenant's audit events.
			TenantID:       me.OrganizationID,
			ExpiresIn:      time.Hour,
			AllowedColumns: []string{"occurred_at", "action", "actor", "target"},
		})
		if err != nil {
			http.Error(w, "mint failed", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"token": token})
	}
}
