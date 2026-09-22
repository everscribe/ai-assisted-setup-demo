package conformance

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"
)

// A JSON null is equivalent to omitting the field. On an optional field
// that means the documented default applies; on a required field it
// means the field is missing. Neither is a type error.
//
// This is asserted per app rather than left to cross-app equality,
// because equality only catches a divergence: two ports that both
// rejected null would agree with each other and still be wrong. The
// reference app's encoding/json leaves a string at its zero value on a
// null, which is what the contract now pins.
func TestNullFieldIsEquivalentToOmittingIt(t *testing.T) {
	for _, app := range apps(t) {
		app := app
		t.Run(app.Name, func(t *testing.T) {
			c := NewClient(app)

			// Optional fields: null takes the documented default.
			invited := validated(t, c, http.MethodPost, "/api/members", "u_ada",
				`{"name":"Null Role","email":"null-role@acme.test","role":null}`)
			require.Equal(t, http.StatusCreated, invited.Status, "a null role must be accepted, not rejected")
			require.Equal(t, "member", jsonField(t, invited.Body, "role"),
				"a null role must default to member, exactly as an omitted one does")

			registered := validated(t, c, http.MethodPost, "/api/resources", "u_ada",
				`{"name":"null-optionals","description":null,"sensitivity":null}`)
			require.Equal(t, http.StatusCreated, registered.Status,
				"a null description and sensitivity must be accepted, not rejected")
			require.Equal(t, "", jsonField(t, registered.Body, "description"))
			require.Equal(t, "internal", jsonField(t, registered.Body, "sensitivity"))

			// Required fields: null is a missing field, so each route
			// answers with its own stable "required" message rather than
			// a schema complaint. The messages asserted here are the ones
			// the contract names as stable; none begins "invalid body:".
			for _, tc := range []struct {
				method, path, actor, body, want string
			}{
				{http.MethodPost, "/api/members", "u_ada",
					`{"name":null,"email":"null-name@acme.test"}`, `{"error":"name and email required"}`},
				{http.MethodPost, "/api/login", "",
					`{"email":null}`, `{"error":"email required"}`},
				{http.MethodPut, "/api/members/u_linus/role", "u_ada",
					`{"role":null}`, `{"error":"role must be admin or member"}`},
				{http.MethodPost, "/api/resources", "u_ada",
					`{"name":null}`, `{"error":"name required"}`},
				{http.MethodPost, "/api/resources/r_acme_stripe/requests", "u_linus",
					`{"reason":null}`, `{"error":"reason required"}`},
				{http.MethodPost, "/api/requests/req_acme_1/comments", "u_linus",
					`{"body":null}`, `{"error":"body required"}`},
			} {
				resp := validated(t, c, tc.method, tc.path, tc.actor, tc.body)
				require.Equal(t, http.StatusBadRequest, resp.Status, "%s %s", tc.method, tc.path)
				require.JSONEq(t, tc.want, string(resp.Body), "%s %s", tc.method, tc.path)
			}

			// A wrong type that is not null is still a 400. Only the
			// status is asserted: the reference app appends its decoder's
			// own message, which no other language reproduces.
			for _, tc := range []struct {
				method, path, actor, body string
			}{
				{http.MethodPost, "/api/members", "u_ada", `{"name":"N","email":"n@x.c","role":7}`},
				{http.MethodPost, "/api/resources", "u_ada", `{"name":"n","description":7}`},
				{http.MethodPost, "/api/resources", "u_ada", `{"name":"n","sensitivity":{}}`},
				{http.MethodPost, "/api/login", "", `{"email":7}`},
			} {
				resp := validated(t, c, tc.method, tc.path, tc.actor, tc.body)
				require.Equal(t, http.StatusBadRequest, resp.Status, "%s %s %s", tc.method, tc.path, tc.body)
			}
		})
	}
}

// Unknown request fields are ignored, not rejected. Every request schema
// in the contract sets additionalProperties: true for this reason; a
// port mirroring a strict schema with serde's deny_unknown_fields or
// Pydantic's extra="forbid" fails here.
func TestUnknownRequestFieldsAreIgnored(t *testing.T) {
	for _, app := range apps(t) {
		app := app
		t.Run(app.Name, func(t *testing.T) {
			c := NewClient(app)

			member := validated(t, c, http.MethodPost, "/api/members", "u_ada",
				`{"name":"Extra Fields","email":"extra-fields@acme.test","bogus":1}`)
			require.Equal(t, http.StatusCreated, member.Status)

			resource := validated(t, c, http.MethodPost, "/api/resources", "u_ada",
				`{"name":"extra-fields-resource","bogus":{"nested":true}}`)
			require.Equal(t, http.StatusCreated, resource.Status)

			request := validated(t, c, http.MethodPost, "/api/resources/r_acme_stripe/requests", "u_linus",
				`{"reason":"unknown fields are ignored","bogus":[1,2]}`)
			require.Equal(t, http.StatusCreated, request.Status)
		})
	}
}

