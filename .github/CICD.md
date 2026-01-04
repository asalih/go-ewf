# CI/CD Pipeline Documentation

This document describes the continuous integration and deployment pipeline for the go-ewf project.

## Overview

The CI/CD pipeline is implemented using GitHub Actions and runs automatically on every push and pull request to the main branches.

## Workflows

### 1. Test Workflow (`.github/workflows/test.yml`)

The main testing workflow that ensures code quality and functionality across different platforms and Go versions.

#### Jobs

##### Test Job
- **Runs on**: Ubuntu, macOS, Windows
- **Go versions**: 1.21, 1.22, 1.23
- **Steps**:
  1. Checkout code
  2. Set up Go with caching
  3. Download and verify dependencies
  4. Run tests with race detection and coverage
  5. Upload coverage to Codecov (Ubuntu + Go 1.23 only)

**Test flags**:
- `-v`: Verbose output
- `-race`: Enable race detector
- `-timeout 10m`: Set timeout to 10 minutes
- `-coverprofile=coverage.out`: Generate coverage report
- `-covermode=atomic`: Use atomic coverage mode for race detector compatibility

##### Benchmark Job
- **Runs on**: Ubuntu latest
- **Go version**: 1.23
- **Steps**:
  1. Checkout code
  2. Set up Go
  3. Run benchmarks with memory profiling

**Benchmark flags**:
- `-bench=.`: Run all benchmarks
- `-benchtime=1s`: Run each benchmark for 1 second
- `-benchmem`: Show memory allocation statistics
- `-run=^$`: Don't run any tests, only benchmarks

##### Lint Job
- **Runs on**: Ubuntu latest
- **Go version**: 1.23
- **Steps**:
  1. Checkout code
  2. Set up Go
  3. Run golangci-lint with configured rules

**Linter timeout**: 5 minutes

##### Build Job
- **Runs on**: Ubuntu, macOS, Windows
- **Go version**: 1.23
- **Steps**:
  1. Checkout code
  2. Set up Go
  3. Build all packages
  4. Build CLI tool

## Triggers

The workflows are triggered on:

```yaml
on:
  push:
    branches: [ main, master, develop ]
  pull_request:
    branches: [ main, master, develop ]
```

This means:
- Every push to `main`, `master`, or `develop` branches
- Every pull request targeting these branches

## Test Matrix

The test job uses a matrix strategy to test across:

| OS | Go Version |
|---|---|
| Ubuntu Latest | 1.21 |
| Ubuntu Latest | 1.22 |
| Ubuntu Latest | 1.23 |
| macOS Latest | 1.21 |
| macOS Latest | 1.22 |
| macOS Latest | 1.23 |
| Windows Latest | 1.21 |
| Windows Latest | 1.22 |
| Windows Latest | 1.23 |

**Total test configurations**: 9 (3 OS × 3 Go versions)

## Code Coverage

Code coverage is collected on every test run and uploaded to Codecov for the primary configuration (Ubuntu + Go 1.23).

To view coverage locally:
```bash
go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out
```

## Linting Configuration

The project uses golangci-lint with the following enabled linters:

- `gofmt`: Check code formatting
- `govet`: Examine Go source code and report suspicious constructs
- `errcheck`: Check for unchecked errors
- `staticcheck`: Advanced Go linter
- `unused`: Check for unused code
- `gosimple`: Suggest code simplifications
- `ineffassign`: Detect ineffectual assignments
- `typecheck`: Type-check Go code
- `exportloopref`: Check for loop variable capture issues
- `gocyclo`: Check cyclomatic complexity (threshold: 15)
- `misspell`: Check for commonly misspelled English words
- `unparam`: Check for unused function parameters
- `unconvert`: Check for unnecessary type conversions

Configuration file: `.golangci.yml`

## Dependabot

Dependabot is configured to automatically create pull requests for:

1. **Go module dependencies**
   - Schedule: Weekly
   - Max open PRs: 10
   
2. **GitHub Actions versions**
   - Schedule: Weekly
   - Max open PRs: 5

Configuration file: `.github/dependabot.yml`

## Badges

The following badges are available in the README:

1. **Tests Badge**: Shows the status of the latest test run
   ```markdown
   [![Tests](https://github.com/asalih/go-ewf/actions/workflows/test.yml/badge.svg)](https://github.com/asalih/go-ewf/actions/workflows/test.yml)
   ```

2. **Go Report Card**: Code quality grade
   ```markdown
   [![Go Report Card](https://goreportcard.com/badge/github.com/asalih/go-ewf)](https://goreportcard.com/report/github.com/asalih/go-ewf)
   ```

3. **Go Reference**: Documentation link
   ```markdown
   [![Go Reference](https://pkg.go.dev/badge/github.com/asalih/go-ewf.svg)](https://pkg.go.dev/github.com/asalih/go-ewf)
   ```

## Running Locally

### Run all checks like CI:

```bash
# 1. Run tests with race detection and coverage
go test -v -race -timeout 10m -coverprofile=coverage.out -covermode=atomic ./...

# 2. Run benchmarks
go test -bench=. -benchtime=1s -benchmem -run=^$ ./...

# 3. Run linter
golangci-lint run --timeout=5m

# 4. Build everything
go build -v ./...
go build -v ./cmd/...
```

### Install tools locally:

```bash
# Install golangci-lint
go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
```

## Codecov Integration (Optional)

To enable Codecov integration:

1. Sign up at https://codecov.io with your GitHub account
2. Add the repository to Codecov
3. Add the `CODECOV_TOKEN` secret to your GitHub repository:
   - Go to Settings → Secrets and variables → Actions
   - Add new repository secret: `CODECOV_TOKEN`
   - Paste the token from Codecov

If you don't want Codecov, the tests will still run successfully (the upload step fails gracefully with `fail_ci_if_error: false`).

## Troubleshooting

### Tests fail locally but pass in CI
- Ensure you're using the same Go version as CI
- Check for OS-specific issues
- Run with race detector: `go test -race ./...`

### Linter errors
- Run locally: `golangci-lint run`
- Fix automatically where possible: `golangci-lint run --fix`
- Check configuration in `.golangci.yml`

### Build failures
- Ensure all dependencies are downloaded: `go mod download`
- Verify modules: `go mod verify`
- Tidy modules: `go mod tidy`

## Best Practices

1. **Always run tests locally** before pushing
2. **Keep dependencies up to date** via Dependabot PRs
3. **Fix linter warnings** - they often catch real bugs
4. **Review coverage reports** to identify untested code
5. **Check benchmark results** to avoid performance regressions

## Future Enhancements

Potential improvements to the CI/CD pipeline:

- [ ] Add security scanning (e.g., gosec, trivy)
- [ ] Add integration test stage with real EWF files
- [ ] Create release workflow for automated versioning
- [ ] Add performance comparison against previous benchmarks
- [ ] Generate and publish test reports as artifacts
- [ ] Add code quality gates (minimum coverage threshold)

