# Four-app demo, Plan 1: restructure and reference app

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Move the existing Go app to `apps/go`, port it from stdlib to Gin behind a `Store` interface with a plumbed context, make its behaviour deterministic, write that behaviour down as `contract/openapi.yaml` and `contract/seed.json`, and add the repo scaffolding that keeps `main` uninstrumented.

**Architecture:** One app becomes the reference implementation for three more that arrive in Plan 3. Handlers depend on a `store.Store` interface implemented only by `store/memory`. There is no database. The app serves its own React SPA from `go:embed`. A `make check-clean` guard, wired into CI, enforces that no Everscribe dependency ever reaches `main`.

**Tech Stack:** Go 1.25, Gin, testify/require, React + Vite, pnpm, Docker Compose, GitHub Actions.

**Spec:** `docs/superpowers/specs/2026-09-10-four-app-demo-design.md`

## Global Constraints

Every task's requirements implicitly include this section.

- **The repo must stay uninstrumented.** No Everscribe SDK in any manifest or lockfile, no `@everscribe/components-*` anywhere, no audit event recorded. The app carries seams (the store interface, the plumbed context) and nothing more.
- **Go directive is `1.25`**, matching `sdk-go`'s minimum. Below that, adding the SDK fails to resolve.
- **Module path is `github.com/everscribe/ai-assisted-setup-demo/apps/go`.** The repo root carries no `go.mod`.
- **Port is 9001**, read from `PORT` with 9001 as fallback.
- **Node package manager is pnpm.** The repo uses npm today; Task 6 migrates it.
- **No em dashes or en dashes** in prose, code comments, or commit messages. Use a hyphen or rephrase.
- **No `Co-Authored-By` trailer** on any commit.
- **Keep code comments short.** One line where one line does. The existing files carry long doc-comment blocks written before this rule; trim them during the port rather than copying them forward.
- **Tests use `t.Parallel()` and `testify/require`.**

---

### Task 1: Move the Go app to `apps/go`

Pure restructure. No behaviour changes. This task is done when the app builds and runs from its new home exactly as it did before.

**Files:**
- Move: `go.mod`, `main.go`, `handlers/`, `models/`, `store/`, `web/`, `Dockerfile` into `apps/go/`
- Modify: `apps/go/go.mod` (module path, go directive)
- Modify: every `.go` file's import block
- Modify: `apps/go/Dockerfile` (golang base image)
- Create: `.gitignore` at repo root, replacing the old one

- [ ] **Step 1: Move the tree**

```bash
mkdir -p apps/go
git mv go.mod main.go handlers models store web Dockerfile apps/go/
git mv .dockerignore apps/go/ 2>/dev/null || true
```

- [ ] **Step 2: Rewrite the module path and go directive**

`apps/go/go.mod` becomes:

```
module github.com/everscribe/ai-assisted-setup-demo/apps/go

go 1.25
```

Then rewrite every import:

```bash
cd apps/go
grep -rl 'github.com/everscribe/ai-assisted-setup-demo' --include='*.go' . \
  | xargs sed -i '' 's|github.com/everscribe/ai-assisted-setup-demo/|github.com/everscribe/ai-assisted-setup-demo/apps/go/|g'
```

Note the trailing slash in the match. Without it the `module` line in `go.mod` would also match if you ran this over non-Go files.

- [ ] **Step 3: Move the .gitignore and keep the embed placeholder**

The old `.gitignore` keeps `/web/dist/index.html` in source control so `go build` succeeds before the SPA is built. That negation must survive the move or `apps/go` stops compiling on a fresh clone. Replace the root `.gitignore` with:

```gitignore
# Go
bin/
*.test
*.out
/apps/*/server

# Node
apps/*/web/node_modules
apps/*/web/.vite

# Built SPA artefacts. Vite overwrites web/dist on every build.
# Keep the placeholder index.html in source control so `go build`
# works before the first build. Ignore the rest.
apps/*/web/dist/*
!apps/*/web/dist/index.html

# Editor / OS
.DS_Store
.idea/
.vscode/
*.swp
```

Confirm the placeholder is still tracked:

```bash
git ls-files apps/go/web/dist/index.html
```

Expected: the path prints. If it prints nothing, `git add -f apps/go/web/dist/index.html`.

- [ ] **Step 4: Bump the Dockerfile base image**

In `apps/go/Dockerfile`, change `FROM golang:1.22-alpine AS build` to `FROM golang:1.25-alpine AS build`. Leave the npm lines alone; Task 6 migrates them to pnpm.

- [ ] **Step 5: Verify it builds and runs**

```bash
cd apps/go && go build ./... && go vet ./...
PORT=9001 go run . &
sleep 2
curl -s localhost:9001/api/health
curl -s -X POST localhost:9001/api/login -d '{"email":"ada@acme.test"}'
kill %1
```

Expected: `{"status":"ok"}` then Ada's user JSON.

- [ ] **Step 6: Commit**

```bash
git add -A
git commit -m "refactor: move the go app to apps/go

Module path becomes .../apps/go and the go directive moves to 1.25 to
match sdk-go's minimum. The web/dist/index.html placeholder and its
gitignore negation move with it, without which apps/go does not compile
on a fresh clone."
```

---

### Task 2: Put handlers behind a `Store` interface with a plumbed context

**Files:**
- Create: `apps/go/store/store.go` (interface + errors only)
- Create: `apps/go/store/memory/memory.go` (the map implementation, moved from the old `store.go`)
- Create: `apps/go/store/memory/seed.go` (moved from the old `seed.go`)
- Create: `apps/go/store/memory/memory_test.go`
- Delete: `apps/go/store/seed.go`
- Modify: every file in `apps/go/handlers/`, `apps/go/main.go`

**Interfaces:**
- Consumes: the models from Task 1, unchanged.
- Produces: `store.Store` (the interface below), `memory.New() *memory.Store`, and the three sentinel errors `store.ErrNotFound`, `store.ErrForbidden`, `store.ErrConflict`. Task 4 consumes all of these.

