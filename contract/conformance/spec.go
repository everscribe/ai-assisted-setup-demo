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
	if declaresNoContent(route.Operation, resp.Status) {
		return validateNoContent(method, path, resp)
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

// declaresNoContent reports whether the operation's response for this
// status has no content object, meaning the contract promises an empty
// body regardless of status code (204 today, but not hardcoded to it).
func declaresNoContent(op *openapi3.Operation, status int) bool {
	if op == nil || op.Responses == nil {
		return false
	}
	ref := op.Responses.Status(status)
	if ref == nil || ref.Value == nil {
		return false
	}
	return len(ref.Value.Content) == 0
}

// validateNoContent enforces the contract's no-content rule directly,
// since kin-openapi has nothing to check a body or Content-Type against
// when the response declares no content object.
func validateNoContent(method, path string, resp *Response) error {
	if len(resp.Body) != 0 {
		return fmt.Errorf("%s %s -> %d: response declares no content but body is not empty", method, path, resp.Status)
	}
	if resp.Header.Get("Content-Type") != "" {
		return fmt.Errorf("%s %s -> %d: response declares no content but Content-Type is set", method, path, resp.Status)
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
