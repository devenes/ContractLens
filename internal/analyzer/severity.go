package analyzer

// ClassifySeverity determines the severity and breaking status of a finding
// based on deterministic contract rules.
func ClassifySeverity(kind string, isRequired bool, strictAdditionalProperties bool) (severity string, breaking bool) {
	switch kind {
	case KindRequiredFieldMissing:
		return SeverityHigh, true

	case KindTypeMismatch:
		return SeverityHigh, true

	case KindEnumMismatch:
		return SeverityHigh, true

	case KindNullableMismatch:
		if isRequired {
			return SeverityHigh, true
		}
		return SeverityMedium, false

	case KindUnexpectedField:
		if strictAdditionalProperties {
			return SeverityMedium, false
		}
		return SeverityLow, false

	case KindDocumentedFieldMissing:
		return SeverityLow, false

	default:
		return SeverityLow, false
	}
}

// CalculateOverallRisk computes the aggregate risk level from a list of findings.
func CalculateOverallRisk(findings []Finding) string {
	if len(findings) == 0 {
		return RiskNone
	}

	hasHigh := false
	hasMedium := false
	hasLow := false

	for _, f := range findings {
		if f.Breaking || f.Severity == SeverityHigh {
			hasHigh = true
		} else if f.Severity == SeverityMedium {
			hasMedium = true
		} else if f.Severity == SeverityLow {
			hasLow = true
		}
	}

	if hasHigh {
		return RiskHigh
	}
	if hasMedium {
		return RiskMedium
	}
	if hasLow {
		return RiskLow
	}
	return RiskNone
}
