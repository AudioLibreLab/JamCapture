package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/audiolibrelab/jamcapture/internal/audio"
	"github.com/audiolibrelab/jamcapture/internal/config"
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

var configInitCmd = &cobra.Command{
	Use:   "init",
	Short: "Auto-detect audio sources and write a starter configuration",
	Long: `Detect available PipeWire/JACK audio sources and generate a ready-to-use
configuration file at ~/.config/jamcapture.yaml.

If a configuration file already exists it is backed up with a timestamp suffix
before the new one is written.

After running this command you can:
  - List detected sources:  jamcapture sources
  - Inspect the config:     jamcapture config show
  - Start the server:       jamcapture serve`,
	// Override PersistentPreRunE so this command works without a pre-existing config.
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		setupLogging(verboseLevel)
		return nil
	},
	RunE: func(cmd *cobra.Command, args []string) error {
		dest := os.ExpandEnv("$HOME/.config/jamcapture.yaml")

		// Back up existing config
		if _, err := os.Stat(dest); err == nil {
			backup := dest + "." + time.Now().Format("20060102T150405") + ".bak"
			if err := os.Rename(dest, backup); err != nil {
				return fmt.Errorf("cannot back up existing config to %s: %w", backup, err)
			}
			fmt.Printf("Existing config backed up to: %s\n", backup)
		}

		if err := autoInitConfig(dest); err != nil {
			return err
		}

		fmt.Printf("Configuration written to: %s\n\n", dest)
		fmt.Println("Next steps:")
		fmt.Println("  jamcapture sources       — verify detected sources")
		fmt.Println("  jamcapture config show   — inspect resolved config")
		fmt.Println("  jamcapture serve         — start the web server")
		return nil
	},
}

// autoInitConfig detects audio sources and writes a generated config to configPath.
// Called by configInitCmd and by runServe on first startup.
func autoInitConfig(configPath string) error {
	backend := &audio.PipeWireBackend{}
	ports, err := backend.ListSources()
	if err != nil {
		return fmt.Errorf("failed to list audio sources (is PipeWire running?): %w", err)
	}

	hwInputs, swSources := audio.CategorizeAndGroup(ports)
	devices, genericPorts := config.IdentifyDevices(hwInputs)
	running := config.MatchSoftwareSources(swSources)

	// Detection summary
	fmt.Println("Sources détectées :")
	for _, dev := range devices {
		n := 0
		for _, ch := range dev.Template.Channels {
			if dev.ActualPorts[ch.PortSuffix] {
				n++
			}
		}
		fmt.Printf("  Matériel : %s (%d canaux)\n", dev.Template.DisplayName, n)
	}
	if len(genericPorts) > 0 {
		fmt.Printf("  Matériel inconnu : %d port(s)\n", len(genericPorts))
	}
	for _, sw := range running {
		fmt.Printf("  Logiciel  : %s (%s)\n", sw.AppName, sw.AudioMode)
	}
	if len(devices) == 0 && len(genericPorts) == 0 && len(running) == 0 {
		return fmt.Errorf("no audio sources detected — is PipeWire running and any device connected?")
	}

	root := config.GenerateConfig(devices, running, genericPorts)
	if root == nil {
		return fmt.Errorf("no audio sources detected — is PipeWire running and any device connected?")
	}

	// Profile summary
	names := make([]string, 0, len(root.Configs))
	for name := range root.Configs {
		names = append(names, name)
	}
	sort.Strings(names)
	fmt.Printf("\nProfils générés  : %s\n", strings.Join(names, ", "))
	fmt.Printf("Config active    : %s\n", root.ActiveConfig)

	data, err := yaml.Marshal(root)
	if err != nil {
		return fmt.Errorf("cannot serialize config: %w", err)
	}

	if err := os.MkdirAll(filepath.Dir(configPath), 0o755); err != nil {
		return fmt.Errorf("cannot create config directory: %w", err)
	}

	if err := os.WriteFile(configPath, data, 0o644); err != nil {
		return fmt.Errorf("cannot write config to %s: %w", configPath, err)
	}

	return nil
}

func init() {
	configCmd.AddCommand(configShowCmd)
	configCmd.AddCommand(configEditCmd)
	configCmd.AddCommand(configInitCmd)
}
