package analyzer_test

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/devenes/ContractLens/internal/analyzer"
)

func TestInferJSONType(t *testing.T) {
	tests := []struct {
		name         string
		input        any
		wantTypeName string
	}{
		{
			name:         "string",
			input:        "hello",
			wantTypeName: "string",
		},
		{
			name:         "boolean true",
			input:        true,
			wantTypeName: "boolean",
		},
		{
			name:         "boolean false",
			input:        false,
			wantTypeName: "boolean",
		},
		{
			name:         "null",
			input:        nil,
			wantTypeName: "null",
		},
		{
			name:         "json.Number integer",
			input:        json.Number("42"),
			wantTypeName: "integer",
		},
		{
			name:         "json.Number float",
			input:        json.Number("42.5"),
			wantTypeName: "number",
		},
		{
			name:         "json.Number scientific notation",
			input:        json.Number("1e5"),
			wantTypeName: "number",
		},
		{
			name:         "raw int",
			input:        123,
			wantTypeName: "integer",
		},
		{
			name:         "raw float64",
			input:        123.45,
			wantTypeName: "number",
		},
		{
			name:         "object map",
			input:        map[string]any{"key": "val"},
			wantTypeName: "object",
		},
		{
			name:         "array slice",
			input:        []any{"item1", "item2"},
			wantTypeName: "array",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotType, _ := analyzer.InferJSONType(tt.input)
			if gotType != tt.wantTypeName {
				t.Errorf("InferJSONType(%v) = %q, want %q", tt.input, gotType, tt.wantTypeName)
			}
		})
	}
}

func TestDecodeJSON(t *testing.T) {
	rawJSON := `{
		"id": 100,
		"ratio": 3.1415,
		"active": true,
		"name": "ContractLens"
	}`

	val, err := analyzer.DecodeJSON(strings.NewReader(rawJSON))
	if err != nil {
		t.Fatalf("DecodeJSON failed: %v", err)
	}

	m, ok := val.(map[string]any)
	if !ok {
		t.Fatalf("expected map[string]any, got %T", val)
	}

	idType, idVal := analyzer.InferJSONType(m["id"])
	if idType != "integer" || idVal != int64(100) {
		t.Errorf("id: expected (integer, 100), got (%s, %v)", idType, idVal)
	}

	ratioType, ratioVal := analyzer.InferJSONType(m["ratio"])
	if ratioType != "number" || ratioVal != 3.1415 {
		t.Errorf("ratio: expected (number, 3.1415), got (%s, %v)", ratioType, ratioVal)
	}
}

func TestIsTypeCompatible(t *testing.T) {
	tests := []struct {
		expected string
		actual   string
		want     bool
	}{
		{"string", "string", true},
		{"number", "number", true},
		{"integer", "integer", true},
		{"number", "integer", true},  // integer satisfies number
		{"integer", "number", false}, // float does not satisfy integer
		{"boolean", "string", false},
		{"object", "array", false},
	}

	for _, tt := range tests {
		got := analyzer.IsTypeCompatible(tt.expected, tt.actual)
		if got != tt.want {
			t.Errorf("IsTypeCompatible(%q, %q) = %v, want %v", tt.expected, tt.actual, got, tt.want)
		}
	}
}