- [ ] **Step 1: Write the failing test**

Create `apps/go/store/memory/memory_test.go`:

```go
package memory_test

import (
	"testing"

	"github.com/everscribe/ai-assisted-setup-demo/apps/go/store"
	"github.com/everscribe/ai-assisted-setup-demo/apps/go/store/memory"
	"github.com/stretchr/testify/require"
)

// memory.Store must satisfy the interface handlers depend on.
var _ store.Store = (*memory.Store)(nil)

func TestGetUserAcceptsContext(t *testing.T) {
	t.Parallel()
	s := memory.New()
	require.NoError(t, s.Seed(t.Context()))

	u, err := s.GetUser(t.Context(), "u_ada")
	require.NoError(t, err)
	require.Equal(t, "ada@acme.test", u.Email)
}

func TestCrossOrgAccessIsNotFound(t *testing.T) {
	t.Parallel()
	s := memory.New()
	require.NoError(t, s.Seed(t.Context()))

	// r_globex_k8s belongs to org_globex, not org_acme.
	_, err := s.GetResource(t.Context(), "org_acme", "r_globex_k8s")
	require.ErrorIs(t, err, store.ErrNotFound)
}

func TestListsReturnAnError(t *testing.T) {
	t.Parallel()
	s := memory.New()
	require.NoError(t, s.Seed(t.Context()))

	members, err := s.ListOrgMembers(t.Context(), "org_acme")
	require.NoError(t, err)
	require.Len(t, members, 2)
}
```

- [ ] **Step 2: Run it to make sure it fails**

```bash
cd apps/go && go test ./store/... -run 'TestGetUserAcceptsContext|TestCrossOrg|TestListsReturn' -v
```

Expected: compile failure, `undefined: store.Store` and `undefined: memory`.

- [ ] **Step 3: Write the interface**

Replace `apps/go/store/store.go` entirely with the interface and the errors. The map implementation moves to `memory` in the next step.

```go
// Package store is the persistence boundary. memory.Store is the only
// implementation today.
package store

import (
	"context"
	"errors"

	"github.com/everscribe/ai-assisted-setup-demo/apps/go/models"
)

// ErrNotFound covers both a missing resource and one that lives in a
// different organization. The two are deliberately indistinguishable so
// the tenant boundary does not leak existence.
var ErrNotFound = errors.New("not found")

// ErrForbidden is a structurally disallowed action, such as a non-admin
// managing members. Handlers map it to 403.
var ErrForbidden = errors.New("forbidden")

// ErrConflict is a state transition that does not apply, such as
// approving an already-denied request. Handlers map it to 409.
var ErrConflict = errors.New("conflict")

// Store is the persistence boundary. Every method takes a context, every
// method returns an error, and every mutation returns the updated entity.
// Tenant scoping lives here rather than in handlers: methods that touch a
// tenant-owned row take orgID and return ErrNotFound on a cross-tenant miss.
type Store interface {
	Seed(ctx context.Context) error

	ListOrganizations(ctx context.Context) ([]*models.Organization, error)
	GetOrganization(ctx context.Context, id string) (*models.Organization, error)

	GetUser(ctx context.Context, id string) (*models.User, error)
	FindUserByEmail(ctx context.Context, email string) (*models.User, error)
	ListOrgMembers(ctx context.Context, orgID string) ([]*models.User, error)
	InviteMember(ctx context.Context, orgID, name, email, role string) (*models.User, error)
	RemoveMember(ctx context.Context, orgID, userID string) error
	UpdateMemberRole(ctx context.Context, orgID, userID, role string) (*models.User, error)

	ListOrgResources(ctx context.Context, orgID string) ([]*models.Resource, error)
	GetResource(ctx context.Context, orgID, resourceID string) (*models.Resource, error)
	RegisterResource(ctx context.Context, orgID, name, description, sensitivity, registeredBy string) (*models.Resource, error)
	DecommissionResource(ctx context.Context, orgID, resourceID string) (*models.Resource, error)

	ListResourceRequests(ctx context.Context, orgID, resourceID string) ([]*models.AccessRequest, error)
	GetRequest(ctx context.Context, orgID, requestID string) (*models.AccessRequest, error)
	RequestAccess(ctx context.Context, orgID, resourceID, requestedBy, reason string) (*models.AccessRequest, error)
	ApproveRequest(ctx context.Context, orgID, requestID, approvedBy string) (*models.AccessRequest, error)
	DenyRequest(ctx context.Context, orgID, requestID, deniedBy string) (*models.AccessRequest, error)
	RevokeRequest(ctx context.Context, orgID, requestID, revokedBy string) (*models.AccessRequest, error)

	ListRequestComments(ctx context.Context, orgID, requestID string) ([]*models.Comment, error)
	AddComment(ctx context.Context, orgID, requestID, authorID, body string) (*models.Comment, error)
}
```

- [ ] **Step 4: Move the implementation into `store/memory`**

```bash
cd apps/go
mkdir -p store/memory
git mv store/seed.go store/memory/seed.go
# copy the map implementation out of the old store.go before overwriting it
```

In `store/memory/memory.go`, keep the `Store` struct, `New`, `removeString`, and `randomID` exactly as they are, then apply this transform to all twenty methods and `Seed`:

1. `package store` becomes `package memory`.
2. Add `import "context"` and import the `store` package for the sentinel errors.
3. Every `ErrNotFound` becomes `store.ErrNotFound`. Same for `ErrForbidden` and `ErrConflict`.
4. Every method takes `ctx context.Context` as its first parameter. The parameter is unused by the map implementation; name it `ctx` anyway rather than `_`, because a database-backed implementation would use it and the point of the seam is that it looks real.
5. The three methods that return a bare slice now return `(slice, error)` with a `nil` error.

Two worked examples. A read that already returned an error:

```go
// before
func (s *Store) GetUser(id string) (*models.User, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	u, ok := s.users[id]
	if !ok {
		return nil, ErrNotFound
	}
	return u, nil
}

// after
func (s *Store) GetUser(ctx context.Context, id string) (*models.User, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	u, ok := s.users[id]
	if !ok {
		return nil, store.ErrNotFound
	}
	return u, nil
}
```

