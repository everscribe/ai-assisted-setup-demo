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

		// JSON escaping inside a string is not normative: Go escapes <, >,
		// & and U+2028 where the other three emit them raw. What the
		// contract does pin is that the value survives, so this step
		// registers a resource carrying all four and the comparison below
		// reads it back through "resources after changes". Both spellings
		// parse to the same string, which is why comparing parsed bodies
		// rather than response bytes is the whole point of Normalizer.
		{"register resource with escapable characters", "POST", "/api/resources", "u_ada",
			"{\"name\":\"vault<&>prod\",\"description\":\"tags <b> & a line\u2028separator\",\"sensitivity\":\"restricted\"}"},

		{"invite member", "POST", "/api/members", "u_ada", `{"name":"Barbara Liskov","email":"barbara@acme.test","role":"member"}`},
		{"members after invite", "GET", "/api/members", "u_ada", ""},
		{"decommission", "POST", "/api/resources/r_acme_s3/decommission", "u_ada", ""},
		{"resources after changes", "GET", "/api/resources", "u_ada", ""},

		// These six exist because the sequence otherwise uses only GET and
		// POST, so six documented contract rules were never exercised and a
		// single port diverging alone would have gone unnoticed.
		{"head health", "HEAD", "/api/health", "u_ada", ""},
		{"options resources", "OPTIONS", "/api/resources", "u_ada", ""},
		{"trailing slash on a real path", "GET", "/api/resources/", "u_ada", ""},
		{"trailing slash on an unknown path", "GET", "/api/no-such-route/", "u_ada", ""},
		{"unparseable body", "POST", "/api/resources/r_nope/requests", "u_linus", `{`},
		{"role not trimmed", "PUT", "/api/members/u_linus/role", "u_ada", `{"role":" admin "}`},

		// Two more rules the contract states and nothing here exercised.
		// A rule the harness is blind to is the mistake the six steps
		// above were added to correct, so neither arrived as contract
		// text alone.
		//
		// A case variant of a field name is an unknown field, so this
		// login must answer as {} does. Go bound it and returned 200
		// with Ada's record while Node and Rust answered 400, on login
		// and on the role change.
		//
		// A path with an empty segment is the API's own JSON 404. Go
		// answered 301 with an HTML body, Rust 200 with Ada's record,
		// Node 404.
		//
		// Both are rejected requests: neither reaches a store, so no
		// list above them changes and a replay answers identically.
		{"case-variant field name on login", "POST", "/api/login", "", `{"Email":"ada@acme.test"}`},
		{"empty path segment", "GET", "/api//me", "u_ada", ""},

		// Emails are not unique, so login has to say which of two users
		// sharing one it returns. These two steps put that answer in front
		// of the equality layer, so a port resolving the duplicate
		// differently from the others fails here.
		//
		// They do not check the ordering rule openapi.yaml states for that
		// lookup. They cannot: a duplicate minted at runtime is always the
		// later row, so created_at order and a store's own iteration order
		// always agree here and a store that does no ordering passes. Each
		// store's suite plants an older duplicate to pin the rule itself.
		// See TestAppsAgreeWhichUserADuplicateEmailResolvesTo.
		//
		// Both steps come after "members after invite" so no list
		// comparison above them changes, and replaying the script only
		// adds another duplicate, which changes no answer.
		{"invite a duplicate email", "POST", "/api/members", "u_ada", `{"name":"Ada Impostor","email":"ada@acme.test","role":"member"}`},
		{"login with the duplicated email", "POST", "/api/login", "", `{"email":"ada@acme.test"}`},

		// A percent-encoded octet is not decoded before routing, so
		// /api/%6de is an unregistered path, while an octet inside a
		// captured id is decoded and names the same row the plain
		// spelling does. Go answered the first with Ada's record until
		// the contract pinned it. Both are reads, so the script stays
		// re-runnable and no list comparison above them moves.
		{"percent-encoded path", "GET", "/api/%6de", "u_ada", ""},
		{"percent-encoded id", "GET", "/api/resources/r%5Facme_pg/requests", "u_ada", ""},

		// A dot segment is not resolved before routing, so these are
		// unregistered paths and answer as /api/no-such-route does. Go
		// answered 301 with an HTML body from ServeMux, which cleans the
		// path, where Node, Rust and Python answered the JSON 404. Same
		// class as the three steps above; both are reads, so the script
		// stays re-runnable.
		{"dot segment", "GET", "/api/./me", "u_ada", ""},
		{"dot-dot segment", "GET", "/api/resources/../me", "u_ada", ""},
		{"dot segment on a real path", "GET", "/api/me/.", "u_ada", ""},

		// The dot-segment rule from the other side, and with no actor,
		// which is the only spelling that shows it. An octet inside a
		// captured id decodes to a dot segment, but the path still
		// matched a registered route byte for byte, so the route's own
		// 401 fires. A port that hands the decoded path to its path
		// guard answers 404 instead and leaks that the route does not
		// exist. With an actor both answers collapse to 404, which is
		// why these steps send none. Go shipped exactly that regression
		// for one commit, and nothing in the suite saw it.
		{"encoded dot in an id, no actor", "GET", "/api/resources/%2e/requests", "", ""},
		{"encoded dot-dot in an id, no actor", "GET", "/api/resources/%2e%2e/requests", "", ""},
		{"encoded slash in an id, no actor", "GET", "/api/resources/%2F/requests", "", ""},

		// Check precedence, on every admin-gated route that reads a
		// body. The malformed body is what makes these discriminate: a
		// port that parses before it checks the role answers 400 or 422
		// where the contract requires 403, which is exactly what FastAPI
		// does with a Pydantic body model bound as a route parameter.
		// The admin rows are the positive control: the same body from an
		// admin reaches the 400 the member never gets to, so a route that
		// refused everyone could not pass this block. All six are
		// rejected before any store is touched, so the script stays
		// re-runnable and no list comparison above them moves.
		{"malformed body, member, invite member", "POST", "/api/members", "u_linus", `{`},
		{"malformed body, admin, invite member", "POST", "/api/members", "u_ada", `{`},
		{"malformed body, member, role change", "PUT", "/api/members/u_linus/role", "u_linus", `{`},
		{"malformed body, admin, role change", "PUT", "/api/members/u_linus/role", "u_ada", `{`},
		{"malformed body, member, register resource", "POST", "/api/resources", "u_linus", `{`},
		{"malformed body, admin, register resource", "POST", "/api/resources", "u_ada", `{`},

		{"unknown path", "GET", "/api/no-such-route", "u_ada", ""},
		{"method mismatch", "GET", "/api/login", "u_ada", ""},
		{"logout", "POST", "/api/logout", "u_ada", ""},
	}
}
