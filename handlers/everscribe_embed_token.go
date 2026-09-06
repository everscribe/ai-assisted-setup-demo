package handlers

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/everscribe/ai-assisted-setup-demo/store"
	"github.com/everscribe/sdk-go/pkg/minter"
)

// EmbedToken mints a short-lived Everscribe embed token for the
// authenticated user, scoped to their organization (tenant) so the
// embed only shows events for that org.
//
// POST /everscribe/embed-token → {"token": "<jwt>"}
func EmbedToken(m *minter.Client, s *store.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID := r.Header.Get("X-User-ID")
		if userID == "" {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}

		user, ok := s.GetUser(userID)
		if !ok {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}

		token, err := m.MintToken(r.Context(), minter.TokenOptions{
			TenantID:  user.OrganizationID,
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
