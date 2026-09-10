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
	root := http.NewServeMux()
	root.Handle("/api/", api)
	root.Handle("/", staticHandler)

	log.Printf("listening on http://localhost:%s", port)
	if err := http.ListenAndServe(":"+port, root); err != nil {
		log.Fatal(err)
	}
}

func newStaticHandler() (http.Handler, error) {
	sub, err := fs.Sub(staticFS, "web/dist")
	if err != nil {
		return nil, err
	}
	return http.FileServer(http.FS(sub)), nil
}
