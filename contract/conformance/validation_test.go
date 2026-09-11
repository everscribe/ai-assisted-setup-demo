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
