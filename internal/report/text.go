package report

import (
	"fmt"
	"strings"

	"github.com/devenes/ContractLens/internal/analyzer"
)

// FormatText renders the report in a clean, human-readable terminal format.
func FormatText(r *Report) string {
	var b strings.Builder

	b.WriteString("ContractLens\n")
	b.WriteString("API Contract Drift Detector\n\n")

	if r.Endpoint != "" {
		if r.Status != "" {
			b.WriteString(fmt.Sprintf("Endpoint: %s (HTTP %s)\n\n", r.Endpoint, r.Status))
		} else {
			b.WriteString(fmt.Sprintf("Endpoint: %s\n\n", r.Endpoint))
		}
	}

	// Risk Header
	riskLabel := strings.ToUpper(r.Risk)
	if riskLabel == "NONE" {
		riskLabel = "NO RISK"
	} else {
		riskLabel = riskLabel + " RISK"
	}

	breakingCount := 0
	for _, f := range r.Findings {
		if f.Breaking {
			breakingCount++
		}
	}

	if breakingCount > 0 {
		b.WriteString(fmt.Sprintf("%s (%d breaking change(s))\n\n", riskLabel, breakingCount))
	} else {
		b.WriteString(fmt.Sprintf("%s\n\n", riskLabel))
	}

	if len(r.Findings) == 0 {
		b.WriteString("✓ Contract matches: no drift detected.\n\n")
	} else {
		for _, f := range r.Findings {
			formatFindingText(&b, f)
		}
	}

	// AI Analysis Section
	if r.AIAnalysis != nil {
		b.WriteString("AI Analysis\n")
		b.WriteString("------------\n\n")

		if r.AIAnalysis.Explanation != "" {
			b.WriteString(strings.TrimSpace(r.AIAnalysis.Explanation))
			b.WriteString("\n\n")
		}

		if r.AIAnalysis.Impact != "" {
			b.WriteString("Impact Assessment:\n")
			b.WriteString(strings.TrimSpace(r.AIAnalysis.Impact))
			b.WriteString("\n\n")
		}

		if len(r.AIAnalysis.Remediations) > 0 {
			b.WriteString("Recommended action:\n")
			for _, rec := range r.AIAnalysis.Remediations {
				b.WriteString(fmt.Sprintf("• %s\n", strings.TrimSpace(rec)))
			}
			b.WriteString("\n")
		}
	} else if len(r.Recommendations) > 0 && breakingCount > 0 {
		b.WriteString("Recommendations\n")
		b.WriteString("----------------\n\n")
		for _, rec := range r.Recommendations {
			b.WriteString(fmt.Sprintf("• %s\n", strings.TrimSpace(rec)))
		}
		b.WriteString("\n")
	}

	return strings.TrimRight(b.String(), "\n") + "\n"
}

func formatFindingText(b *strings.Builder, f analyzer.Finding) {
	var glyph string
	var statusSuffix string

	switch {
	case f.Breaking:
		glyph = "✗"
		statusSuffix = " — breaking"
	case f.Kind == analyzer.KindUnexpectedField:
		glyph = "+"
		statusSuffix = ""
	case f.Kind == analyzer.KindDocumentedFieldMissing:
		glyph = "⚠"
		statusSuffix = " (optional)"
	default:
		glyph = "⚠"
		statusSuffix = ""
	}

	b.WriteString(fmt.Sprintf("%s %s\n", glyph, f.Path))

	if f.ExpectedType != "" {
		b.WriteString(fmt.Sprintf("  Expected: %s\n", f.ExpectedType))
	}
	if f.ActualType != "" {
		b.WriteString(fmt.Sprintf("  Actual:   %s\n", f.ActualType))
	}
	if f.ExpectedValue != nil {
		b.WriteString(fmt.Sprintf("  Expected Value: %v\n", f.ExpectedValue))
	}
	if f.ActualValue != nil {
		b.WriteString(fmt.Sprintf("  Actual Value:   %v\n", f.ActualValue))
	}

	msg := f.Message
	if msg != "" {
		b.WriteString(fmt.Sprintf("  %s%s\n", msg, statusSuffix))
	}
	b.WriteString("\n")
}
