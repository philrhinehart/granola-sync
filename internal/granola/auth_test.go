package granola

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/suite"
)

type AuthSuite struct {
	suite.Suite
	tempDir string
}

func TestAuthSuite(t *testing.T) {
	suite.Run(t, new(AuthSuite))
}

func (s *AuthSuite) SetupTest() {
	var err error
	s.tempDir, err = os.MkdirTemp("", "auth-test-*")
	s.Require().NoError(err)
}

func (s *AuthSuite) TearDownTest() {
	_ = os.RemoveAll(s.tempDir)
}

func (s *AuthSuite) TestLoadAuthToken() {
	content := `{"workos_tokens": "{\"access_token\": \"test-token-123\", \"refresh_token\": \"rt\"}"}`
	err := os.WriteFile(filepath.Join(s.tempDir, "supabase.json"), []byte(content), 0o644)
	s.Require().NoError(err)

	token, err := LoadAuthToken(s.tempDir)
	s.NoError(err)
	s.Equal("test-token-123", token)
}

func (s *AuthSuite) TestLoadAuthTokenMissingFile() {
	_, err := LoadAuthToken(s.tempDir)
	s.Error(err)
	s.Contains(err.Error(), "reading supabase.json")
}

func (s *AuthSuite) TestLoadAuthTokenMissingWorkOSTokens() {
	content := `{"other_field": "value"}`
	err := os.WriteFile(filepath.Join(s.tempDir, "supabase.json"), []byte(content), 0o644)
	s.Require().NoError(err)

	_, err = LoadAuthToken(s.tempDir)
	s.Error(err)
	s.Contains(err.Error(), "workos_tokens not found")
}

func (s *AuthSuite) TestLoadAuthTokenMissingAccessToken() {
	content := `{"workos_tokens": "{\"refresh_token\": \"rt\"}"}`
	err := os.WriteFile(filepath.Join(s.tempDir, "supabase.json"), []byte(content), 0o644)
	s.Require().NoError(err)

	_, err = LoadAuthToken(s.tempDir)
	s.Error(err)
	s.Contains(err.Error(), "access_token not found")
}

func (s *AuthSuite) TestLoadAuthTokenInvalidJSON() {
	content := `{invalid json}`
	err := os.WriteFile(filepath.Join(s.tempDir, "supabase.json"), []byte(content), 0o644)
	s.Require().NoError(err)

	_, err = LoadAuthToken(s.tempDir)
	s.Error(err)
	s.Contains(err.Error(), "parsing supabase.json")
}

func (s *AuthSuite) TestLoadAuthTokenInvalidWorkOSTokensJSON() {
	content := `{"workos_tokens": "not valid json"}`
	err := os.WriteFile(filepath.Join(s.tempDir, "supabase.json"), []byte(content), 0o644)
	s.Require().NoError(err)

	_, err = LoadAuthToken(s.tempDir)
	s.Error(err)
	s.Contains(err.Error(), "parsing workos_tokens")
}
