package cmd

import (
	"fmt"
	"github.com/audiolibrelab/jamcapture/internal/config"
	"github.com/audiolibrelab/jamcapture/internal/service"

	"github.com/spf13/cobra"
)

var mixCmd = &cobra.Command{
	Use:   "mix [song-name]",
	Short: "Mix recorded tracks with volume and delay adjustments",
	Long: `Mix the recorded guitar and backing tracks with configurable volume levels
and delay compensation for Bluetooth latency. Outputs a mixed FLAC file.`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		songName := args[0]

		// Create service instance
		svc := service.New(cfg, cfgFile, nil)

		// Get command line overrides
		guitarVol, _ := cmd.Flags().GetFloat64("guitar-volume")
		backingVol, _ := cmd.Flags().GetFloat64("backing-volume")
		delay, _ := cmd.Flags().GetInt("delay")

		// Display effective values
		currentCfg := svc.GetConfig()
		effectiveGuitarVol := getVolumeFromConfig(currentCfg, "guitar")
		effectiveBackingVol := getVolumeFromConfig(currentCfg, "monitor")
		effectiveDelay := currentCfg.GetChannelDelay("monitor")

		if guitarVol > 0 {
			effectiveGuitarVol = guitarVol
		}
		if backingVol > 0 {
			effectiveBackingVol = backingVol
		}
		if delay >= 0 {
			effectiveDelay = delay
		}

		fmt.Printf("Mixing song: %s\n", songName)
		fmt.Printf("Guitar volume: %.1f\n", effectiveGuitarVol)
		fmt.Printf("Backing volume: %.1f\n", effectiveBackingVol)
		fmt.Printf("Backing track delay: %dms\n", effectiveDelay)

		var err error
		if guitarVol > 0 || backingVol > 0 || delay >= 0 {
			err = svc.MixWithOptions(songName, guitarVol, backingVol, delay)
		} else {
			err = svc.Mix(songName)
		}

		if err != nil {
			return fmt.Errorf("mixing failed: %w", err)
		}

		fmt.Println("Mixing completed successfully")

		// Execute pipeline if specified
		return executePipeline(songName, 'm')
	},
}

func init() {
	mixCmd.Flags().Float64P("guitar-volume", "g", 0, "guitar volume (overrides config)")
	mixCmd.Flags().Float64P("backing-volume", "b", 0, "backing volume (overrides config)")
	mixCmd.Flags().IntP("delay", "d", -1, "backing track delay in ms (overrides config)")
}

// Helper function to get volume from new config format
func getVolumeFromConfig(cfg *config.Config, channelName string) float64 {
	return cfg.GetChannelVolume(channelName)
}