package main

import (
	"os"

	"github.com/spf13/cobra"
)

func main() {
	rootCmd := &cobra.Command{
		Use:   "granola-sync",
		Short: "Sync Granola meeting notes to Logseq",
		Long:  "A daemon that monitors Granola meeting notes and syncs them to Logseq pages and journal entries.",
	}

	rootCmd.AddCommand(
		newRunCmd(),
		newStartCmd(),
		newStatusCmd(),
		newLogsCmd(),
		newUnloadCmd(),
		newConfigCmd(),
	)

	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}
