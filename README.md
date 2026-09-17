# ContractLens

**API contract drift detection with Go + Google ADK + Gemini.**

[![CI](https://github.com/devenes/ContractLens/actions/workflows/ci.yml/badge.svg)](https://github.com/devenes/ContractLens/actions)
[![Security Scan](https://github.com/devenes/ContractLens/actions/workflows/security.yml/badge.svg)](https://github.com/devenes/ContractLens/actions)
[![License](https://img.shields.io/badge/License-Apache%202.0-blue.svg)](LICENSE)
[![Go Version](https://img.shields.io/badge/Go-1.25%2B-00ADD8.svg)](https://golang.org)

ContractLens detects schema differences between an OpenAPI specification and an observed JSON API response deterministically, then uses an AI agent powered by **Google Agent Development Kit (ADK) for Go** and **Gemini** to explain verified findings, assess consumer impact, and recommend actionable remediation.

---

## Architecture

```
                      +-----------------------------+
                      |     OpenAPI Spec (YAML)     |
                      +-----------------------------+
                                     |
                                     v
                      +-----------------------------+
                      |    OpenAPI Parser (Go)      |
                      |   (kin-openapi v0.149.0)    |
                      +-----------------------------+
                                     |
                                     v
                      +-----------------------------+
                      |   Normalized Operation &    |
                      |       Response Schema       |
                      +-----------------------------+
                                     |
+---------------------+              |
| Observed JSON Resp  |              |
+---------------------+              |
           |                         |
           v                         v
+---------------------+  +--------------------------+
| JSON Type Inference |->| Deterministic Comparator |
+---------------------+  |    (Strict Go Engine)    |
                         +--------------------------+
                                     |
                         +--------------------------+
                         |    Verified Findings     |
                         |  (Breaking / Severity)   |
                         +--------------------------+
                                     |
                 +-------------------+-------------------+
                 | (`diff` mode)                         | (`analyze` mode)
                 v                                       v
     +-----------------------+              +--------------------------+
     | Direct Terminal /     |              |     Google ADK Agent     |
     | JSON Findings Report  |              |  (llmagent + Gemini)     |
     +-----------------------+              +--------------------------+
                                                         |
                                                         v
                                            +--------------------------+
                                            | Structured Report with   |
                                            | AI Explanation & Fixes   |
                                            +--------------------------+
```

### Why Deterministic Facts > LLM Guesses

In conventional AI tooling, prompts frequently ask the model to:

> *"Compare this OpenAPI spec and JSON payload and tell me what is wrong."*

This approach causes hallucinations, missed field mismatches, non-reproducible CI results, and unnecessary token usage.

ContractLens enforces a strict architectural separation:
1. **Go performs deterministic analysis**: Missing required fields, type mismatches, enum violations, unexpected properties, and nullable mismatches are verified using strict Go code and `kin-openapi`.
2. **Google ADK Agent reasons over verified facts**: The AI agent receives structured, machine-verified findings. It explains the technical changes, assesses likely downstream impact on client SDKs and consumers, and recommends remediation steps.

The agent is forbidden from inventing findings or hallucinating API behavior.

---

## Quickstart

### 1. Clone and Build

```bash
git clone https://github.com/devenes/ContractLens.git
cd ContractLens

go mod download
go build -o bin/contractlens ./cmd/contractlens
```

### 2. Run Deterministic Drift Detection (100% Offline)

`diff` runs completely locally without an API key, network access, or cloud services:

```bash
./bin/contractlens diff \
  --spec examples/openapi.yaml \
  --response examples/response-breaking.json
```

**Output:**

```text
ContractLens
API Contract Drift Detector

Endpoint: GET /users/{id} (HTTP 200)

HIGH RISK (1 breaking change(s))

✗ id
  Expected: string
  Actual:   integer
  Actual Value:   99812
  Type mismatch — expected string, got integer — breaking

⚠ profile
  Expected: object
  Documented optional field is missing from response (optional)

Recommendations
----------------

• Restore the documented type/values or update the OpenAPI contract if the change is intentional.
• Regenerate client SDKs and verify downstream consumers after updating the contract.
• Confirm whether missing optional fields are expected under this scenario.
```

### 3. Run AI Explanation with Google ADK + Gemini

To enable the AI explanation layer:

```bash
export GOOGLE_API_KEY="your_api_key_here"

./bin/contractlens analyze \
  --spec examples/openapi.yaml \
  --response examples/response-breaking.json
```

**Output:**

```text
ContractLens
API Contract Drift Detector

Endpoint: GET /users/{id} (HTTP 200)

HIGH RISK (1 breaking change(s))

✗ id
  Expected: string
  Actual:   integer
  Actual Value:   99812
  Type mismatch — expected string, got integer — breaking

⚠ profile
  Expected: object
  Documented optional field is missing from response (optional)

AI Analysis
------------

The `id` field in the response was returned as an integer (99812) instead of the documented string type.
In addition, the optional `profile` object was omitted.

Impact Assessment:
Generated SDK clients and statically-typed consumers (e.g. Go, TypeScript, Java) expecting a string `id`
may encounter JSON deserialization errors or runtime type exceptions when parsing this response.

Recommended action:
• Restore string serialization for `id` in the backend API response handler.
• If `id` was intentionally migrated to an integer, update the OpenAPI contract and regenerate downstream client SDKs.
```

---

## Machine-Readable JSON Output & CI Integration

ContractLens supports stable, indented JSON output formatted specifically for CI pipelines:

```bash
./bin/contractlens diff \
  --spec examples/openapi.yaml \
  --response examples/response-breaking.json \
  --format json
```

**JSON Output:**

```json
{
  "endpoint": "GET /users/{id}",
  "status": "200",
  "summary": "The response contains 1 breaking issue(s) and 1 non-breaking issue(s).",
  "risk": "high",
  "breaking": true,
  "spec_file": "examples/openapi.yaml",
  "response_file": "examples/response-breaking.json",
  "findings": [
    {
      "path": "id",
      "kind": "type_mismatch",
      "expected_type": "string",
      "actual_type": "integer",
      "actual_value": 99812,
      "breaking": true,
      "severity": "high",
      "message": "Type mismatch — expected string, got integer"
    },
    {
      "path": "profile",
      "kind": "documented_field_missing",
      "expected_type": "object",
      "breaking": false,
      "severity": "low",
      "message": "Documented optional field is missing from response"
    }
  ],
  "recommendations": [
    "Restore the documented type/values or update the OpenAPI contract if the change is intentional.",
    "Regenerate client SDKs and verify downstream consumers after updating the contract.",
    "Confirm whether missing optional fields are expected under this scenario."
  ]
}
```

### CI Breaking-Change Gate

You can easily gate pull requests or integration test suites:

```bash
# Option A: Built-in --exit-code flag (exits 1 if breaking changes exist)
./bin/contractlens diff \
  --spec openapi.yaml \
  --response response.json \
  --exit-code

# Option B: Pipe JSON to jq
if ./bin/contractlens diff --spec openapi.yaml --response response.json --format json | jq -e '.breaking == true' > /dev/null; then
  echo "Contract drift contains breaking changes!"
  exit 1
fi
```

---

## Drift Classification Rules

| Finding Kind | Condition | Breaking | Severity |
| :--- | :--- | :---: | :---: |
| `required_field_missing` | Property in OpenAPI `required` is missing from JSON | **Yes** | **HIGH** |
| `type_mismatch` | Actual JSON type conflicts with OpenAPI schema type | **Yes** | **HIGH** |
| `enum_mismatch` | Observed value is outside documented OpenAPI enum | **Yes** | **HIGH** |
| `nullable_mismatch` | Value is `null` but OpenAPI schema does not permit null | **Yes** (if required) | **HIGH / MEDIUM** |
| `unexpected_field` | Field in JSON is not documented (strict / relaxed) | No | **MEDIUM / LOW** |
| `documented_field_missing` | Documented optional field is absent from response | No | **LOW** |

---

## Configuration

| Variable / Flag | Description | Default |
| :--- | :--- | :--- |
| `GOOGLE_API_KEY` | Google Gemini API key (required for `analyze`) | *None* |
| `GEMINI_API_KEY` | Fallback Gemini API key variable | *None* |
| `CONTRACTLENS_MODEL` | Gemini model name | `gemini-flash-latest` |
| `--model`, `-m` | CLI flag to override the Gemini model | `gemini-flash-latest` |
| `--endpoint`, `-e` | Target operation (e.g. `"GET /users/{id}"`) | *Auto-detected if single* |
| `--status`, `-c` | Target HTTP status code | `200` |
| `--format`, `-f` | Output format: `text` or `json` | `text` |
| `--exit-code` | Exit with status 1 on breaking drift | `false` |

---

## Running with Docker

You can run ContractLens without installing Go using Docker:

```bash
# Build the minimal non-root image
docker build -t contractlens .

# Run deterministic diff
docker run --rm \
  -v "$(pwd)/examples:/data:ro" \
  contractlens diff \
  --spec /data/openapi.yaml \
  --response /data/response-breaking.json

# Run AI analyze
docker run --rm \
  -e GOOGLE_API_KEY="$GOOGLE_API_KEY" \
  -v "$(pwd)/examples:/data:ro" \
  contractlens analyze \
  --spec /data/openapi.yaml \
  --response /data/response-breaking.json
```

---

## Testing

The project maintains 100% offline unit tests for core comparison, schema resolution, and agent orchestration.

```bash
# Run all unit tests with race detection
go test -v -race ./...

# Run static analysis / vet
go vet ./...

# Check code formatting
gofmt -l .

# Run vulnerability scan
go run golang.org/x/vuln/cmd/govulncheck@latest ./...
```

To run the optional live integration test against Gemini:

```bash
export GOOGLE_API_KEY="your_api_key_here"
export LIVE_GEMINI_TEST="true"
go test -v ./internal/agent -run TestLiveGeminiIntegration
```

## Contributing

Please read [CONTRIBUTING.md](CONTRIBUTING.md) for details on code style, testing requirements, and the pull request process.

## Security

Please report vulnerabilities responsibly according to our [SECURITY.md](SECURITY.md).

## License

ContractLens is open-source software licensed under the [Apache-2.0 License](LICENSE).
