package main

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"

	"github.com/philrhinehart/granola-sync/internal/config"
	"github.com/philrhinehart/granola-sync/internal/granola"
)

func newConfigCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "config [key] [value]",
		Short: "View or set configuration",
		Long: `View or set configuration values.

Examples:
  granola-sync config                  # Show all config values
  granola-sync config user_email       # Get a specific value
  granola-sync config user_email a@b.c # Set a value
  granola-sync config init             # Interactive setup wizard`,
		Args: cobra.MaximumNArgs(2),
		RunE: runConfig,
	}

	initCmd := &cobra.Command{
		Use:   "init",
		Short: "Interactive configuration setup",
		Long:  "Run the interactive setup wizard to configure granola-sync.",
		RunE:  runConfigInit,
	}
	cmd.AddCommand(initCmd)

	return cmd
}

func runConfig(cmd *cobra.Command, args []string) error {
	cfg, err := config.Load("")
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}

	switch len(args) {
	case 0:
		// Show all config as YAML
		data, err := yaml.Marshal(cfg)
		if err != nil {
			return fmt.Errorf("marshaling config: %w", err)
		}
		fmt.Print(string(data))
		return nil

	case 1:
		// Get a specific value
		value, err := cfg.Get(args[0])
		if err != nil {
			return err
		}
		fmt.Println(value)
		return nil

	case 2:
		// Set a value
		if err := cfg.Set(args[0], args[1]); err != nil {
			return err
		}
		if err := cfg.Save(""); err != nil {
			return fmt.Errorf("saving config: %w", err)
		}
		fmt.Printf("Set %s = %s\n", args[0], args[1])
		return nil

	default:
		return fmt.Errorf("too many arguments")
	}
}

func runConfigInit(cmd *cobra.Command, args []string) error {
	fmt.Println("granola-sync configuration wizard")
	fmt.Println("==================================")
	fmt.Println()

	scanner := bufio.NewScanner(os.Stdin)
	cfg := config.DefaultConfig()

	// Step 1: Logseq graph path
	logseqPath, err := promptLogseqPath(scanner)
	if err != nil {
		return err
	}
	cfg.LogseqBasePath = logseqPath

	// Step 2: User email
	fmt.Print("Enter your email address (used to identify you in meetings): ")
	if !scanner.Scan() {
		return fmt.Errorf("reading input")
	}
	email := strings.TrimSpace(scanner.Text())
	if email == "" {
		return fmt.Errorf("email is required")
	}
	cfg.UserEmail = email
	fmt.Println()

	// Step 3: User name (with examples from Granola cache)
	userName, err := promptUserName(scanner, cfg.GranolaCachePath)
	if err != nil {
		return err
	}
	cfg.UserName = userName

	// Save config
	configPath := config.ConfigPath()
	if err := cfg.Save(configPath); err != nil {
		return fmt.Errorf("saving config: %w", err)
	}

	fmt.Println()
	fmt.Println("Configuration saved to:", configPath)
	fmt.Println()
	fmt.Println("Next steps:")
	fmt.Println("  granola-sync start   # Start the background service")
	fmt.Println("  granola-sync status  # Check service status")
	fmt.Println("  granola-sync logs    # View service logs")

	return nil
}

func promptLogseqPath(scanner *bufio.Scanner) (string, error) {
	// Auto-detect Logseq graphs
	graphs := findLogseqGraphs()

	if len(graphs) > 0 {
		fmt.Println("Found Logseq graphs:")
		for i, g := range graphs {
			fmt.Printf("  %d) %s\n", i+1, g)
		}
		fmt.Printf("  %d) Enter a custom path\n", len(graphs)+1)
		fmt.Println()
		fmt.Printf("Select an option [1-%d]: ", len(graphs)+1)

		if !scanner.Scan() {
			return "", fmt.Errorf("reading input")
		}
		input := strings.TrimSpace(scanner.Text())

		var choice int
		if _, err := fmt.Sscanf(input, "%d", &choice); err == nil {
			if choice >= 1 && choice <= len(graphs) {
				fmt.Println()
				return graphs[choice-1], nil
			}
		}
		// Fall through to custom path input
	}

	fmt.Print("Enter the path to your Logseq graph: ")
	if !scanner.Scan() {
		return "", fmt.Errorf("reading input")
	}
	path := strings.TrimSpace(scanner.Text())
	if path == "" {
		return "", fmt.Errorf("logseq path is required")
	}

	// Expand ~ in path
	if strings.HasPrefix(path, "~/") {
		homeDir, _ := os.UserHomeDir()
		path = filepath.Join(homeDir, path[2:])
	}

	fmt.Println()
	return path, nil
}

func findLogseqGraphs() []string {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil
	}

	// Check iCloud Logseq directory
	icloudPath := filepath.Join(homeDir, "Library", "Mobile Documents", "iCloud~com~logseq~logseq", "Documents")
	entries, err := os.ReadDir(icloudPath)
	if err != nil {
		return nil
	}

	var graphs []string
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		graphPath := filepath.Join(icloudPath, entry.Name())

		// Check if it has pages/ and journals/ directories (valid Logseq graph)
		pagesPath := filepath.Join(graphPath, "pages")
		journalsPath := filepath.Join(graphPath, "journals")

		pagesInfo, pagesErr := os.Stat(pagesPath)
		journalsInfo, journalsErr := os.Stat(journalsPath)

		if pagesErr == nil && pagesInfo.IsDir() && journalsErr == nil && journalsInfo.IsDir() {
			graphs = append(graphs, graphPath)
		}
	}

	return graphs
}

func promptUserName(scanner *bufio.Scanner, granolaPath string) (string, error) {
	// Try to get example names from Granola cache
	examples := getExampleNamesFromGranola(granolaPath)

	if len(examples) > 0 {
		fmt.Printf("Enter your name (e.g. %s): ", strings.Join(examples, ", "))
	} else {
		fmt.Print("Enter your name (as it appears in meetings): ")
	}

	if !scanner.Scan() {
		return "", fmt.Errorf("reading input")
	}
	name := strings.TrimSpace(scanner.Text())
	if name == "" {
		return "", fmt.Errorf("name is required")
	}

	return name, nil
}

func getExampleNamesFromGranola(cachePath string) []string {
	docs, err := granola.ParseCache(cachePath)
	if err != nil {
		return nil
	}

	// Collect unique names from all documents
	nameCount := make(map[string]int)
	for _, doc := range docs {
		for _, name := range doc.GetAttendeeNames() {
			nameCount[name]++
		}
	}

	// Sort by frequency and return top 2-3
	type nameFreq struct {
		name  string
		count int
	}
	var sorted []nameFreq
	for name, count := range nameCount {
		sorted = append(sorted, nameFreq{name, count})
	}

	// Simple bubble sort (small list)
	for i := 0; i < len(sorted); i++ {
		for j := i + 1; j < len(sorted); j++ {
			if sorted[j].count > sorted[i].count {
				sorted[i], sorted[j] = sorted[j], sorted[i]
			}
		}
	}

	// Return top 2-3 names
	var result []string
	for i := 0; i < len(sorted) && i < 3; i++ {
		result = append(result, fmt.Sprintf("%q", sorted[i].name))
	}
	return result
}
