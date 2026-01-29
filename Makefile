.PHONY: build install install-binary install-service uninstall clean run test backfill dry-run

BINARY_NAME=granola-sync
BUILD_DIR=./build
INSTALL_PATH=/usr/local/bin/$(BINARY_NAME)

# Build the binary
build:
	@mkdir -p $(BUILD_DIR)
	go build -o $(BUILD_DIR)/$(BINARY_NAME) ./cmd/granola-sync

# Install everything (requires sudo for binary installation)
install: build install-binary install-service

# Install just the binary (requires sudo)
install-binary:
	sudo cp $(BUILD_DIR)/$(BINARY_NAME) $(INSTALL_PATH)
	sudo chmod +x $(INSTALL_PATH)
	@echo "Binary installed to $(INSTALL_PATH)"

# Install and load the launchd service
install-service:
	./scripts/install.sh

# Uninstall the service
uninstall:
	-launchctl unload ~/Library/LaunchAgents/com.granola-sync.plist 2>/dev/null
	-rm ~/Library/LaunchAgents/com.granola-sync.plist
	-sudo rm $(INSTALL_PATH)
	@echo "Uninstalled granola-sync"

# Clean build artifacts
clean:
	rm -rf $(BUILD_DIR)
	rm -f $(BINARY_NAME)

# Run locally (without installing)
run: build
	$(BUILD_DIR)/$(BINARY_NAME) -v

# Run tests
test:
	go test ./...

# Backfill all meetings (dry run)
dry-run: build
	$(BUILD_DIR)/$(BINARY_NAME) --backfill --dry-run

# Backfill all meetings
backfill: build
	$(BUILD_DIR)/$(BINARY_NAME) --backfill

# Show service status
status:
	@launchctl list | grep granola-sync || echo "Service not running"

# View logs
logs:
	tail -f ~/.config/granola-sync/stderr.log

# Stop the service
stop:
	launchctl stop com.granola-sync

# Start the service
start:
	launchctl start com.granola-sync
