package main

import (
	"bufio"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/spf13/cobra"

	"github.com/philrhinehart/granola-sync/internal/config"
	"github.com/philrhinehart/granola-sync/internal/granola"
	"github.com/philrhinehart/granola-sync/internal/service"
	"github.com/philrhinehart/granola-sync/internal/state"
	"github.com/philrhinehart/granola-sync/internal/sync"
)

var (
	cfgPath  string
	backfill bool
	sinceStr string
	dryRun   bool
	verbose  bool
)

func main() {
	rootCmd := &cobra.Command{
		Use:   "granola-sync",
		Short: "Sync Granola meeting notes to Logseq",
		Long:  "A daemon that monitors Granola meeting notes and syncs them to Logseq pages and journal entries.",
	}

	// Run command - watch mode (current default behavior)
	runCmd := &cobra.Command{
		Use:   "run",
		Short: "Run in watch mode (foreground)",
		Long:  "Start granola-sync in watch mode, monitoring for changes and syncing automatically.",
		RunE:  runWatch,
	}
	runCmd.Flags().StringVarP(&cfgPath, "config", "c", "", "path to config file")
	runCmd.Flags().BoolVar(&backfill, "backfill", false, "sync all historic meetings")
	runCmd.Flags().StringVar(&sinceStr, "since", "", "backfill meetings since date (YYYY-MM-DD)")
	runCmd.Flags().BoolVar(&dryRun, "dry-run", false, "show what would be synced without making changes")
	runCmd.Flags().BoolVarP(&verbose, "verbose", "v", false, "enable verbose logging")

	// Start command - install and start launchd service
	startCmd := &cobra.Command{
		Use:   "start",
		Short: "Install and start the launchd service",
		Long:  "Install the launchd plist and start granola-sync as a background service.",
		RunE:  runStart,
	}

	// Stop command - stop the launchd service
	stopCmd := &cobra.Command{
		Use:   "stop",
		Short: "Stop the launchd service",
		RunE:  runStop,
	}

	// Status command - show service status
	statusCmd := &cobra.Command{
		Use:   "status",
		Short: "Show service status",
		RunE:  runStatus,
	}

	// Logs command - tail the service logs
	logsCmd := &cobra.Command{
		Use:   "logs",
		Short: "Tail the service logs",
		RunE:  runLogs,
	}

	// Unload command - unload and remove the service
	unloadCmd := &cobra.Command{
		Use:   "unload",
		Short: "Unload and remove the launchd service",
		RunE:  runUnload,
	}

	rootCmd.AddCommand(runCmd, startCmd, stopCmd, statusCmd, logsCmd, unloadCmd)

	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func runWatch(cmd *cobra.Command, args []string) error {
	// Setup logging
	logLevel := slog.LevelInfo
	if verbose {
		logLevel = slog.LevelDebug
	}
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: logLevel})))

	// Load config
	cfg, err := config.Load(cfgPath)
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}

	if verbose {
		slog.Debug("config loaded",
			"granola_cache", cfg.GranolaCachePath,
			"logseq_base", cfg.LogseqBasePath,
			"state_db", cfg.StateDBPath,
			"user_email", cfg.UserEmail,
			"user_name", cfg.UserName,
		)
	}

	// Ensure directories exist
	if err := cfg.EnsureDirectories(); err != nil {
		return fmt.Errorf("ensuring directories: %w", err)
	}

	// Open state store
	store, err := state.NewStore(cfg.StateDBPath)
	if err != nil {
		return fmt.Errorf("opening state store: %w", err)
	}
	defer func() { _ = store.Close() }()

	syncer := sync.NewSyncer(cfg, store)

	// Parse since date if provided
	var since *time.Time
	if sinceStr != "" {
		t, err := time.Parse("2006-01-02", sinceStr)
		if err != nil {
			return fmt.Errorf("parsing since date: %w", err)
		}
		since = &t
	}

	// Backfill mode
	if backfill {
		return doBackfill(syncer, since, dryRun)
	}

	// Watch mode
	return doWatch(cfg, syncer, since, dryRun)
}