// A mixed-case spelling of a registered path must be as anonymous as an
// unknown path. Without this, a mixed-case probe distinguishes a
// registered path (401 with no actor) from an unregistered one (404),
// which is the same existence leak the trailing-slash rule closes.
func TestMixedCasePathIsTheSame404AsAnUnknownPath(t *testing.T) {
	for _, app := range apps(t) {
		app := app
		t.Run(app.Name, func(t *testing.T) {
			c := NewClient(app)

			unknown := validated(t, c, http.MethodGet, "/api/no-such-route", "u_ada", "")
			require.Equal(t, http.StatusNotFound, unknown.Status)

			for _, path := range []string{"/api/ME", "/api/Members", "/api/RESOURCES"} {
				// Positive control: the lowercase spelling must work, or
				// a 404 on the mixed-case one proves nothing.
				lower := validated(t, c, http.MethodGet, lowercasePath(path), "u_ada", "")
				require.Equal(t, http.StatusOK, lower.Status, "positive control for %s", path)

				mixed := validated(t, c, http.MethodGet, path, "u_ada", "")
				require.Equal(t, unknown.Status, mixed.Status, path)
				require.Equal(t, string(unknown.Body), string(mixed.Body), path)

				// With no actor it must still be a 404, not the 401 a
				// registered path answers. That is the leak itself.
				anon := validated(t, c, http.MethodGet, path, "", "")
				require.Equal(t, http.StatusNotFound, anon.Status,
					"%s with no actor must not reveal that the path exists", path)
			}

			// A mixed-case spelling of the mount must not reach the API.
			requireDidNotReachTheAPI(t, c, "/API/me")
		})
	}
}

// A percent-encoded spelling of a registered path must be as anonymous
// as an unknown path. Same leak as the mixed-case probe above: without
// this, %6de answers 401 with no actor where an unregistered path
// answers 404. Go served Ada's record here, because Gin reads URL.Path
// with UseRawPath off, until the contract pinned the rule.
func TestPercentEncodedPathIsTheSame404AsAnUnknownPath(t *testing.T) {
	for _, app := range apps(t) {
		app := app
		t.Run(app.Name, func(t *testing.T) {
			c := NewClient(app)

			unknown := validated(t, c, http.MethodGet, "/api/no-such-route", "u_ada", "")
			require.Equal(t, http.StatusNotFound, unknown.Status)

			for _, pair := range [][2]string{
				{"/api/%6de", "/api/me"},
				{"/api/%72esources", "/api/resources"},
				{"/api/%6d%65mbers", "/api/members"},
			} {
				encoded, plain := pair[0], pair[1]

				// Positive control: the decoded spelling must work, or a
				// 404 on the encoded one proves nothing.
				ok := validated(t, c, http.MethodGet, plain, "u_ada", "")
				require.Equal(t, http.StatusOK, ok.Status, "positive control for %s", encoded)

				got := validated(t, c, http.MethodGet, encoded, "u_ada", "")
				require.Equal(t, unknown.Status, got.Status, encoded)
				require.Equal(t, string(unknown.Body), string(got.Body), encoded)

				anon := validated(t, c, http.MethodGet, encoded, "", "")
				require.Equal(t, http.StatusNotFound, anon.Status,
					"%s with no actor must not reveal that the path exists", encoded)
			}

			// Raw routing is not raw path parameters. An octet inside a
			// captured id is decoded, so this names the same row the
			// plain spelling does. A port that turns decoding off
			// wholesale to pass the checks above fails here.
			encodedID := validated(t, c, http.MethodGet, "/api/resources/r%5Facme_pg/requests", "u_ada", "")
			plainID := validated(t, c, http.MethodGet, "/api/resources/r_acme_pg/requests", "u_ada", "")
			require.Equal(t, http.StatusOK, encodedID.Status, "a percent-encoded octet in an id is decoded")
			require.Equal(t, string(plainID.Body), string(encodedID.Body))

			// And with no actor an encoded octet in an id must still be
			// the route's 401, not a 404. A port that hands the decoded
			// path to a path guard answers 404 and leaks that the route
			// does not exist. Only this shape shows it: with an actor
			// both answers collapse to 404. Go shipped exactly that for
			// one commit. c.Do, not validated, because %2F decodes to a
			// slash and the spec router cannot route the result.
			for _, path := range []string{
				"/api/resources/%2e/requests",
				"/api/resources/%2e%2e/requests",
				"/api/resources/%2F/requests",
			} {
				anonID, err := c.Do(t.Context(), http.MethodGet, path, "", "")
				require.NoError(t, err)
				require.Equal(t, http.StatusUnauthorized, anonID.Status,
					"%s must reach the route, not a path guard", path)
			}

			// The mount is raw too. Starlette routes on the ASGI-decoded
			// scope["path"], so a port can serve Ada's record here
			// without noticing.
			requireDidNotReachTheAPI(t, c, "/%61pi/me")
		})
	}
}

