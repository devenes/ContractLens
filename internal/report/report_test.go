package report_test

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/contractlens/contractlens/internal/analyzer"
	"github.com/contractlens/contractlens/internal/report"
)

func TestNewDiffReport_Matching(t *testing.T) {
	res := &analyzer.AnalysisResult{
		Endpoint: "GET /users/{id}",
		Status:   "200",
		Risk:     analyzer.RiskNone,
		Breaking: false,
		Findings: nil,
	}

	r := report.NewDiffReport("spec.yaml", "response.json", res)
	if r.Breaking {
		t.Error("expected breaking=false")
	}
	if r.Risk != analyzer.RiskNone {
		t.Errorf("expected RiskNone, got %s", r.Risk)
	}

	text := report.FormatText(r)
	if !strings.Contains(text, "NO RISK") {
		t.Errorf("expected text to contain NO RISK, got:\n%s", text)
	}
	if !strings.Contains(text, "Contract matches: no drift detected") {
		t.Errorf("expected text to contain match message, got:\n%s", text)
	}

	jsonData, err := report.FormatJSON(r)
	if err != nil {
		t.Fatalf("FormatJSON failed: %v", err)
	}

	var m map[string]any
	if err := json.Unmarshal(jsonData, &m); err != nil {
		t.Fatalf("unmarshal json failed: %v", err)
	}

	if m["breaking"] != false {
		t.Errorf("expected breaking=false in JSON, got %v", m["breaking"])
	}
}

func TestNewDiffReport_BreakingDrift(t *testing.T) {
	res := &analyzer.AnalysisResult{
		Endpoint: "GET /users/{id}",
		Status:   "200",
		Risk:     analyzer.RiskHigh,
		Breaking: true,
		Findings: []analyzer.Finding{
			{
				Path:         "user.id",
				Kind:         analyzer.KindTypeMismatch,
				ExpectedType: "string",
				ActualType:   "integer",
				Breaking:     true,
				Severity:     analyzer.SeverityHigh,
				Message:      "Type mismatch — expected string, got integer",
			},
			{
				Path:         "user.optionalNote",
				Kind:         analyzer.KindDocumentedFieldMissing,
				ExpectedType: "string",
				Breaking:     false,
				Severity:     analyzer.SeverityLow,
				Message:      "Documented optional field is missing",
			},
		},
	}

	r := report.NewDiffReport("spec.yaml", "response.json", res)
	if !r.Breaking {
		t.Error("expected breaking=true")
	}
	if r.Risk != analyzer.RiskHigh {
		t.Errorf("expected RiskHigh, got %s", r.Risk)
	}

	text := report.FormatText(r)
	if !strings.Contains(text, "HIGH RISK") {
		t.Errorf("expected HIGH RISK in text output:\n%s", text)
	}
	if !strings.Contains(text, "✗ user.id") {
		t.Errorf("expected '✗ user.id' in text output:\n%s", text)
	}
	if !strings.Contains(text, "⚠ user.optionalNote") {
		t.Errorf("expected '⚠ user.optionalNote' in text output:\n%s", text)
	}

	jsonData, err := report.FormatJSON(r)
	if err != nil {
		t.Fatalf("FormatJSON failed: %v", err)
	}

	var m map[string]any
	if err := json.Unmarshal(jsonData, &m); err != nil {
		t.Fatalf("unmarshal json failed: %v", err)
	}

	if m["breaking"] != true {
		t.Errorf("expected breaking=true in JSON, got %v", m["breaking"])
	}
	if m["risk"] != "high" {
		t.Errorf("expected risk=high in JSON, got %v", m["risk"])
	}
}

func TestNewAnalyzeReport(t *testing.T) {
	res := &analyzer.AnalysisResult{
		Endpoint: "GET /users/{id}",
		Status:   "200",
		Risk:     analyzer.RiskHigh,
		Breaking: true,
		Findings: []analyzer.Finding{
			{
				Path:         "user.id",
				Kind:         analyzer.KindTypeMismatch,
				ExpectedType: "string",
				ActualType:   "integer",
				Breaking:     true,
				Severity:     analyzer.SeverityHigh,
				Message:      "Type mismatch",
			},
		},
	}

	ai := &report.AIReport{
		Explanation: "The user.id field changed from string to integer.",
		Impact:      "Clients expecting a string may fail JSON deserialization.",
		Remediations: []string{
			"Restore string format in backend serializer.",
			"Update OpenAPI contract if intentional.",
		},
	}

	r := report.NewAnalyzeReport("spec.yaml", "response.json", res, ai)
	if r.AIAnalysis == nil {
		t.Fatal("expected AIAnalysis to be populated")
	}

	text := report.FormatText(r)
	if !strings.Contains(text, "AI Analysis") {
		t.Errorf("expected text to contain 'AI Analysis', got:\n%s", text)
	}
	if !strings.Contains(text, "Restore string format") {
		t.Errorf("expected remediation in text:\n%s", text)
	}
}
