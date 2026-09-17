package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"google.golang.org/genai"

	adkagent "google.golang.org/adk/v2/agent"
	"google.golang.org/adk/v2/agent/llmagent"
	"google.golang.org/adk/v2/model"
	"google.golang.org/adk/v2/model/gemini"
	"google.golang.org/adk/v2/runner"

	"github.com/devenes/ContractLens/internal/analyzer"
)

// DefaultModel is the recommended stable Gemini model in ADK Go examples.
const DefaultModel = "gemini-flash-latest"

// Config configures the ContractLens ADK agent.
type Config struct {
	ModelName string
	APIKey    string
	// Model overrides the underlying model (useful for unit tests with a mock model).
	Model model.LLM
}

// AnalysisInput contains the verified findings to be explained.
type AnalysisInput struct {
	Endpoint string
	Status   string
	Risk     string
	Breaking bool
	Findings []analyzer.Finding
}

// AnalysisOutput contains the structured explanation and remediation advice from the agent.
type AnalysisOutput struct {
	Explanation  string   `json:"explanation"`
	Impact       string   `json:"impact"`
	Remediations []string `json:"remediations"`
}

// Service defines the contract for analyzing verified findings.
type Service interface {
	Analyze(ctx context.Context, input AnalysisInput) (*AnalysisOutput, error)
}

// ADKAgent orchestrates the ADK Go v2 agent execution.
type ADKAgent struct {
	runner *runner.Runner
	agent  adkagent.Agent
}

// New creates and initializes a new ADKAgent using Google ADK Go v2.
func New(ctx context.Context, cfg Config) (*ADKAgent, error) {
	var m model.LLM

	if cfg.Model != nil {
		m = cfg.Model
	} else {
		apiKey := cfg.APIKey
		if apiKey == "" {
			apiKey = os.Getenv("GOOGLE_API_KEY")
			if apiKey == "" {
				apiKey = os.Getenv("GEMINI_API_KEY")
			}
		}

		if apiKey == "" {
			return nil, fmt.Errorf("GOOGLE_API_KEY is required for `analyze`.\nUse `diff` for deterministic analysis without an API key")
		}

		modelName := cfg.ModelName
		if modelName == "" {
			if envModel := os.Getenv("CONTRACTLENS_MODEL"); envModel != "" {
				modelName = envModel
			} else {
				modelName = DefaultModel
			}
		}

		var err error
		m, err = gemini.NewModel(ctx, modelName, &genai.ClientConfig{
			APIKey: apiKey,
		})
		if err != nil {
			return nil, fmt.Errorf("failed to initialize Gemini model %q: %w", modelName, err)
		}
	}

	a, err := llmagent.New(llmagent.Config{
		Name:        "contractlens_analyst",
		Model:       m,
		Description: "API contract drift analysis and remediation agent",
		Instruction: SystemInstruction,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to initialize ADK LLMAgent: %w", err)
	}

	r, err := runner.NewInMemory("contractlens", a)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize ADK runner: %w", err)
	}

	return &ADKAgent{
		runner: r,
		agent:  a,
	}, nil
}

// Analyze sends verified findings to the ADK agent and decodes the structured response.
func (a *ADKAgent) Analyze(ctx context.Context, input AnalysisInput) (*AnalysisOutput, error) {
	promptText, err := BuildUserPrompt(input)
	if err != nil {
		return nil, fmt.Errorf("failed to build agent prompt: %w", err)
	}

	msg := genai.NewContentFromText(promptText, genai.RoleUser)
	sessionID := fmt.Sprintf("session-%d", time.Now().UnixNano())
	userID := "contractlens-cli"

	var responseText strings.Builder

	for event, err := range a.runner.Run(ctx, userID, sessionID, msg, adkagent.RunConfig{}) {
		if err != nil {
			return nil, fmt.Errorf("agent execution failed: %w", err)
		}
		if event == nil {
			continue
		}

		// Collect text from final model response
		if event.LLMResponse.Content != nil {
			for _, part := range event.LLMResponse.Content.Parts {
				if part.Text != "" {
					responseText.WriteString(part.Text)
				}
			}
		}
	}

	rawText := strings.TrimSpace(responseText.String())
	if rawText == "" {
		return nil, fmt.Errorf("agent returned empty response")
	}

	return parseAnalysisOutput(rawText)
}

// parseAnalysisOutput parses JSON from model output, handling potential markdown fences.
func parseAnalysisOutput(raw string) (*AnalysisOutput, error) {
	cleaned := strings.TrimSpace(raw)

	// Remove markdown fences like ```json ... ``` or ``` ... ```
	if strings.HasPrefix(cleaned, "```") {
		lines := strings.Split(cleaned, "\n")
		if len(lines) >= 2 {
			if strings.HasPrefix(lines[0], "```") {
				lines = lines[1:]
			}
			if len(lines) > 0 && strings.HasPrefix(lines[len(lines)-1], "```") {
				lines = lines[:len(lines)-1]
			}
			cleaned = strings.TrimSpace(strings.Join(lines, "\n"))
		}
	}

	var out AnalysisOutput
	if err := json.Unmarshal([]byte(cleaned), &out); err != nil {
		// Fallback: If model returned plain text rather than structured JSON,
		// wrap the text into the explanation field so information isn't lost.
		return &AnalysisOutput{
			Explanation: cleaned,
			Impact:      "Consult the explanation above for potential consumer impact.",
			Remediations: []string{
				"Restore the documented contract or update the OpenAPI specification.",
			},
		}, nil
	}

	return &out, nil
}
