package report

import (
	"encoding/json"
	"fmt"
)

// FormatJSON marshals the report to indented, stable JSON bytes.
func FormatJSON(r *Report) ([]byte, error) {
	if r == nil {
		return nil, fmt.Errorf("report cannot be nil")
	}

	data, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("failed to format report as JSON: %w", err)
	}

	return data, nil
}