A list that did not:

```go
// before
func (s *Store) ListOrgMembers(orgID string) []*models.User { ... return out }

// after
func (s *Store) ListOrgMembers(ctx context.Context, orgID string) ([]*models.User, error) {
	... 
	return out, nil
}
```

`Seed` becomes `func (s *Store) Seed(ctx context.Context) error` and returns `nil`.

The transform is mechanical across all twenty. Apply it uniformly rather than improving individual methods; behaviour changes belong in Tasks 3 and 4.

- [ ] **Step 5: Update the handlers and main.go**

Every handler constructor changes `s *store.Store` to `s store.Store`, and every call site gains `r.Context()` as its first argument. Example:

```go
// before
func Me(s *store.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		u, ok := actorFromRequest(r, s)
		...
	}
}

// after
func Me(s store.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		u, ok := actorFromRequest(r, s)
		...
	}
}
```

And `actorFromRequest` in `handlers.go`:

```go
func actorFromRequest(r *http.Request, s store.Store) (*models.User, bool) {
	id := r.Header.Get("X-User-ID")
	if id == "" {
		return nil, false
	}
	u, err := s.GetUser(r.Context(), id)
	if err != nil {
		return nil, false
	}
	return u, true
}
```

In `main.go`, `st := store.New(); st.Seed()` becomes:

```go
st := memory.New()
if err := st.Seed(context.Background()); err != nil {
	log.Fatalf("seed: %v", err)
}
```

- [ ] **Step 6: Run the tests**

```bash
cd apps/go && go build ./... && go test ./store/... -v
```

Expected: PASS, including the `var _ store.Store = (*memory.Store)(nil)` compile-time assertion.

- [ ] **Step 7: Commit**

```bash
git add -A
git commit -m "refactor: put handlers behind a store interface

Handlers now depend on store.Store rather than a concrete map. Every
method takes a context and returns an error, and mutations return the
updated entity, so the seam looks like a real repository layer. The map
implementation moves to store/memory unchanged."
```

---

### Task 3: Make the app deterministic

Two changes the conformance harness depends on: seeded timestamps stop moving, and every list has a total order. Both are behaviour changes with tests.

**Files:**
- Modify: `apps/go/store/memory/seed.go`
- Modify: `apps/go/store/memory/memory.go` (the five sort sites)
- Modify: `apps/go/store/memory/memory_test.go`

**Interfaces:**
- Consumes: `memory.New`, `Seed` from Task 2.
- Produces: fixed seed IDs and timestamps that Task 5 writes into `contract/seed.json`.

- [ ] **Step 1: Write the failing tests**

Append to `apps/go/store/memory/memory_test.go`:

```go
func TestSeedTimestampsAreFixed(t *testing.T) {
	t.Parallel()
	a, b := memory.New(), memory.New()
	require.NoError(t, a.Seed(t.Context()))
	require.NoError(t, b.Seed(t.Context()))

	ua, err := a.GetUser(t.Context(), "u_ada")
	require.NoError(t, err)
	ub, err := b.GetUser(t.Context(), "u_ada")
	require.NoError(t, err)

	// Two independently seeded stores must agree exactly. With now-relative
	// offsets these differ by the time between the two Seed calls.
	require.True(t, ua.CreatedAt.Equal(ub.CreatedAt),
		"seed timestamps must be absolute, got %s and %s", ua.CreatedAt, ub.CreatedAt)
	require.Equal(t, 2026, ua.CreatedAt.Year())
}

func TestCommentOrderBreaksTiesOnID(t *testing.T) {
	t.Parallel()
	s := memory.New()
	require.NoError(t, s.Seed(t.Context()))

	// Post three comments fast enough to collide on the timestamp.
	for i := 0; i < 3; i++ {
		_, err := s.AddComment(t.Context(), "org_acme", "req_acme_1", "u_ada", "note")
		require.NoError(t, err)
	}

	got, err := s.ListRequestComments(t.Context(), "org_acme", "req_acme_1")
	require.NoError(t, err)
	require.GreaterOrEqual(t, len(got), 3)

	for i := 1; i < len(got); i++ {
		prev, cur := got[i-1], got[i]
		if prev.CreatedAt.Equal(cur.CreatedAt) {
			require.Less(t, prev.ID, cur.ID,
				"comments with equal timestamps must order by id ascending")
		} else {
			require.True(t, prev.CreatedAt.Before(cur.CreatedAt))
		}
	}
}
```

`req_acme_1` is the seeded pending request on `r_acme_pg` in `org_acme`. The five seeded request ids are `req_acme_1` (pending), `req_acme_2` (approved), `req_acme_3` (denied), `req_globex_1` (approved), and `req_globex_2` (revoked).

- [ ] **Step 2: Run them to make sure they fail**

```bash
cd apps/go && go test ./store/... -run 'TestSeedTimestampsAreFixed|TestCommentOrder' -v
```

Expected: `TestSeedTimestampsAreFixed` fails on the `Equal` assertion. `TestCommentOrderBreaksTiesOnID` may pass by luck; run it several times with `-count=20` to see it flake. That flakiness is the bug.

- [ ] **Step 3: Pin the seed timestamps**

In `store/memory/seed.go`, replace the now-relative header:

```go
// before
now := time.Now().UTC()
earlier := now.Add(-2 * time.Hour)
yesterday := now.Add(-24 * time.Hour)

// after
// Fixed instants, not now-relative offsets, so two independently seeded
// stores agree exactly and the contract can assert created_at directly.
now := time.Date(2026, 3, 14, 12, 0, 0, 0, time.UTC)
earlier := now.Add(-2 * time.Hour)
yesterday := now.Add(-24 * time.Hour)
```

Everything downstream of those three variables stays as it is.

- [ ] **Step 4: Give every list a total order**

There are five sort sites in `store/memory/memory.go`. Three already sort by ID alone and are total; leave them. The two that sort by `CreatedAt` need an ID tiebreak and a stable sort:

