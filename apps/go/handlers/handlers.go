// Package handlers exposes the HTTP handlers for the demo. Each file
// groups one resource: auth, members, boards, tasks. Handlers depend on
// a *store.Store and reach the authenticated user via actorFromRequest,
// which reads the X-User-ID header set by the SPA's login.
//
// All mutating routes are workspace-scoped: the store enforces that
// resources live in the actor's workspace, returning ErrNotFound on
// cross-workspace access. Handlers translate ErrNotFound → 404 and
// ErrForbidden → 403.
package handlers

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/everscribe/ai-assisted-setup-demo/apps/go/models"
	"github.com/everscribe/ai-assisted-setup-demo/apps/go/store"
)

// actorFromRequest looks up the user identified by the X-User-ID header.
// Returns (user, true) on hit; (nil, false) if the header is missing or
// the id doesn't resolve. Handlers should return 401 on the false case.
func actorFromRequest(r *http.Request, s store.Store) (*models.User, bool) {
	id := r.Header.Get("X-User-ID")
	if id == "" {
		return nil, false
	}
	u, err := s.GetUser(r.Context(), id)
	if err != nil {
		return nil, false
	}
	return u, true
}

// writeJSON serializes v as JSON with the given status. Convenience to
// keep the body of every handler short.
func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

// httpError surfaces a store error as an HTTP response. ErrNotFound → 404,
// ErrForbidden → 403, anything else → 500. The body is the plain error
// message - enough for the SPA's error banner.
func httpError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, store.ErrNotFound):
		http.Error(w, "not found", http.StatusNotFound)
	case errors.Is(err, store.ErrForbidden):
		http.Error(w, "forbidden", http.StatusForbidden)
	default:
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

// decodeJSON reads a JSON request body into dst. Returns a 400 on parse
// failure (so the handler can `return` immediately).
func decodeJSON(w http.ResponseWriter, r *http.Request, dst any) bool {
	if err := json.NewDecoder(r.Body).Decode(dst); err != nil {
		http.Error(w, "invalid body: "+err.Error(), http.StatusBadRequest)
		return false
	}
	return true
}

// requireAdmin returns true if the actor's role is admin. Handlers that
// need admin gate themselves with this before mutating.
func requireAdmin(w http.ResponseWriter, u *models.User) bool {
	if u.Role != models.RoleAdmin {
		http.Error(w, "admin role required", http.StatusForbidden)
		return false
	}
	return true
}
