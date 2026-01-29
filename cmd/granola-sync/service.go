package main

import (
	"bufio"
	"fmt"
	"io"
	"os"

	"github.com/spf13/cobra"

	"github.com/philrhinehart/granola-sync/internal/service"
)

func newStartCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "start",
		Short: "Install and start the launchd service",
		Long:  "Install the launchd plist and start granola-sync as a background service.",
		RunE:  runStart,
	}
}

func newStopCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "stop",
		Short: "Stop the launchd service",
		RunE:  runStop,
	}
}

func newStatusCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Show service status",
		RunE:  runStatus,
	}
}

func newLogsCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "logs",
		Short: "Tail the service logs",
		RunE:  runLogs,
	}
}

func newUnloadCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "unload",
		Short: "Unload and remove the launchd service",
		RunE:  runUnload,
	}
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
	defer func() { _ = file.Close() }()

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
