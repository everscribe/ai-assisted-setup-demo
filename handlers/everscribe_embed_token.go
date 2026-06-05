package handlers

import (
	"net/http"

	"github.com/everscribe/sdk-go/everscribe"

	"github.com/everscribe/ai-assisted-setup-demo/store"
)

// EverscribeEmbedToken mints a short-lived embed token for the currently
// authenticated user and returns it as JSON. The Everscribe React component
// calls this endpoint (via tokenEndpoint="/api/everscribe/embed-token") to
// obtain a token before rendering the audit-event feed.
//
// Credentials and project ID are read from the environment via NewFromEnv;
// set EVERSCRIBE_API_KEY and EVERSCRIBE_PROJECT_ID before starting the server.
//
// POST /api/everscribe/embed-token → {"token": "<signed-embed-token>"}
func EverscribeEmbedToken(s *store.Store) http.HandlerFunc {
	client := everscribe.NewFromEnv()

	return func(w http.ResponseWriter, r *http.Request) {
		u, ok := actorFromRequest(r, s)
		if !ok {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}

		token, err := client.MintEmbedToken(r.Context(), everscribe.EmbedTokenParams{
			UserID:         u.ID,
			OrganizationID: u.OrganizationID,
		})
		if err != nil {
			http.Error(w, "failed to mint embed token", http.StatusInternalServerError)
			return
		}

		writeJSON(w, http.StatusOK, map[string]string{"token": token})
	}
}
