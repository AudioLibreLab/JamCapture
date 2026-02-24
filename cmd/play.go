package cmd

import (
	"fmt"
	"github.com/audiolibrelab/jamcapture/internal/service"

	"github.com/spf13/cobra"
)

var playCmd = &cobra.Command{
	Use:   "play [song-name]",
	Short: "Play the mixed audio file",
	Long: `Play the mixed FLAC file using the system's default audio player.
Will attempt to use VLC if available, otherwise falls back to system default.`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		songName := args[0]

		fmt.Printf("Playing song: %s\n", songName)

		// Create service instance
		svc := service.New(cfg, cfgFile, nil)

		err := svc.Play(songName)
		if err != nil {
			return fmt.Errorf("playback failed: %w", err)
		}

		return nil
	},
}