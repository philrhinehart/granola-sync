package main

import (
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/spf13/cobra"

	"github.com/philrhinehart/granola-sync/internal/config"
	"github.com/philrhinehart/granola-sync/internal/granola"
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

func newRunCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "run",
		Short: "Run in watch mode (foreground)",
		Long:  "Start granola-sync in watch mode, monitoring for changes and syncing automatically.",
		RunE:  runWatch,
	}
	cmd.Flags().StringVarP(&cfgPath, "config", "c", "", "path to config file")
	cmd.Flags().BoolVar(&backfill, "backfill", false, "sync all historic meetings")
	cmd.Flags().StringVar(&sinceStr, "since", "", "backfill meetings since date (YYYY-MM-DD)")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "show what would be synced without making changes")
	cmd.Flags().BoolVarP(&verbose, "verbose", "v", false, "enable verbose logging")
	return cmd
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
