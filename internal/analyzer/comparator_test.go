package analyzer_test

import (
	"context"
	"testing"

	"github.com/devenes/ContractLens/internal/analyzer"
	"github.com/devenes/ContractLens/internal/openapi"
)

const testSpecYAML = `
openapi: 3.0.3
info:
  title: Test API
  version: 1.0.0
paths:
  /users/{id}:
    get:
      parameters:
        - name: id
          in: path
          required: true
          schema:
            type: string
      responses:
        '200':
          description: User response
          content:
            application/json:
              schema:
                type: object
                required:
                  - id
                  - email
                properties:
                  id:
                    type: string
                  email:
                    type: string
                  role:
                    type: string
                    enum:
                      - admin
                      - member
                  age:
                    type: integer
                  profile:
                    type: object
                    required:
                      - displayName
                    properties:
                      displayName:
                        type: string
                      bio:
                        type: string
                  tags:
                    type: array
                    items:
                      type: string
`

func loadTestSchema(t *testing.T) *openapi.EndpointSchema {
	t.Helper()
	doc, err := openapi.LoadFromData(context.Background(), []byte(testSpecYAML))
	if err != nil {
		t.Fatalf("LoadFromData failed: %v", err)
	}

	m, p, op, err := doc.FindOperation("GET /users/{id}")
	if err != nil {
		t.Fatalf("FindOperation failed: %v", err)
	}

	es, err := doc.ResolveEndpointSchema(m, p, op, "200")
	if err != nil {
		t.Fatalf("ResolveEndpointSchema failed: %v", err)
	}
	return es
}

func TestCompare_MatchingResponse(t *testing.T) {
	es := loadTestSchema(t)

	observed := map[string]any{
		"id":    "user_123",
		"email": "user@example.com",
		"role":  "admin",
		"age":   int64(28),
		"profile": map[string]any{
			"displayName": "Alice",
			"bio":         "Software Engineer",
		},
		"tags": []any{"lead", "backend"},
	}

	result := analyzer.Compare("GET /users/{id}", "200", es.JSONSchema, observed)
	if result.HasBreaking() {
		t.Errorf("expected no breaking findings, got breaking findings: %+v", result.Findings)
	}
	if len(result.Findings) != 0 {
		t.Errorf("expected 0 findings for matching response, got %d: %+v", len(result.Findings), result.Findings)
	}
	if result.Risk != analyzer.RiskNone {
		t.Errorf("expected RiskNone, got %s", result.Risk)
	}
}

func TestCompare_RequiredFieldMissing(t *testing.T) {
	es := loadTestSchema(t)

	// Missing required field 'email'
	observed := map[string]any{
		"id": "user_123",
	}

	result := analyzer.Compare("GET /users/{id}", "200", es.JSONSchema, observed)
	if !result.HasBreaking() {
		t.Error("expected breaking finding for missing required field, got false")
	}

	var foundRequiredMissing bool
	for _, f := range result.Findings {
		if f.Path == "email" && f.Kind == analyzer.KindRequiredFieldMissing {
			foundRequiredMissing = true
			if !f.Breaking {
				t.Error("expected breaking=true for required field missing")
			}
			if f.Severity != analyzer.SeverityHigh {
				t.Errorf("expected severity=high, got %s", f.Severity)
			}
		}
	}
	if !foundRequiredMissing {
		t.Errorf("did not find required_field_missing for email in: %+v", result.Findings)
	}
}

func TestCompare_TypeMismatch(t *testing.T) {
	es := loadTestSchema(t)

	// 'id' expected string, but number provided
	observed := map[string]any{
		"id":    int64(12345),
		"email": "user@example.com",
	}

	result := analyzer.Compare("GET /users/{id}", "200", es.JSONSchema, observed)
	if !result.HasBreaking() {
		t.Error("expected breaking finding for type mismatch, got false")
	}

	var foundTypeMismatch bool
	for _, f := range result.Findings {
		if f.Path == "id" && f.Kind == analyzer.KindTypeMismatch {
			foundTypeMismatch = true
			if f.ActualType != "integer" {
				t.Errorf("expected actual_type=integer, got %s", f.ActualType)
			}
			if f.Severity != analyzer.SeverityHigh {
				t.Errorf("expected severity=high, got %s", f.Severity)
			}
		}
	}
	if !foundTypeMismatch {
		t.Errorf("did not find type_mismatch for id in: %+v", result.Findings)
	}
}

