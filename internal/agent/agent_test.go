package agent_test

import (
	"context"
	"iter"
	"os"
	"strings"
	"testing"

	"google.golang.org/genai"

	"google.golang.org/adk/v2/model"

	"github.com/contractlens/contractlens/internal/agent"
	"github.com/contractlens/contractlens/internal/analyzer"
)

// mockModel implements model.LLM for deterministic offline testing.
type mockModel struct {
	name     string
	response string
	err      error
}

func (m *mockModel) Name() string {
	if m.name != "" {
		return m.name
	}
	return "mock-gemini-model"
}

func (m *mockModel) GenerateContent(ctx context.Context, req *model.LLMRequest, stream bool) iter.Seq2[*model.LLMResponse, error] {
	return func(yield func(*model.LLMResponse, error) bool) {
		if m.err != nil {
			yield(nil, m.err)
			return
		}
		resp := &model.LLMResponse{
			Content: genai.NewContentFromText(m.response, genai.RoleModel),
		}
		yield(resp, nil)
	}
}

func TestNew_MissingAPIKey(t *testing.T) {
	// Ensure environment variables are clear for this test
	origKey := os.Getenv("GOOGLE_API_KEY")
	origGemini := os.Getenv("GEMINI_API_KEY")
	os.Unsetenv("GOOGLE_API_KEY")
	os.Unsetenv("GEMINI_API_KEY")
	defer func() {
		if origKey != "" {
			os.Setenv("GOOGLE_API_KEY", origKey)
		}
		if origGemini != "" {
			os.Setenv("GEMINI_API_KEY", origGemini)
		}
	}()

	_, err := agent.New(context.Background(), agent.Config{})
	if err == nil {
		t.Fatal("expected error when API key is missing, got nil")
	}

	if !strings.Contains(err.Error(), "GOOGLE_API_KEY is required for `analyze`") {
		t.Errorf("expected error message to mention GOOGLE_API_KEY, got %q", err.Error())
	}
}

func TestAnalyze_WithMockModel(t *testing.T) {
	ctx := context.Background()

	mockJSONResponse := `{
		"explanation": "The user.id field was returned as an integer instead of the documented string format.",
		"impact": "Generated client SDKs and strongly typed consumers may fail JSON deserialization.",
		"remediations": [
			"Restore string formatting for user.id in the backend response serializer.",
			"Update OpenAPI specification if user.id integer format is an intentional redesign."
		]
	}`

	mock := &mockModel{
		response: mockJSONResponse,
	}

	ag, err := agent.New(ctx, agent.Config{
		Model: mock,
	})
	if err != nil {
		t.Fatalf("agent.New failed with mock model: %v", err)
	}

	input := agent.AnalysisInput{
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
		},
	}

	out, err := ag.Analyze(ctx, input)
	if err != nil {
		t.Fatalf("ag.Analyze failed: %v", err)
	}

	if out == nil {
		t.Fatal("expected non-nil AnalysisOutput")
	}

	if !strings.Contains(out.Explanation, "user.id") {
		t.Errorf("unexpected explanation: %q", out.Explanation)
	}

	if len(out.Remediations) != 2 {
		t.Errorf("expected 2 remediations, got %d", len(out.Remediations))
	}
}

func TestAnalyze_WithMarkdownFences(t *testing.T) {
	ctx := context.Background()

	mockFencedResponse := "```json\n" + `{
		"explanation": "Field user.role has invalid enum value 'guest'.",
		"impact": "Consumers with enum checks may reject this payload.",
		"remediations": [
			"Add 'guest' to the OpenAPI enum specification."
		]
	}` + "\n```"

	mock := &mockModel{
		response: mockFencedResponse,
	}

	ag, err := agent.New(ctx, agent.Config{
		Model: mock,
	})
	if err != nil {
		t.Fatalf("agent.New failed: %v", err)
	}

	input := agent.AnalysisInput{
		Endpoint: "GET /users/{id}",
		Status:   "200",
		Risk:     analyzer.RiskHigh,
		Breaking: true,
		Findings: []analyzer.Finding{
			{
				Path:     "user.role",
				Kind:     analyzer.KindEnumMismatch,
				Breaking: true,
				Severity: analyzer.SeverityHigh,
			},
		},
	}

	out, err := ag.Analyze(ctx, input)
	if err != nil {
		t.Fatalf("Analyze failed: %v", err)
	}

	if !strings.Contains(out.Explanation, "invalid enum") {
		t.Errorf("expected explanation to parse properly, got: %s", out.Explanation)
	}
}

func TestAnalyze_FallbackPlainText(t *testing.T) {
	ctx := context.Background()

	plainText := "The contract has drifted because user.createdAt was missing from the response."

	mock := &mockModel{
		response: plainText,
	}

	ag, err := agent.New(ctx, agent.Config{
		Model: mock,
	})
	if err != nil {
		t.Fatalf("agent.New failed: %v", err)
	}

	input := agent.AnalysisInput{
		Endpoint: "GET /users/{id}",
		Status:   "200",
		Risk:     analyzer.RiskHigh,
		Breaking: true,
		Findings: []analyzer.Finding{
			{
				Path:     "user.createdAt",
				Kind:     analyzer.KindRequiredFieldMissing,
				Breaking: true,
				Severity: analyzer.SeverityHigh,
			},
		},
	}

	out, err := ag.Analyze(ctx, input)
	if err != nil {
		t.Fatalf("Analyze failed: %v", err)
	}

	if out.Explanation != plainText {
		t.Errorf("expected explanation to be %q, got %q", plainText, out.Explanation)
	}
}

// Optional live integration test (only runs when LIVE_GEMINI_TEST=true and GOOGLE_API_KEY is present).
func TestLiveGeminiIntegration(t *testing.T) {
	if os.Getenv("LIVE_GEMINI_TEST") != "true" {
		t.Skip("skipping live Gemini integration test; set LIVE_GEMINI_TEST=true to run")
	}

	if os.Getenv("GOOGLE_API_KEY") == "" && os.Getenv("GEMINI_API_KEY") == "" {
		t.Skip("skipping live test: neither GOOGLE_API_KEY nor GEMINI_API_KEY is set")
	}

	ctx := context.Background()
	ag, err := agent.New(ctx, agent.Config{})
	if err != nil {
		t.Fatalf("agent.New with live credentials failed: %v", err)
	}

	input := agent.AnalysisInput{
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
		},
	}

	out, err := ag.Analyze(ctx, input)
	if err != nil {
		t.Fatalf("Live Analyze call failed: %v", err)
	}

	if out.Explanation == "" {
		t.Error("expected non-empty explanation from live Gemini model")
	}
}
