package openapi

import (
	"github.com/getkin/kin-openapi/openapi3"
)

// EndpointSchema encapsulates a resolved OpenAPI operation and its response schema.
type EndpointSchema struct {
	Method     string
	Path       string
	StatusCode string
	Operation  *openapi3.Operation
	Response   *openapi3.Response
	JSONSchema *openapi3.Schema
}

// OperationInfo holds metadata about an available operation in the spec.
type OperationInfo struct {
	Method      string `json:"method"`
	Path        string `json:"path"`
	OperationID string `json:"operation_id,omitempty"`
	Summary     string `json:"summary,omitempty"`
}

// String returns a formatted representation like "GET /users/{id}".
func (o OperationInfo) String() string {
	return o.Method + " " + o.Path
}
