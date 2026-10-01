ROOT_PATH := $(dir $(realpath $(lastword $(MAKEFILE_LIST))))
COVERAGE_PATH := $(ROOT_PATH).coverage/

# Use the locally installed Go toolchain.
export GOTOOLCHAIN=local

# Ensure our local tools are preferred
export PATH := $(ROOT_PATH)tools/bin:$(PATH)

include $(ROOT_PATH)tools/tools.mk

.PHONY: mockgen
mockgen: install-mockgen
	@echo "Generating mocks..."
	@rm -rf mock
	@mkdir -p mock
	$(MOCKGEN) -destination=mock/Repository.go -package=mock github.com/e2engine/core/repository EntityRepository,ResourceRepository,TestRepository,TestExecutionRepository,TestSuiteExecutionRepository
	$(MOCKGEN) -destination=mock/Runtime.go -package=mock github.com/e2engine/core/execute/runtime Service,ServiceProvider,Router,RouterProvider

.PHONY: lint
lint: install-golangci-lint
	@echo "Running linter..."
	$(GOLANGCI_LINT) run

.PHONY: test
test:
	@echo "Running tests..."
	@go clean -testcache
	@go test ./... -count=1 -timeout=600s

.PHONY: test-cov
test-cov:
	@echo "Running tests with coverage..."
	@go clean -testcache
	@rm -rf $(COVERAGE_PATH)
	@mkdir -p $(COVERAGE_PATH)
	@go test -v -coverpkg=./... ./... -coverprofile $(COVERAGE_PATH)coverage.txt -count=1 -timeout=600s -tags=debug
	@go tool cover -func=$(COVERAGE_PATH)coverage.txt -o $(COVERAGE_PATH)functions.txt
	@go tool cover -html=$(COVERAGE_PATH)coverage.txt -o $(COVERAGE_PATH)coverage.html

.PHONY: test-race
test-race:
	@echo "Running tests with race detector..."
	@go clean -testcache
	@go test ./... -race -count=1 -timeout=600s

.PHONY: verify
verify: mockgen lint test-race
	@echo "All verifications passed successfully."