// A path with a dot segment must be as anonymous as an unknown path.
// Same leak as the two probes above: a port that resolves the segment
// before routing answers 301 to the cleaned path, and a redirect tells a
// caller the cleaned path exists, because an unregistered path has
// nothing to redirect to. Go answered exactly that, from net/http's
// ServeMux, while Node, Rust and Python answered the JSON 404.
func TestDotSegmentPathIsTheSame404AsAnUnknownPath(t *testing.T) {
	for _, app := range apps(t) {
		app := app
		t.Run(app.Name, func(t *testing.T) {
			c := NewClient(app)

			unknown := validated(t, c, http.MethodGet, "/api/no-such-route", "u_ada", "")
			require.Equal(t, http.StatusNotFound, unknown.Status)

			// The last two are the percent-encoded spellings of the
			// first: a guard that only reads the raw path lets a %2e
			// through to the framework's own resolution.
			for _, pair := range [][2]string{
				{"/api/./me", "/api/me"},
				{"/api/resources/../me", "/api/me"},
				{"/api/me/.", "/api/me"},
				{"/api/%2e/me", "/api/me"},
				{"/api/resources/%2e%2e/me", "/api/me"},
			} {
				dotted, plain := pair[0], pair[1]

				// Positive control: the resolved spelling must work, or
				// a 404 on the dotted one proves nothing.
				ok := validated(t, c, http.MethodGet, plain, "u_ada", "")
				require.Equal(t, http.StatusOK, ok.Status, "positive control for %s", dotted)

				got := validated(t, c, http.MethodGet, dotted, "u_ada", "")
				require.Equal(t, unknown.Status, got.Status, dotted)
				require.Equal(t, string(unknown.Body), string(got.Body), dotted)
				require.Empty(t, got.Header.Get("Location"), "%s must not redirect", dotted)

				anon := validated(t, c, http.MethodGet, dotted, "", "")
				require.Equal(t, http.StatusNotFound, anon.Status,
					"%s with no actor must not reveal that the path exists", dotted)
			}

			// The mount is not resolved either.
			requireDidNotReachTheAPI(t, c, "/./api/me")
		})
	}
}

// Check precedence, on every route that reads a body: 401, then 403, then
// parsing. The contract's example is POST /api/resources; FastAPI reorders
// this by binding a body model before the role check. A malformed body
// discriminates, since a valid body answers 403 regardless of order
// (confirmed: moving require_admin after the parse in the Python app stayed
// green). The admin row is a positive control: 403 for a member alone would
// prove nothing. Asserted per app, not by equality, which only flags
// divergence: ports that all parsed first would agree and still be wrong.
func TestEveryBodyRouteChecksAuthAndRoleBeforeParsing(t *testing.T) {
	for _, app := range apps(t) {
		app := app
		t.Run(app.Name, func(t *testing.T) {
			c := NewClient(app)

			// Every authenticated route that reads a body. login is
			// absent on purpose: it is anonymous, so it has no earlier
			// check to run ahead of the parse.
			for _, r := range []struct {
				name, method, path string
				adminOnly          bool
			}{
				{"POST /api/members", http.MethodPost, "/api/members", true},
				{"PUT /api/members/{id}/role", http.MethodPut, "/api/members/u_linus/role", true},
				{"POST /api/resources", http.MethodPost, "/api/resources", true},
				{"POST /api/resources/{id}/requests", http.MethodPost, "/api/resources/r_acme_pg/requests", false},
				{"POST /api/requests/{id}/comments", http.MethodPost, "/api/requests/req_acme_1/comments", false},
			} {
				const malformed = `{`

				anon := validated(t, c, r.method, r.path, "", malformed)
				require.Equal(t, http.StatusUnauthorized, anon.Status,
					"%s: no actor plus a malformed body must be 401, not 400", r.name)

				want := http.StatusBadRequest
				if r.adminOnly {
					want = http.StatusForbidden
				}
				member := validated(t, c, r.method, r.path, "u_linus", malformed)
				require.Equal(t, want, member.Status,
					"%s: a member plus a malformed body", r.name)
				if r.adminOnly {
					require.JSONEq(t, `{"error":"admin role required"}`, string(member.Body), r.name)
				}

				// Positive control: the same body from an admin reaches
				// the 400 the member never gets to.
				admin := validated(t, c, r.method, r.path, "u_ada", malformed)
				require.Equal(t, http.StatusBadRequest, admin.Status,
					"%s: an admin plus a malformed body must reach the parse", r.name)
			}
		})
	}
}

