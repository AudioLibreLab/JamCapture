package cmd

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"github.com/audiolibrelab/jamcapture/internal/config"
	"github.com/audiolibrelab/jamcapture/internal/service"
	"strings"

	"github.com/spf13/cobra"
)

var runCmd = &cobra.Command{
	Use:   "run [song-name]",
	Short: "Execute pipeline steps on a song",
	Long: `Execute the specified pipeline steps on a song. Use -p to specify which steps to run.
If no pipeline is specified, will try to infer the action based on existing files.`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		songName := args[0]

		if pipeline == "" {
			return fmt.Errorf("no pipeline specified, use -p flag (e.g., -p rmp)")
		}

		steps := []rune(strings.ToLower(pipeline))

		// Create log writer based on verbose level
		var logWriter io.Writer = io.Discard
		if verboseLevel >= 1 {
			logWriter = os.Stderr
		}

		// Create service instance
		svc := service.New(cfg, cfgFile, logWriter)

		for i, step := range steps {
			fmt.Printf("Pipeline: executing step %d/%d: '%c'...\n", i+1, len(steps), step)

			switch step {
			case 'r':
				if err := svc.StartReady(songName); err != nil {
					return fmt.Errorf("pipeline ready failed: %w", err)
				}

				// Wait for user input to stop recording
				fmt.Println("Pipeline: waiting for sources... Recording will start automatically - Press Enter to stop...")
				scanner := bufio.NewScanner(os.Stdin)
				scanner.Scan()

				if err := svc.StopRecording(); err != nil {
					return fmt.Errorf("pipeline record stop failed: %w", err)
				}
				fmt.Println("Pipeline: recording completed")

			case 'm':
				// Get command line overrides for mix
				guitarVol, _ := cmd.Flags().GetFloat64("guitar-volume")
				backingVol, _ := cmd.Flags().GetFloat64("backing-volume")
				delay, _ := cmd.Flags().GetInt("delay")

				// Display effective values
				currentCfg := svc.GetConfig()
				effectiveGuitarVol := getRunVolumeFromConfig(currentCfg, "guitar")
				effectiveBackingVol := getRunVolumeFromConfig(currentCfg, "monitor")
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
					return fmt.Errorf("pipeline mix failed: %w", err)
				}
				fmt.Println("Pipeline: mixing completed")

			case 'p':
				fmt.Printf("Playing song: %s\n", songName)
				if err := svc.Play(songName); err != nil {
					return fmt.Errorf("pipeline play failed: %w", err)
				}
				fmt.Println("Pipeline: playback completed")

			default:
				return fmt.Errorf("unknown pipeline step: '%c' (valid: r=record, m=mix, p=play)", step)
			}
		}

		return nil
	},
}

func init() {
	// Add mix-specific flags to run command
	runCmd.Flags().Float64P("guitar-volume", "g", 0, "guitar volume (overrides config)")
	runCmd.Flags().Float64P("backing-volume", "b", 0, "backing volume (overrides config)")
	runCmd.Flags().IntP("delay", "d", -1, "backing track delay in ms (overrides config)")
	runCmd.Flags().StringP("output", "o", "", "output directory (overrides config)")
}

// Helper function to get volume from new config format for run command
func getRunVolumeFromConfig(cfg *config.Config, channelName string) float64 {
	return cfg.GetChannelVolume(channelName)
}