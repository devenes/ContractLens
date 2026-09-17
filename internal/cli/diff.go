package cli

import (
	"context"
	"flag"
	"fmt"
	"io"
	"strings"

	"github.com/contractlens/contractlens/internal/analyzer"
	"github.com/contractlens/contractlens/internal/openapi"
	"github.com/contractlens/contractlens/internal/report"
)

// RunDiff executes the deterministic contract comparison without requiring AI or network access.
func RunDiff(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("diff", flag.ContinueOnError)
	fs.SetOutput(stderr)

	var specPath string
	var responsePath string
	var endpoint string
	var statusCode string
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
	fs.StringVar(&format, "format", "text", "Output format: text (default) or json")
	fs.StringVar(&format, "f", "text", "Output format (shorthand)")
	fs.BoolVar(&exitCode, "exit-code", false, "Exit with code 1 if breaking changes are detected")

	fs.Usage = func() {
		fmt.Fprintf(stderr, `Usage: contractlens diff --spec <path> --response <path> [options]

Deterministic API contract drift detector (runs completely offline without an LLM).

Options:
  -s, --spec <path>        Path to OpenAPI specification file (required)
  -r, --response <path>    Path to observed JSON API response file (required)
  -e, --endpoint <op>      Target operation (e.g. "GET /users/{id}"). Auto-detected if single operation in spec
  -c, --status <code >     Expected HTTP status code (default: 200)
  -f, --format <format>    Output format: text (default) or json
      --exit-code          Exit with code 1 if any breaking drift is detected (useful for CI)
  -h, --help               Show help for diff command
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

	// 1. Load and validate OpenAPI document
	doc, err := openapi.Load(ctx, specPath)
	if err != nil {
		fmt.Fprintf(stderr, "Error: %v\n", err)
		return 1
	}

	// 2. Resolve operation
	method, path, op, err := doc.FindOperation(endpoint)
	if err != nil {
		fmt.Fprintf(stderr, "Error: %v\n", err)
		return 1
	}

	// 3. Resolve endpoint response schema
	normStatus := openapi.FormatStatusCode(statusCode)
	endpointSchema, err := doc.ResolveEndpointSchema(method, path, op, normStatus)
	if err != nil {
		fmt.Fprintf(stderr, "Error: %v\n", err)
		return 1
	}

	// 4. Decode observed JSON response
	observedJSON, err := analyzer.DecodeJSONFile(responsePath)
	if err != nil {
		fmt.Fprintf(stderr, "Error: %v\n", err)
		return 1
	}

	// 5. Deterministic comparison
	opKey := fmt.Sprintf("%s %s", method, path)
	result := analyzer.Compare(opKey, normStatus, endpointSchema.JSONSchema, observedJSON)

	// 6. Generate report
	rep := report.NewDiffReport(specPath, responsePath, result)

	// 7. Render output
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
