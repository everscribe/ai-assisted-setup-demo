package handlers

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/everscribe/sdk-go/pkg/minter"
)

// EverscribeEmbedToken mints a short-lived embed token for the
// authenticated user and returns it as JSON. The token is scoped to the
// user's organization (tenant) so the embed only shows that tenant's
// events.
//
// POST /everscribe/embed-token → {"token": "<jwt>"}
func EverscribeEmbedToken(m *minter.Client) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Re-use the same actorFromRequest pattern every other handler uses.
		// actorFromRequest reads the *models.User planted by withSession; if
		// no valid session is present it returns ok=false.
		me, ok := actorFromRequest(r, nil)
		if !ok {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}

		token, err := m.MintToken(r.Context(), minter.TokenOptions{
			// Scope the embed to the signed-in user's organization so that
			// members of one tenant cannot see another tenant's audit events.
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
