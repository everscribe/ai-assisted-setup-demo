# Four-app demo, Plan 2: the conformance harness

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build the harness that decides whether an app conforms to `contract/openapi.yaml`, so every app Plan 3 adds arrives with an acceptance test already waiting instead of being declared correct by inspection.

**Architecture:** A Go test module in `contract/conformance/` with its own `go.mod`. It is a pure HTTP client with no handlers, so it stays invisible to the instrument agent. It discovers running apps by probing `/api/health`, then applies three layers: schema conformance against the contract, security invariants asserted explicitly, and cross-app equality over normalized responses. With one app up, the equality layer skips itself.

**Tech Stack:** Go 1.25, `github.com/getkin/kin-openapi` v0.149.0 for OpenAPI 3 response validation, testify/require, Docker Compose.

**Spec:** `docs/superpowers/specs/2026-09-10-four-app-demo-design.md`

## Global Constraints

Every task's requirements implicitly include this section.

- **The repo must stay uninstrumented.** No Everscribe SDK in any manifest or lockfile, no `@everscribe/components-*` anywhere, no audit events. The harness knows nothing about Everscribe, which is exactly why it can be pointed at an instrumented branch to ask whether behaviour changed.
- **The harness never modifies an app.** It is HTTP-only. It must not import anything from `apps/go`.
- **Go directive `1.25.0`** in `contract/conformance/go.mod`. Module path `github.com/everscribe/ai-assisted-setup-demo/contract/conformance`.
- **No em dashes or en dashes** in prose, code comments, or commit messages. Use a hyphen or rephrase.
- **No `Co-Authored-By` trailer** on any commit. Write the message and stop.
- **Keep code comments short.** One line where one line does.
- **Tests use `t.Parallel()` where safe and `testify/require`.** Note the exception in Task 5: the lifecycle sequence mutates shared app state, so those tests must NOT be parallel.
- **The four apps and their ports are fixed:** go 9001, node 9002, rust 9003, python 9004. Only `go` exists today.

## Facts established before this plan was written

These were verified by running them, not assumed. Trust them.

- `kin-openapi` v0.149.0 loads `contract/openapi.yaml`, validates it, and its router resolves requests. It correctly accepts a valid `User` body and rejects one with missing fields, a wrong type, and a bad enum.
- `contract/openapi.yaml` declares `servers: [{url: /api}]`, so **the router's route paths are `/me`, `/members`, and so on, without the `/api` prefix**. Requests still go to `/api/me` on the wire. This trips people up.
- `contract/openapi.yaml` has 14 paths and 18 operations, plus an `x-ordering` extension holding both the declared orders and worked examples over the seed.
- `contract/seed.json` holds `_notes`, 2 organizations, 4 users, 5 resources, 7 access requests, and 4 comments. The `organizations` collection is documented as unverifiable because no route exposes it.
- The seed's ordering examples are `GET /api/resources/r_acme_pg/requests` returning `[req_acme_1, req_acme_4, req_acme_5]` and `GET /api/requests/req_acme_1/comments` returning `[c_acme_1, c_acme_2, c_acme_3]`. Both groups contain a pair tied on `created_at`, so they fail for any port that drops the `id` tiebreak.

---

### Task 1: Module skeleton, app discovery, and the HTTP client

**Files:**
- Create: `contract/conformance/go.mod`
- Create: `contract/conformance/apps.go`
- Create: `contract/conformance/client.go`
- Create: `contract/conformance/apps_test.go`

**Interfaces:**
- Consumes: nothing from earlier tasks. This is the first task.
- Produces: `App{Name, BaseURL string}`, `Discover(ctx context.Context) []App`, `Client{App App}`, `NewClient(App) *Client`, and `(*Client).Do(ctx context.Context, method, path, actor, body string) (*Response, error)` returning `Response{Status int, Header http.Header, Body []byte}`. Tasks 3 through 6 all use these.

- [ ] **Step 1: Write the failing test**

Create `contract/conformance/apps_test.go`:

```go
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
```

The last test matters: sending `X-User-ID: ""` is not the same as sending no header, and the 401 invariant in Task 4 depends on the difference.

- [ ] **Step 2: Run it to make sure it fails**

```bash
cd contract/conformance && go test ./... -run 'TestDiscover|TestClient' -v
```

Expected: compile failure, `undefined: discover`, `undefined: App`, `undefined: NewClient`.

- [ ] **Step 3: Create the module**

```bash
cd contract/conformance
go mod init github.com/everscribe/ai-assisted-setup-demo/contract/conformance
go mod edit -go=1.25.0
go get github.com/stretchr/testify
go get github.com/getkin/kin-openapi/openapi3
go get github.com/getkin/kin-openapi/openapi3filter
go get github.com/getkin/kin-openapi/routers/gorillamux
```

- [ ] **Step 4: Write `apps.go`**

```go
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
```

- [ ] **Step 5: Write `client.go`**

```go
package conformance

import (
	"context"
	"io"
	"net/http"
	"strings"
	"time"
)

// Response is one captured HTTP response.
type Response struct {
	Status int
	Header http.Header
	Body   []byte
}

// Client talks to one app.
type Client struct {
	App  App
	http *http.Client
}

func NewClient(a App) *Client {
	return &Client{App: a, http: &http.Client{Timeout: 10 * time.Second}}
}

// Do sends one request. An empty actor sends no X-User-ID header, which is
// not the same as sending an empty one. An empty body sends no body.
func (c *Client) Do(ctx context.Context, method, path, actor, body string) (*Response, error) {
	var rdr io.Reader
	if body != "" {
		rdr = strings.NewReader(body)
	}
	req, err := http.NewRequestWithContext(ctx, method, c.App.BaseURL+path, rdr)
	if err != nil {
		return nil, err
	}
	if actor != "" {
		req.Header.Set("X-User-ID", actor)
	}
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	return &Response{Status: resp.StatusCode, Header: resp.Header, Body: raw}, nil
}
```

