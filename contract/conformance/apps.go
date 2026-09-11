// Package conformance decides whether a running app conforms to
// contract/openapi.yaml. It is HTTP only and imports nothing from apps/.
package conformance

import (
	"context"
	"net/http"
	"os"
	"strings"
	"time"
)

// App is one running demo app.
type App struct {
	Name    string
	BaseURL string
}

// knownApps is the fixed roster. Ports match docker-compose.yml.
var knownApps = []App{
	{Name: "go", BaseURL: "http://localhost:9001"},
	{Name: "node", BaseURL: "http://localhost:9002"},
	{Name: "rust", BaseURL: "http://localhost:9003"},
	{Name: "python", BaseURL: "http://localhost:9004"},
}

// Discover returns the apps answering on /api/health. CONFORMANCE_APPS
// overrides the roster, as "name=url,name=url".
func Discover(ctx context.Context) []App {
	return discover(ctx, roster())
}

func roster() []App {
	raw := os.Getenv("CONFORMANCE_APPS")
	if raw == "" {
		return knownApps
	}
	var out []App
	for _, pair := range strings.Split(raw, ",") {
		name, url, ok := strings.Cut(strings.TrimSpace(pair), "=")
		if ok && name != "" && url != "" {
			out = append(out, App{Name: name, BaseURL: strings.TrimRight(url, "/")})
		}
	}
	return out
}

func discover(ctx context.Context, candidates []App) []App {
	client := &http.Client{Timeout: 2 * time.Second}
	var up []App
	for _, a := range candidates {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, a.BaseURL+"/api/health", nil)
		if err != nil {
			continue
		}
		resp, err := client.Do(req)
		if err != nil {
			continue
		}
		_ = resp.Body.Close()
		if resp.StatusCode == http.StatusOK {
			up = append(up, a)
		}
	}
	return up
}
