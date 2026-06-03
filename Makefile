.PHONY: help build test lint fmt update clean install demo check update test-coverage validate-cli validate-spec dev-deps vuln sec release release-dry-run

# Binary name
BINARY_NAME := go-bricks-openapi
VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")

# Pinned scanner versions — identical to CI so the local release gate matches.
GOVULNCHECK_VERSION := v1.1.4
GOSEC_VERSION := v2.26.1

# Pinned redocly CLI version for the structural-validation gate. Pinned (not
# @latest) so an upstream release cannot silently change the gate or break CI.
REDOCLY_VERSION := 2.31.5
# Fixture project the spec gate generates from, plus a throwaway output path.
# nested_schema is the richest fixture (recursive $refs, nested + sliced structs),
# so redocly validates the hardest case the generator produces.
SPEC_FIXTURE := internal/spectest/testdata/nested_schema
SPEC_TMP := $(CURDIR)/.openapi-fixture-spec.yaml

# Default target
help: ## Show this help message
	@echo "go-bricks-openapi tool commands:"
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "  %-15s %s\n", $$1, $$2}'

build: ## Build the CLI tool
	go build -ldflags "-X main.version=$(VERSION)" -o $(BINARY_NAME) ./cmd/go-bricks-openapi

test: ## Run all tests
	go test -race ./cmd/... ./internal/commands/... ./internal/generator/... ./internal/analyzer/... ./internal/spectest/...

test-coverage: ## Run tests with coverage
	go test -race -coverprofile=coverage.out ./cmd/... ./internal/commands/... ./internal/generator/... ./internal/analyzer/... ./internal/spectest/...
	go tool cover -html=coverage.out -o coverage.html

lint: ## Run golangci-lint
	golangci-lint run

fmt: ## Format Go code
	go fmt ./...

update: ## Update dependencies to latest versions
	go get -u ./...
	go mod tidy

clean: ## Clean build cache and binaries
	go clean -cache -testcache
	rm -f $(BINARY_NAME) coverage.out coverage.html $(SPEC_TMP)

install: build ## Install the CLI tool locally
	go install -ldflags "-X main.version=$(VERSION)" ./cmd/go-bricks-openapi

demo: build ## Run demo generation on demo project (requires cloning demo repo)
	./$(BINARY_NAME) doctor --project $(SPEC_FIXTURE)
	@echo "To run demo generation:"
	@echo "1. Clone demo project: git clone https://github.com/gaborage/go-bricks-demo-project.git"
	@echo "2. Generate spec: ./$(BINARY_NAME) generate --project ../go-bricks-demo-project/openapi-demo --output demo-spec.yaml"

validate-cli: build ## Validate CLI commands work correctly
	./$(BINARY_NAME) version
	./$(BINARY_NAME) --help
	# doctor must run against a real go-bricks project (dep + modules present);
	# the framework root has neither, which the stricter PR13 doctor now fails.
	./$(BINARY_NAME) doctor --project $(SPEC_FIXTURE)
	# Exercise generate end-to-end and sanity-check the output (network-free;
	# full redocly lint lives in validate-spec).
	./$(BINARY_NAME) generate --project $(SPEC_FIXTURE) --output $(SPEC_TMP)
	@grep -q "openapi: 3.0.1" $(SPEC_TMP); status=$$?; rm -f $(SPEC_TMP); \
		test $$status -eq 0 || (echo "generated spec missing the OpenAPI version marker" && exit 1)
	@echo "✓ CLI validation passed"

validate-spec: build ## Generate a fixture spec and validate it with redocly (requires npx; CI/Unix)
	./$(BINARY_NAME) generate --project $(SPEC_FIXTURE) --output $(SPEC_TMP)
	npx -y @redocly/cli@$(REDOCLY_VERSION) lint $(SPEC_TMP) --config redocly.yaml
	@rm -f $(SPEC_TMP)
	@echo "✓ Spec validation passed"

# validate-spec is intentionally excluded from check: it requires npx/network
# (redocly) and is a CI-only gate. The in-process kin-openapi validation in
# internal/spectest already runs under `test`, so `check` stays offline/fast.
check: fmt lint test validate-cli ## Run fmt, lint, test, and CLI validation (pre-commit checks)

# Security scanners (pinned; mirror the CI gates so the local release gate matches)
vuln: ## Run govulncheck vulnerability scan
	go run golang.org/x/vuln/cmd/govulncheck@$(GOVULNCHECK_VERSION) ./...

sec: ## Run gosec security scanner (excludes testdata fixture modules, like CI)
	go run github.com/securego/gosec/v2/cmd/gosec@$(GOSEC_VERSION) -exclude-dir=testdata ./...

# Development helpers
dev-deps: ## Install development dependencies
	go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest

# Release helpers
release: ## Cut a signed release tag (usage: make release VERSION=v0.2.0). Run AFTER merging the release-please PR.
	@# VERSION has a git-describe default (used by build/install), so require an explicit
	@# command-line override here rather than a non-empty check (which would always pass).
	@test "$(origin VERSION)" = "command line" || { echo "Error: VERSION is required, e.g. 'make release VERSION=v0.2.0'"; exit 1; }
	@VERSION=$(VERSION) ./scripts/release.sh

release-dry-run: ## Test release build without publishing
	@echo "Testing release build for version: $(VERSION)"
	GOOS=linux GOARCH=amd64 go build -ldflags "-X main.version=$(VERSION)" -o $(BINARY_NAME)-linux-amd64 ./cmd/go-bricks-openapi
	GOOS=darwin GOARCH=amd64 go build -ldflags "-X main.version=$(VERSION)" -o $(BINARY_NAME)-darwin-amd64 ./cmd/go-bricks-openapi
	GOOS=darwin GOARCH=arm64 go build -ldflags "-X main.version=$(VERSION)" -o $(BINARY_NAME)-darwin-arm64 ./cmd/go-bricks-openapi
	GOOS=windows GOARCH=amd64 go build -ldflags "-X main.version=$(VERSION)" -o $(BINARY_NAME)-windows-amd64.exe ./cmd/go-bricks-openapi
	@echo "✓ Release builds completed"
	@ls -la $(BINARY_NAME)-*
	@rm -f $(BINARY_NAME)-*