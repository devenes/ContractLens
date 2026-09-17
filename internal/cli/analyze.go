package cli

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/contractlens/contractlens/internal/agent"
	"github.com/contractlens/contractlens/internal/analyzer"
	"github.com/contractlens/contractlens/internal/openapi"
	"github.com/contractlens/contractlens/internal/report"
)

// RunAnalyze executes deterministic drift analysis and invokes the Google ADK agent
// to explain findings, assess impact, and recommend remediation.
func RunAnalyze(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("analyze", flag.ContinueOnError)
	fs.SetOutput(stderr)

	var specPath string
	var responsePath string
	var endpoint string
	var statusCode string
	var modelName string
	var format string
	var exitCode bool

	fs.StringVar(&specPath, "spec", "", "Path to OpenAPI specification (YAML or JSON)")
	fs.StringVar(&specPath, "s", "", "Path to OpenAPI specification (shorthand)")
	fs.StringVar(&responsePath, "response", "", "Path to observed JSON API response")
	fs.StringVar(&responsePath, "r", "", "Path to observed JSON API response (shorthand)")
	fs.StringVar(&endpoint, "endpoint", "", "Target operation (e.g. \"GET /users/{id}\")")
	fs.StringVar(&endpoint, "e", "", "Target operation (shorthand)")
	fs.StringVar(&statusCode, "status", "200", "Expected HTTP status code (default: 200)")
	fs.StringVar(&statusCode, "c", "200", "Expected HTTP status code (shorthand)")
	fs.StringVar(&modelName, "model", "", "Gemini model (default: gemini-flash-latest, or CONTRACTLENS_MODEL)")
	fs.StringVar(&modelName, "m", "", "Gemini model (shorthand)")
	fs.StringVar(&format, "format", "text", "Output format: text (default) or json")
	fs.StringVar(&format, "f", "text", "Output format (shorthand)")
	fs.BoolVar(&exitCode, "exit-code", false, "Exit with code 1 if breaking changes are detected")

	fs.Usage = func() {
		fmt.Fprintf(stderr, `Usage: contractlens analyze --spec <path> --response <path> [options]

Detect API contract drift deterministically and explain findings with Google ADK + Gemini.

Options:
  -s, --spec <path>        Path to OpenAPI specification file (required)
  -r, --response <path>    Path to observed JSON API response file (required)
  -e, --endpoint <op>      Target operation (e.g. "GET /users/{id}"). Auto-detected if single operation in spec
  -c, --status <code>      Expected HTTP status code (default: 200)
  -m, --model <model>      Gemini model to use (default: gemini-flash-latest, or CONTRACTLENS_MODEL)
  -f, --format <format>    Output format: text (default) or json
      --exit-code          Exit with code 1 if any breaking drift is detected
  -h, --help               Show help for analyze command

Environment Variables:
  GOOGLE_API_KEY           Google Gemini API key (required for analyze)
  CONTRACTLENS_MODEL       Default Gemini model override
`)
	}

	if err := fs.Parse(args); err != nil {
		if err == flag.ErrHelp {
			return 0
		}
		return 1
	}

	if specPath == "" {
		fmt.Fprintf(stderr, "Error: --spec is required\n\n")
		fs.Usage()
		return 1
	}
	if responsePath == "" {
		fmt.Fprintf(stderr, "Error: --response is required\n\n")
		fs.Usage()
		return 1
	}

	// 1. Verify credentials upfront before performing work
	apiKey := os.Getenv("GOOGLE_API_KEY")
	if apiKey == "" {
		apiKey = os.Getenv("GEMINI_API_KEY")
	}
	if apiKey == "" {
		fmt.Fprintf(stderr, "Error: GOOGLE_API_KEY is required for `analyze`.\nUse `diff` for deterministic analysis without an API key.\n")
		return 1
	}

	// 2. Load and validate OpenAPI document
	doc, err := openapi.Load(ctx, specPath)
	if err != nil {
		fmt.Fprintf(stderr, "Error: %v\n", err)
		return 1
	}

	// 3. Resolve operation
	method, path, op, err := doc.FindOperation(endpoint)
	if err != nil {
		fmt.Fprintf(stderr, "Error: %v\n", err)
		return 1
	}

	// 4. Resolve endpoint response schema
	normStatus := openapi.FormatStatusCode(statusCode)
	endpointSchema, err := doc.ResolveEndpointSchema(method, path, op, normStatus)
	if err != nil {
		fmt.Fprintf(stderr, "Error: %v\n", err)
		return 1
	}

	// 5. Decode observed JSON response
	observedJSON, err := analyzer.DecodeJSONFile(responsePath)
	if err != nil {
		fmt.Fprintf(stderr, "Error: %v\n", err)
		return 1
	}

	// 6. Deterministic comparison
	opKey := fmt.Sprintf("%s %s", method, path)
	diffResult := analyzer.Compare(opKey, normStatus, endpointSchema.JSONSchema, observedJSON)

	// 7. Initialize ADK agent and run analysis
	ag, err := agent.New(ctx, agent.Config{
		ModelName: modelName,
		APIKey:    apiKey,
	})
	if err != nil {
		fmt.Fprintf(stderr, "Error initializing ADK agent: %v\n", err)
		return 1
	}

	agentInput := agent.AnalysisInput{
		Endpoint: opKey,
		Status:   normStatus,
		Risk:     diffResult.Risk,
		Breaking: diffResult.Breaking,
		Findings: diffResult.Findings,
	}

	aiOut, err := ag.Analyze(ctx, agentInput)
	if err != nil {
		fmt.Fprintf(stderr, "Error during AI analysis: %v\n", err)
		return 1
	}

	// 8. Generate composite report
	aiReport := &report.AIReport{
		Explanation:  aiOut.Explanation,
		Impact:       aiOut.Impact,
		Remediations: aiOut.Remediations,
	}
	rep := report.NewAnalyzeReport(specPath, responsePath, diffResult, aiReport)

	// 9. Render output
	if strings.ToLower(format) == "json" {
		data, err := report.FormatJSON(rep)
		if err != nil {
			fmt.Fprintf(stderr, "Error formatting JSON report: %v\n", err)
			return 1
		}
		fmt.Fprintln(stdout, string(data))
	} else {
		text := report.FormatText(rep)
		fmt.Fprint(stdout, text)
	}

	if exitCode && rep.Breaking {
		return 1
	}

	return 0
}
