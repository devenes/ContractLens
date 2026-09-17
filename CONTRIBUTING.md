# Contributing to ContractLens

Thank you for your interest in contributing to ContractLens!

## Code of Conduct

We are committed to providing a welcoming, productive, and inclusive environment for everyone. Please be respectful and constructive in all interactions.

## Development Setup

### Prerequisites
- Go 1.25 or higher
- Git

### Local Workflow

1. Fork and clone the repository:
   ```bash
   git clone https://github.com/contractlens/contractlens.git
   cd contractlens
   ```

2. Download dependencies:
   ```bash
   go mod download
   ```

3. Run the test suite:
   ```bash
   go test -v -race ./...
   ```

4. Format and vet your code:
   ```bash
   gofmt -w .
   go vet ./...
   ```

5. Build the CLI locally:
   ```bash
   go build -o bin/contractlens ./cmd/contractlens
   ```

## Pull Request Guidelines

- Ensure all existing and new unit tests pass (`go test -race ./...`).
- Verify code formatting with `gofmt -l .` (no files should be listed).
- Add clear unit tests for new functionality or bug fixes.
- Keep PRs focused on a single change or feature.
- Follow modern, idiomatic Go conventions:
  - Explicit error handling and error wrapping (`%w`).
  - Small, focused functions.
  - Zero ungrounded AI assumptions: deterministic checks must remain deterministic in Go.
