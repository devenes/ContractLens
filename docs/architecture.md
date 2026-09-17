# ContractLens Architecture

ContractLens is a developer CLI tool designed to detect and remediate API contract drift between OpenAPI specifications and observed JSON API responses.

## Core Design Principle: Deterministic Facts > LLM Guesses

In conventional AI-augmented tooling, a common pitfall is asking the LLM to inspect raw schemas and JSON payloads directly:

> *"Compare this OpenAPI document and JSON response and tell me what is wrong."*

This approach introduces nondeterminism, hallucinated schema differences, missed type mismatches, and high token costs.

ContractLens enforces an architectural boundary: **The LLM is NEVER responsible for detecting raw schema differences.**

Deterministic Go code performs strict, rule-based schema verification. The AI agent acts strictly as an **explainer, impact assessor, and remediation advisor** grounded upon verified facts.

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
                                     v
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

---

## 1. OpenAPI Parser (`internal/openapi`)

- **Library**: `github.com/getkin/kin-openapi v0.149.0`
- **Capabilities**:
  - Full OpenAPI 3.0 and 3.1 specification support.
  - Automatic circular and file-based `$ref` pointer resolution.
  - Document validation checking parameter rules and schema syntax.
  - Operation resolution by `"METHOD /path"` (e.g. `GET /users/{id}`), `operationId`, or automatic detection when a single operation exists.
  - Media type resolution targeting `application/json` (with fallback to `*/*`).

---

## 2. Deterministic JSON Type Inference (`internal/analyzer`)

Go's default `json.Unmarshal` unmarshals all numbers into `float64`, which risks losing integer precision and confounding `integer` vs `number` validations.

ContractLens decodes payloads with `json.NewDecoder` and `UseNumber()`:
- **`string`**: JSON string literals.
- **`boolean`**: JSON `true` / `false`.
- **`null`**: JSON `null`.
- **`integer`**: A numeric string containing no decimal point (`.`) or exponent (`e`/`E`) that successfully parses into a signed 64-bit integer (`int64`).
- **`number`**: A numeric string containing a decimal point or exponent, or any real number.
- **`object`**: JSON `{...}` decoded into `map[string]any`.
- **`array`**: JSON `[...]` decoded into `[]any`.

### Type Compatibility Rule
- An `integer` satisfies an expected OpenAPI `number` (all integers are real numbers).
- A floating-point `number` does not satisfy an expected OpenAPI `integer`.

---

## 3. Drift Classification and Severity

Findings are strictly classified into deterministic categories:

| Finding Kind | Condition | Breaking? | Severity |
| :--- | :--- | :---: | :---: |
| `required_field_missing` | Property in `schema.Required` is missing from JSON object | **Yes** | **HIGH** |
| `type_mismatch` | Runtime JSON type does not match expected OpenAPI type | **Yes** | **HIGH** |
| `enum_mismatch` | Runtime value is not in `schema.Enum` list | **Yes** | **HIGH** |
| `nullable_mismatch` | Value is `null` but `!schema.PermitsNull()` (required field) | **Yes** | **HIGH** |
| `nullable_mismatch` | Value is `null` but `!schema.PermitsNull()` (optional field) | No | **MEDIUM** |
| `unexpected_field` | Field in JSON is not documented, with `additionalProperties: false` | No | **MEDIUM** |
| `unexpected_field` | Field in JSON is not documented (relaxed) | No | **LOW** |
| `documented_field_missing` | Documented optional property is absent from response | No | **LOW** |

### Overall Risk Scoring
- **HIGH**: If any finding is breaking or classified as HIGH severity.
- **MEDIUM**: If any finding is classified as MEDIUM severity and none are HIGH.
- **LOW**: If findings contain only informational LOW items.
- **NONE**: Zero drift detected.

---

## 4. Google ADK Agent (`internal/agent`)

- **Module**: `google.golang.org/adk/v2 v2.4.0`
- **Model**: Gemini (default: `gemini-flash-latest`, configurable via `CONTRACTLENS_MODEL` or `--model`).
- **Runtime**: `runner.NewInMemory` provides an isolated turn-based session for executing the `llmagent.New` analyst.
- **Role**:
  - The agent is prompted with authoritative JSON containing the verified findings.
  - The system prompt forbids inventing endpoints, hallucinating fields, or guessing unstated internal architectures.
  - Calibrated language is enforced ("may affect", "could break", "likely impact").
  - Output is formatted as structured JSON: `explanation`, `impact`, and `remediations`.

---

## 5. Offline-First CLI (`internal/cli`)

- `contractlens diff`: Runs 100% offline without Gemini, Google Cloud, or network access. Suitable for pre-commit hooks and air-gapped CI.
- `contractlens diff --format json`: Emits machine-readable JSON with a top-level `"breaking": true|false` flag for CI exit gates.
- `contractlens analyze`: Extends the deterministic diff with AI impact assessment and remediation steps using Google ADK.