```go
// requests: newest first, id ascending on a tie
sort.SliceStable(out, func(i, j int) bool {
	if out[i].CreatedAt.Equal(out[j].CreatedAt) {
		return out[i].ID < out[j].ID
	}
	return out[i].CreatedAt.After(out[j].CreatedAt)
})

// comments: oldest first, id ascending on a tie
sort.SliceStable(out, func(i, j int) bool {
	if out[i].CreatedAt.Equal(out[j].CreatedAt) {
		return out[i].ID < out[j].ID
	}
	return out[i].CreatedAt.Before(out[j].CreatedAt)
})
```

Change the other three from `sort.Slice` to `sort.SliceStable` as well. Sorting by ID alone is already total so it makes no difference here, but the three new ports in Plan 3 are written by reading this file, and a uniform rule is one less thing to get wrong.

- [ ] **Step 5: Run the tests**

```bash
cd apps/go && go test ./store/... -count=20
```

Expected: PASS, twenty times. The `-count=20` matters: a single green run does not distinguish a fixed tiebreak from a lucky one.

- [ ] **Step 6: Commit**

```bash
git add -A
git commit -m "fix: make seed timestamps and list ordering deterministic

Seed built every timestamp from time.Now(), so two stores never agreed
and the contract could not assert created_at. Lists sorted on CreatedAt
with no tiebreak over random hex ids, via an unstable sort, so equal
timestamps ordered arbitrarily. Both are load-bearing for the cross-app
comparison in plan 3."
```

---

### Task 4: Port the handlers to Gin, with JSON errors

**Files:**
- Modify: `apps/go/main.go`
- Modify: all five files in `apps/go/handlers/`
- Create: `apps/go/handlers/handlers_test.go`
- Modify: `apps/go/go.mod`, `apps/go/go.sum`

**Interfaces:**
- Consumes: `store.Store` and the sentinel errors from Task 2.
- Produces: `handlers.Router(s store.Store) *gin.Engine`, which Task 5's contract test and Plan 2's harness both use.

- [ ] **Step 1: Add Gin and testify**

```bash
cd apps/go
go get github.com/gin-gonic/gin@latest
go get github.com/stretchr/testify@latest
go mod tidy
```

- [ ] **Step 2: Write the failing test**

Create `apps/go/handlers/handlers_test.go`:

```go
package handlers_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/everscribe/ai-assisted-setup-demo/apps/go/handlers"
	"github.com/everscribe/ai-assisted-setup-demo/apps/go/store/memory"
	"github.com/stretchr/testify/require"
)

func newRouter(t *testing.T) http.Handler {
	t.Helper()
	s := memory.New()
	require.NoError(t, s.Seed(t.Context()))
	return handlers.Router(s)
}

func do(t *testing.T, r http.Handler, method, path, actor, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	if actor != "" {
		req.Header.Set("X-User-ID", actor)
	}
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

func TestErrorsAreJSON(t *testing.T) {
	t.Parallel()
	r := newRouter(t)

	w := do(t, r, http.MethodGet, "/api/me", "", "")
	require.Equal(t, http.StatusUnauthorized, w.Code)
	require.Equal(t, "application/json; charset=utf-8", w.Header().Get("Content-Type"))

	var body map[string]string
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	require.Equal(t, "unauthorized", body["error"])
}

func TestUnknownAPIPathIsJSON404(t *testing.T) {
	t.Parallel()
	r := newRouter(t)

	w := do(t, r, http.MethodGet, "/api/nope", "u_ada", "")
	require.Equal(t, http.StatusNotFound, w.Code)
	require.JSONEq(t, `{"error":"not found"}`, w.Body.String())
}

func TestCrossTenantReadIs404WithAPositiveControl(t *testing.T) {
	t.Parallel()
	r := newRouter(t)

	// Positive control first. Without it this test also passes when the
	// route was never registered, since an unknown path returns the same
	// body by design.
	ok := do(t, r, http.MethodGet, "/api/resources/r_acme_pg/requests", "u_ada", "")
	require.Equal(t, http.StatusOK, ok.Code, "positive control: same-tenant read must work")

	// u_ada is in org_acme; r_globex_k8s is in org_globex.
	w := do(t, r, http.MethodGet, "/api/resources/r_globex_k8s/requests", "u_ada", "")
	require.Equal(t, http.StatusNotFound, w.Code)
	require.JSONEq(t, `{"error":"not found"}`, w.Body.String())
}

func TestMemberCannotWriteAdminRoute(t *testing.T) {
	t.Parallel()
	r := newRouter(t)

	// u_linus is a member of org_acme, not an admin.
	w := do(t, r, http.MethodPost, "/api/resources", "u_linus",
		`{"name":"x","description":"y","sensitivity":"internal"}`)
	require.Equal(t, http.StatusForbidden, w.Code)
	require.JSONEq(t, `{"error":"admin role required"}`, w.Body.String())
}

func TestAnonymousRoutes(t *testing.T) {
	t.Parallel()
	r := newRouter(t)

	require.Equal(t, http.StatusOK, do(t, r, http.MethodGet, "/api/health", "", "").Code)
	require.Equal(t, http.StatusOK,
		do(t, r, http.MethodPost, "/api/login", "", `{"email":"ada@acme.test"}`).Code)
}
```

- [ ] **Step 3: Run it to make sure it fails**

```bash
cd apps/go && go test ./handlers/... -v
```

Expected: compile failure, `undefined: handlers.Router`.

- [ ] **Step 4: Write the Gin router and the JSON error helper**

Add to `apps/go/handlers/handlers.go`:

