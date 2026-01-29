# Colors
GREEN=\033[0;32m
RESET=\033[0m
CHECKMARK=✓

BINARY_NAME=granola-sync
BUILD_DIR=./build

# Build the binary
.PHONY: build
build:
	@mkdir -p $(BUILD_DIR)
	go build -o $(BUILD_DIR)/$(BINARY_NAME) ./cmd/granola-sync

# Install the binary using go install
.PHONY: install
install:
	go install ./cmd/granola-sync
	@echo "Installed. Run 'granola-sync start' to start the service."

# Clean build artifacts
.PHONY: clean
clean:
	rm -rf $(BUILD_DIR)
	rm -f $(BINARY_NAME)
	rm -f coverage.out junit.xml

# Dev setup - install all dev dependencies and hooks
.PHONY: setup
setup:
	brew install golangci-lint || brew upgrade golangci-lint
	go install gotest.tools/gotestsum@latest
	go install golang.org/x/tools/cmd/deadcode@latest
	git config core.hooksPath scripts/hooks
	@echo "$(GREEN)$(CHECKMARK) Setup complete$(RESET)"

# Run tests with gotestsum
.PHONY: test
test: require-gotestsum
	gotestsum --format pkgname-and-test-fails ./...

# Run unit tests with coverage
.PHONY: test_unit
test_unit: require-gotestsum
	gotestsum --format pkgname-and-test-fails --junitfile junit.xml -- -coverprofile=coverage.out ./...

# Run linter with auto-fix
.PHONY: lint
lint: require-golangci-lint require-deadcode
	golangci-lint run --fix ./...
	deadcode ./...

# Run formatter
.PHONY: fmt
fmt: require-golangci-lint
	golangci-lint fmt ./...

# Run go vet
.PHONY: vet
vet:
	go vet ./...

# Create a new release
.PHONY: release
release:
	@./scripts/release.sh

# Helper targets
.PHONY: require-gotestsum
require-gotestsum:
	@which gotestsum > /dev/null || (echo "gotestsum not found. Run: make setup" && exit 1)

.PHONY: require-golangci-lint
require-golangci-lint:
	@which golangci-lint > /dev/null || (echo "golangci-lint not found. Run: make setup" && exit 1)

.PHONY: require-deadcode
require-deadcode:
	@which deadcode > /dev/null || (echo "deadcode not found. Run: make setup" && exit 1)
