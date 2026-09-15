// ai-assisted-setup-demo is a deliberately un-instrumented multi-tenant SaaS demo.
//
// The point of this repo is to be a clean target for Everscribe's
// auto-instrument feature: clone it, run our hosted instrument flow
// against it, watch the agent walk these handlers, propose audit events,
// and open a PR that adds the SDK + middleware + per-handler event calls
// (and drops the React audit-trail component into the SPA).
//
// Domain: internal access portal. Organizations (tenants) manage who has
// access to internal resources - databases, prod environments, customer
// data, third-party SaaS. Members request access with a reason; admins
// approve, deny, or revoke. Every mutating handler is an audit-worthy
// security event.
//
// Auth model is intentionally minimal: the SPA POSTs an email to
// /api/login to look up a seeded user, then attaches X-User-ID on every
// request. Real apps would use sessions or JWTs - the auto-instrument
// agent works with either shape.
package main

import (
	"context"
	"embed"
	"io/fs"
	"log"
	"net/http"
	"os"
	"strings"

	"github.com/everscribe/ai-assisted-setup-demo/apps/go/handlers"
	"github.com/everscribe/ai-assisted-setup-demo/apps/go/store/memory"
)

//go:embed all:web/dist
var staticFS embed.FS

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "9001"
	}

	st := memory.New()
	if err := st.Seed(context.Background()); err != nil {
		log.Fatalf("seed: %v", err)
	}

	api := handlers.Router(st)

	// Static SPA. /api/* hits the gin router; everything else falls
	// through to the embedded React build under web/dist.
	staticHandler, err := newStaticHandler()
	if err != nil {
		log.Fatalf("static fs: %v", err)
	}
	root := newRootMux(api, staticHandler)

	log.Printf("listening on http://localhost:%s", port)
	if err := http.ListenAndServe(":"+port, root); err != nil {
		log.Fatal(err)
	}
}

// newRootMux mounts the API under /api and the SPA everywhere else.
// Both "/api" and "/api/" are registered: with only the subtree, the mux
// answers a bare GET /api with a 301 and an HTML body, which is the same
// existence oracle the router closes for trailing slashes, on an
// otherwise all-JSON API.
//
// The path guard wraps the mux rather than living inside it: ServeMux
// cleans the path itself, so neither /api//me nor /api/./me reaches a
// handler, both 301 to /api/me. Same leak, one level up.
func newRootMux(api, static http.Handler) http.Handler {
	root := http.NewServeMux()
	root.Handle("/api", api)
	root.Handle("/api/", api)
	root.Handle("/", static)
	return rejectUncleanPaths(root)
}

// rejectUncleanPaths answers the API's own JSON 404 for any path
// ServeMux would clean and redirect: an empty segment, or a "." or ".."
// segment. Both spellings are checked, raw and decoded, so a %2e cannot
// smuggle a dot segment past the guard into the mux's redirect.
func rejectUncleanPaths(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if unclean(r.URL.EscapedPath()) || unclean(r.URL.Path) {
			w.Header().Set("Content-Type", "application/json; charset=utf-8")
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte(`{"error":"not found"}`))
			return
		}
		next.ServeHTTP(w, r)
	})
}

// unclean reports whether a path carries an empty, "." or ".." segment.
// A single trailing slash is the trailing-slash rule, not an empty
// segment, so it is left alone.
func unclean(p string) bool {
	if strings.Contains(p, "//") {
		return true
	}
	for _, seg := range strings.Split(p, "/") {
		if seg == "." || seg == ".." {
			return true
		}
	}
	return false
}

func newStaticHandler() (http.Handler, error) {
	sub, err := fs.Sub(staticFS, "web/dist")
	if err != nil {
		return nil, err
	}
	return http.FileServer(http.FS(sub)), nil
}
