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

	// RFC 9110 4.3.2: a server MUST NOT send content in response to HEAD,
	// so a bodiless 405 for HEAD must validate.
	require.NoError(t, s.ValidateResponse(ctx, http.MethodHead, "/api/health",
		jsonResp(405, "")), "a bodiless HEAD 405 must validate")

	// The exemption is about the body, not a blanket pass for HEAD: a
	// status outside the accepted set must still fail.
	require.Error(t, s.ValidateResponse(ctx, http.MethodHead, "/api/health",
		jsonResp(200, "")), "a HEAD response with an unaccepted status must still fail")
}

func TestValidateResponseAcceptsBodilessNoContent(t *testing.T) {
	t.Parallel()
	s := testSpec(t)
	// 204 carries no Content-Type and no body, by design.
	resp := &Response{Status: 204, Header: http.Header{}, Body: nil}
	require.NoError(t, s.ValidateResponse(context.Background(), http.MethodPost, "/api/logout", resp))
}

func TestValidateResponseRejectsNoContentWithBody(t *testing.T) {
	t.Parallel()
	s := testSpec(t)
	resp := &Response{Status: 204, Header: http.Header{}, Body: []byte(`{"ok":true}`)}
	require.Error(t, s.ValidateResponse(context.Background(), http.MethodPost, "/api/logout", resp))
}

func TestValidateResponseRejectsNoContentWithContentType(t *testing.T) {
	t.Parallel()
	s := testSpec(t)
	resp := &Response{
		Status: 204,
		Header: http.Header{"Content-Type": []string{"application/json"}},
		Body:   nil,
	}
	require.Error(t, s.ValidateResponse(context.Background(), http.MethodPost, "/api/logout", resp))
}
