# granola-sync

Sync [Granola](https://granola.ai) meeting notes to [Logseq](https://logseq.com).

## Features

- Watches Granola's local cache for new/updated meetings
- Creates Logseq pages for each meeting with transcript, notes, and metadata
- Adds journal entries linking to meeting pages
- Runs as a macOS launchd service for always-on syncing
- Supports backfilling historical meetings

## Warning

This tool reads from Granola's local SQLite cache file. **This is an unofficial integration** - the cache format is undocumented and may change at any time without notice. If Granola updates their cache format, this tool may stop working until updated.

## Quick Start

```bash
# Install
go install github.com/philrhinehart/granola-sync/cmd/granola-sync@latest

# Configure (interactive wizard)
granola-sync config init

# Start as background service
granola-sync start

# Check status
granola-sync status

# View logs
granola-sync logs
```

## Commands

```
granola-sync config              # Show all config values
granola-sync config <key>        # Get a specific value
granola-sync config <key> <val>  # Set a value
granola-sync config init         # Interactive setup wizard

granola-sync run       # Watch mode (foreground)
granola-sync start     # Install and start launchd service
granola-sync stop      # Stop the launchd service
granola-sync status    # Show service status
granola-sync logs      # View service logs
granola-sync unload    # Unload and remove the service
```

### Run flags

```
granola-sync run [flags]

Flags:
  -c, --config string   path to config file
      --backfill        sync all historic meetings
      --since string    backfill meetings since date (YYYY-MM-DD)
      --dry-run         show what would be synced without making changes
  -v, --verbose         enable verbose logging
```

## Configuration

Config file location: `~/.config/granola-sync/config.yaml`

```yaml
# Required: path to your Logseq graph
logseq_base_path: ~/Library/Mobile Documents/iCloud~com~logseq~logseq/Documents/YourGraph

# Required: your email (to identify you in meeting participants)
user_email: your@email.com

# Required: your display name for journal entries
user_name: Your Name

# Optional: custom path to Granola cache (auto-detected by default)
# granola_cache_path: ~/Library/Application Support/Granola/cache.db

# Optional: debounce time in seconds (default: 2)
# debounce_seconds: 2
```

## Development

```bash
# Setup dev environment
make setup

# Run tests
make test

# Run linter
make lint

# Build
make build

# Run locally
./build/granola-sync run -v
```

### Releasing

```bash
make release
```

## License

MIT License - see [LICENSE](LICENSE) for details.