func TestCompare_EnumMismatch(t *testing.T) {
	es := loadTestSchema(t)

	// 'role' is documented as ['admin', 'member'], observed is 'guest'
	observed := map[string]any{
		"id":    "user_123",
		"email": "user@example.com",
		"role":  "guest",
	}

	result := analyzer.Compare("GET /users/{id}", "200", es.JSONSchema, observed)
	if !result.HasBreaking() {
		t.Error("expected breaking finding for enum mismatch, got false")
	}

	var foundEnumMismatch bool
	for _, f := range result.Findings {
		if f.Path == "role" && f.Kind == analyzer.KindEnumMismatch {
			foundEnumMismatch = true
			if f.Severity != analyzer.SeverityHigh {
				t.Errorf("expected severity=high, got %s", f.Severity)
			}
		}
	}
	if !foundEnumMismatch {
		t.Errorf("did not find enum_mismatch for role in: %+v", result.Findings)
	}
}

func TestCompare_UnexpectedField(t *testing.T) {
	es := loadTestSchema(t)

	// 'extraProp' is not documented
	observed := map[string]any{
		"id":        "user_123",
		"email":     "user@example.com",
		"extraProp": "unexpected value",
	}

	result := analyzer.Compare("GET /users/{id}", "200", es.JSONSchema, observed)
	var foundUnexpected bool
	for _, f := range result.Findings {
		if f.Path == "extraProp" && f.Kind == analyzer.KindUnexpectedField {
			foundUnexpected = true
			if f.Breaking {
				t.Error("unexpected field should not be breaking by default")
			}
			if f.Severity != analyzer.SeverityLow {
				t.Errorf("expected severity=low, got %s", f.Severity)
			}
		}
	}
	if !foundUnexpected {
		t.Errorf("did not find unexpected_field for extraProp in: %+v", result.Findings)
	}
}

func TestCompare_DocumentedOptionalMissing(t *testing.T) {
	es := loadTestSchema(t)

	// 'age' is documented optional, not provided
	observed := map[string]any{
		"id":    "user_123",
		"email": "user@example.com",
	}

	result := analyzer.Compare("GET /users/{id}", "200", es.JSONSchema, observed)
	var foundOptionalMissing bool
	for _, f := range result.Findings {
		if f.Path == "age" && f.Kind == analyzer.KindDocumentedFieldMissing {
			foundOptionalMissing = true
			if f.Breaking {
				t.Error("documented optional field missing should not be breaking")
			}
			if f.Severity != analyzer.SeverityLow {
				t.Errorf("expected severity=low, got %s", f.Severity)
			}
		}
	}
	if !foundOptionalMissing {
		t.Errorf("did not find documented_field_missing for age in: %+v", result.Findings)
	}
}

func TestCompare_NestedObjectMismatch(t *testing.T) {
	es := loadTestSchema(t)

	// Nested required field 'profile.displayName' is missing
	observed := map[string]any{
		"id":    "user_123",
		"email": "user@example.com",
		"profile": map[string]any{
			"bio": "Developer",
		},
	}

	result := analyzer.Compare("GET /users/{id}", "200", es.JSONSchema, observed)
	var foundNestedMissing bool
	for _, f := range result.Findings {
		if f.Path == "profile.displayName" && f.Kind == analyzer.KindRequiredFieldMissing {
			foundNestedMissing = true
			if !f.Breaking {
				t.Error("expected nested required field missing to be breaking")
			}
		}
	}
	if !foundNestedMissing {
		t.Errorf("did not find profile.displayName in findings: %+v", result.Findings)
	}
}

func TestCompare_ArrayItemsMismatch(t *testing.T) {
	es := loadTestSchema(t)

	// 'tags' items are documented as string, but number provided
	observed := map[string]any{
		"id":    "user_123",
		"email": "user@example.com",
		"tags":  []any{123, 456},
	}

	result := analyzer.Compare("GET /users/{id}", "200", es.JSONSchema, observed)
	var foundArrayItemMismatch bool
	for _, f := range result.Findings {
		if f.Path == "tags[]" && f.Kind == analyzer.KindTypeMismatch {
			foundArrayItemMismatch = true
			if !f.Breaking {
				t.Error("expected array item type mismatch to be breaking")
			}
		}
	}
	if !foundArrayItemMismatch {
		t.Errorf("did not find tags[] item mismatch in findings: %+v", result.Findings)
	}
}

func TestCompare_NullableMismatch(t *testing.T) {
	es := loadTestSchema(t)

	// 'email' is not nullable, but value is null
	observed := map[string]any{
		"id":    "user_123",
		"email": nil,
	}

	result := analyzer.Compare("GET /users/{id}", "200", es.JSONSchema, observed)
	var foundNullableMismatch bool
	for _, f := range result.Findings {
		if f.Path == "email" && f.Kind == analyzer.KindNullableMismatch {
			foundNullableMismatch = true
			if !f.Breaking {
				t.Error("expected nullable mismatch for required field to be breaking")
			}
		}
	}
	if !foundNullableMismatch {
		t.Errorf("did not find nullable_mismatch for email in: %+v", result.Findings)
	}
}
