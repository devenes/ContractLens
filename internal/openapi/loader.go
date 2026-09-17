package openapi

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/getkin/kin-openapi/openapi3"
)

// Doc wraps a validated kin-openapi document.
type Doc struct {
	T *openapi3.T
}

// Load reads and validates an OpenAPI 3.x document from a file.
func Load(ctx context.Context, path string) (*Doc, error) {
	if strings.TrimSpace(path) == "" {
		return nil, fmt.Errorf("spec path cannot be empty")
	}

	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("spec file does not exist: %s", path)
		}
		return nil, fmt.Errorf("failed to access spec file %s: %w", path, err)
	}
	if info.IsDir() {
		return nil, fmt.Errorf("spec path is a directory, expected a file: %s", path)
	}

	loader := openapi3.NewLoader()
	loader.IsExternalRefsAllowed = true

	doc, err := loader.LoadFromFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to parse OpenAPI document: %w", err)
	}

	if err := doc.Validate(ctx); err != nil {
		return nil, fmt.Errorf("failed to validate OpenAPI document: %w", err)
	}

	return &Doc{T: doc}, nil
}

// LoadFromData loads and validates an OpenAPI 3.x document from raw bytes.
func LoadFromData(ctx context.Context, data []byte) (*Doc, error) {
	loader := openapi3.NewLoader()
	loader.IsExternalRefsAllowed = true

	doc, err := loader.LoadFromData(data)
	if err != nil {
		return nil, fmt.Errorf("failed to parse OpenAPI document: %w", err)
	}

	if err := doc.Validate(ctx); err != nil {
		return nil, fmt.Errorf("failed to validate OpenAPI document: %w", err)
	}

	return &Doc{T: doc}, nil
}