- [ ] **Step 6: Run the tests**

```bash
cd contract/conformance && go test ./... -count=1 -v
```

Expected: PASS, three tests. No app needs to be running; these use `httptest`.

- [ ] **Step 7: Commit**

```bash
git add contract/conformance
git commit -m "feat(conformance): app discovery and http client

The harness finds apps by probing /api/health rather than parsing
compose, so it naturally handles the one-app state plans 1 and 2 leave
behind. An empty actor sends no X-User-ID header at all, which the 401
invariant depends on."
```

---

### Task 2: The normalizer

The one piece of real logic in the harness, and the one place a bug produces false confidence. It gets its own unit tests and no network.

**Files:**
- Create: `contract/conformance/normalize.go`
- Create: `contract/conformance/normalize_test.go`
- Create: `contract/conformance/seed.go`

**Interfaces:**
- Consumes: nothing.
- Produces: `LoadSeedIDs(path string) (map[string]bool, error)` and `Normalizer{SeedIDs map[string]bool}` with `NewNormalizer(map[string]bool) *Normalizer` and `(*Normalizer).Normalize(body []byte) ([]byte, error)`. Task 6 uses these for cross-app equality.

- [ ] **Step 1: Write the failing tests**

Create `contract/conformance/normalize_test.go`:

```go
package conformance

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func seedSet() map[string]bool {
	return map[string]bool{"u_ada": true, "r_acme_pg": true, "req_acme_1": true, "org_acme": true}
}

func TestNormalizeLeavesSeededIDsAlone(t *testing.T) {
	t.Parallel()
	n := NewNormalizer(seedSet())
	got, err := n.Normalize([]byte(`{"id":"u_ada","organization_id":"org_acme"}`))
	require.NoError(t, err)
	require.JSONEq(t, `{"id":"u_ada","organization_id":"org_acme"}`, string(got))
}

func TestNormalizeRewritesGeneratedIDsInOrderOfAppearance(t *testing.T) {
	t.Parallel()
	n := NewNormalizer(seedSet())
	got, err := n.Normalize([]byte(`[{"id":"9f2a10bc4d5e"},{"id":"11bb22cc33dd"},{"id":"9f2a10bc4d5e"}]`))
	require.NoError(t, err)
	require.JSONEq(t, `[{"id":"$id1"},{"id":"$id2"},{"id":"$id1"}]`, string(got),
		"the same generated id must map to the same placeholder every time")
}

func TestNormalizeReplacesTimestamps(t *testing.T) {
	t.Parallel()
	n := NewNormalizer(seedSet())
	got, err := n.Normalize([]byte(`{"created_at":"2026-03-14T12:00:00Z","decided_at":"2026-09-10T17:35:21.226491Z"}`))
	require.NoError(t, err)
	require.JSONEq(t, `{"created_at":"$ts1","decided_at":"$ts2"}`, string(got))
}

func TestNormalizeRejectsNonUTCTimestamps(t *testing.T) {
	t.Parallel()
	n := NewNormalizer(seedSet())
	for _, bad := range []string{
		`{"created_at":"2026-03-14T12:00:00+00:00"}`,
		`{"created_at":"2026-03-14T12:00:00"}`,
		`{"created_at":"2026-03-14 12:00:00Z"}`,
	} {
		_, err := n.Normalize([]byte(bad))
		require.Error(t, err, "must reject %s", bad)
		require.Contains(t, err.Error(), "timestamp")
	}
}

func TestNormalizeAcceptsAnyFractionalPrecision(t *testing.T) {
	t.Parallel()
	n := NewNormalizer(seedSet())
	// Go trims trailing zeros, Node emits 3 digits, Python 6, Rust 9.
	// All four are conforming; only the Z suffix and UTC are pinned.
	for _, ok := range []string{
		`{"created_at":"2026-03-14T12:00:00Z"}`,
		`{"created_at":"2026-03-14T12:00:00.123Z"}`,
		`{"created_at":"2026-03-14T12:00:00.123456Z"}`,
		`{"created_at":"2026-03-14T12:00:00.123456789Z"}`,
	} {
		got, err := n.Normalize([]byte(ok))
		require.NoError(t, err, "must accept %s", ok)
		require.JSONEq(t, `{"created_at":"$ts1"}`, string(got))
	}
}

func TestNormalizeIsStableAcrossKeyOrder(t *testing.T) {
	t.Parallel()
	n1, n2 := NewNormalizer(seedSet()), NewNormalizer(seedSet())
	a, err := n1.Normalize([]byte(`{"b":1,"a":2}`))
	require.NoError(t, err)
	b, err := n2.Normalize([]byte(`{"a":2,"b":1}`))
	require.NoError(t, err)
	require.Equal(t, string(a), string(b), "output must not depend on input key order")
}
```

- [ ] **Step 2: Run them to make sure they fail**

```bash
cd contract/conformance && go test ./... -run TestNormalize -v
```

Expected: compile failure, `undefined: NewNormalizer`.

- [ ] **Step 3: Write `seed.go`**

