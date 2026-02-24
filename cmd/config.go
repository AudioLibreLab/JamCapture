package cmd

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"

	"github.com/spf13/cobra"
)

var configCmd = &cobra.Command{
	Use:   "config",
	Short: "Manage configuration",
	Long:  `View and manage JamCapture configuration settings.`,
}

var configShowCmd = &cobra.Command{
	Use:   "show",
	Short: "Show current configuration",
	RunE: func(cmd *cobra.Command, args []string) error {
		out, err := yaml.Marshal(cfg)
		if err != nil {
			return fmt.Errorf("error marshaling config: %w", err)
		}
		fmt.Print(string(out))
		return nil
	},
}

var configEditCmd = &cobra.Command{
	Use:   "edit",
	Short: "Edit configuration file",
	RunE: func(cmd *cobra.Command, args []string) error {
		editor := os.Getenv("EDITOR")
		if editor == "" {
			editor = "nano"
		}

		configPath := os.ExpandEnv("$HOME/.config/jamcapture.yaml")
		fmt.Printf("Opening %s with %s...\n", configPath, editor)

		// This would need to be implemented with exec.Command
		return fmt.Errorf("edit command not yet implemented - please edit %s manually", configPath)
	},
}

func init() {
	configCmd.AddCommand(configShowCmd)
	configCmd.AddCommand(configEditCmd)
}
