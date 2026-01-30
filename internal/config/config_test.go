package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/suite"
)

type ConfigSuite struct {
	suite.Suite
	tempDir string
}

func TestConfigSuite(t *testing.T) {
	suite.Run(t, new(ConfigSuite))
}

func (s *ConfigSuite) SetupTest() {
	var err error
	s.tempDir, err = os.MkdirTemp("", "config-test-*")
	s.Require().NoError(err)
}

func (s *ConfigSuite) TearDownTest() {
	_ = os.RemoveAll(s.tempDir)
}

func (s *ConfigSuite) TestLoadDefaults() {
	// Load from non-existent path returns defaults
	cfg, err := Load(filepath.Join(s.tempDir, "nonexistent.yaml"))
	s.NoError(err)
	s.NotNil(cfg)
	s.Equal(30, cfg.DebounceSeconds)
	s.Equal(60, cfg.MinAgeSeconds)
	s.Equal("info", cfg.LogLevel)
}

func (s *ConfigSuite) TestLoadFromFile() {
	configPath := filepath.Join(s.tempDir, "config.yaml")
	content := `
user_email: test@example.com
debounce_seconds: 10
log_level: debug
`
	s.Require().NoError(os.WriteFile(configPath, []byte(content), 0o644))

	cfg, err := Load(configPath)
	s.NoError(err)
	s.Equal("test@example.com", cfg.UserEmail)
	s.Equal(10, cfg.DebounceSeconds)
	s.Equal("debug", cfg.LogLevel)
}

func (s *ConfigSuite) TestLoadInvalidYAML() {
	configPath := filepath.Join(s.tempDir, "config.yaml")
	// Use truly invalid YAML (unclosed bracket)
	s.Require().NoError(os.WriteFile(configPath, []byte("user_email: [unclosed"), 0o644))

	_, err := Load(configPath)
	s.Error(err)
	s.Contains(err.Error(), "parsing config")
}

func (s *ConfigSuite) TestGet() {
	tests := []struct {
		name       string
		key        string
		wantErr    bool
		allowEmpty bool
	}{
		{"valid_user_email", "user_email", false, false},
		{"valid_debounce", "debounce_seconds", false, false},
		{"valid_min_age", "min_age_seconds", false, false},
		{"valid_log_level", "log_level", false, false},
		{"valid_granola_path", "granola_cache_path", false, false},
		{"valid_logseq_path", "logseq_base_path", false, false},
		{"valid_state_path", "state_db_path", false, false},
		{"valid_user_name", "user_name", false, true}, // user_name is empty by default
		{"invalid_key", "unknown_key", true, false},
	}

	cfg := DefaultConfig()
	cfg.UserEmail = "test@example.com"

	for _, tt := range tests {
		s.Run(tt.name, func() {
			val, err := cfg.Get(tt.key)
			if tt.wantErr {
				s.Error(err)
				s.Contains(err.Error(), "unknown config key")
			} else {
				s.NoError(err)
				if !tt.allowEmpty {
					s.NotEmpty(val)
				}
			}
		})
	}
}

func (s *ConfigSuite) TestSet() {
	tests := []struct {
		name    string
		key     string
		value   string
		wantErr bool
		verify  func(*Config)
	}{
		{
			name:    "set_string",
			key:     "user_email",
			value:   "new@example.com",
			wantErr: false,
			verify:  func(c *Config) { s.Equal("new@example.com", c.UserEmail) },
		},
		{
			name:    "set_int",
			key:     "debounce_seconds",
			value:   "15",
			wantErr: false,
			verify:  func(c *Config) { s.Equal(15, c.DebounceSeconds) },
		},
		{
			name:    "invalid_int",
			key:     "debounce_seconds",
			value:   "not_a_number",
			wantErr: true,
		},
		{
			name:    "set_min_age",
			key:     "min_age_seconds",
			value:   "120",
			wantErr: false,
			verify:  func(c *Config) { s.Equal(120, c.MinAgeSeconds) },
		},
		{
			name:    "invalid_min_age",
			key:     "min_age_seconds",
			value:   "abc",
			wantErr: true,
		},
		{
			name:    "set_log_level",
			key:     "log_level",
			value:   "debug",
			wantErr: false,
			verify:  func(c *Config) { s.Equal("debug", c.LogLevel) },
		},
		{
			name:    "set_user_name",
			key:     "user_name",
			value:   "Test User",
			wantErr: false,
			verify:  func(c *Config) { s.Equal("Test User", c.UserName) },
		},
		{
			name:    "invalid_key",
			key:     "unknown",
			value:   "value",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		s.Run(tt.name, func() {
			cfg := DefaultConfig()
			err := cfg.Set(tt.key, tt.value)
			if tt.wantErr {
				s.Error(err)
			} else {
				s.NoError(err)
				if tt.verify != nil {
					tt.verify(cfg)
				}
			}
		})
	}
}

func (s *ConfigSuite) TestSave() {
	cfg := DefaultConfig()
	cfg.UserEmail = "saved@example.com"
	cfg.DebounceSeconds = 20

	configPath := filepath.Join(s.tempDir, "subdir", "config.yaml")

	err := cfg.Save(configPath)
	s.NoError(err)

	// Verify file exists
	_, err = os.Stat(configPath)
	s.NoError(err)

	// Verify content can be loaded back
	loaded, err := Load(configPath)
	s.NoError(err)
	s.Equal("saved@example.com", loaded.UserEmail)
	s.Equal(20, loaded.DebounceSeconds)
}

func (s *ConfigSuite) TestExpandPath() {
	homeDir, _ := os.UserHomeDir()

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"tilde_expansion", "~/Documents", filepath.Join(homeDir, "Documents")},
		{"absolute_path", "/absolute/path", "/absolute/path"},
		{"relative_path", "relative/path", "relative/path"},
		{"empty_path", "", ""},
	}

	for _, tt := range tests {
		s.Run(tt.name, func() {
			result := expandPath(tt.input)
			s.Equal(tt.expected, result)
		})
	}
}

func (s *ConfigSuite) TestPathExpansionOnLoad() {
	configPath := filepath.Join(s.tempDir, "config.yaml")
	content := `
granola_cache_path: ~/cache.json
logseq_base_path: ~/logseq
state_db_path: ~/state.db
`
	s.Require().NoError(os.WriteFile(configPath, []byte(content), 0o644))

	cfg, err := Load(configPath)
	s.NoError(err)

	homeDir, _ := os.UserHomeDir()
	s.Equal(filepath.Join(homeDir, "cache.json"), cfg.GranolaCachePath)
	s.Equal(filepath.Join(homeDir, "logseq"), cfg.LogseqBasePath)
	s.Equal(filepath.Join(homeDir, "state.db"), cfg.StateDBPath)
}

func (s *ConfigSuite) TestPathExpansionOnSet() {
	cfg := DefaultConfig()

	homeDir, _ := os.UserHomeDir()

	err := cfg.Set("logseq_base_path", "~/Documents/logseq")
	s.NoError(err)
	s.Equal(filepath.Join(homeDir, "Documents/logseq"), cfg.LogseqBasePath)
}
