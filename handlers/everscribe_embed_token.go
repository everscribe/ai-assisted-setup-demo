package handlers

import (
	"net/http"

	"github.com/everscribe/sdk-go/everscribe"

	"github.com/everscribe/ai-assisted-setup-demo/store"
)

// EverscribeEmbedToken mints a short-lived embed token for the
// authenticated user and returns it as JSON. The Everscribe React
// component fetches this endpoint before rendering the audit feed.
//
// POST /api/everscribe/embed-token → {"token": "<signed-jwt>"}
func EverscribeEmbedToken(s *store.Store) http.HandlerFunc {
	// Build the SDK client once per handler registration; credentials
	// are read from EVERSCRIBE_API_KEY and EVERSCRIBE_PROJECT_ID env vars.
	client := everscribe.NewFromEnv()

	return func(w http.ResponseWriter, r *http.Request) {
		me, ok := actorFromRequest(r, s)
		if !ok {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}

		token, err := client.MintEmbedToken(r.Context(), everscribe.EmbedTokenParams{
			UserID:    me.ID,
			UserEmail: me.Email,
			UserName:  me.Name,
			OrgID:     me.OrganizationID,
		})
		if err != nil {
			http.Error(w, "failed to mint embed token", http.StatusInternalServerError)
			return
		}

		writeJSON(w, http.StatusOK, map[string]string{"token": token})
	}
}