```go
package conformance

import (
	"encoding/json"
	"os"
)

// LoadSeedIDs collects every id declared in contract/seed.json. Ids in
// this set are stable across apps and must not be normalized; anything
// else was minted at runtime.
func LoadSeedIDs(path string) (map[string]bool, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var doc map[string]json.RawMessage
	if err := json.Unmarshal(raw, &doc); err != nil {
		return nil, err
	}
	ids := map[string]bool{}
	for key, val := range doc {
		if key == "_notes" {
			continue
		}
		var rows []map[string]any
		if err := json.Unmarshal(val, &rows); err != nil {
			continue
		}
		for _, row := range rows {
			if id, ok := row["id"].(string); ok {
				ids[id] = true
			}
		}
	}
	return ids, nil
}
```

- [ ] **Step 4: Write `normalize.go`**

```go
package conformance

import (
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strings"
)

// utcTimestamp is the only accepted shape: RFC 3339, UTC, literal Z.
// The fractional part is optional and of any length, because Go trims
// trailing zeros while Node emits 3 digits, Python 6, and Rust 9.
var utcTimestamp = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}(\.\d+)?Z$`)

// looksLikeTimestamp catches near misses so they fail loudly rather than
// being treated as an ordinary string that happens to differ.
var looksLikeTimestamp = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}[T ]\d{2}:\d{2}:\d{2}`)

// Normalizer rewrites runtime-varying values so two apps' responses can be
// compared. Seeded ids survive; generated ids become $id1, $id2 in order of
// first appearance; every timestamp becomes $ts1, $ts2 the same way.
type Normalizer struct {
	seedIDs map[string]bool
	ids     map[string]string
	stamps  map[string]string
}

func NewNormalizer(seedIDs map[string]bool) *Normalizer {
	return &Normalizer{seedIDs: seedIDs, ids: map[string]string{}, stamps: map[string]string{}}
}

// Normalize parses body as JSON, rewrites it, and re-encodes with sorted
// keys so the output does not depend on the app's key order.
func (n *Normalizer) Normalize(body []byte) ([]byte, error) {
	if len(body) == 0 {
		return body, nil
	}
	var v any
	if err := json.Unmarshal(body, &v); err != nil {
		return nil, fmt.Errorf("normalize: %w", err)
	}
	out, err := n.walk(v, "")
	if err != nil {
		return nil, err
	}
	return json.Marshal(out)
}

func (n *Normalizer) walk(v any, key string) (any, error) {
	switch t := v.(type) {
	case map[string]any:
		out := make(map[string]any, len(t))
		for _, k := range sortedKeys(t) {
			w, err := n.walk(t[k], k)
			if err != nil {
				return nil, err
			}
			out[k] = w
		}
		return out, nil
	case []any:
		out := make([]any, len(t))
		for i, item := range t {
			w, err := n.walk(item, key)
			if err != nil {
				return nil, err
			}
			out[i] = w
		}
		return out, nil
	case string:
		return n.scalar(t, key)
	default:
		return v, nil
	}
}

func (n *Normalizer) scalar(s, key string) (any, error) {
	if isTimestampKey(key) || looksLikeTimestamp.MatchString(s) {
		if !utcTimestamp.MatchString(s) {
			return nil, fmt.Errorf("normalize: timestamp %q is not RFC 3339 UTC with a Z suffix", s)
		}
		return n.placeholder(n.stamps, s, "$ts"), nil
	}
	if isIDKey(key) && !n.seedIDs[s] && s != "" {
		return n.placeholder(n.ids, s, "$id"), nil
	}
	return s, nil
}

func (n *Normalizer) placeholder(table map[string]string, value, prefix string) string {
	if existing, ok := table[value]; ok {
		return existing
	}
	next := fmt.Sprintf("%s%d", prefix, len(table)+1)
	table[value] = next
	return next
}

func isTimestampKey(k string) bool {
	return strings.HasSuffix(k, "_at")
}

func isIDKey(k string) bool {
	return k == "id" || strings.HasSuffix(k, "_id") || strings.HasSuffix(k, "_by")
}

func sortedKeys(m map[string]any) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
```

Note `isIDKey` covers `_by` as well as `_id`, because `decided_by` and `revoked_by` carry user ids.

- [ ] **Step 5: Run the tests**

```bash
cd contract/conformance && go test ./... -run TestNormalize -count=1 -v
```

Expected: PASS, six tests.

- [ ] **Step 6: Commit**

```bash
git add contract/conformance
git commit -m "feat(conformance): the response normalizer

Seeded ids survive, generated ids and timestamps become placeholders
numbered by first appearance, and output keys are sorted so comparison
does not depend on an app's key order. Timestamps must be RFC 3339 UTC
with a Z suffix; the fractional part is free, because Go trims trailing
zeros while Node emits 3 digits, Python 6, and Rust 9. This is why the
harness compares instants rather than strings."
```

---

### Task 3: Layer 1, schema conformance

**Files:**
- Create: `contract/conformance/spec.go`
- Create: `contract/conformance/spec_test.go`

**Interfaces:**
- Consumes: `Response` from Task 1.
- Produces: `LoadSpec(path string) (*Spec, error)` and `(*Spec).ValidateResponse(ctx context.Context, method, path string, resp *Response) error`. Tasks 4, 5, and 6 call `ValidateResponse` on every response they receive.

- [ ] **Step 1: Write the failing test**

Create `contract/conformance/spec_test.go`:

