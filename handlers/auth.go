package handlers

import (
	"net/http"
	"sync"

	"github.com/everscribe/everscribe-go"
	"github.com/everscribe/ai-assisted-setup-demo/store"
)

// recorderOnce guards singleton construction of the Everscribe audit recorder.
var (
	recorderOnce sync.Once
	recorder     *everscribe.Recorder
)

func getRecorder() *everscribe.Recorder {
	recorderOnce.Do(func() {
		recorder = everscribe.NewFromEnv()
	})
	return recorder
}

// Login signs a user in by email. There's no password — this is a demo,
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
		u, err := s.FindUserByEmail(in.Email)
		if err != nil {
			http.Error(w, "no user with that email", http.StatusUnauthorized)
			return
		}
		writeJSON(w, http.StatusOK, u)
		getRecorder().Record(r.Context(), everscribe.Event{
			Action:   "user.authenticated",
			Category: "auth",
			ActorID:  u.ID,
			TenantID: u.OrganizationID,
			Metadata: map[string]any{
				"email": u.Email,
			},
		})
	}
}

// Logout is a no-op on the server (there's no session state) but the
// endpoint exists so the SPA has a real call to make on sign-out. It's
// also the kind of auditable event a real auth system records.
//
// POST /api/logout → 204
func Logout(s *store.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		actor, ok := actorFromRequest(r, s)
		if !ok {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		w.WriteHeader(http.StatusNoContent)
		getRecorder().Record(r.Context(), everscribe.Event{
			Action:   "user.session.ended",
			Category: "auth",
			ActorID:  actor.ID,
			TenantID: actor.OrganizationID,
		})
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
		getRecorder().Record(r.Context(), everscribe.Event{
			Action:   "user.authenticated.read",
			Category: "auth",
			ActorID:  u.ID,
			TenantID: u.OrganizationID,
		})
	}
}