```go
// fail writes a JSON error body. Every error response in this app is
// JSON; the contract depends on it and http.Error writes plain text.
func fail(c *gin.Context, status int, msg string) {
	c.AbortWithStatusJSON(status, gin.H{"error": msg})
}

// failStore maps a store error to its status. ErrNotFound covers both a
// missing row and a cross-tenant miss, on purpose.
func failStore(c *gin.Context, err error) {
	switch {
	case errors.Is(err, store.ErrNotFound):
		fail(c, http.StatusNotFound, "not found")
	case errors.Is(err, store.ErrForbidden):
		fail(c, http.StatusForbidden, "forbidden")
	case errors.Is(err, store.ErrConflict):
		fail(c, http.StatusConflict, "conflict")
	default:
		fail(c, http.StatusInternalServerError, err.Error())
	}
}

// actor resolves X-User-ID. Handlers call requireActor instead.
func actor(c *gin.Context, s store.Store) (*models.User, bool) {
	id := c.GetHeader("X-User-ID")
	if id == "" {
		return nil, false
	}
	u, err := s.GetUser(c.Request.Context(), id)
	if err != nil {
		return nil, false
	}
	return u, true
}

func requireActor(c *gin.Context, s store.Store) (*models.User, bool) {
	u, ok := actor(c, s)
	if !ok {
		fail(c, http.StatusUnauthorized, "unauthorized")
		return nil, false
	}
	return u, true
}

func requireAdmin(c *gin.Context, u *models.User) bool {
	if u.Role != models.RoleAdmin {
		fail(c, http.StatusForbidden, "admin role required")
		return false
	}
	return true
}

// Router builds the full API. The eighteen registrations here are the
// contract; all four apps in this repo register the same set.
func Router(s store.Store) *gin.Engine {
	gin.SetMode(gin.ReleaseMode)
	r := gin.New()
	r.Use(gin.Recovery())

	// An unknown /api/* path returns the same JSON 404 as a cross-tenant
	// miss. The two are told apart by the request path, not the response.
	r.NoRoute(func(c *gin.Context) {
		fail(c, http.StatusNotFound, "not found")
	})

	api := r.Group("/api")
	{
		api.POST("/login", Login(s))
		api.POST("/logout", Logout(s))
		api.GET("/me", Me(s))

		api.GET("/members", ListMembers(s))
		api.POST("/members", InviteMember(s))
		api.DELETE("/members/:id", RemoveMember(s))
		api.PUT("/members/:id/role", ChangeMemberRole(s))

		api.GET("/resources", ListResources(s))
		api.POST("/resources", RegisterResource(s))
		api.POST("/resources/:id/decommission", DecommissionResource(s))

		api.GET("/resources/:id/requests", ListResourceRequests(s))
		api.POST("/resources/:id/requests", RequestAccess(s))
		api.POST("/requests/:id/approve", ApproveRequest(s))
		api.POST("/requests/:id/deny", DenyRequest(s))
		api.POST("/requests/:id/revoke", RevokeRequest(s))

		api.GET("/requests/:id/comments", ListRequestComments(s))
		api.POST("/requests/:id/comments", AddComment(s))

		api.GET("/health", func(c *gin.Context) {
			c.JSON(http.StatusOK, gin.H{"status": "ok"})
		})
	}
	return r
}
```

Gin cannot register `/resources/:id/requests` and `/requests/:id/comments` under conflicting wildcard names in the same position. Both use `:id`, which is consistent, so this set registers cleanly. If Gin panics with a wildcard conflict, the cause is a mismatched parameter name, not a route that needs restructuring.

- [ ] **Step 5: Convert every handler**

Each handler changes from `func Name(s store.Store) http.HandlerFunc` to `func Name(s store.Store) gin.HandlerFunc`. The transform per handler:

- `w http.ResponseWriter, r *http.Request` becomes `c *gin.Context`
- `r.PathValue("id")` becomes `c.Param("id")`
- `decodeJSON(w, r, &in)` becomes `c.ShouldBindJSON(&in)`, failing with `fail(c, http.StatusBadRequest, "invalid body: "+err.Error())`
- `writeJSON(w, status, v)` becomes `c.JSON(status, v)`
- `http.Error(w, msg, status)` becomes `fail(c, status, msg)`
- `httpError(w, err)` becomes `failStore(c, err)`
- every store call gains `c.Request.Context()` as its first argument

Worked example, the whole of `Me`:

```go
// GET /api/me
func Me(s store.Store) gin.HandlerFunc {
	return func(c *gin.Context) {
		u, ok := requireActor(c, s)
		if !ok {
			return
		}
		c.JSON(http.StatusOK, u)
	}
}
```

And one that takes a body and a path param, `RequestAccess`:

```go
// POST /api/resources/:id/requests
func RequestAccess(s store.Store) gin.HandlerFunc {
	return func(c *gin.Context) {
		u, ok := requireActor(c, s)
		if !ok {
			return
		}
		var in struct {
			Reason string `json:"reason"`
		}
		if err := c.ShouldBindJSON(&in); err != nil {
			fail(c, http.StatusBadRequest, "invalid body: "+err.Error())
			return
		}
		req, err := s.RequestAccess(c.Request.Context(), u.OrganizationID, c.Param("id"), u.ID, in.Reason)
		if err != nil {
			failStore(c, err)
			return
		}
		c.JSON(http.StatusCreated, req)
	}
}
```

Delete `writeJSON`, `httpError`, `decodeJSON`, and `actorFromRequest` once nothing references them.

Preserve every existing status code. `Logout` still returns 204, `RequestAccess` still returns 201, and the admin gates stay on the same routes they guard today.

- [ ] **Step 6: Rewrite main.go**

```go
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
```

The outer `ServeMux` stays. Gin owns `/api/`; the embedded SPA owns everything else. Keeping the split means the SPA fallback does not have to be expressed as a Gin `NoRoute`, which would collide with the JSON 404 the contract requires.

- [ ] **Step 7: Run the tests**

```bash
cd apps/go && go build ./... && go test ./... -count=1 -v
```

Expected: PASS, all five handler tests and all store tests.

- [ ] **Step 8: Smoke test the real binary**

```bash
cd apps/go && PORT=9001 go run . &
sleep 2
curl -si localhost:9001/api/me | head -1                      # 401
curl -s localhost:9001/api/me                                 # {"error":"unauthorized"}
curl -s localhost:9001/api/nope -H 'X-User-ID: u_ada'         # {"error":"not found"}
curl -s localhost:9001/ | head -c 40                          # the SPA, not JSON
kill %1
```

