# Colors
GREEN=\033[0;32m
RESET=\033[0m
CHECKMARK=✓

BINARY_NAME=granola-sync
BUILD_DIR=./build
GOBIN=$(shell go env GOPATH)/bin
INSTALL_PATH=$(GOBIN)/$(BINARY_NAME)

# Build the binary
.PHONY: build
build:
	@mkdir -p $(BUILD_DIR)
	go build -o $(BUILD_DIR)/$(BINARY_NAME) ./cmd/granola-sync

# Install everything
.PHONY: install
install: install-binary install-service

# Install just the binary using go install
.PHONY: install-binary
install-binary:
	go install ./cmd/granola-sync
	@echo "Binary installed to $(INSTALL_PATH)"

# Install and load the launchd service
.PHONY: install-service
install-service:
	./scripts/install.sh

# Uninstall the service
.PHONY: uninstall-service
uninstall-service:
	-launchctl unload ~/Library/LaunchAgents/com.granola-sync.plist 2>/dev/null
	-rm ~/Library/LaunchAgents/com.granola-sync.plist
	@echo "Uninstalled granola-sync service"

# Uninstall everything (service and binary)
.PHONY: uninstall
uninstall: uninstall-service
	-rm $(INSTALL_PATH)
	@echo "Uninstalled granola-sync binary"

# Clean build artifacts
.PHONY: clean
clean:
	rm -rf $(BUILD_DIR)
	rm -f $(BINARY_NAME)
	rm -f coverage.out junit.xml

# Run locally (without installing)
.PHONY: run
run: build
	$(BUILD_DIR)/$(BINARY_NAME) -v

# Dev setup - install all dev dependencies and hooks
.PHONY: setup
setup:
	brew install golangci-lint || brew upgrade golangci-lint
	go install gotest.tools/gotestsum@latest
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
lint: require-golangci-lint
	golangci-lint run --fix ./...

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

# Backfill all meetings (dry run)
.PHONY: dry-run
dry-run: build
	$(BUILD_DIR)/$(BINARY_NAME) --backfill --dry-run

# Backfill all meetings
.PHONY: backfill
backfill: build
	$(BUILD_DIR)/$(BINARY_NAME) --backfill

# Show service status
.PHONY: status
status:
	@launchctl list | grep granola-sync || echo "Service not running"

# View logs
.PHONY: logs
logs:
	tail -f ~/.config/granola-sync/stderr.log

# Stop the service
.PHONY: stop
stop:
	launchctl stop com.granola-sync

# Start the service
.PHONY: start
start:
	launchctl start com.granola-sync