```go
package conformance

import (
	"context"
	"net/http"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func testSpec(t *testing.T) *Spec {
	t.Helper()
	s, err := LoadSpec(filepath.Join("..", "openapi.yaml"))
	require.NoError(t, err)
	return s
}

func jsonResp(status int, body string) *Response {
	return &Response{
		Status: status,
		Header: http.Header{"Content-Type": []string{"application/json; charset=utf-8"}},
		Body:   []byte(body),
	}
}

func TestSpecLoads(t *testing.T) {
	t.Parallel()
	require.NotNil(t, testSpec(t))
}

func TestValidateResponseAcceptsAConformingUser(t *testing.T) {
	t.Parallel()
	s := testSpec(t)
	body := `{"id":"u_ada","name":"Ada Lovelace","email":"ada@acme.test","role":"admin","organization_id":"org_acme","created_at":"2026-03-13T12:00:00Z"}`
	require.NoError(t, s.ValidateResponse(context.Background(), http.MethodGet, "/api/me", jsonResp(200, body)))
}

func TestValidateResponseRejectsDrift(t *testing.T) {
	t.Parallel()
	s := testSpec(t)
	cases := map[string]string{
		"missing required fields": `{"id":"u_ada","role":"admin"}`,
		"wrong type":              `{"id":"u_ada","name":"Ada","email":"a@b.c","role":"admin","organization_id":"org_acme","created_at":12345}`,
		"unknown enum":            `{"id":"u_ada","name":"Ada","email":"a@b.c","role":"wizard","organization_id":"org_acme","created_at":"2026-03-13T12:00:00Z"}`,
	}
	for name, body := range cases {
		t.Run(name, func(t *testing.T) {
			require.Error(t, s.ValidateResponse(context.Background(), http.MethodGet, "/api/me", jsonResp(200, body)))
		})
	}
}

func TestValidateResponseCoversErrorBodies(t *testing.T) {
	t.Parallel()
	s := testSpec(t)
	require.NoError(t, s.ValidateResponse(context.Background(), http.MethodGet, "/api/me", jsonResp(401, `{"error":"unauthorized"}`)))
}

func TestValidateResponseHandlesUndeclaredRoutes(t *testing.T) {
	t.Parallel()
	s := testSpec(t)
	ctx := context.Background()

	// The router cannot route either of these, so they fall through to the
	// Error-schema check. Both are documented in the contract's prose.
	require.NoError(t, s.ValidateResponse(ctx, http.MethodGet, "/api/no-such-route",
		jsonResp(404, `{"error":"not found"}`)))
	require.NoError(t, s.ValidateResponse(ctx, http.MethodGet, "/api/login",
		jsonResp(405, `{"error":"method not allowed"}`)))

	// The fallback must stay strict.
	require.Error(t, s.ValidateResponse(ctx, http.MethodGet, "/api/no-such-route",
		jsonResp(404, `{"message":"nope"}`)), "a 404 whose body is not an Error must fail")
	require.Error(t, s.ValidateResponse(ctx, http.MethodGet, "/api/no-such-route",
		jsonResp(200, `{}`)), "an unroutable path returning 200 must fail")
}

func TestValidateResponseAcceptsBodilessNoContent(t *testing.T) {
	t.Parallel()
	s := testSpec(t)
	// 204 carries no Content-Type and no body, by design.
	resp := &Response{Status: 204, Header: http.Header{}, Body: nil}
	require.NoError(t, s.ValidateResponse(context.Background(), http.MethodPost, "/api/logout", resp))
}
```

- [ ] **Step 2: Run it to make sure it fails**

```bash
cd contract/conformance && go test ./... -run TestSpec -v
```

Expected: compile failure, `undefined: LoadSpec`.

- [ ] **Step 3: Write `spec.go`**

```go
package conformance

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/getkin/kin-openapi/openapi3filter"
	"github.com/getkin/kin-openapi/routers"
	"github.com/getkin/kin-openapi/routers/gorillamux"
)

// Spec is contract/openapi.yaml, loaded and routable.
type Spec struct {
	doc    *openapi3.T
	router routers.Router
}

// LoadSpec reads and validates the contract.
func LoadSpec(path string) (*Spec, error) {
	loader := openapi3.NewLoader()
	doc, err := loader.LoadFromFile(path)
	if err != nil {
		return nil, fmt.Errorf("load spec: %w", err)
	}
	if err := doc.Validate(context.Background()); err != nil {
		return nil, fmt.Errorf("spec is not valid openapi: %w", err)
	}
	router, err := gorillamux.NewRouter(doc)
	if err != nil {
		return nil, fmt.Errorf("build router: %w", err)
	}
	return &Spec{doc: doc, router: router}, nil
}

// ValidateResponse checks one response against its declared schema. The
// path is the wire path, including the /api prefix; the spec declares
// servers: [{url: /api}], so the router strips it internally.
func (s *Spec) ValidateResponse(ctx context.Context, method, path string, resp *Response) error {
	req, err := http.NewRequest(method, "http://localhost"+path, nil)
	if err != nil {
		return err
	}
	route, pathParams, err := s.router.FindRoute(req)
	if err != nil {
		return s.validateUndeclared(method, path, resp, err)
	}
	in := &openapi3filter.ResponseValidationInput{
		RequestValidationInput: &openapi3filter.RequestValidationInput{
			Request: req, PathParams: pathParams, Route: route,
		},
		Status: resp.Status,
		Header: resp.Header,
		Options: &openapi3filter.Options{IncludeResponseStatus: true},
	}
	in.SetBodyBytes(resp.Body)
	if err := openapi3filter.ValidateResponse(ctx, in); err != nil {
		return fmt.Errorf("%s %s -> %d: %w", method, path, resp.Status, err)
	}
	return nil
}

// validateUndeclared handles the two responses the contract documents in
// prose rather than as operations: the unknown-path 404 and the
// method-mismatch 405. The router cannot route either, so they are checked
// against the Error schema directly. Any other status here is a real gap.
func (s *Spec) validateUndeclared(method, path string, resp *Response, routeErr error) error {
	if resp.Status != http.StatusNotFound && resp.Status != http.StatusMethodNotAllowed {
		return fmt.Errorf("no route in contract for %s %s (status %d): %w", method, path, resp.Status, routeErr)
	}
	schemaRef, ok := s.doc.Components.Schemas["Error"]
	if !ok {
		return fmt.Errorf("contract has no Error schema")
	}
	var v any
	if err := json.Unmarshal(resp.Body, &v); err != nil {
		return fmt.Errorf("%s %s -> %d: body is not json: %w", method, path, resp.Status, err)
	}
	if err := schemaRef.Value.VisitJSON(v); err != nil {
		return fmt.Errorf("%s %s -> %d: body is not an Error: %w", method, path, resp.Status, err)
	}
	return nil
}
```

