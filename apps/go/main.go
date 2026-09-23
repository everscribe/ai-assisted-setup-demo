// ai-assisted-setup-demo is a deliberately un-instrumented multi-tenant SaaS demo:
// a clean target for Everscribe's auto-instrument feature to walk, propose
// audit events for, and open a PR against (SDK + middleware + per-handler
// event calls, plus the React audit-trail component in the SPA).
//
// Domain: internal access portal. Organizations manage access to internal
// resources; members request, admins approve/deny/revoke. Every mutating
// handler is an audit-worthy security event.
//
// Auth is intentionally minimal: the SPA POSTs an email to /api/login to
// look up a seeded user, then attaches X-User-ID on every request.
package main

import (
	"context"
	"embed"
	"io/fs"
	"log"
	"net/http"
	"os"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/everscribe/sdk-go/everscribe"
	"github.com/everscribe/sdk-go/pkg/event"

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

	// Credentials come from EVERSCRIBE_PROJECT_ID and EVERSCRIBE_API_KEY.
	// A missing credential turns auditing off rather than killing the
	// process: this app has to stay runnable on a clean checkout, which is
	// what make dev-go and the conformance harness depend on. The log line
	// is the only thing standing between that and a deploy that looks
	// healthy while silently recording nothing, so it says so plainly.
	var auditMW []gin.HandlerFunc
	var embedToken http.Handler

	es, err := everscribe.NewFromEnv()
	if err != nil {
		log.Printf("everscribe: audit logging DISABLED: %v", err)
	} else {
		rec := es.NewRecorder()
		defer rec.Close()

		// ActorContext resolves X-User-ID into the request context; the
		// audit middleware's resolver reads it from there. Order is
		// load-bearing: gin runs these in the order they are added, so the
		// audit middleware must come second or it would resolve against a
		// context nothing had populated yet and record every event
		// anonymously.
		auditMW = []gin.HandlerFunc{
			handlers.ActorContext(st),
			event.GinMiddleware(event.Options{
				ActorResolver: handlers.ActorResolver,
				Recorder:      rec,
			}),
		}
		embedToken = handlers.EmbedToken(es.NewMinter(), st)
	}

	api := handlers.Router(st, auditMW...)

	// Static SPA. /api/* hits the gin router; everything else falls
	// through to the embedded React build under web/dist.
	staticHandler, err := newStaticHandler()
	if err != nil {
		log.Fatalf("static fs: %v", err)
	}
	root := newRootMux(api, staticHandler, embedToken)

	log.Printf("listening on http://localhost:%s", port)
	if err := http.ListenAndServe(":"+port, root); err != nil {
		log.Fatal(err)
	}
}

// newRootMux mounts the API under /api, the embed-token endpoint at
// /everscribe/embed-token, and the SPA everywhere else.
//
// The embed route sits outside /api deliberately: it is not part of
// contract/openapi.yaml, and the conformance harness compares the four
// apps over the /api surface only. Putting it under /api would make this
// app answer a path the other three do not.
// Both "/api" and "/api/" are registered: with only the subtree, the mux
// answers a bare GET /api with a 301 and an HTML body, which is the same
// existence oracle the router closes for trailing slashes, on an
// otherwise all-JSON API.
//
// The path guard wraps the mux rather than living inside it: ServeMux
// cleans the path itself, so neither /api//me nor /api/./me reaches a
// handler, both 301 to /api/me. Same leak, one level up.
// A nil embedToken means Everscribe is not configured, and the route is
// simply not registered: it falls through to the SPA like any other
// unknown path, rather than existing and answering an error.
func newRootMux(api, static, embedToken http.Handler) http.Handler {
	root := http.NewServeMux()
	root.Handle("/api", api)
	root.Handle("/api/", api)
	if embedToken != nil {
		root.Handle("/everscribe/embed-token", embedToken)
	}
	root.Handle("/", static)
	return rejectUncleanPaths(root)
}

// rejectUncleanPaths answers the API's own JSON 404 for any path
// ServeMux would clean and redirect: an empty segment, or a "." or ".."
// segment.
//
// The raw spelling only. ServeMux matches on escaped segments, so a %2e
// is already an ordinary octet to it and needs no guard, while checking
// the decoded path here would reject a percent-encoded octet inside a
// captured id and break the rule three lines under the dot-segment one
// in openapi.yaml: raw routing is not raw path parameters.
func rejectUncleanPaths(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if unclean(r.URL.EscapedPath()) {
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
