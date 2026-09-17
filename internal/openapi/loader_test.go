package openapi_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/contractlens/contractlens/internal/openapi"
)

const sampleSpecYAML = `
openapi: 3.0.3
info:
  title: Sample API
  version: 1.0.0
paths:
  /users/{id}:
    get:
      operationId: getUserById
      summary: Get user by ID
      parameters:
        - name: id
          in: path
          required: true
          schema:
            type: string
      responses:
        '200':
          description: Successful response
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/User'
components:
  schemas:
    User:
      type: object
      required:
        - id
        - createdAt
      properties:
        id:
          type: string
          description: Unique user identifier
        createdAt:
          type: string
          format: date-time
        name:
          type: string
        status:
          type: string
          enum:
            - active
            - disabled
        profile:
          type: object
          properties:
            bio:
              type: string
        tags:
          type: array
          items:
            type: string
`

func TestLoad_Valid(t *testing.T) {
	ctx := context.Background()
	doc, err := openapi.LoadFromData(ctx, []byte(sampleSpecYAML))
	if err != nil {
		t.Fatalf("expected valid spec to load, got error: %v", err)
	}

	ops := doc.ListOperations()
	if len(ops) != 1 {
		t.Fatalf("expected 1 operation, got %d", len(ops))
	}

	if ops[0].Method != "GET" || ops[0].Path != "/users/{id}" {
		t.Errorf("unexpected operation: %+v", ops[0])
	}
}

func TestLoad_FileNotFound(t *testing.T) {
	ctx := context.Background()
	_, err := openapi.Load(ctx, "non_existent_file.yaml")
	if err == nil {
		t.Fatal("expected error for non-existent file, got nil")
	}
	if want := "spec file does not exist"; !contains(err.Error(), want) {
		t.Errorf("expected error containing %q, got %q", want, err.Error())
	}
}

func TestLoad_InvalidYAML(t *testing.T) {
	ctx := context.Background()
	_, err := openapi.LoadFromData(ctx, []byte("invalid: yaml: [broken"))
	if err == nil {
		t.Fatal("expected error for invalid YAML, got nil")
	}
}

func TestFindOperation(t *testing.T) {
	ctx := context.Background()
	doc, err := openapi.LoadFromData(ctx, []byte(sampleSpecYAML))
	if err != nil {
		t.Fatalf("LoadFromData failed: %v", err)
	}

	tests := []struct {
		name       string
		selector   string
		wantMethod string
		wantPath   string
		wantErr    bool
	}{
		{
			name:       "auto-detect single operation with empty selector",
			selector:   "",
			wantMethod: "GET",
			wantPath:   "/users/{id}",
			wantErr:    false,
		},
		{
			name:       "match by METHOD /path",
			selector:   "GET /users/{id}",
			wantMethod: "GET",
			wantPath:   "/users/{id}",
			wantErr:    false,
		},
		{
			name:       "match case-insensitively",
			selector:   "get /users/{id}",
			wantMethod: "GET",
			wantPath:   "/users/{id}",
			wantErr:    false,
		},
		{
			name:       "match by operationId",
			selector:   "getUserById",
			wantMethod: "GET",
			wantPath:   "/users/{id}",
			wantErr:    false,
		},
		{
			name:     "unknown operation selector",
			selector: "POST /other",
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m, p, op, err := doc.FindOperation(tt.selector)
			if (err != nil) != tt.wantErr {
				t.Fatalf("FindOperation(%q) error = %v, wantErr %v", tt.selector, err, tt.wantErr)
			}
			if !tt.wantErr {
				if m != tt.wantMethod || p != tt.wantPath || op == nil {
					t.Errorf("got method=%s path=%s op=%v, want %s %s", m, p, op, tt.wantMethod, tt.wantPath)
				}
			}
		})
	}
}

func TestResolveEndpointSchema(t *testing.T) {
	ctx := context.Background()
	doc, err := openapi.LoadFromData(ctx, []byte(sampleSpecYAML))
	if err != nil {
		t.Fatalf("LoadFromData failed: %v", err)
	}

	m, p, op, err := doc.FindOperation("GET /users/{id}")
	if err != nil {
		t.Fatalf("FindOperation failed: %v", err)
	}

	endpointSchema, err := doc.ResolveEndpointSchema(m, p, op, "200")
	if err != nil {
		t.Fatalf("ResolveEndpointSchema failed: %v", err)
	}

	if endpointSchema.JSONSchema == nil {
		t.Fatal("expected resolved JSONSchema, got nil")
	}

	if len(endpointSchema.JSONSchema.Properties) == 0 {
		t.Fatal("expected user properties in resolved schema")
	}

	// Test property search
	idProp := openapi.FindPropertySchema(endpointSchema.JSONSchema, "id")
	if idProp == nil || !idProp.Type.Is("string") {
		t.Errorf("expected id property to be string schema, got %v", idProp)
	}

	bioProp := openapi.FindPropertySchema(endpointSchema.JSONSchema, "profile.bio")
	if bioProp == nil || !bioProp.Type.Is("string") {
		t.Errorf("expected profile.bio property to be string schema, got %v", bioProp)
	}

	tagProp := openapi.FindPropertySchema(endpointSchema.JSONSchema, "tags[]")
	if tagProp == nil || !tagProp.Type.Is("string") {
		t.Errorf("expected tags[] item schema to be string schema, got %v", tagProp)
	}
}

func TestLoad_FromFile(t *testing.T) {
	tmpDir := t.TempDir()
	specPath := filepath.Join(tmpDir, "openapi.yaml")
	if err := os.WriteFile(specPath, []byte(sampleSpecYAML), 0644); err != nil {
		t.Fatalf("failed to write temp spec: %v", err)
	}

	doc, err := openapi.Load(context.Background(), specPath)
	if err != nil {
		t.Fatalf("failed to load from file: %v", err)
	}

	if doc == nil || doc.T == nil {
		t.Fatal("doc or doc.T is nil")
	}
}

func contains(s, substr string) bool {
	return filepath.Clean(s) != "" && len(s) >= len(substr) && (s == substr || filepath.Base(s) == substr || stringContains(s, substr))
}

func stringContains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 || (len(s) > 0 && len(substr) > 0 && searchSubstring(s, substr)))
}

func searchSubstring(s, substr string) bool {
	for i := 0; i+len(substr) <= len(s); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
