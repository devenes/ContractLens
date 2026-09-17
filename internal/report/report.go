package report

import (
	"fmt"

	"github.com/contractlens/contractlens/internal/analyzer"
)

// AIReport contains the AI agent's assessment of verified contract drift.
type AIReport struct {
	Explanation  string   `json:"explanation,omitempty"`
	Impact       string   `json:"impact,omitempty"`
	Remediations []string `json:"remediations,omitempty"`
}

// Report is the top-level structured output of ContractLens.
type Report struct {
	Endpoint        string             `json:"endpoint"`
	Status          string             `json:"status,omitempty"`
	Summary         string             `json:"summary"`
	Risk            string             `json:"risk"`
	Breaking        bool               `json:"breaking"`
	SpecFile        string             `json:"spec_file,omitempty"`
	ResponseFile    string             `json:"response_file,omitempty"`
	Findings        []analyzer.Finding `json:"findings"`
	Recommendations []string           `json:"recommendations,omitempty"`
	AIAnalysis      *AIReport          `json:"ai_analysis,omitempty"`
}

// NewDiffReport constructs a Report from deterministic analysis findings without AI.
func NewDiffReport(specFile, responseFile string, res *analyzer.AnalysisResult) *Report {
	breakingCount := 0
	nonBreakingCount := 0
	for _, f := range res.Findings {
		if f.Breaking {
			breakingCount++
		} else {
			nonBreakingCount++
		}
	}

	summary := generateDeterministicSummary(breakingCount, nonBreakingCount)
	recs := generateDeterministicRecommendations(res.Findings)

	return &Report{
		Endpoint:        res.Endpoint,
		Status:          res.Status,
		Summary:         summary,
		Risk:            res.Risk,
		Breaking:        res.Breaking,
		SpecFile:        specFile,
		ResponseFile:    responseFile,
		Findings:        res.Findings,
		Recommendations: recs,
	}
}

// NewAnalyzeReport constructs a Report combining deterministic findings and AI agent insights.
func NewAnalyzeReport(specFile, responseFile string, res *analyzer.AnalysisResult, ai *AIReport) *Report {
	rep := NewDiffReport(specFile, responseFile, res)

	if ai != nil {
		if ai.Explanation != "" {
			rep.Summary = ai.Explanation
		}
		if len(ai.Remediations) > 0 {
			rep.Recommendations = ai.Remediations
		}
		rep.AIAnalysis = ai
	}

	return rep
}

func generateDeterministicSummary(breakingCount, nonBreakingCount int) string {
	total := breakingCount + nonBreakingCount
	if total == 0 {
		return "No contract drift detected. The response fully conforms to the OpenAPI specification."
	}

	if breakingCount > 0 && nonBreakingCount > 0 {
		return fmt.Sprintf("The response contains %d breaking issue(s) and %d non-breaking issue(s).",
			breakingCount, nonBreakingCount)
	}
	if breakingCount > 0 {
		return fmt.Sprintf("The response contains %d breaking issue(s).", breakingCount)
	}
	return fmt.Sprintf("The response contains %d non-breaking informational difference(s).", nonBreakingCount)
}

func generateDeterministicRecommendations(findings []analyzer.Finding) []string {
	if len(findings) == 0 {
		return nil
	}

	var recs []string
	hasBreaking := false
	hasUnexpected := false
	hasOptionalMissing := false

	for _, f := range findings {
		if f.Breaking {
			hasBreaking = true
		}
		if f.Kind == analyzer.KindUnexpectedField {
			hasUnexpected = true
		}
		if f.Kind == analyzer.KindDocumentedFieldMissing {
			hasOptionalMissing = true
		}
	}

	if hasBreaking {
		recs = append(recs, "Restore the documented type/values or update the OpenAPI contract if the change is intentional.")
		recs = append(recs, "Regenerate client SDKs and verify downstream consumers after updating the contract.")
	}
	if hasUnexpected {
		recs = append(recs, "Document newly observed fields in the OpenAPI specification to maintain contract accuracy.")
	}
	if hasOptionalMissing {
		recs = append(recs, "Confirm whether missing optional fields are expected under this scenario.")
	}

	return recs
}
