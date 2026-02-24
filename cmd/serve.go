package cmd

import (
	"fmt"
	"log/slog"
	"os"

	"github.com/audiolibrelab/jamcapture/internal/server"
	"github.com/spf13/cobra"
)

var serveCmd = &cobra.Command{
	Use:   "serve",
	Short: "Start the web server for remote control",
	Long: `Start the JamCapture web server to control recording via a web interface.
This allows you to control recording from your smartphone or any device on the same network.

The server will display the local network URL for easy access from mobile devices.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		port, _ := cmd.Flags().GetString("port")

		// Handle config file path - use default if not specified
		configPath := cfgFile
		if configPath == "" {
			configPath = os.ExpandEnv("$HOME/.config/jamcapture.yaml")
		}

		// Create and start the web server
		srv, err := server.New(configPath, port)
		if err != nil {
			return fmt.Errorf("failed to create server: %w", err)
		}

		slog.Info("JamCapture web server starting", "port", port, "config", configPath)

		// Start server (this blocks)
		if err := srv.Start(); err != nil {
			return fmt.Errorf("server failed: %w", err)
		}

		return nil
	},
}

func init() {
	serveCmd.Flags().String("port", "8080", "port for the web server")
}