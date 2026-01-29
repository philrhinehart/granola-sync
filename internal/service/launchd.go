// Package service provides macOS launchd service management.
package service

import (
	_ "embed"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

//go:embed launchd.plist.tmpl
var plistTemplate string

const (
	ServiceLabel = "com.granola-sync"
	PlistName    = "com.granola-sync.plist"
)

// plistPath returns the path to the plist file in LaunchAgents.
func plistPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("getting home directory: %w", err)
	}
	return filepath.Join(home, "Library", "LaunchAgents", PlistName), nil
}

// LogPath returns the path to the service stderr log file.
func LogPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("getting home directory: %w", err)
	}
	return filepath.Join(home, ".config", "granola-sync", "stderr.log"), nil
}

// Install generates the plist, copies it to LaunchAgents, and loads the service.
func Install() error {
	// Get binary path
	binaryPath, err := exec.LookPath("granola-sync")
	if err != nil {
		// Try GOPATH/bin
		gopath := os.Getenv("GOPATH")
		if gopath == "" {
			home, _ := os.UserHomeDir()
			gopath = filepath.Join(home, "go")
		}
		binaryPath = filepath.Join(gopath, "bin", "granola-sync")
		if _, err := os.Stat(binaryPath); err != nil {
			return fmt.Errorf("granola-sync binary not found in PATH or GOPATH/bin")
		}
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("getting home directory: %w", err)
	}

	// Ensure config directory exists
	configDir := filepath.Join(home, ".config", "granola-sync")
	if err := os.MkdirAll(configDir, 0o755); err != nil {
		return fmt.Errorf("creating config directory: %w", err)
	}

	// Generate plist content
	plistContent := strings.ReplaceAll(plistTemplate, "__BINARY_PATH__", binaryPath)
	plistContent = strings.ReplaceAll(plistContent, "~", home)

	// Ensure LaunchAgents directory exists
	launchAgentsDir := filepath.Join(home, "Library", "LaunchAgents")
	if err := os.MkdirAll(launchAgentsDir, 0o755); err != nil {
		return fmt.Errorf("creating LaunchAgents directory: %w", err)
	}

	// Unload if already loaded
	_ = Unload()

	// Write plist file
	plistFile, err := plistPath()
	if err != nil {
		return err
	}
	if err := os.WriteFile(plistFile, []byte(plistContent), 0o644); err != nil {
		return fmt.Errorf("writing plist file: %w", err)
	}

	// Load the service
	cmd := exec.Command("launchctl", "load", plistFile)
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("loading service: %s: %w", string(output), err)
	}

	return nil
}

// Unload stops the service and removes the plist file.
func Unload() error {
	plistFile, err := plistPath()
	if err != nil {
		return err
	}

	// Unload the service (ignore error if not loaded)
	cmd := exec.Command("launchctl", "unload", plistFile)
	_ = cmd.Run()

	// Remove the plist file
	if err := os.Remove(plistFile); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("removing plist file: %w", err)
	}

	return nil
}

// Status represents the service status.
type Status struct {
	Running bool
	PID     int
	Label   string
}

// GetStatus returns the current service status.
func GetStatus() (*Status, error) {
	cmd := exec.Command("launchctl", "list")
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("listing services: %w", err)
	}

	for _, line := range strings.Split(string(output), "\n") {
		if strings.Contains(line, ServiceLabel) {
			fields := strings.Fields(line)
			if len(fields) >= 3 {
				status := &Status{
					Label:   ServiceLabel,
					Running: fields[0] != "-",
				}
				if status.Running {
					_, _ = fmt.Sscanf(fields[0], "%d", &status.PID)
				}
				return status, nil
			}
		}
	}

	return nil, nil // Service not found
}
