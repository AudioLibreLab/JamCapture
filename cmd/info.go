package cmd

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
)

var infoCmd = &cobra.Command{
	Use:   "info [song-name]",
	Short: "Show resolved configuration and file paths for a song",
	Long:  `Display the resolved configuration with inheritance indicators and file paths for the given song name. Shows which values are inherited from default vs profile-specific.`,
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		songName := args[0]

		// Clean song name using the same logic as the recorder
		cleanName := cleanFileName(songName)

		// Build file paths
		mkvPath := filepath.Join(cfg.Output.Directory, cleanName+".mkv")
		mixedPath := filepath.Join(cfg.Output.Directory, cleanName+"."+cfg.Output.Format)

		// Display file paths
		fmt.Printf("=== FILE PATHS ===\n")
		fmt.Printf("output_mkv: %s\n", mkvPath)
		fmt.Printf("output_mixed: %s\n", mixedPath)
		fmt.Printf("clean_name: %s\n", cleanName)

		// Display resolved configuration with inheritance indicators
		fmt.Printf("\n=== RESOLVED CONFIGURATION ===\n")

		// Audio configuration
		fmt.Printf("\n[Audio]\n")
		fmt.Printf("sample_rate: %d %s\n", cfg.Audio.SampleRate, getInheritanceIndicator(cfg.Inheritance.Audio.SampleRate))
		fmt.Printf("interface: %s %s\n", cfg.Audio.Interface, getInheritanceIndicator(cfg.Inheritance.Audio.Interface))

		// Channels configuration
		fmt.Printf("\n[Channels]\n")
		for i, channel := range cfg.Channels {
			channelInheritance := cfg.Inheritance.Channels[channel.Name]
			fmt.Printf("%d. name: %s\n", i, channel.Name)
			fmt.Printf("   sources: %s %s\n", strings.Join(channel.Sources, ", "), getInheritanceIndicator(channelInheritance.Source))
			fmt.Printf("   audioMode: %s\n", channel.AudioMode)
			fmt.Printf("   type: %s %s\n", channel.Type, getInheritanceIndicator(channelInheritance.Type))
		}

		// Mix configuration (now part of channels)
		fmt.Printf("\n[Mix]\n")
		fmt.Printf("channels:\n")
		for _, channel := range cfg.Channels {
			inheritanceStatus := cfg.Inheritance.Channels[channel.Name]
			fmt.Printf("  %s: volume=%.1f %s, delay=%d %s\n",
				channel.Name, channel.Volume, getInheritanceIndicator(inheritanceStatus.Volume),
				channel.Delay, getInheritanceIndicator(inheritanceStatus.Delay))
		}

		// Output configuration
		fmt.Printf("\n[Output]\n")
		fmt.Printf("directory: %s %s\n", cfg.Output.Directory, getInheritanceIndicator(cfg.Inheritance.Output.Directory))
		fmt.Printf("format: %s %s\n", cfg.Output.Format, getInheritanceIndicator(cfg.Inheritance.Output.Format))

		return nil
	},
}

// cleanFileName replicates the logic from the recorder
func cleanFileName(name string) string {
	// Remove special characters and replace spaces with underscores
	// Allows: letters, numbers, spaces, hyphens, underscores
	var result strings.Builder
	for _, r := range name {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == ' ' || r == '-' || r == '_' {
			result.WriteRune(r)
		}
	}
	return strings.ReplaceAll(strings.TrimSpace(result.String()), " ", "_")
}

// getInheritanceIndicator returns a formatted indicator for inheritance status
func getInheritanceIndicator(status string) string {
	switch status {
	case "inherited":
		return "[inherited]"
	case "profile-specific":
		return "[profile-specific]"
	default:
		return "[unknown]"
	}
}

func init() {
	rootCmd.AddCommand(infoCmd)
}