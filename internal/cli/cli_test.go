package cli_test

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/contractlens/contractlens/internal/cli"
)

func findProjectRoot(t *testing.T) string {
	t.Helper()
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get working dir: %v", err)
	}
	// Walk up to find go.mod
	dir := wd
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatalf("could not locate go.mod from %s", wd)
		}
		dir = parent
	}
}

func TestRunVersion(t *testing.T) {
	var buf bytes.Buffer
	code := cli.RunVersion(&buf)
	if code != 0 {
		t.Errorf("expected exit code 0, got %d", code)
	}
	if !strings.Contains(buf.String(), "ContractLens") {
		t.Errorf("expected output to contain ContractLens, got %q", buf.String())
	}
}

func TestExecute_RootUsage(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := cli.Execute(context.Background(), []string{"help"}, &stdout, &stderr)
	if code != 0 {
		t.Errorf("expected exit code 0 for help, got %d", code)
	}
	if !strings.Contains(stdout.String(), "ContractLens") {
		t.Errorf("expected help output to mention ContractLens, got:\n%s", stdout.String())
	}
}

func TestDiff_ValidResponse(t *testing.T) {
	root := findProjectRoot(t)
	specPath := filepath.Join(root, "examples", "openapi.yaml")
	respPath := filepath.Join(root, "examples", "response-valid.json")

	var stdout, stderr bytes.Buffer
	code := cli.Execute(context.Background(), []string{
		"diff",
		"--spec", specPath,
		"--response", respPath,
	}, &stdout, &stderr)

	if code != 0 {
		t.Fatalf("expected code 0, got %d. stderr: %s", code, stderr.String())
	}

	out := stdout.String()
	if !strings.Contains(out, "NO RISK") {
		t.Errorf("expected output to contain 'NO RISK', got:\n%s", out)
	}
	if !strings.Contains(out, "Contract matches") {
		t.Errorf("expected match confirmation, got:\n%s", out)
	}
}

func TestDiff_BreakingResponse_Text(t *testing.T) {
	root := findProjectRoot(t)
	specPath := filepath.Join(root, "examples", "openapi.yaml")
	respPath := filepath.Join(root, "examples", "response-breaking.json")

	var stdout, stderr bytes.Buffer
	code := cli.Execute(context.Background(), []string{
		"diff",
		"--spec", specPath,
		"--response", respPath,
	}, &stdout, &stderr)

	if code != 0 {
		t.Fatalf("expected code 0 without --exit-code, got %d. stderr: %s", code, stderr.String())
	}

	out := stdout.String()
	if !strings.Contains(out, "HIGH RISK") {
		t.Errorf("expected output to contain 'HIGH RISK', got:\n%s", out)
	}
	if !strings.Contains(out, "✗ id") {
		t.Errorf("expected output to mention ✗ id, got:\n%s", out)
	}
}

func TestDiff_BreakingResponse_ExitCode(t *testing.T) {
	root := findProjectRoot(t)
	specPath := filepath.Join(root, "examples", "openapi.yaml")
	respPath := filepath.Join(root, "examples", "response-breaking.json")

	var stdout, stderr bytes.Buffer
	code := cli.Execute(context.Background(), []string{
		"diff",
		"--spec", specPath,
		"--response", respPath,
		"--exit-code",
	}, &stdout, &stderr)

	if code != 1 {
		t.Fatalf("expected exit code 1 with --exit-code on breaking response, got %d", code)
	}
}

func TestDiff_FormatJSON(t *testing.T) {
	root := findProjectRoot(t)
	specPath := filepath.Join(root, "examples", "openapi.yaml")
	respPath := filepath.Join(root, "examples", "response-breaking.json")

	var stdout, stderr bytes.Buffer
	code := cli.Execute(context.Background(), []string{
		"diff",
		"--spec", specPath,
		"--response", respPath,
		"--format", "json",
	}, &stdout, &stderr)

	if code != 0 {
		t.Fatalf("expected code 0, got %d. stderr: %s", code, stderr.String())
	}

	var reportMap map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &reportMap); err != nil {
		t.Fatalf("invalid JSON output: %v. Raw:\n%s", err, stdout.String())
	}

	if reportMap["breaking"] != true {
		t.Errorf("expected breaking=true in JSON, got %v", reportMap["breaking"])
	}
	if reportMap["risk"] != "high" {
		t.Errorf("expected risk=high in JSON, got %v", reportMap["risk"])
	}
}

func TestDiff_MissingFlags(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := cli.Execute(context.Background(), []string{"diff"}, &stdout, &stderr)
	if code != 1 {
		t.Errorf("expected exit code 1 for missing flags, got %d", code)
	}
	if !strings.Contains(stderr.String(), "--spec is required") {
		t.Errorf("expected error about --spec, got: %s", stderr.String())
	}
}

func TestAnalyze_MissingAPIKey(t *testing.T) {
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

	root := findProjectRoot(t)
	specPath := filepath.Join(root, "examples", "openapi.yaml")
	respPath := filepath.Join(root, "examples", "response-breaking.json")

	var stdout, stderr bytes.Buffer
	code := cli.Execute(context.Background(), []string{
		"analyze",
		"--spec", specPath,
		"--response", respPath,
	}, &stdout, &stderr)

	if code != 1 {
		t.Fatalf("expected exit code 1 without API key, got %d", code)
	}

	if !strings.Contains(stderr.String(), "GOOGLE_API_KEY is required for `analyze`") {
		t.Errorf("expected error message to mention GOOGLE_API_KEY, got:\n%s", stderr.String())
	}
}