**Why the fallback exists, verified by running it.** `gorillamux.FindRoute`
returns an error for both `GET /api/login` (method not allowed) and
`/api/no-such-route` (no matching operation), so `ValidateResponse` never
runs for either. Without the fallback, the harness would fail on exactly the
two steps that exercise the 405 and unknown-path behaviours the contract
went to some trouble to pin down. The fallback stays strict: a 404 carrying
`{"message":"nope"}` is rejected, and an unknown path returning 200 is
rejected.

`IncludeResponseStatus: true` makes an undeclared status code an error rather than a silent pass. That is the setting that catches a port returning 200 where the contract says 201.

- [ ] **Step 4: Run the tests**

```bash
cd contract/conformance && go test ./... -run TestSpec -count=1 -v
```

Expected: PASS. If `TestValidateResponseAcceptsBodilessNoContent` fails, the contract declares a body for 204 where the app sends none; fix the contract, not the test, and say so in the commit.

- [ ] **Step 5: Commit**

```bash
git add contract/conformance
git commit -m "feat(conformance): layer 1, schema validation

Every response is validated against its declared schema, with
IncludeResponseStatus so an undeclared status code fails rather than
passing silently. Note the contract declares servers: [{url: /api}], so
router paths carry no /api prefix while wire paths do."
```

---

### Task 4: Layer 3, security invariants

Layer 3 before layer 2 on purpose: these must be right rather than merely consistent, and they are the assertions that work with a single app running.

**Files:**
- Create: `contract/conformance/security_test.go`
- Create: `contract/conformance/harness_test.go`

**Interfaces:**
- Consumes: `Discover`, `NewClient`, `Client.Do`, `Response` from Task 1; `LoadSpec`, `Spec.ValidateResponse` from Task 3.
- Produces: `apps(t *testing.T) []App` and `validated(t *testing.T, c *Client, method, path, actor, body string) *Response`, a helper that performs a request AND schema-validates it. Tasks 5 and 6 use both.

- [ ] **Step 1: Write the shared harness helpers**

Create `contract/conformance/harness_test.go`:

```go
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
```

- [ ] **Step 2: Write the failing security tests**

Create `contract/conformance/security_test.go`:

