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
	"errors"
	"io/fs"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/everscribe/ai-assisted-setup-demo/handlers"
	"github.com/everscribe/ai-assisted-setup-demo/internal/audit"
	"github.com/everscribe/ai-assisted-setup-demo/store"
	everscribe "github.com/everscribe/sdk-go"
	"github.com/everscribe/sdk-go/pkg/event"
	"github.com/everscribe/sdk-go/pkg/recorder"
)

//go:embed all:web/dist
var staticFS embed.FS

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "9001"
	}

	st := store.New()
	st.Seed()

	// ── Everscribe audit recorder ────────────────────────────────────────────
	es, err := everscribe.NewFromEnv()
	if err != nil {
		log.Fatalf("everscribe: %v", err)
	}
	rec := es.NewRecorder(recorder.WithBufferSize(2000))

	auditMW := event.Middleware(event.Options{
		ActorResolver: audit.ResolveActor(st),
		Recorder:      rec,
	})
	// ────────────────────────────────────────────────────────────────────────

	// ── Everscribe embed token minter ────────────────────────────────────────
	m := es.NewMinter()
	// ────────────────────────────────────────────────────────────────────────

	api := http.NewServeMux()

	// Auth
	api.HandleFunc("POST /api/login", handlers.Login(st))
	api.HandleFunc("POST /api/logout", handlers.Logout(st))
	api.HandleFunc("GET /api/me", handlers.Me(st))

	// Members (organization-scoped; admin-only writes)
	api.HandleFunc("GET /api/members", handlers.ListMembers(st))
	api.HandleFunc("POST /api/members", handlers.InviteMember(st))
	api.HandleFunc("DELETE /api/members/{id}", handlers.RemoveMember(st))
	api.HandleFunc("PUT /api/members/{id}/role", handlers.ChangeMemberRole(st))

	// Resources (things you can request access to)
	api.HandleFunc("GET /api/resources", handlers.ListResources(st))
	api.HandleFunc("POST /api/resources", handlers.RegisterResource(st))
	api.HandleFunc("POST /api/resources/{id}/decommission", handlers.DecommissionResource(st))

	// Access requests
	api.HandleFunc("GET /api/resources/{id}/requests", handlers.ListResourceRequests(st))
	api.HandleFunc("POST /api/resources/{id}/requests", handlers.RequestAccess(st))
	api.HandleFunc("POST /api/requests/{id}/approve", handlers.ApproveRequest(st))
	api.HandleFunc("POST /api/requests/{id}/deny", handlers.DenyRequest(st))
	api.HandleFunc("POST /api/requests/{id}/revoke", handlers.RevokeRequest(st))

	// Comments
	api.HandleFunc("GET /api/requests/{id}/comments", handlers.ListRequestComments(st))
	api.HandleFunc("POST /api/requests/{id}/comments", handlers.AddComment(st))

	// Health
	api.HandleFunc("GET /api/health", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	})

	// Wire: WithSession plants identity → auditMW reads it via ActorResolver.
	// Order matters: WithSession must be outermost so identity is on the
	// context before the audit adapter's ActorResolver runs.
	apiHandler := handlers.WithSession(st, auditMW(api))

	// Static SPA. /api/* hits the API mux; everything else falls
	// through to the embedded React build under web/dist.
	staticHandler, err := newStaticHandler()
	if err != nil {
		log.Fatalf("static fs: %v", err)
	}
	root := http.NewServeMux()
	root.Handle("/api/", apiHandler)

	// Everscribe embed-token minter route. Registered on the root mux so
	// it is reachable at the path the SPA's <AuditTrail> component posts
	// to. The handler enforces X-User-ID auth itself.
	root.HandleFunc("POST /everscribe/embed-token", handlers.EmbedToken(m, st))

	root.Handle("/", staticHandler)

	// ── Graceful shutdown ────────────────────────────────────────────────────
	ctx, stop := signal.NotifyContext(context.Background(),
		os.Interrupt, syscall.SIGTERM)
	defer stop()

	srv := &http.Server{Addr: ":" + port, Handler: root}
	go func() {
		log.Printf("listening on http://localhost:%s", port)
		if err := srv.ListenAndServe(); err != nil &&
			!errors.Is(err, http.ErrServerClosed) {
			log.Print(err)
			stop()
		}
	}()

	<-ctx.Done()
	if err := srv.Shutdown(context.Background()); err != nil {
		log.Printf("shutdown: %v", err)
	}
	// Close after Shutdown so in-flight requests finish recording first.
	if err := rec.Close(); err != nil {
		log.Printf("everscribe: drain incomplete: %v", err)
	}
	// ────────────────────────────────────────────────────────────────────────
}

func newStaticHandler() (http.Handler, error) {
	sub, err := fs.Sub(staticFS, "web/dist")
	if err != nil {
		return nil, err
	}
	return http.FileServer(http.FS(sub)), nil
}