- [ ] **Step 9: Commit**

```bash
git add -A
git commit -m "refactor: port the handlers to gin with JSON errors

Every error response was plain text via http.Error; the contract requires
JSON, so errors now go through a single fail helper. Unknown /api paths
and cross-tenant misses return an identical {\"error\":\"not found\"} so
the tenant boundary does not leak existence. The outer ServeMux keeps
serving the embedded SPA so gin's NoRoute stays free for the JSON 404."
```

---

### Task 5: Write the contract

**Files:**
- Create: `contract/openapi.yaml`
- Create: `contract/seed.json`
- Create: `apps/go/handlers/contract_test.go`

**Interfaces:**
- Consumes: `handlers.Router` from Task 4, plus the two test helpers Task 4 defines in `apps/go/handlers/handlers_test.go`: `newRouter(t *testing.T) http.Handler` and `do(t *testing.T, r http.Handler, method, path, actor, body string) *httptest.ResponseRecorder`. Both files are in package `handlers_test`, so do not redefine them here; read Task 4's test file first.
- Produces: `contract/openapi.yaml` and `contract/seed.json`, which Plan 2's harness reads and Plan 3's ports are written against.

- [ ] **Step 1: Write the failing test**

Create `apps/go/handlers/contract_test.go`:

```go
package handlers_test

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestSeedMatchesContract asserts the running app serves exactly what
// contract/seed.json declares. Each app hardcodes its own seed, so this
// is what stops the four from drifting.
func TestSeedMatchesContract(t *testing.T) {
	t.Parallel()
	r := newRouter(t)

	raw, err := os.ReadFile(filepath.Join("..", "..", "..", "contract", "seed.json"))
	require.NoError(t, err)

	var want struct {
		Users     []map[string]any `json:"users"`
		Resources []map[string]any `json:"resources"`
	}
	require.NoError(t, json.Unmarshal(raw, &want))

	w := do(t, r, http.MethodGet, "/api/members", "u_ada", "")
	require.Equal(t, http.StatusOK, w.Code)

	var got []map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &got))

	var wantAcme []map[string]any
	for _, u := range want.Users {
		if u["organization_id"] == "org_acme" {
			wantAcme = append(wantAcme, u)
		}
	}
	require.Equal(t, wantAcme, got, "served members must match contract/seed.json exactly")
}
```

- [ ] **Step 2: Run it to make sure it fails**

```bash
cd apps/go && go test ./handlers/... -run TestSeedMatchesContract -v
```

Expected: FAIL, `no such file or directory` for `contract/seed.json`.

- [ ] **Step 3: Generate seed.json from the running app**

Do not hand-write this file. Boot the app and capture what it actually serves, then check the result over against the seed source by eye.

```bash
cd apps/go && PORT=9001 go run . &
sleep 2
mkdir -p ../../contract
{
  echo '{'
  echo '  "users":'      && curl -s localhost:9001/api/members   -H 'X-User-ID: u_ada'   > /tmp/acme_users.json
  echo '  "resources":'  && curl -s localhost:9001/api/resources -H 'X-User-ID: u_ada'   > /tmp/acme_res.json
} >/dev/null
kill %1
```

Then assemble `contract/seed.json` by hand from those captures plus the Globex half (fetch with `X-User-ID: u_grace`), so the file covers both tenants:

```json
{
  "organizations": [
    {"id": "org_acme",   "name": "Acme Corp"},
    {"id": "org_globex", "name": "Globex"}
  ],
  "users": [
    {"id": "u_ada",   "name": "Ada Lovelace",   "email": "ada@acme.test",     "role": "admin",  "organization_id": "org_acme",   "created_at": "2026-03-13T12:00:00Z"},
    {"id": "u_linus", "name": "Linus Torvalds", "email": "linus@acme.test",   "role": "member", "organization_id": "org_acme",   "created_at": "2026-03-13T12:00:00Z"},
    {"id": "u_grace", "name": "Grace Hopper",   "email": "grace@globex.test", "role": "admin",  "organization_id": "org_globex", "created_at": "2026-03-13T12:00:00Z"},
    {"id": "u_alan",  "name": "Alan Turing",    "email": "alan@globex.test",  "role": "member", "organization_id": "org_globex", "created_at": "2026-03-13T12:00:00Z"}
  ],
  "resources": [],
  "access_requests": [],
  "comments": []
}
```

Fill `resources`, `access_requests`, and `comments` from the captures. The `created_at` values above assume the `time.Date(2026, 3, 14, 12, 0, 0, 0, time.UTC)` anchor from Task 3 with the `yesterday` offset; use whatever the app actually emits rather than these literals if they differ.

- [ ] **Step 4: Write openapi.yaml**

Cover all eighteen routes. The pieces that are load-bearing rather than descriptive:

```yaml
openapi: 3.0.3
info:
  title: Access portal demo
  version: "1.0.0"
  description: >
    The contract all four demo apps implement. Written from the Go app's
    behaviour. Every app registers exactly these eighteen routes.

components:
  schemas:
    Error:
      type: object
      required: [error]
      properties:
        error: {type: string}
      additionalProperties: false
    User:
      type: object
      required: [id, name, email, role, organization_id, created_at]
      properties:
        id: {type: string}
        name: {type: string}
        email: {type: string}
        role: {type: string, enum: [admin, member]}
        organization_id: {type: string}
        created_at: {type: string, format: date-time}
      additionalProperties: false
  responses:
    NotFound:
      description: >
        Returned both for an unknown path and for a resource in another
        tenant. The two are deliberately identical so the boundary does
        not leak existence.
      content:
        application/json:
          schema: {$ref: '#/components/schemas/Error'}
          example: {error: not found}

x-ordering:
  description: >
    Total order for every list endpoint. The domain key, then id ascending
    as tiebreak. Without the tiebreak, equal timestamps order arbitrarily
    and Go's unstable sort diverges from the stable sorts in JS, Python,
    and Rust.
  GET /api/members:                   [id asc]
  GET /api/resources:                 [id asc]
  GET /api/resources/{id}/requests:   [created_at desc, id asc]
  GET /api/requests/{id}/comments:    [created_at asc, id asc]
```

