package openapi

import (
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/getkin/kin-openapi/openapi3"
)

// ListOperations returns all available operations in the document.
func (d *Doc) ListOperations() []OperationInfo {
	if d.T == nil || d.T.Paths == nil {
		return nil
	}

	var ops []OperationInfo
	for path, item := range d.T.Paths.Map() {
		if item == nil {
			continue
		}
		for method, op := range item.Operations() {
			if op == nil {
				continue
			}
			ops = append(ops, OperationInfo{
				Method:      strings.ToUpper(method),
				Path:        path,
				OperationID: op.OperationID,
				Summary:     op.Summary,
			})
		}
	}

	sort.Slice(ops, func(i, j int) bool {
		if ops[i].Path == ops[j].Path {
			return ops[i].Method < ops[j].Method
		}
		return ops[i].Path < ops[j].Path
	})

	return ops
}

// FindOperation resolves an operation from an optional selector string.
// Supported formats:
//   - "GET /users/{id}" or "get /users/{id}"
//   - OperationID (e.g. "getUserById")
//   - Empty string (if exactly one operation exists in the spec, it is automatically chosen)
func (d *Doc) FindOperation(selector string) (method string, path string, op *openapi3.Operation, err error) {
	ops := d.ListOperations()
	if len(ops) == 0 {
		return "", "", nil, fmt.Errorf("no operations found in OpenAPI specification")
	}

	trimmed := strings.TrimSpace(selector)

	// Auto-detect if empty
	if trimmed == "" {
		if len(ops) == 1 {
			opInfo := ops[0]
			item := d.T.Paths.Value(opInfo.Path)
			return opInfo.Method, opInfo.Path, item.GetOperation(strings.ToUpper(opInfo.Method)), nil
		}

		var available []string
		for _, o := range ops {
			available = append(available, o.String())
		}
		return "", "", nil, fmt.Errorf("multiple operations found in spec; please specify one with --endpoint (available: %s)", strings.Join(available, ", "))
	}

	// Try "METHOD /path" pattern
	parts := strings.Fields(trimmed)
	if len(parts) == 2 {
		targetMethod := strings.ToUpper(parts[0])
		targetPath := parts[1]

		if item := d.T.Paths.Value(targetPath); item != nil {
			if operation := item.GetOperation(strings.ToUpper(targetMethod)); operation != nil {
				return targetMethod, targetPath, operation, nil
			}
		}
	}

	// Try matching by OperationID
	for _, opInfo := range ops {
		if opInfo.OperationID != "" && strings.EqualFold(opInfo.OperationID, trimmed) {
			item := d.T.Paths.Value(opInfo.Path)
			return opInfo.Method, opInfo.Path, item.GetOperation(strings.ToUpper(opInfo.Method)), nil
		}
	}

	// If single path matched without method prefix
	if strings.HasPrefix(trimmed, "/") {
		if item := d.T.Paths.Value(trimmed); item != nil {
			itemOps := item.Operations()
			if len(itemOps) == 1 {
				for m, o := range itemOps {
					return strings.ToUpper(m), trimmed, o, nil
				}
			}
		}
	}

	var available []string
	for _, o := range ops {
		available = append(available, o.String())
	}
	return "", "", nil, fmt.Errorf("operation %q not found in specification (available: %s)", selector, strings.Join(available, ", "))
}

// ResolveEndpointSchema retrieves the response schema for a chosen operation and status code.
func (d *Doc) ResolveEndpointSchema(method, path string, op *openapi3.Operation, statusCode string) (*EndpointSchema, error) {
	if op == nil {
		return nil, fmt.Errorf("operation cannot be nil")
	}

	if op.Responses == nil || op.Responses.Len() == 0 {
		return nil, fmt.Errorf("operation %s %s does not define any responses", method, path)
	}

	var targetResp *openapi3.Response
	var resolvedStatus string

	// 1. Try exact status code (e.g. "200")
	if statusCode != "" {
		if respRef := op.Responses.Value(statusCode); respRef != nil && respRef.Value != nil {
			targetResp = respRef.Value
			resolvedStatus = statusCode
		}
	}

	// 2. If not found or status not specified, look for 200, 201, 204, or first 2xx
	if targetResp == nil {
		for _, code := range []string{"200", "201", "202", "203", "204"} {
			if respRef := op.Responses.Value(code); respRef != nil && respRef.Value != nil {
				targetResp = respRef.Value
				resolvedStatus = code
				break
			}
		}
	}

	// 3. Fallback to "default"
	if targetResp == nil {
		if def := op.Responses.Default(); def != nil && def.Value != nil {
			targetResp = def.Value
			resolvedStatus = "default"
		}
	}

	// 4. Fallback to first available response in map
	if targetResp == nil {
		for code, ref := range op.Responses.Map() {
			if ref != nil && ref.Value != nil {
				targetResp = ref.Value
				resolvedStatus = code
				break
			}
		}
	}

	if targetResp == nil {
		return nil, fmt.Errorf("no suitable response found for operation %s %s (requested status: %s)", method, path, statusCode)
	}

	// Find application/json schema
	var schema *openapi3.Schema
	if mediaType := targetResp.Content.Get("application/json"); mediaType != nil && mediaType.Schema != nil {
		schema = mediaType.Schema.Value
	} else if mediaType := targetResp.Content.Get("*/*"); mediaType != nil && mediaType.Schema != nil {
		schema = mediaType.Schema.Value
	}

	if schema == nil {
		var availableMedia []string
		for ct := range targetResp.Content {
			availableMedia = append(availableMedia, ct)
		}
		return nil, fmt.Errorf("response %s for %s %s does not define an application/json schema (available: %s)",
			resolvedStatus, method, path, strings.Join(availableMedia, ", "))
	}

	return &EndpointSchema{
		Method:     method,
		Path:       path,
		StatusCode: resolvedStatus,
		Operation:  op,
		Response:   targetResp,
		JSONSchema: schema,
	}, nil
}

// FindPropertySchema navigates dot-separated paths (e.g., "user.profile.name" or "items[].id")
// to locate a property schema within a root schema.
func FindPropertySchema(root *openapi3.Schema, dotPath string) *openapi3.Schema {
	if root == nil || dotPath == "" {
		return root
	}

	cleanPath := strings.TrimPrefix(dotPath, ".")
	parts := strings.Split(cleanPath, ".")

	current := root
	for _, part := range parts {
		if current == nil {
			return nil
		}

		// Handle array notation like "items[]" or "users[]"
		cleanPart := part
		isArrayElem := false
		if strings.HasSuffix(part, "[]") {
			cleanPart = strings.TrimSuffix(part, "[]")
			isArrayElem = true
		} else if idx := strings.Index(part, "["); idx != -1 && strings.HasSuffix(part, "]") {
			cleanPart = part[:idx]
			isArrayElem = true
		}

		if cleanPart != "" {
			propRef := current.Properties[cleanPart]
			if propRef == nil || propRef.Value == nil {
				return nil
			}
			current = propRef.Value
		}

		if isArrayElem {
			if current.Items == nil || current.Items.Value == nil {
				return nil
			}
			current = current.Items.Value
		}
	}

	return current
}

// FormatStatusCode normalizes status input (e.g. integer 200 or string "200").
func FormatStatusCode(status string) string {
	s := strings.TrimSpace(status)
	if s == "" {
		return "200"
	}
	if n, err := strconv.Atoi(s); err == nil {
		return strconv.Itoa(n)
	}
	return s
}
