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
		if err := svc.StartReady(songName); err != nil {
			slog.Error("StartReady failed", "error", err)
			return fmt.Errorf("failed to start ready: %w", err)
		}

		slog.Info("Waiting for audio sources... Recording will start automatically - Press Ctrl+C to stop")

		// Handle interruption
		sigChan := make(chan os.Signal, 1)
		signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

		// Wait for interrupt signal
		<-sigChan
		slog.Info("Stopping recording...")

		// Stop recording
		if err := svc.StopRecording(); err != nil {
			return fmt.Errorf("failed to stop recording: %w", err)
		}

		// Execute pipeline if specified
		return executePipeline(songName, 'r')
	},
}

func init() {
	recordCmd.Flags().StringP("output", "o", "", "output directory (overrides config)")
}