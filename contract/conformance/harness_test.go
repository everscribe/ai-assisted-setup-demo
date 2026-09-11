package conformance

import (
	"context"
	"path/filepath"
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

// apps returns the running apps, skipping the test when none are up so
// `go test ./...` is green on a machine with nothing running.
func apps(t *testing.T) []App {
	t.Helper()
	up := Discover(context.Background())
	if len(up) == 0 {
		t.Skip("no apps responding on /api/health; run `make up` first")
	}
	return up
}

// validated performs a request and schema-checks the response. Every
// request the harness makes goes through here, so layer 1 covers the
// whole surface rather than a sample of it.
func validated(t *testing.T, c *Client, method, path, actor, body string) *Response {
	t.Helper()
	resp, err := c.Do(context.Background(), method, path, actor, body)
	require.NoError(t, err, "%s: %s %s", c.App.Name, method, path)
	require.NoError(t, sharedSpec(t).ValidateResponse(context.Background(), method, path, resp),
		"%s: schema violation", c.App.Name)
	return resp
}
