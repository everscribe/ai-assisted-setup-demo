package handlers

import (
	"net/http"

	"github.com/everscribe/sdk-go/pkg/event"

	"github.com/everscribe/ai-assisted-setup-demo/store"
)

// Login signs a user in by email. There's no password - this is a demo,
// the email picks one of the seeded users from the cast. The frontend
// stashes the returned user id in localStorage and sends it as
// X-User-ID on every subsequent request.
//
// POST /api/login   {"email": "ada@acme.test"} → User
func Login(s *store.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var in struct {
			Email string `json:"email"`
		}
		if !decodeJSON(w, r, &in) {
			return
		}
		if in.Email == "" {
			http.Error(w, "email required", http.StatusBadRequest)
			return
		}

		e := event.Current(r.Context())
		e.Action = "user.login"
		e.Target = event.Target{Type: "user"}

		u, err := s.FindUserByEmail(in.Email)
		if err != nil {
			http.Error(w, "no user with that email", http.StatusUnauthorized)
			return
		}
		e.TenantID = u.OrganizationID
		e.Target.ID = u.ID
		writeJSON(w, http.StatusOK, u)
	}
}

// Logout is a no-op on the server (there's no session state) but the
// endpoint exists so the SPA has a real call to make on sign-out. It's
// also the kind of auditable event a real auth system records.
//
// POST /api/logout → 204
func Logout(s *store.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		e := event.Current(r.Context())
		e.Action = "user.logout"
		e.Target = event.Target{Type: "user"}

		u, ok := actorFromRequest(r, s)
		if !ok {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		e.TenantID = u.OrganizationID
		e.Target.ID = u.ID
		w.WriteHeader(http.StatusNoContent)
	}
}

// Me returns the currently authenticated user. The SPA calls this on
// load to rehydrate its session from localStorage.
//
// GET /api/me → User
func Me(s *store.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		u, ok := actorFromRequest(r, s)
		if !ok {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		writeJSON(w, http.StatusOK, u)
	}
}
