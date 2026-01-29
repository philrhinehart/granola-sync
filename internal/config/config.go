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
		LogseqBasePath:   filepath.Join(homeDir, "Library", "Mobile Documents", "iCloud~com~logseq~logseq", "Documents", "AngelList"),
		StateDBPath:      filepath.Join(homeDir, ".config", "granola-sync", "state.db"),
		DebounceSeconds:  5,
		MinAgeSeconds:    60,
		LogLevel:         "info",
	}
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
	if err := os.MkdirAll(stateDir, 0755); err != nil {
		return fmt.Errorf("creating state directory: %w", err)
	}

	// Ensure logseq pages and journals directories exist
	pagesDir := filepath.Join(c.LogseqBasePath, "pages")
	if err := os.MkdirAll(pagesDir, 0755); err != nil {
		return fmt.Errorf("creating pages directory: %w", err)
	}

	journalsDir := filepath.Join(c.LogseqBasePath, "journals")
	if err := os.MkdirAll(journalsDir, 0755); err != nil {
		return fmt.Errorf("creating journals directory: %w", err)
	}

	return nil
}