func doBackfill(syncer *sync.Syncer, since *time.Time, dryRun bool) error {
	if dryRun {
		fmt.Print("DRY RUN - showing what would be synced:\n\n")
	} else {
		slog.Info("starting backfill")
	}

	result, err := syncer.Sync(since, dryRun)
	if err != nil {
		return fmt.Errorf("sync failed: %w", err)
	}

	fmt.Printf("\nSync complete:\n")
	fmt.Printf("  New meetings: %d\n", result.NewMeetings)
	fmt.Printf("  Updated meetings: %d\n", result.UpdatedMeetings)
	fmt.Printf("  Journal entries: %d\n", result.NewJournals)
	if len(result.Errors) > 0 {
		fmt.Printf("  Errors: %d\n", len(result.Errors))
		for _, e := range result.Errors {
			slog.Error("sync error", "error", e)
		}
	}

	return nil
}

func doWatch(cfg *config.Config, syncer *sync.Syncer, since *time.Time, dryRun bool) error {
	slog.Info("starting watch mode", "path", cfg.GranolaCachePath)

	// Do initial sync
	slog.Info("performing initial sync")
	if _, err := syncer.Sync(since, dryRun); err != nil {
		slog.Error("initial sync failed", "error", err)
	}

	// Setup file watcher
	onChange := func() {
		slog.Info("cache file changed, syncing")
		result, err := syncer.Sync(since, dryRun)
		if err != nil {
			slog.Error("sync failed", "error", err)
			return
		}
		if result.NewMeetings > 0 || result.UpdatedMeetings > 0 {
			slog.Info("sync complete",
				"new", result.NewMeetings,
				"updated", result.UpdatedMeetings,
				"journals", result.NewJournals,
			)
		}
	}

	watcher, err := granola.NewWatcher(cfg.GranolaCachePath, cfg.DebounceSeconds, onChange)
	if err != nil {
		return fmt.Errorf("creating watcher: %w", err)
	}

	if err := watcher.Start(); err != nil {
		return fmt.Errorf("starting watcher: %w", err)
	}

	// Wait for shutdown signal
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	slog.Info("watching for changes (press Ctrl+C to stop)")
	<-sigChan

	slog.Info("shutting down")
	watcher.Stop()

	return nil
}

func runStart(cmd *cobra.Command, args []string) error {
	fmt.Println("Installing and starting granola-sync service...")

	if err := service.Install(); err != nil {
		return fmt.Errorf("installing service: %w", err)
	}

	fmt.Println("Service installed and started successfully!")
	fmt.Println("")
	fmt.Println("Commands:")
	fmt.Println("  granola-sync status  - Check service status")
	fmt.Println("  granola-sync logs    - View service logs")
	fmt.Println("  granola-sync stop    - Stop the service")
	fmt.Println("  granola-sync unload  - Remove the service")

	return nil
}

func runStop(cmd *cobra.Command, args []string) error {
	if err := service.Stop(); err != nil {
		return fmt.Errorf("stopping service: %w", err)
	}
	fmt.Println("Service stopped.")
	return nil
}

func runStatus(cmd *cobra.Command, args []string) error {
	status, err := service.GetStatus()
	if err != nil {
		return fmt.Errorf("getting status: %w", err)
	}

	if status == nil {
		fmt.Println("Service is not installed.")
		return nil
	}

	if status.Running {
		fmt.Printf("Service is running (PID: %d)\n", status.PID)
	} else {
		fmt.Println("Service is installed but not running.")
	}

	return nil
}

func runLogs(cmd *cobra.Command, args []string) error {
	logPath, err := service.LogPath()
	if err != nil {
		return err
	}

	file, err := os.Open(logPath)
	if err != nil {
		if os.IsNotExist(err) {
			fmt.Println("No logs found. Service may not have run yet.")
			return nil
		}
		return fmt.Errorf("opening log file: %w", err)
	}
	defer file.Close()

	// Seek to last 10KB or start of file
	stat, err := file.Stat()
	if err != nil {
		return fmt.Errorf("getting file info: %w", err)
	}
	if stat.Size() > 10240 {
		_, _ = file.Seek(-10240, io.SeekEnd)
		// Skip partial line
		reader := bufio.NewReader(file)
		_, _ = reader.ReadString('\n')
		// Print remaining
		for {
			line, err := reader.ReadString('\n')
			if err != nil {
				break
			}
			fmt.Print(line)
		}
	} else {
		// Print entire file
		_, _ = io.Copy(os.Stdout, file)
	}

	fmt.Printf("\n--- Log file: %s ---\n", logPath)
	fmt.Println("Use 'tail -f' for live updates: tail -f", logPath)

	return nil
}

func runUnload(cmd *cobra.Command, args []string) error {
	if err := service.Unload(); err != nil {
		return fmt.Errorf("unloading service: %w", err)
	}
	fmt.Println("Service unloaded and removed.")
	return nil
}
