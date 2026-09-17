package analyzer_test

import (
	"testing"

	"github.com/devenes/ContractLens/internal/analyzer"
)

func TestClassifySeverity(t *testing.T) {
	tests := []struct {
		name             string
		kind             string
		isRequired       bool
		strictAdditional bool
		wantSeverity     string
		wantBreaking     bool
	}{
		{
			name:         "required field missing",
			kind:         analyzer.KindRequiredFieldMissing,
			isRequired:   true,
			wantSeverity: analyzer.SeverityHigh,
			wantBreaking: true,
		},
		{
			name:         "type mismatch",
			kind:         analyzer.KindTypeMismatch,
			isRequired:   true,
			wantSeverity: analyzer.SeverityHigh,
			wantBreaking: true,
		},
		{
			name:         "enum mismatch",
			kind:         analyzer.KindEnumMismatch,
			isRequired:   true,
			wantSeverity: analyzer.SeverityHigh,
			wantBreaking: true,
		},
		{
			name:         "nullable mismatch required",
			kind:         analyzer.KindNullableMismatch,
			isRequired:   true,
			wantSeverity: analyzer.SeverityHigh,
			wantBreaking: true,
		},
		{
			name:         "nullable mismatch optional",
			kind:         analyzer.KindNullableMismatch,
			isRequired:   false,
			wantSeverity: analyzer.SeverityMedium,
			wantBreaking: false,
		},
		{
			name:         "documented optional missing",
			kind:         analyzer.KindDocumentedFieldMissing,
			isRequired:   false,
			wantSeverity: analyzer.SeverityLow,
			wantBreaking: false,
		},
		{
			name:             "unexpected field relaxed",
			kind:             analyzer.KindUnexpectedField,
			strictAdditional: false,
			wantSeverity:     analyzer.SeverityLow,
			wantBreaking:     false,
		},
		{
			name:             "unexpected field strict",
			kind:             analyzer.KindUnexpectedField,
			strictAdditional: true,
			wantSeverity:     analyzer.SeverityMedium,
			wantBreaking:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sev, breaking := analyzer.ClassifySeverity(tt.kind, tt.isRequired, tt.strictAdditional)
			if sev != tt.wantSeverity || breaking != tt.wantBreaking {
				t.Errorf("ClassifySeverity(%q) = (%s, %v), want (%s, %v)",
					tt.kind, sev, breaking, tt.wantSeverity, tt.wantBreaking)
			}
		})
	}
}

func TestCalculateOverallRisk(t *testing.T) {
	tests := []struct {
		name     string
		findings []analyzer.Finding
		wantRisk string
	}{
		{
			name:     "empty findings",
			findings: nil,
			wantRisk: analyzer.RiskNone,
		},
		{
			name: "single low finding",
			findings: []analyzer.Finding{
				{Severity: analyzer.SeverityLow, Breaking: false},
			},
			wantRisk: analyzer.RiskLow,
		},
		{
			name: "single medium finding",
			findings: []analyzer.Finding{
				{Severity: analyzer.SeverityMedium, Breaking: false},
			},
			wantRisk: analyzer.RiskMedium,
		},
		{
			name: "high and low findings",
			findings: []analyzer.Finding{
				{Severity: analyzer.SeverityLow, Breaking: false},
				{Severity: analyzer.SeverityHigh, Breaking: true},
			},
			wantRisk: analyzer.RiskHigh,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := analyzer.CalculateOverallRisk(tt.findings)
			if got != tt.wantRisk {
				t.Errorf("CalculateOverallRisk() = %s, want %s", got, tt.wantRisk)
			}
		})
	}
}
