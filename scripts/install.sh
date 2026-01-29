#!/bin/bash
set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

echo -e "${GREEN}Installing granola-sync...${NC}"

# Check if binary exists
BINARY_PATH="/usr/local/bin/granola-sync"
if [ ! -f "$BINARY_PATH" ]; then
    echo -e "${RED}Error: Binary not found at $BINARY_PATH${NC}"
    echo "Please run 'make build' first, then 'sudo make install-binary'"
    exit 1
fi

# Create config directory
CONFIG_DIR="$HOME/.config/granola-sync"
mkdir -p "$CONFIG_DIR"
echo -e "Created config directory: ${YELLOW}$CONFIG_DIR${NC}"

# Copy example config if no config exists
if [ ! -f "$CONFIG_DIR/config.yaml" ]; then
    SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
    if [ -f "$SCRIPT_DIR/../configs/config.yaml.example" ]; then
        cp "$SCRIPT_DIR/../configs/config.yaml.example" "$CONFIG_DIR/config.yaml"
        echo -e "Created default config: ${YELLOW}$CONFIG_DIR/config.yaml${NC}"
        echo -e "${YELLOW}Please edit the config file to match your setup!${NC}"
    fi
fi

# Install launchd plist
PLIST_SRC="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/../configs/com.granola-sync.plist"
PLIST_DEST="$HOME/Library/LaunchAgents/com.granola-sync.plist"

# Unload if already loaded
if launchctl list | grep -q "com.granola-sync"; then
    echo "Unloading existing service..."
    launchctl unload "$PLIST_DEST" 2>/dev/null || true
fi

# Expand ~ in plist paths
sed "s|~|$HOME|g" "$PLIST_SRC" > "$PLIST_DEST"
echo -e "Installed plist: ${YELLOW}$PLIST_DEST${NC}"

# Load the service
launchctl load "$PLIST_DEST"
echo -e "${GREEN}Service loaded!${NC}"

# Check if running
sleep 1
if launchctl list | grep -q "com.granola-sync"; then
    echo -e "${GREEN}granola-sync is now running!${NC}"
else
    echo -e "${RED}Warning: Service may not have started. Check logs at $CONFIG_DIR/stderr.log${NC}"
fi

echo ""
echo "Commands:"
echo "  Stop:    launchctl stop com.granola-sync"
echo "  Start:   launchctl start com.granola-sync"
echo "  Status:  launchctl list | grep granola-sync"
echo "  Logs:    tail -f $CONFIG_DIR/stderr.log"
echo "  Unload:  launchctl unload $PLIST_DEST"
