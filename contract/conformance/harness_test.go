package conformance

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"
)

var (
	specOnce   sync.Once
	loadedSpec *Spec
	specErr    error
)

func sharedSpec(t *testing.T) *Spec {
	t.Helper()
	specOnce.Do(func() {
		loadedSpec, specErr = LoadSpec(filepath.Join("..", "openapi.yaml"))
	})
	require.NoError(t, specErr)
	return loadedSpec
}

// apps returns the running apps. With nothing up it skips, so
// `go test ./...` stays green with no apps running. CONFORMANCE_REQUIRE_APPS
// (set by `make verify`) fails instead: a run that checked nothing must
// never report as a pass.
//
// CONFORMANCE_EXPECT_APPS names how many apps should be there, derived by
// `make verify` from docker-compose.yml. Without it, an app missing from
// compose but still in the roster is silently never tested and verify
// still exits 0, since discovery only fails when every app is missing.
func apps(t *testing.T) []App {
	t.Helper()
	up := Discover(context.Background())

	if want := expectedApps(t); want > len(up) {
		t.Fatalf("CONFORMANCE_EXPECT_APPS=%d but only %d app(s) answered /api/health (%s); "+
			"an app in the roster that never started would otherwise be silently untested",
			want, len(up), names(up))
	}

	if len(up) == 0 {
		if os.Getenv("CONFORMANCE_REQUIRE_APPS") != "" {
			t.Fatal("CONFORMANCE_REQUIRE_APPS is set but no app answered /api/health; every live test would have been skipped")
		}
		t.Skip("no apps responding on /api/health; run `make up` first")
	}
	return up
}

// expectedApps reads CONFORMANCE_EXPECT_APPS. Unset or empty means no
// expectation, which is what keeps a bare `go test ./...` skipping
// rather than failing. A value that is not a positive integer is fatal:
// a typo must not quietly disable the guard.
func expectedApps(t *testing.T) int {
	t.Helper()
	want, err := parseExpectedApps(os.Getenv("CONFORMANCE_EXPECT_APPS"))
	require.NoError(t, err)
	return want
}

func parseExpectedApps(raw string) (int, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return 0, nil
	}
	n, err := strconv.Atoi(raw)
	if err != nil || n < 1 {
		return 0, fmt.Errorf("CONFORMANCE_EXPECT_APPS must be a positive integer, got %q", raw)
	}
	return n, nil
}

// names renders the discovered apps for a failure message.
func names(up []App) string {
	if len(up) == 0 {
		return "none"
	}
	out := make([]string, 0, len(up))
	for _, a := range up {
		out = append(out, a.Name)
	}
	return strings.Join(out, ", ")
}

// validated performs a request and schema-checks the response. Every
// request the harness makes goes through here, so layer 1 covers the
// whole surface rather than a sample of it. note adds caller context to
// the failure message, which the lifecycle uses to name the step that
// broke rather than leaving the reader to count the script.
func validated(t *testing.T, c *Client, method, path, actor, body string, note ...string) *Response {
	t.Helper()
	where := fmt.Sprintf("%s: %s %s", c.App.Name, method, path)
	if len(note) > 0 {
		where += " [" + strings.Join(note, " ") + "]"
	}
	resp, err := c.Do(context.Background(), method, path, actor, body)
	require.NoError(t, err, where)
	require.NoError(t, sharedSpec(t).ValidateResponse(context.Background(), method, path, resp),
		"%s: schema violation", where)
	return resp
}