Write the remaining schemas (`Organization`, `Resource`, `AccessRequest`, `Comment`) from `apps/go/models/types.go`, matching the JSON tags exactly, and one path entry per route with its status codes. Every 4xx response references `#/components/schemas/Error`.

- [ ] **Step 5: Run the test**

```bash
cd apps/go && go test ./handlers/... -run TestSeedMatchesContract -v
```

Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add -A
git commit -m "feat: write the contract the four apps implement

openapi.yaml covers all eighteen routes and declares a total order for
every list endpoint. seed.json is generated from the running app rather
than hand-written, and a test asserts the app still matches it, which is
what stops the four ports drifting in plan 3."
```

---

### Task 6: Root tooling and the pnpm migration

**Files:**
- Create: `Makefile`, `docker-compose.yml`, `README.md` at the repo root
- Create: `apps/go/README.md`
- Modify: `apps/go/Dockerfile`
- Delete: `apps/go/web/package-lock.json`
- Create: `apps/go/web/pnpm-lock.yaml`
- Delete: the old root `Makefile` if Task 1 left one in `apps/go`

**Interfaces:**
- Consumes: the working app from Task 4.
- Produces: `make up`, `make down`, `make dev-go`, and a `docker-compose.yml` that Plan 2's `make verify` extends.

- [ ] **Step 1: Migrate the frontend from npm to pnpm**

```bash
cd apps/go/web
corepack enable
pnpm import          # reads the existing package-lock.json, writes pnpm-lock.yaml
git rm -f package-lock.json
pnpm install --frozen-lockfile
pnpm run build       # confirm the SPA still builds from the new lockfile
```

Order matters: `pnpm import` reads `package-lock.json`, so it has to run before the file is deleted. Deleting first loses the resolved versions and `pnpm install` will re-resolve them, which is a silent dependency bump in a task that is supposed to be a lockfile format change.

Confirm `pnpm-lock.yaml` exists and `package-lock.json` is gone. Task 7's guard scans both, but leaving a stale npm lockfile in the tree means a dependency could be pinned in a file nobody updates.

- [ ] **Step 2: Update the Dockerfile for pnpm**

```dockerfile
FROM node:20-alpine AS web
WORKDIR /app/web
RUN corepack enable
COPY web/package.json web/pnpm-lock.yaml ./
RUN pnpm install --frozen-lockfile
COPY web/ ./
RUN pnpm run build
```

Leave stages 2 and 3 as they are, with `golang:1.25-alpine` from Task 1.

- [ ] **Step 3: Write the root Makefile**

```make
.PHONY: up down verify check-clean dev-go

up:
	docker compose up --build -d

down:
	docker compose down

# Plan 2 replaces this with the conformance harness.
verify:
	@echo "conformance harness lands in plan 2"

dev-go:
	cd apps/go/web && pnpm install && pnpm run build
	cd apps/go && PORT=9001 go run .
```

`check-clean` is Task 7.

- [ ] **Step 4: Write docker-compose.yml**

```yaml
services:
  go:
    build: ./apps/go
    ports: ["9001:9001"]
    environment:
      PORT: "9001"
```

Node, Rust, and Python services arrive in Plan 3 on 9002, 9003, and 9004. `make verify` skips cross-app equality while fewer than two are defined.

- [ ] **Step 5: Write the READMEs**

Root `README.md` covers: what the repo is, that the apps are deliberately uninstrumented and why, the app matrix, how to run one, and how to point the hosted flow or the BYOK skill at a single app. Carry over the existing README's local-dev and Docker sections, repointed at `apps/go`.

`apps/go/README.md` is short: what the app is, `make dev-go`, and the port.

- [ ] **Step 6: Verify**

```bash
make up
sleep 5
curl -s localhost:9001/api/health
curl -s localhost:9001/ | head -c 40
make down
```

Expected: `{"status":"ok"}` and the SPA's HTML.

- [ ] **Step 7: Commit**

```bash
git add -A
git commit -m "build: add root tooling and migrate the frontend to pnpm

The frontend was npm with a checked-in package-lock.json, which would
have moved into apps/go unscanned by the guard. Compose defines the go
service only; plan 3 adds the other three."
```

---

### Task 7: The clean guard and CI

The invariant this whole repo exists to hold. It is not done when it is written; it is done when it has been observed failing on a planted violation in every scanned file type and passing on a clean tree.

**Files:**
- Modify: `Makefile`
- Create: `.github/workflows/ci.yml`
- Create: `scripts/check-clean-test.sh`

**Interfaces:**
- Consumes: the repo layout from Tasks 1 and 6.
- Produces: `make check-clean`, which CI runs on every push to `main`.

- [ ] **Step 1: Add the guard to the Makefile**

Copy this verbatim from the spec's "The clean guard" section. It is tab-indented; a Makefile with spaces will not run.

```make
check-clean:
	@files=""; \
	for f in apps/*/go.mod apps/*/go.sum \
	         apps/*/package.json apps/*/pnpm-lock.yaml apps/*/package-lock.json \
	         apps/*/web/package.json apps/*/web/pnpm-lock.yaml \
	         apps/*/web/package-lock.json apps/*/web/yarn.lock \
	         apps/*/Cargo.toml apps/*/Cargo.lock apps/*/pyproject.toml apps/*/uv.lock \
	         apps/*/web/index.html apps/*/web/dist/index.html; do \
	  [ -f "$$f" ] && files="$$files $$f"; \
	done; \
	hits=""; [ -n "$$files" ] && hits=$$(grep -nE '@everscribe/(sdk-node|components-)|everscribe/sdk-go|"everscribe[^"]*"|^[[:space:]]*everscribe[[:space:]]*=' $$files 2>/dev/null); \
	src=$$(grep -rnE '@everscribe/components-' apps/*/web/src 2>/dev/null); \
	if [ -n "$$hits$$src" ]; then \
	  echo "everscribe dependency on main:"; echo "$$hits"; echo "$$src"; exit 1; \
	fi