```go
package conformance

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"
)

// Every route except the two anonymous ones must 401 without an actor.
func TestMissingActorIs401(t *testing.T) {
	for _, app := range apps(t) {
		app := app
		t.Run(app.Name, func(t *testing.T) {
			c := NewClient(app)
			for _, r := range []struct{ method, path string }{
				{http.MethodGet, "/api/me"},
				{http.MethodPost, "/api/logout"},
				{http.MethodGet, "/api/members"},
				{http.MethodPost, "/api/members"},
				{http.MethodGet, "/api/resources"},
				{http.MethodPost, "/api/resources"},
				{http.MethodGet, "/api/resources/r_acme_pg/requests"},
				{http.MethodGet, "/api/requests/req_acme_1/comments"},
			} {
				resp := validated(t, c, r.method, r.path, "", "")
				require.Equal(t, http.StatusUnauthorized, resp.Status, "%s %s", r.method, r.path)
			}
			// The two anonymous routes.
			require.Equal(t, http.StatusOK, validated(t, c, http.MethodGet, "/api/health", "", "").Status)
			require.Equal(t, http.StatusOK,
				validated(t, c, http.MethodPost, "/api/login", "", `{"email":"ada@acme.test"}`).Status)
		})
	}
}

// A cross-tenant read is 404, and each one is paired with a positive
// control. Without the control this test also passes when the route was
// never registered, because an unknown path returns the identical body.
func TestCrossTenantReadsAre404WithPositiveControls(t *testing.T) {
	for _, app := range apps(t) {
		app := app
		t.Run(app.Name, func(t *testing.T) {
			c := NewClient(app)
			cases := []struct {
				name, method, sameTenant, crossTenant string
			}{
				{"resource requests", http.MethodGet,
					"/api/resources/r_acme_pg/requests", "/api/resources/r_globex_k8s/requests"},
				{"request comments", http.MethodGet,
					"/api/requests/req_acme_1/comments", "/api/requests/req_globex_1/comments"},
			}
			for _, tc := range cases {
				t.Run(tc.name, func(t *testing.T) {
					ok := validated(t, c, tc.method, tc.sameTenant, "u_ada", "")
					require.Equal(t, http.StatusOK, ok.Status,
						"positive control: the route must work same-tenant before its 404 means anything")

					miss := validated(t, c, tc.method, tc.crossTenant, "u_ada", "")
					require.Equal(t, http.StatusNotFound, miss.Status)
					require.JSONEq(t, `{"error":"not found"}`, string(miss.Body))
				})
			}
		})
	}
}

// A cross-tenant miss and an unknown path must be byte-identical, so the
// boundary does not leak existence.
func TestCrossTenantAndUnknownPathAreIndistinguishable(t *testing.T) {
	for _, app := range apps(t) {
		app := app
		t.Run(app.Name, func(t *testing.T) {
			c := NewClient(app)
			cross := validated(t, c, http.MethodGet, "/api/resources/r_globex_k8s/requests", "u_ada", "")
			unknown, err := c.Do(t.Context(), http.MethodGet, "/api/no-such-route", "u_ada", "")
			require.NoError(t, err)
			require.Equal(t, cross.Status, unknown.Status)
			require.Equal(t, string(cross.Body), string(unknown.Body))
		})
	}
}

func TestMemberCannotUseAdminRoutes(t *testing.T) {
	for _, app := range apps(t) {
		app := app
		t.Run(app.Name, func(t *testing.T) {
			c := NewClient(app)
			for _, r := range []struct {
				method, path, body string
			}{
				{http.MethodPost, "/api/resources", `{"name":"x","description":"y","sensitivity":"internal"}`},
				{http.MethodPost, "/api/members", `{"name":"X","email":"x@acme.test","role":"member"}`},
				{http.MethodPost, "/api/requests/req_acme_1/approve", ""},
				{http.MethodPost, "/api/requests/req_acme_1/deny", ""},
			} {
				resp := validated(t, c, r.method, r.path, "u_linus", r.body)
				require.Equal(t, http.StatusForbidden, resp.Status, "%s %s", r.method, r.path)
				require.JSONEq(t, `{"error":"admin role required"}`, string(resp.Body))
			}
		})
	}
}

// req_acme_3 is seeded denied. Denied is terminal.
func TestDeniedRequestCannotBeApproved(t *testing.T) {
	for _, app := range apps(t) {
		app := app
		t.Run(app.Name, func(t *testing.T) {
			c := NewClient(app)
			resp := validated(t, c, http.MethodPost, "/api/requests/req_acme_3/approve", "u_ada", "")
			require.Equal(t, http.StatusConflict, resp.Status)
		})
	}
}
```

These tests do not call `t.Parallel()`. They share one app's mutable state with Task 5's sequence, and parallel execution would interleave mutations.

- [ ] **Step 3: Run them against a live app**

```bash
make up
sleep 5
cd contract/conformance && go test ./... -run 'TestMissingActor|TestCrossTenant|TestMemberCannot|TestDeniedRequest' -count=1 -v
cd ../.. && make down
```

Expected: PASS for the `go` app, every subtest.

- [ ] **Step 4: Prove the positive control does its job**

This is the step that verifies the test rather than the app. Temporarily break the app and confirm the right test fails for the right reason:

```bash
# comment out the ListResourceRequests route registration in
# apps/go/handlers/handlers.go, then:
make up && sleep 5
cd contract/conformance && go test ./... -run TestCrossTenantReadsAre404 -count=1
```

Expected: FAIL on the positive control assertion, with the message "the route must work same-tenant before its 404 means anything", NOT a pass. Restore the route, re-run, confirm green, and record both outputs in your report.

- [ ] **Step 5: Commit**

```bash
git add contract/conformance
git commit -m "feat(conformance): layer 3, security invariants

Cross-tenant 404s are paired with positive controls, because a
cross-tenant miss and an unregistered route return byte-identical
responses by design, so without the control the assertion also passes
when the handler does not exist."
```

---

### Task 5: The lifecycle sequence and ordering

**Files:**
- Create: `contract/conformance/sequence.go`
- Create: `contract/conformance/sequence_test.go`
- Create: `contract/conformance/ordering_test.go`

**Interfaces:**
- Consumes: everything from Tasks 1 through 4.
- Produces: `Step{Name, Method, Path, Actor, Body string}`, `Lifecycle() []Step`, and `Run(t *testing.T, c *Client) []*Response`. Task 6 replays the same sequence against every app and compares.

- [ ] **Step 1: Write `sequence.go`**

