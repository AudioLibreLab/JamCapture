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

	// Auto-generate config if absent or invalid
	if err := ensureValidConfig(configPath); err != nil {
		return err
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

		// Create web server first (it owns the service)
		portStr := fmt.Sprintf("%d", port)
		srv, err := server.New(configPath, portStr)
		if err != nil {
			return fmt.Errorf("failed to create server: %w", err)
		}

		// Get the service instance from the server
		svc := srv.GetService()

		var wg sync.WaitGroup
		_, cancel := context.WithCancel(context.Background())
		defer cancel()

		// Start web server in background
		wg.Add(1)
		go func() {
			defer wg.Done()
			slog.Info("Starting web server", "port", port)
			if err := srv.Start(); err != nil {
				slog.Error("Web server error", "error", err)
			}
		}()

		// Wait a moment for web server to start
		time.Sleep(500 * time.Millisecond)

		// Create system tray using the shared service from the server
		tray := systray.New(svc, port)

		// Handle graceful shutdown
		sigChan := make(chan os.Signal, 1)
		signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

		go func() {
			<-sigChan
			slog.Info("Received shutdown signal")
			cancel()
			// Shutdown web server first
			srv.Shutdown()
			// Quit system tray to unblock Run()
			tray.Shutdown()
		}()

		// Run system tray (this blocks until quit button or external signal)
		tray.Run()

		// Tray exited — shut down web server and wait
		srv.Shutdown()
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

// ensureValidConfig generates a config if absent. If the file exists but is
// invalid it returns a clear error — the user should fix it or run
// 'jamcapture config init' to regenerate.
func ensureValidConfig(configPath string) error {
	_, statErr := os.Stat(configPath)
	if statErr == nil {
		if _, valErr := config.ValidateConfigurationFormat(configPath); valErr != nil {
			return fmt.Errorf("invalid config file %s: %w\nFix it manually or run 'jamcapture config init' to regenerate", configPath, valErr)
		}
		return nil
	}
	if !os.IsNotExist(statErr) {
		return fmt.Errorf("cannot access config file: %w", statErr)
	}

	slog.Info("No config file found — auto-detecting audio sources", "path", configPath)
	if initErr := autoInitConfig(configPath); initErr != nil {
		return fmt.Errorf("auto-config failed: %w\nRun 'jamcapture config init' or create %s manually", initErr, configPath)
	}
	slog.Info("Config written from detected sources", "path", configPath)
	fmt.Printf("Config written to %s — edit it to customise volumes and profiles.\n", configPath)
	return nil
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

