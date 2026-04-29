package cmd

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"strconv"
	"sync"
	"syscall"
	"time"

	"github.com/audiolibrelab/jamcapture/internal/config"
	"github.com/audiolibrelab/jamcapture/internal/server"
	"github.com/audiolibrelab/jamcapture/internal/service"
	"github.com/audiolibrelab/jamcapture/internal/systray"
	"github.com/spf13/cobra"
)

var serveCmd = &cobra.Command{
	Use:   "serve",
	Short: "Start the web server with optional system tray",
	Long: `Start the JamCapture web server to control recording via a web interface.
This allows you to control recording from your smartphone or any device on the same network.

By default, if system tray is supported, it will also start a system tray icon for
desktop control. Use --no-tray to disable the system tray and run in headless mode.

The server will display the local network URL for easy access from mobile devices.`,
	RunE: runServe,
}

func init() {
	serveCmd.Flags().String("port", "8080", "port for the web server")
	serveCmd.Flags().Bool("no-tray", false, "disable system tray (run in headless mode)")
}

func runServe(cmd *cobra.Command, args []string) error {
	port, _ := cmd.Flags().GetString("port")
	noTray, _ := cmd.Flags().GetBool("no-tray")

	// Handle config file path - use default if not specified
	configPath := cfgFile
	if configPath == "" {
		configPath = os.ExpandEnv("$HOME/.config/jamcapture.yaml")
	}

	// Check system tray support
	traySupported := systray.IsSupportedVerbose()
	enableTray := traySupported && !noTray

	if noTray {
		slog.Info("System tray disabled by user request")
	} else if !traySupported {
		slog.Info("System tray not supported on this system, running in headless mode")
	}

	// Convert port to int for system tray
	portInt, err := strconv.Atoi(port)
	if err != nil {
		return fmt.Errorf("invalid port number: %s", port)
	}

	// Start server with optional system tray
	return startServer(configPath, portInt, enableTray)
}

// startServer starts the web server with optional system tray integration
func startServer(configPath string, port int, enableTray bool) error {
	portStr := fmt.Sprintf("%d", port)

	if enableTray {
		slog.Info("Starting JamCapture with system tray and web server", "port", port, "config", configPath)

		// Load configuration if not already loaded
		var currentCfg *config.Config
		var currentCfgFile string

		if cfg != nil {
			// Use global config if available
			currentCfg = cfg
			currentCfgFile = cfgFile
		} else {
			// Load config manually for system tray mode
			var err error
			currentCfg, err = config.LoadWithProfile(configPath, profile)
			if err != nil {
				return fmt.Errorf("failed to load config: %w", err)
			}
			currentCfgFile = configPath
		}

		// Create service for system tray
		svc := service.New(currentCfg, currentCfgFile, os.Stdout)

		var wg sync.WaitGroup
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		// Start web server in background
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := startWebServer(ctx, configPath, port); err != nil {
				slog.Error("Web server error", "error", err)
			}
		}()

		// Wait a moment for web server to start
		time.Sleep(500 * time.Millisecond)

		// Create system tray
		tray := systray.New(svc, port)

		// Handle graceful shutdown
		sigChan := make(chan os.Signal, 1)
		signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

		go func() {
			<-sigChan
			slog.Info("Received shutdown signal")
			cancel()
			// Quit system tray to unblock Run()
			tray.Shutdown()
		}()

		// Run system tray (this blocks until quit or shutdown)
		tray.Run()

		// Clean shutdown
		cancel()
		wg.Wait()

		slog.Info("JamCapture stopped")
		return nil

	} else {
		// Headless mode (original behavior)
		slog.Info("JamCapture web server starting (headless mode)", "port", port, "config", configPath)

		// Create and start the web server
		srv, err := server.New(configPath, portStr)
		if err != nil {
			return fmt.Errorf("failed to create server: %w", err)
		}

		// Start server (this blocks)
		if err := srv.Start(); err != nil {
			return fmt.Errorf("server failed: %w", err)
		}

		return nil
	}
}

// startWebServer starts the web server in the background
func startWebServer(ctx context.Context, configFile string, port int) error {
	portStr := fmt.Sprintf("%d", port)

	srv, err := server.New(configFile, portStr)
	if err != nil {
		return fmt.Errorf("failed to create server: %w", err)
	}

	// Start server in goroutine
	serverErr := make(chan error, 1)
	go func() {
		slog.Info("Starting web server", "port", port)
		if err := srv.Start(); err != nil {
			serverErr <- err
		}
	}()

	// Wait for context cancellation or server error
	select {
	case <-ctx.Done():
		slog.Info("Shutting down web server")
		return nil

	case err := <-serverErr:
		return err
	}
}