package granola

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// supabaseFile represents the structure of Granola's supabase.json
type supabaseFile struct {
	WorkOSTokens string `json:"workos_tokens"`
}

// workOSTokens represents the parsed workos_tokens JSON string
type workOSTokens struct {
	AccessToken string `json:"access_token"`
}

// LoadAuthToken reads the Granola auth token from supabase.json in the given directory.
func LoadAuthToken(granolaDir string) (string, error) {
	path := filepath.Join(granolaDir, "supabase.json")
	data, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("reading supabase.json: %w", err)
	}

	var sf supabaseFile
	if err := json.Unmarshal(data, &sf); err != nil {
		return "", fmt.Errorf("parsing supabase.json: %w", err)
	}

	if sf.WorkOSTokens == "" {
		return "", fmt.Errorf("workos_tokens not found in supabase.json")
	}

	var tokens workOSTokens
	if err := json.Unmarshal([]byte(sf.WorkOSTokens), &tokens); err != nil {
		return "", fmt.Errorf("parsing workos_tokens: %w", err)
	}

	if tokens.AccessToken == "" {
		return "", fmt.Errorf("access_token not found in workos_tokens")
	}

	return tokens.AccessToken, nil
}
