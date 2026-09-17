package analyzer

// Finding kinds
const (
	KindRequiredFieldMissing   = "required_field_missing"
	KindDocumentedFieldMissing = "documented_field_missing"
	KindUnexpectedField        = "unexpected_field"
	KindTypeMismatch           = "type_mismatch"
	KindEnumMismatch           = "enum_mismatch"
	KindNullableMismatch       = "nullable_mismatch"
)

// Severity levels
const (
	SeverityHigh   = "high"
	SeverityMedium = "medium"
	SeverityLow    = "low"
)

// Risk levels
const (
	RiskHigh   = "high"
	RiskMedium = "medium"
	RiskLow    = "low"
	RiskNone   = "none"
)

// Finding represents a single verified contract drift item.
type Finding struct {
	Path          string `json:"path"`
	Kind          string `json:"kind"`
	ExpectedType  string `json:"expected_type,omitempty"`
	ActualType    string `json:"actual_type,omitempty"`
	ExpectedValue any    `json:"expected_value,omitempty"`
	ActualValue   any    `json:"actual_value,omitempty"`
	Breaking      bool   `json:"breaking"`
	Severity      string `json:"severity"`
	Message       string `json:"message"`
}

// AnalysisResult aggregates all deterministic drift findings for an endpoint response.
type AnalysisResult struct {
	Endpoint string    `json:"endpoint"`
	Status   string    `json:"status"`
	Risk     string    `json:"risk"`
	Breaking bool      `json:"breaking"`
	Findings []Finding `json:"findings"`
}

// HasBreaking reports whether any finding in the result is classified as breaking.
func (r *AnalysisResult) HasBreaking() bool {
	for _, f := range r.Findings {
		if f.Breaking {
			return true
		}
	}
	return false
}