```go
package conformance

// Step is one request in the lifecycle sequence.
type Step struct {
	Name   string
	Method string
	Path   string
	Actor  string
	Body   string
}

// Lifecycle is the scripted sequence replayed against every app. It
// covers login, listing, requesting access, commenting, approving, and
// revoking. It is deliberately free of generated ids: every path uses a
// seeded id, so the same script runs against any app without threading
// state between steps.
func Lifecycle() []Step {
	return []Step{
		{"login as admin", "POST", "/api/login", "", `{"email":"ada@acme.test"}`},
		{"whoami", "GET", "/api/me", "u_ada", ""},
		{"list members", "GET", "/api/members", "u_ada", ""},
		{"list resources", "GET", "/api/resources", "u_ada", ""},
		{"list requests on pg", "GET", "/api/resources/r_acme_pg/requests", "u_ada", ""},
		{"list comments", "GET", "/api/requests/req_acme_1/comments", "u_ada", ""},

		{"member requests access", "POST", "/api/resources/r_acme_s3/requests", "u_linus", `{"reason":"quarterly audit"}`},
		{"member comments", "POST", "/api/requests/req_acme_1/comments", "u_linus", `{"body":"still needed"}`},
		{"admin comments", "POST", "/api/requests/req_acme_1/comments", "u_ada", `{"body":"approved verbally"}`},
		{"comments after posting", "GET", "/api/requests/req_acme_1/comments", "u_ada", ""},

		{"approve pending", "POST", "/api/requests/req_acme_1/approve", "u_ada", ""},
		{"revoke it", "POST", "/api/requests/req_acme_1/revoke", "u_ada", ""},
		{"approve already revoked", "POST", "/api/requests/req_acme_1/approve", "u_ada", ""},
		{"deny already denied", "POST", "/api/requests/req_acme_3/deny", "u_ada", ""},

		{"register resource", "POST", "/api/resources", "u_ada", `{"name":"vault-prod","description":"secrets","sensitivity":"restricted"}`},
		{"invite member", "POST", "/api/members", "u_ada", `{"name":"Barbara Liskov","email":"barbara@acme.test","role":"member"}`},
		{"members after invite", "GET", "/api/members", "u_ada", ""},
		{"decommission", "POST", "/api/resources/r_acme_s3/decommission", "u_ada", ""},
		{"resources after changes", "GET", "/api/resources", "u_ada", ""},

		{"unknown path", "GET", "/api/no-such-route", "u_ada", ""},
		{"method mismatch", "GET", "/api/login", "u_ada", ""},
		{"logout", "POST", "/api/logout", "u_ada", ""},
	}
}
```

Two steps here are load-bearing and easy to lose in a refactor. "approve already revoked" and "deny already denied" exercise the 409 path, and "comments after posting" is the step whose ordering the tiebreak governs.

- [ ] **Step 2: Write the failing test**

Create `contract/conformance/sequence_test.go`:

```go
package conformance

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// Run replays the lifecycle against one app, schema-checking every
// response, and returns them in order.
func Run(t *testing.T, c *Client) []*Response {
	t.Helper()
	steps := Lifecycle()
	out := make([]*Response, 0, len(steps))
	for _, s := range steps {
		resp := validated(t, c, s.Method, s.Path, s.Actor, s.Body)
		out = append(out, resp)
	}
	return out
}

func TestLifecycleRunsCleanAgainstEveryApp(t *testing.T) {
	for _, app := range apps(t) {
		app := app
		t.Run(app.Name, func(t *testing.T) {
			responses := Run(t, NewClient(app))
			require.Len(t, responses, len(Lifecycle()))
			for i, r := range responses {
				require.NotEqual(t, 0, r.Status, "step %d (%s) produced no status", i, Lifecycle()[i].Name)
				require.Less(t, r.Status, 500, "step %d (%s) returned %d", i, Lifecycle()[i].Name, r.Status)
			}
		})
	}
}
```

The `< 500` assertion is the point: a 500 anywhere in the sequence means an app crashed on a request the contract says is legal.

- [ ] **Step 3: Write the ordering test**

Create `contract/conformance/ordering_test.go`:

```go
package conformance

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"
)

// The contract's x-ordering block declares a total order for every list
// endpoint: the domain key, then id ascending as tiebreak. These two
// fixtures each contain a pair tied on created_at, so they fail for any
// app that drops the tiebreak or serves insertion order.
func TestSeededListsMatchTheDeclaredOrder(t *testing.T) {
	for _, app := range apps(t) {
		app := app
		t.Run(app.Name, func(t *testing.T) {
			c := NewClient(app)
			cases := []struct {
				path string
				want []string
			}{
				{"/api/resources/r_acme_pg/requests", []string{"req_acme_1", "req_acme_4", "req_acme_5"}},
				{"/api/requests/req_acme_1/comments", []string{"c_acme_1", "c_acme_2", "c_acme_3"}},
			}
			for _, tc := range cases {
				resp := validated(t, c, http.MethodGet, tc.path, "u_ada", "")
				require.Equal(t, http.StatusOK, resp.Status)

				var rows []struct {
					ID string `json:"id"`
				}
				require.NoError(t, json.Unmarshal(resp.Body, &rows))

				var got []string
				for _, r := range rows {
					got = append(got, r.ID)
				}
				require.Equal(t, tc.want, got[:len(tc.want)], "%s: declared order violated", tc.path)
			}
		})
	}
}
```

The slice `got[:len(tc.want)]` matters: the lifecycle sequence adds comments, so the list grows. Only the seeded prefix has a pinned order.

- [ ] **Step 4: Run both, ordering first**

Ordering must run before the lifecycle mutates state. Go runs tests within a file in source order but across files alphabetically, so `ordering_test.go` precedes `sequence_test.go`. Do not rely on that alone; run them explicitly:

```bash
make up && sleep 5
cd contract/conformance
go test ./... -run TestSeededListsMatchTheDeclaredOrder -count=1 -v
go test ./... -run TestLifecycleRunsCleanAgainstEveryApp -count=1 -v
cd ../.. && make down
```

Expected: both PASS.

- [ ] **Step 5: Prove the ordering test can fail**

```bash
# in apps/go/store/memory/memory.go, change the two sort.SliceStable
# calls in ListResourceRequests and ListRequestComments to drop the
# id tiebreak, leaving only the CreatedAt comparison
make up && sleep 5
cd contract/conformance && go test ./... -run TestSeededLists -count=1
```

Expected: FAIL, showing the tied pair in the wrong order. Restore the tiebreak, confirm green, and record both outputs in your report. A test that cannot fail is not a test.

- [ ] **Step 6: Commit**