```

Do not "simplify" this into `! grep ... || exit 1`. `grep` exits 2 when a file argument is unreadable and `!` inverts 2 to 0, so that form prints the violation and then reports clean. Missing files are the normal state here: there is no `uv.lock` until Plan 3, no `Cargo.lock` until Plan 3, and no `apps/go/go.sum` until Task 4 adds Gin.

- [ ] **Step 2: Write the acceptance test**

Create `scripts/check-clean-test.sh`. This is the evidence the guard works, and it is the deliverable of this task.

```bash
#!/usr/bin/env bash
# Proves make check-clean catches a violation in every scanned file type
# and passes on a clean tree. Run from the repo root.
set -u
MAKEFILE="$PWD/Makefile"
fails=0

run() { # name, setup-command
  D=$(mktemp -d)
  cp "$MAKEFILE" "$D/Makefile"
  ( cd "$D"
    mkdir -p apps/go/web/dist apps/node/web/src apps/python apps/rust
    printf 'module github.com/everscribe/ai-assisted-setup-demo/apps/go\n' > apps/go/go.mod
    printf '<html></html>\n' > apps/go/web/dist/index.html
    eval "$2"
    if make check-clean >/dev/null 2>&1; then echo "  MISSED  $1"; exit 1; fi
    echo "  caught  $1" )
  [ $? -ne 0 ] && fails=$((fails+1))
  rm -rf "$D"
}

run "go.mod"            'echo "require github.com/everscribe/sdk-go v0.1.0" >> apps/go/go.mod'
run "go.sum"            'echo "github.com/everscribe/sdk-go v0.1.0 h1:x" > apps/go/go.sum'
run "package.json"      'echo "{\"dependencies\":{\"@everscribe/sdk-node\":\"^0.1.0\"}}" > apps/node/package.json'
run "pnpm-lock.yaml"    'echo "  '\''@everscribe/sdk-node'\'':" > apps/node/pnpm-lock.yaml'
run "package-lock.json" 'echo "{\"packages\":{\"node_modules/@everscribe/components-react\":{}}}" > apps/go/web/package-lock.json'
run "yarn.lock"         'echo "\"@everscribe/components-element@^0.1.0\":" > apps/node/web/yarn.lock'
run "web/package.json"  'echo "{\"dependencies\":{\"@everscribe/components-element\":\"^0.1\"}}" > apps/node/web/package.json'
run "CDN tag in dist"   'echo "<script src=\"//cdn/@everscribe/components-element\"></script>" > apps/go/web/dist/index.html'
run "import in web/src" 'echo "import \"@everscribe/components-react\";" > apps/node/web/src/main.ts'
run "pyproject.toml"    'printf "[project]\ndependencies = [\n    \"everscribe>=0.1\",\n]\n" > apps/python/pyproject.toml'
run "uv.lock"           'printf "[[package]]\nname = \"everscribe\"\n" > apps/python/uv.lock'
run "Cargo.toml"        'printf "[dependencies]\neverscribe = \"0.1\"\n" > apps/rust/Cargo.toml'
run "Cargo.lock"        'printf "[[package]]\nname = \"everscribe\"\n" > apps/rust/Cargo.lock'

D=$(mktemp -d); cp "$MAKEFILE" "$D/Makefile"
( cd "$D"; mkdir -p apps/go
  printf 'module github.com/everscribe/ai-assisted-setup-demo/apps/go\n' > apps/go/go.mod
  make check-clean >/dev/null 2>&1 ) \
  && echo "  clean tree exits 0" \
  || { echo "  FALSE POSITIVE on a clean tree"; fails=$((fails+1)); }
rm -rf "$D"

[ "$fails" -eq 0 ] && echo "check-clean: all cases pass" || { echo "check-clean: $fails failures"; exit 1; }
```

```bash
chmod +x scripts/check-clean-test.sh
```

- [ ] **Step 3: Run it, and paste the output into the commit**

```bash
./scripts/check-clean-test.sh
```

Expected: thirteen `caught` lines, one `clean tree exits 0`, and `check-clean: all cases pass`. If any line says `MISSED`, the guard is wrong and this task is not done.

Also confirm the guard passes against the real repo, where `apps/go/go.mod` legitimately contains `everscribe` in its module path:

```bash
make check-clean && echo "real repo: clean"
```

- [ ] **Step 4: Write the CI workflow**

Create `.github/workflows/ci.yml`:

```yaml
name: ci

on:
  push:
    branches: [main]
  pull_request:

jobs:
  clean:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - name: no everscribe dependency on main
        run: make check-clean
      - name: the guard itself still works
        run: ./scripts/check-clean-test.sh

  go:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with:
          go-version: '1.25'
      - run: cd apps/go && go build ./... && go vet ./...
      - run: cd apps/go && go test ./... -count=1
```

The second step in the `clean` job is the point. A guard that has stopped working looks exactly like a repo with no violations, so CI checks the guard, not only the tree.

- [ ] **Step 5: Commit**

```bash
git add -A
git commit -m "ci: enforce that main carries no everscribe dependency

check-clean scans every manifest, lockfile, and checked-in index.html,
and matches exact dependency identifiers so the go module path does not
false-positive. scripts/check-clean-test.sh proves it catches a planted
violation in all thirteen scanned locations and passes on a clean tree.
CI runs both, because a guard that has quietly stopped working is
indistinguishable from a clean repo."
```

---

## Done when

- `make up` serves the app on 9001 and `make down` stops it.
- `cd apps/go && go test ./... -count=1` passes.
- `./scripts/check-clean-test.sh` reports all cases passing.
- `contract/openapi.yaml` describes all eighteen routes and `contract/seed.json` matches what the app serves.
- No Everscribe dependency appears anywhere in the tree.

Plan 2 builds the conformance harness against this app. Plan 3 adds the other three.