// TestEscapableCharactersSurviveARoundTrip is the absolute half of the
// JSON-escaping rule. TestAppsAgreeWithEachOther only pins agreement: if
// all four ports stripped "<", its comparison would still pass and the
// contract would still claim the value survives. This asserts the value
// itself, per app, so the claim is checked rather than assumed.
func TestEscapableCharactersSurviveARoundTrip(t *testing.T) {
	for _, app := range apps(t) {
		app := app
		t.Run(app.Name, func(t *testing.T) {
			c := NewClient(app)

			const name = "vault<&>prod"
			const description = "tags <b> & a line\u2028separator"

			created := validated(t, c, http.MethodPost, "/api/resources", "u_ada",
				`{"name":"`+name+`","description":"`+description+`","sensitivity":"restricted"}`)
			require.Equal(t, http.StatusCreated, created.Status)
			require.Equal(t, name, jsonField(t, created.Body, "name"))
			require.Equal(t, description, jsonField(t, created.Body, "description"))

			// Read it back, so this pins what was stored rather than an
			// echo of the request body.
			id := jsonField(t, created.Body, "id")
			list := validated(t, c, http.MethodGet, "/api/resources", "u_ada", "")
			var rows []map[string]any
			require.NoError(t, json.Unmarshal(list.Body, &rows))
			found := false
			for _, r := range rows {
				if r["id"] == id {
					found = true
					require.Equal(t, name, r["name"])
					require.Equal(t, description, r["description"])
				}
			}
			require.True(t, found, "the resource just registered is missing from the list")
		})
	}
}

// requireDidNotReachTheAPI asserts the one property the contract pins
// above the /api mount: the request never became an API answer.
//
// The status is deliberately not asserted: the static layer above the
// mount is unspecified, and the four apps disagree (GET /%2e is 301 on
// Go/Node, 307 on Rust, 404 on Python; GET /index.html is 301 on Go, 200
// on the rest). What must hold on all four is the security half: no user
// record comes back.
func requireDidNotReachTheAPI(t *testing.T, c *Client, path string) {
	t.Helper()

	// Positive control: the same actor on the real path does get a
	// record, so the assertions below are about the path.
	inside := validated(t, c, http.MethodGet, "/api/me", "u_ada", "")
	require.Equal(t, http.StatusOK, inside.Status, "positive control for %s", path)

	outside, err := c.Do(t.Context(), http.MethodGet, path, "u_ada", "")
	require.NoError(t, err)
	require.NotEqual(t, string(inside.Body), string(outside.Body),
		"%s must not answer as the API does", path)
	require.NotContains(t, string(outside.Body), "u_ada",
		"%s must not serve a user record", path)
}

// lowercasePath lowercases a path for the positive controls above.
func lowercasePath(p string) string {
	out := []byte(p)
	for i, b := range out {
		if b >= 'A' && b <= 'Z' {
			out[i] = b + ('a' - 'A')
		}
	}
	return string(out)
}

// jsonField reads one string field out of a JSON object body.
func jsonField(t *testing.T, body []byte, key string) string {
	t.Helper()
	var row map[string]any
	require.NoError(t, json.Unmarshal(body, &row))
	v, ok := row[key]
	require.True(t, ok, "body has no %q field: %s", key, body)
	s, ok := v.(string)
	require.True(t, ok, "%q is not a string: %v", key, v)
	return s
}
