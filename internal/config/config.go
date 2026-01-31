package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

type Config struct {
	GranolaCachePath string `yaml:"granola_cache_path"`
	LogseqBasePath   string `yaml:"logseq_base_path"`
	StateDBPath      string `yaml:"state_db_path"`
	DebounceSeconds  int    `yaml:"debounce_seconds"`
	MinAgeSeconds    int    `yaml:"min_age_seconds"`
	LogLevel         string `yaml:"log_level"`
	UserEmail        string `yaml:"user_email"`
	UserName         string `yaml:"user_name"`
}

func DefaultConfig() *Config {
	homeDir, _ := os.UserHomeDir()
	return &Config{
		GranolaCachePath: filepath.Join(homeDir, "Library", "Application Support", "Granola", "cache-v3.json"),
		LogseqBasePath:   findLogseqGraph(homeDir),
		StateDBPath:      filepath.Join(homeDir, ".config", "granola-sync", "state.db"),
		DebounceSeconds:  30,
		MinAgeSeconds:    60,
		LogLevel:         "info",
	}
}

// findLogseqGraph searches common locations for a Logseq graph and returns the first one found.
// Returns empty string if no graph is found (user must configure manually).
func findLogseqGraph(homeDir string) string {
	// Common Logseq graph locations to check
	candidates := []string{
		filepath.Join(homeDir, "Documents", "logseq"),
		filepath.Join(homeDir, "logseq"),
		filepath.Join(homeDir, "Documents", "Logseq"),
		filepath.Join(homeDir, "Logseq"),
	}

	for _, path := range candidates {
		if isLogseqGraph(path) {
			return path
		}
	}

	return "" // No graph found, user must configure
}

// isLogseqGraph checks if a directory appears to be a Logseq graph
// by looking for characteristic subdirectories (pages/, journals/, or logseq/).
func isLogseqGraph(path string) bool {
	info, err := os.Stat(path)
	if err != nil || !info.IsDir() {
		return false
	}

	// A Logseq graph typically has at least one of these directories
	markers := []string{"pages", "journals", "logseq"}
	for _, marker := range markers {
		markerPath := filepath.Join(path, marker)
		if info, err := os.Stat(markerPath); err == nil && info.IsDir() {
			return true
		}
	}

	return false
}

func Load(path string) (*Config, error) {
	cfg := DefaultConfig()

	if path == "" {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return cfg, nil // Return defaults if can't find home
		}
		path = filepath.Join(homeDir, ".config", "granola-sync", "config.yaml")
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return cfg, nil // Use defaults if config doesn't exist
		}
		return nil, fmt.Errorf("reading config: %w", err)
	}

	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("parsing config: %w", err)
	}

	// Expand paths
	cfg.GranolaCachePath = expandPath(cfg.GranolaCachePath)
	cfg.LogseqBasePath = expandPath(cfg.LogseqBasePath)
	cfg.StateDBPath = expandPath(cfg.StateDBPath)

	return cfg, nil
}

func expandPath(path string) string {
	if strings.HasPrefix(path, "~/") {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return path
		}
		return filepath.Join(homeDir, path[2:])
	}
	return path
}

func (c *Config) EnsureDirectories() error {
	// Ensure state directory exists
	stateDir := filepath.Dir(c.StateDBPath)
	if err := os.MkdirAll(stateDir, 0o755); err != nil {
		return fmt.Errorf("creating state directory: %w", err)
	}

	// Ensure logseq pages and journals directories exist
	pagesDir := filepath.Join(c.LogseqBasePath, "pages")
	if err := os.MkdirAll(pagesDir, 0o755); err != nil {
		return fmt.Errorf("creating pages directory: %w", err)
	}

	journalsDir := filepath.Join(c.LogseqBasePath, "journals")
	if err := os.MkdirAll(journalsDir, 0o755); err != nil {
		return fmt.Errorf("creating journals directory: %w", err)
	}

	return nil
}

// ConfigPath returns the default config file path
func ConfigPath() string {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(homeDir, ".config", "granola-sync", "config.yaml")
}

// Save writes the config to the specified path
func (c *Config) Save(path string) error {
	if path == "" {
		path = ConfigPath()
	}

	// Ensure directory exists
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("creating config directory: %w", err)
	}

	data, err := yaml.Marshal(c)
	if err != nil {
		return fmt.Errorf("marshaling config: %w", err)
	}

	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("writing config file: %w", err)
	}

	return nil
}

// Get returns a config value by key name
func (c *Config) Get(key string) (string, error) {
	switch key {
	case "granola_cache_path":
		return c.GranolaCachePath, nil
	case "logseq_base_path":
		return c.LogseqBasePath, nil
	case "state_db_path":
		return c.StateDBPath, nil
	case "debounce_seconds":
		return fmt.Sprintf("%d", c.DebounceSeconds), nil
	case "min_age_seconds":
		return fmt.Sprintf("%d", c.MinAgeSeconds), nil
	case "log_level":
		return c.LogLevel, nil
	case "user_email":
		return c.UserEmail, nil
	case "user_name":
		return c.UserName, nil
	default:
		return "", fmt.Errorf("unknown config key: %s", key)
	}
}

// Set sets a config value by key name
func (c *Config) Set(key, value string) error {
	switch key {
	case "granola_cache_path":
		c.GranolaCachePath = expandPath(value)
	case "logseq_base_path":
		c.LogseqBasePath = expandPath(value)
	case "state_db_path":
		c.StateDBPath = expandPath(value)
	case "debounce_seconds":
		var v int
		if _, err := fmt.Sscanf(value, "%d", &v); err != nil {
			return fmt.Errorf("invalid value for debounce_seconds: %w", err)
		}
		c.DebounceSeconds = v
	case "min_age_seconds":
		var v int
		if _, err := fmt.Sscanf(value, "%d", &v); err != nil {
			return fmt.Errorf("invalid value for min_age_seconds: %w", err)
		}
		c.MinAgeSeconds = v
	case "log_level":
		c.LogLevel = value
	case "user_email":
		c.UserEmail = value
	case "user_name":
		c.UserName = value
	default:
		return fmt.Errorf("unknown config key: %s", key)
	}
	return nil
}
