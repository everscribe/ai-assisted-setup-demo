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

// NewClient returns a client that never follows a redirect. No route in
// this contract redirects, so a 3xx is always a violation, and following
// one hides it: Go's ServeMux answered GET /api/./me with a 301 to
// /api/me, and a following client replayed X-User-ID against the target
// and reported a 200 carrying Ada's record, which is the existence
// oracle the path rules close, laundered into a success.
func NewClient(a App) *Client {
	return &Client{App: a, http: &http.Client{
		Timeout:       10 * time.Second,
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
	}}
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
