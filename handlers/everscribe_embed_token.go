package handlers

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/everscribe/sdk-go/pkg/minter"
)

// EverscribeEmbedToken mints a short-lived embed token for the signed-in
// user and returns it as JSON. The token is tenant-scoped to the user's
// organization so the embed only shows events for that org.
//
// POST /everscribe/embed-token → {"token": "<jwt>"}
func EverscribeEmbedToken(m *minter.Client) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Reuse the same actorFromRequest helper used by all other handlers
		// so auth is consistent across the whole API surface.
		me, ok := actorFromRequest(r, nil)
		if !ok {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}

		token, err := m.MintToken(r.Context(), minter.TokenOptions{
			// Scope the token to the signed-in user's organization so the
			// embed only surfaces events belonging to that tenant.
			TenantID:  me.OrganizationID,
			ExpiresIn: time.Hour,
		})
		if err != nil {
			http.Error(w, "mint failed", http.StatusInternalServerError)
			return
		}

		// Never let a proxy or CDN cache a per-user token.
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"token": token})
	}
}