```bash
git add contract/conformance
git commit -m "feat(conformance): the lifecycle sequence and ordering checks

The sequence uses only seeded ids so it replays against any app without
threading state between steps. The ordering fixtures each hold a pair
tied on created_at, so they fail for any app that drops the id tiebreak,
which is the divergence Go's unstable sort makes likely."
```

---

### Task 6: Layer 2, cross-app equality, and `make verify`

**Files:**
- Create: `contract/conformance/equality_test.go`
- Modify: `Makefile`
- Modify: `.github/workflows/ci.yml`
- Create: `contract/conformance/README.md`

**Interfaces:**
- Consumes: everything from Tasks 1 through 5.
- Produces: `make verify` as a working target. Plan 3 extends `docker-compose.yml` and this layer starts comparing automatically.

- [ ] **Step 1: Write the equality test**

Create `contract/conformance/equality_test.go`:

```go
package conformance

import (
	"fmt"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

// The four apps are compared to each other, not to a designated
// reference, so no app's runtime output is privileged and drift is
// caught wherever it appears. Skipped with fewer than two apps up,
// which is the state plans 1 and 2 leave behind.
func TestAppsAgreeWithEachOther(t *testing.T) {
	up := apps(t)
	if len(up) < 2 {
		t.Skipf("cross-app equality needs at least two apps, found %d (%s)", len(up), up[0].Name)
	}

	seedIDs, err := LoadSeedIDs(filepath.Join("..", "seed.json"))
	require.NoError(t, err)

	steps := Lifecycle()
	normalized := map[string][]string{}

	for _, app := range up {
		responses := Run(t, NewClient(app))
		n := NewNormalizer(seedIDs)
		rows := make([]string, 0, len(responses))
		for i, r := range responses {
			body, err := n.Normalize(r.Body)
			require.NoError(t, err, "%s: step %d (%s)", app.Name, i, steps[i].Name)
			rows = append(rows, fmt.Sprintf("%d|%s", r.Status, string(body)))
		}
		normalized[app.Name] = rows
	}

	base := up[0].Name
	for _, app := range up[1:] {
		for i := range steps {
			require.Equal(t, normalized[base][i], normalized[app.Name][i],
				"step %d (%s): %s and %s disagree", i, steps[i].Name, base, app.Name)
		}
	}
}
```

Note each app gets its own `Normalizer`. Placeholders are assigned per app in order of first appearance, so two apps that behave identically produce identical placeholder sequences even though their raw ids differ.

- [ ] **Step 2: Run it and confirm it skips**

```bash
make up && sleep 5
cd contract/conformance && go test ./... -run TestAppsAgree -count=1 -v
cd ../.. && make down
```

Expected: SKIP, with the message naming the one app found. The skip is the correct outcome today; Plan 3 turns it on.

- [ ] **Step 3: Wire `make verify`**

Replace the placeholder `verify` target in the root `Makefile`:

```make
verify:
	docker compose up --build -d
	@echo "waiting for apps to answer /api/health"
	@for i in $$(seq 1 30); do \
	  curl -fsS localhost:9001/api/health >/dev/null 2>&1 && break; \
	  sleep 1; \
	done
	@cd contract/conformance && go test ./... -count=1 -v; status=$$?; \
	 cd ../.. && docker compose down; \
	 exit $$status
```

The `status` capture matters: without it, `docker compose down` succeeding would mask a failing test run and `make verify` would report success on a non-conforming app.

- [ ] **Step 4: Verify the exit code actually propagates**

```bash
make verify; echo "verify exit: $?"
```

Expected: 0, with the tests run and compose torn down.

Then prove the failure path, which is the half that matters:

```bash
# temporarily break an assertion, for example change the expected
# ordering in ordering_test.go to a wrong sequence
make verify; echo "verify exit: $?"
```

Expected: non-zero, AND `docker compose ps` shows nothing still running. Restore the test and record both outputs in your report.

- [ ] **Step 5: Add the harness to CI**

In `.github/workflows/ci.yml`, add a third job:

```yaml
  conformance:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with:
          go-version: '1.25'
      - run: make verify
```

The `clean` and `go` jobs stay as they are.

- [ ] **Step 6: Write `contract/conformance/README.md`**

Short. Cover: what the harness is, the three layers and why layer 3 is asserted rather than compared, how to run it (`make verify`, or `make up` plus `go test ./...` while iterating), the `CONFORMANCE_APPS` override, that it skips cleanly when nothing is running, and the one thing a reader most needs to know: it knows nothing about Everscribe, so it passes identically before and after an instrument run, which is what makes it usable to answer "did the agent change how this app behaves?"

- [ ] **Step 7: Commit**

```bash
git add contract/conformance Makefile .github/workflows/ci.yml
git commit -m "feat(conformance): layer 2 and make verify

Cross-app equality compares the apps to each other rather than to a
reference, and skips below two apps, which is where plans 1 and 2 leave
the repo. make verify captures the test exit status before tearing
compose down, so a failing run cannot be masked by a successful
teardown."
```

---

## Done when

- `make verify` brings the app up, runs the harness, tears it down, and propagates a non-zero exit on failure.
- Layers 1 and 3 pass against the Go app. Layer 2 skips with a message naming the single app.
- The normalizer has its own unit tests and rejects a non-UTC timestamp.
- Both "prove it can fail" steps (Task 4 Step 4, Task 5 Step 5) have recorded output showing the test failing for the right reason and then passing again.
- CI runs `make verify` alongside the existing `clean` and `go` jobs.

Plan 3 adds the three apps. Each one arrives to a harness that already knows what conformance means, and cross-app equality switches on as the second app lands.
