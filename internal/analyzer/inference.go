package analyzer

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
)

// DecodeJSON decodes JSON data from an io.Reader preserving numeric precision via json.Number.
func DecodeJSON(r io.Reader) (any, error) {
	dec := json.NewDecoder(r)
	dec.UseNumber()

	var val any
	if err := dec.Decode(&val); err != nil {
		return nil, fmt.Errorf("failed to parse JSON: %w", err)
	}

	// Verify no trailing extra non-whitespace data
	if err := dec.Decode(&struct{}{}); err != io.EOF && err != nil {
		// Only report error if trailing tokens exist
		if !strings.Contains(err.Error(), "EOF") {
			return nil, fmt.Errorf("invalid trailing data in JSON: %w", err)
		}
	}

	return val, nil
}

// DecodeJSONBytes decodes JSON data from a byte slice.
func DecodeJSONBytes(data []byte) (any, error) {
	return DecodeJSON(bytes.NewReader(data))
}

// DecodeJSONFile reads and parses a JSON file from disk.
func DecodeJSONFile(path string) (any, error) {
	if strings.TrimSpace(path) == "" {
		return nil, fmt.Errorf("response path cannot be empty")
	}

	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("response file does not exist: %s", path)
		}
		return nil, fmt.Errorf("failed to access response file %s: %w", path, err)
	}
	if info.IsDir() {
		return nil, fmt.Errorf("response path is a directory, expected a file: %s", path)
	}

	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("failed to open response file %s: %w", path, err)
	}
	defer f.Close()

	val, err := DecodeJSON(f)
	if err != nil {
		return nil, fmt.Errorf("failed to parse response JSON from %s: %w", path, err)
	}

	return val, nil
}

// InferJSONType inspects a decoded JSON value and returns its normalized OpenAPI-compatible
// type name ("string", "integer", "number", "boolean", "null", "object", "array") and converted value.
func InferJSONType(val any) (string, any) {
	if val == nil {
		return "null", nil
	}

	switch v := val.(type) {
	case string:
		return "string", v
	case bool:
		return "boolean", v
	case json.Number:
		str := v.String()
		if !strings.Contains(str, ".") && !strings.ContainsAny(str, "eE") {
			if i, err := v.Int64(); err == nil {
				return "integer", i
			}
		}
		if f, err := v.Float64(); err == nil {
			return "number", f
		}
		return "number", str
	case int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64:
		return "integer", v
	case float32, float64:
		return "number", v
	case map[string]any:
		return "object", v
	case []any:
		return "array", v
	default:
		return fmt.Sprintf("%T", val), v
	}
}

// IsTypeCompatible reports whether an actual JSON type satisfies an expected OpenAPI type.
// Rules:
//   - An "integer" satisfies an expected "number" (all integers are real numbers).
//   - A "number" does NOT satisfy an expected "integer" if it contains a fractional part.
//   - Exact type equality always satisfies.
func IsTypeCompatible(expectedType, actualType string) bool {
	if expectedType == actualType {
		return true
	}
	if expectedType == "number" && actualType == "integer" {
		return true
	}
	return false
}
