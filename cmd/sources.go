package cmd

import (
	"fmt"
	"log/slog"
	"runtime"

	"github.com/audiolibrelab/jamcapture/internal/audio"
	"github.com/audiolibrelab/jamcapture/internal/config"

	"github.com/spf13/cobra"
)

var sourcesCmd = &cobra.Command{
	Use:   "sources",
	Short: "List available audio sources",
	Long:  `List all available audio sources that can be used for recording using the PipeWire backend.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		// Load current configuration to determine which backend(s) to show
		var cfg *config.Config
		var err error

		if cfgFile != "" {
			cfg, err = config.LoadWithProfile(cfgFile, profile)
		}

		if err != nil || cfgFile == "" {
			if err != nil {
				slog.Warn("Could not load configuration, using PipeWire backend", "error", err)
			}
			// Use default config with PipeWire backend
			cfg = &config.Config{
				Audio: config.AudioConfig{Backend: "pipewire"},
			}
		}

		return listAvailableSources(cfg)
	},
}

// listAvailableSources lists available audio sources using PipeWire backend
func listAvailableSources(cfg *config.Config) error {
	fmt.Printf("🎵 Audio Sources (%s)\n", runtime.GOOS)
	fmt.Printf("═══════════════════════════════════════\n\n")

	return listPipeWireSources()
}


// listPipeWireSources lists available PipeWire/JACK sources
func listPipeWireSources() error {
	backend := &audio.PipeWireBackend{}
	sources, err := backend.ListSources()
	if err != nil {
		return fmt.Errorf("failed to get PipeWire sources: %w", err)
	}

	fmt.Printf("📋 PIPEWIRE/JACK SOURCES (%d found):\n", len(sources))
	for i, source := range sources {
		fmt.Printf("  %d. %s\n", i+1, source)
	}

	fmt.Printf("\n💡 PipeWire Usage:\n")
	fmt.Printf("  • Format: \"Device: Audio (hw:X,Y):Z\" or \"Application:port\"\n")
	fmt.Printf("  • Example: \"Scarlett 2i2 USB: Audio (hw:1,0):0\"\n")
	fmt.Printf("  • Configure in channels[].sources: [\"Device: Audio (hw:1,0):0\"]\n\n")

	return nil
}

