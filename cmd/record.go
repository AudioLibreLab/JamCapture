package cmd

import (
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"github.com/audiolibrelab/jamcapture/internal/service"

	"github.com/spf13/cobra"
)

var recordCmd = &cobra.Command{
	Use:   "record [song-name]",
	Short: "Record guitar input and system audio",
	Long: `Record audio from guitar input and system audio monitor simultaneously.
The recording will be saved as an MKV file with separate tracks for guitar and backing audio.`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		songName := args[0]
		slog.Info("Record command started", "song_name", songName)

		// Create log writer based on verbose level
		var logWriter *os.File
		if verboseLevel >= 1 {
			logWriter = os.Stderr
		}

		// Create service instance
		slog.Debug("Creating service instance")
		svc := service.New(cfg, cfgFile, logWriter)

		// Start ready state - recording will start automatically when sources are available
		slog.Info("Calling StartReady to begin source monitoring")
		effectiveName, err := svc.StartReady(songName)
		if err != nil {
			slog.Error("StartReady failed", "error", err)
			return fmt.Errorf("failed to start ready: %w", err)
		}
		// Follow the rename when the requested name was already taken, so the
		// auto-mix below operates on the file we just recorded.
		songName = effectiveName

		slog.Info("Waiting for audio sources... Recording will start automatically - Press Ctrl+C to stop")

		// Handle interruption
		sigChan := make(chan os.Signal, 1)
		signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

		// Wait for interrupt signal
		<-sigChan
		slog.Info("Stopping recording...")

		// Check current status and stop appropriately
		status, _ := svc.GetRecordingStatus()
		switch status {
		case service.StatusRecording:
			// Recording in progress, stop recording
			if err := svc.StopRecording(); err != nil {
				return fmt.Errorf("failed to stop recording: %w", err)
			}
			slog.Info("Recording stopped successfully")

			// Auto-mix the recording
			slog.Info("Auto-mixing recording...", "song", songName)
			if err := svc.Mix(songName); err != nil {
				slog.Error("Auto-mix failed", "error", err)
				// Don't return error, the recording was saved successfully
			} else {
				slog.Info("Auto-mix completed successfully", "song", songName)
			}

		case service.StatusReady:
			// Ready state (waiting for sources), cancel ready
			if err := svc.CancelReady(); err != nil {
				return fmt.Errorf("failed to cancel ready state: %w", err)
			}
			slog.Info("Ready state cancelled, returned to standby")

		default:
			slog.Info("No recording in progress, nothing to stop")
		}

		// Execute pipeline if specified
		return executePipeline(songName, 'r')
	},
}

func init() {
	recordCmd.Flags().StringP("output", "o", "", "output directory (overrides config)")
}