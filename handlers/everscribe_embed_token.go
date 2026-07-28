package handlers

import (
	"net/http"

	everscribe "github.com/everscribe/sdk-go"

	"github.com/everscribe/ai-assisted-setup-demo/store"
)

// EverscribeEmbedToken mints a short-lived Everscribe embed token for the
// currently authenticated user and returns it as JSON.
//
// POST /everscribe/embed-token → {"token": "<signed-token>"}
func EverscribeEmbedToken(s *store.Store) http.HandlerFunc {
	client := everscribe.NewFromEnv()

	return func(w http.ResponseWriter, r *http.Request) {
		me, ok := actorFromRequest(r, s)
		if !ok {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}

		token, err := client.MintEmbedToken(r.Context(), everscribe.EmbedTokenParams{
			UserID:         me.ID,
			OrganizationID: me.OrganizationID,
		})
		if err != nil {
			http.Error(w, "failed to mint embed token", http.StatusInternalServerError)
			return
		}

		writeJSON(w, http.StatusOK, map[string]string{"token": token})
	}
}
