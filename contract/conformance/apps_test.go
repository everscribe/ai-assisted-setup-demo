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

	// Live TCP connection but a failing health check. This must be
	// excluded too, and unlike the TCP-refusal case below it actually
	// exercises the status-code check in discover.
	unhealthy := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer unhealthy.Close()

	got := discover(context.Background(), []App{
		{Name: "live", BaseURL: live.URL},
		{Name: "unhealthy", BaseURL: unhealthy.URL},
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
	var gotContentType string
	var gotBodyLen int64
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, seen = r.Header["X-User-Id"]
		gotContentType = r.Header.Get("Content-Type")
		gotBodyLen = r.ContentLength
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer srv.Close()

	c := NewClient(App{Name: "t", BaseURL: srv.URL})
	_, err := c.Do(context.Background(), http.MethodGet, "/api/me", "", "")
	require.NoError(t, err)
	require.False(t, seen, "an empty actor must send no X-User-ID header at all")
	require.Empty(t, gotContentType, "an empty body must send no Content-Type header")
	require.Zero(t, gotBodyLen, "an empty body must send no body")
}

// The expectation guard is what makes `make verify` fail on an app that
// is in the roster but never started. Its parser is pinned here: unset
// means no expectation (a bare `go test ./...` must still skip), and a
// malformed value is an error rather than a silently disabled guard.
func TestParseExpectedApps(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		raw     string
		want    int
		wantErr bool
	}{
		{raw: "", want: 0},
		{raw: "   ", want: 0},
		{raw: "1", want: 1},
		{raw: " 4 ", want: 4},
		{raw: "0", wantErr: true},
		{raw: "-1", wantErr: true},
		{raw: "two", wantErr: true},
		{raw: "2.5", wantErr: true},
	} {
		got, err := parseExpectedApps(tc.raw)
		if tc.wantErr {
			require.Error(t, err, "%q", tc.raw)
			continue
		}
		require.NoError(t, err, "%q", tc.raw)
		require.Equal(t, tc.want, got, "%q", tc.raw)
	}
}
