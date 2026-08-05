package handlers

import (
	"net/http"

	"github.com/everscribe/ai-assisted-setup-demo/store"
	"github.com/everscribe/everscribe-go/everscribe"
)

// EverscribeEmbedToken mints a short-lived embed token for the
// requesting user and returns it as JSON. The React <EverscribeEvents />
// component fetches this endpoint automatically whenever its token
// nears expiry.
//
// Only authenticated users can reach this handler; non-admin callers
// will receive a 403 because the Audit tab is admin-only in the SPA,
// but we enforce it server-side as well for defence-in-depth.
//
// POST /api/everscribe/embed-token → {"token": "..."}
//
// Register in main.go:
//
//	api.HandleFunc("POST /api/everscribe/embed-token", handlers.EverscribeEmbedToken(st))
//
// Required environment variables (read automatically by NewFromEnv):
//
//	EVERSCRIBE_API_KEY   – your Everscribe secret API key
//	EVERSCRIBE_PROJECT_ID – the project whose events to surface
func EverscribeEmbedToken(s *store.Store) http.HandlerFunc {
	// Initialise the Everscribe client once per handler registration.
	// NewFromEnv reads EVERSCRIBE_API_KEY and EVERSCRIBE_PROJECT_ID from
	// the process environment; it panics at startup (not at request time)
	// if either variable is absent, which surfaces misconfiguration early.
	client := everscribe.NewFromEnv()

	return func(w http.ResponseWriter, r *http.Request) {
		// Resolve the caller from the X-User-ID header (same auth
		// convention used by every other handler in this service).
		me, ok := actorFromRequest(r, s)
		if !ok {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}

		// Restrict embed token issuance to admins only; the Audit tab
		// is not visible to regular members in the UI, but we guard
		// here too so the endpoint cannot be called directly.
		if !requireAdmin(w, me) {
			return
		}

		// Mint the embed token. The SDK scopes it to the authenticated
		// user's identity and the configured project automatically.
		token, err := client.MintEmbedToken(r.Context(), everscribe.MintEmbedTokenParams{
			UserID:         me.ID,
			UserEmail:      me.Email,
			OrganizationID: me.OrganizationID,
		})
		if err != nil {
			http.Error(w, "failed to mint embed token", http.StatusInternalServerError)
			return
		}

		writeJSON(w, http.StatusOK, map[string]string{
			"token": token,
		})
	}
}
