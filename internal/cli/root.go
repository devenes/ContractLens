package cli

import (
	"context"
	"fmt"
	"io"
)

// RootUsage prints the global CLI help text.
func RootUsage(w io.Writer) {
	fmt.Fprintf(w, `ContractLens — AI-Powered API Contract Drift Detector
Built with Go + Google Agent Development Kit (ADK) + Gemini

Usage:
  contractlens <command> [flags]

Available Commands:
  diff        Detect contract drift deterministically (100%% offline, no API key required)
  analyze     Detect contract drift and explain impact/fixes with Google ADK + Gemini
  version     Display version information
  help        Help about any command

Run 'contractlens <command> --help' for more information about a specific command.
`)
}

// Execute parses top-level arguments and dispatches to the matching subcommand.
func Execute(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		RootUsage(stderr)
		return 1
	}

	cmd := args[0]
	rest := args[1:]

	switch cmd {
	case "diff":
		return RunDiff(ctx, rest, stdout, stderr)

	case "analyze":
		return RunAnalyze(ctx, rest, stdout, stderr)

	case "version", "-v", "--version":
		return RunVersion(stdout)

	case "help", "-h", "--help":
		RootUsage(stdout)
		return 0

	default:
		fmt.Fprintf(stderr, "Error: unknown command %q for \"contractlens\"\n\n", cmd)
		RootUsage(stderr)
		return 1
	}
}
