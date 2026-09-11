package conformance

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestDiscoverFindsOnlyResponsiveApps(t *testing.T) {
	t.Parallel()

	live := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/api/health", r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	}))
	defer live.Close()

	got := discover(context.Background(), []App{
		{Name: "live", BaseURL: live.URL},
		{Name: "dead", BaseURL: "http://127.0.0.1:1"},
	})

	require.Len(t, got, 1)
	require.Equal(t, "live", got[0].Name)
}

func TestClientSendsActorHeader(t *testing.T) {
	t.Parallel()

	var gotActor, gotBody string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotActor = r.Header.Get("X-User-ID")
		buf := make([]byte, r.ContentLength)
		_, _ = r.Body.Read(buf)
		gotBody = string(buf)
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer srv.Close()

	c := NewClient(App{Name: "t", BaseURL: srv.URL})
	resp, err := c.Do(context.Background(), http.MethodPost, "/api/things", "u_ada", `{"a":1}`)
	require.NoError(t, err)

	require.Equal(t, http.StatusCreated, resp.Status)
	require.Equal(t, "u_ada", gotActor)
	require.Equal(t, `{"a":1}`, gotBody)
	require.JSONEq(t, `{"ok":true}`, string(resp.Body))
}

func TestClientOmitsActorHeaderWhenEmpty(t *testing.T) {
	t.Parallel()

	seen := true
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, seen = r.Header["X-User-Id"]
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer srv.Close()

	c := NewClient(App{Name: "t", BaseURL: srv.URL})
	_, err := c.Do(context.Background(), http.MethodGet, "/api/me", "", "")
	require.NoError(t, err)
	require.False(t, seen, "an empty actor must send no X-User-ID header at all")
}
