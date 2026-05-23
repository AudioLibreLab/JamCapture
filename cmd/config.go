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
  - Add software sources: jamcapture config initsoftware
  - List detected sources:  jamcapture sources
  - Inspect the config:     jamcapture config show
  - Start the server:       jamcapture serve`,
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		setupLogging(verboseLevel)
		return nil
	},
	RunE: func(cmd *cobra.Command, args []string) error {
		dest := os.ExpandEnv("$HOME/.config/jamcapture.yaml")

		if _, err := os.Stat(dest); err == nil {
			backup := dest + "." + time.Now().Format("20060102T150405") + ".bak"
			if err := os.Rename(dest, backup); err != nil {
				return fmt.Errorf("cannot back up existing config to %s: %w", backup, err)
			}
			fmt.Printf("Existing config backed up to: %s\n", backup)
		}

		if err := runInitHardware(dest); err != nil {
			return err
		}

		fmt.Printf("\nConfiguration written to: %s\n\n", dest)
		fmt.Println("Next steps:")
		fmt.Println("  jamcapture config initsoftware   — add Chrome/Spotify sources")
		fmt.Println("  jamcapture sources               — verify detected sources")
		fmt.Println("  jamcapture config show           — inspect resolved config")
		fmt.Println("  jamcapture serve                 — start the web server")
		return nil
	},
}

var configInitHardwareCmd = &cobra.Command{
	Use:   "inithardware",
	Short: "Detect hardware audio sources and write/update configuration",
	Long: `Detect hardware PipeWire/JACK audio sources and generate a configuration
file at ~/.config/jamcapture.yaml. Uses pw-dump for accurate device names.

To also detect software sources (Chrome, Spotify…):
  jamcapture config initsoftware`,
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		setupLogging(verboseLevel)
		return nil
	},
	RunE: func(cmd *cobra.Command, args []string) error {
		return runInitHardware(os.ExpandEnv("$HOME/.config/jamcapture.yaml"))
	},
}

var (
	initSoftwareNoWait  bool
	initSoftwareTimeout time.Duration
	initSoftwareProfile string
)

var configInitSoftwareCmd = &cobra.Command{
	Use:   "initsoftware",
	Short: "Detect running software audio sources and merge into configuration",
	Long: `Detect software audio sources (Chrome, Spotify, VLC…) that are currently
playing and merge them into the existing configuration.

Start playing your music or open YouTube first, then run this command.
The command polls for audio sources during the detection window (default 10s).

Use --profile to restrict updates to a single hardware profile:
  jamcapture config initsoftware --profile xr18_studio

Use --no-wait to detect only what is playing right now (useful in scripts).`,
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		setupLogging(verboseLevel)
		return nil
	},
	RunE: func(cmd *cobra.Command, args []string) error {
		dest := os.ExpandEnv("$HOME/.config/jamcapture.yaml")
		return runInitSoftware(dest, initSoftwareNoWait, initSoftwareTimeout, initSoftwareProfile)
	},
}

// autoInitConfig detects audio sources and writes a generated config to configPath.
// Called by configInitCmd and by runServe on first startup.
func autoInitConfig(configPath string) error {
	return runInitHardware(configPath)
}

// runInitHardware detects hardware audio sources and writes a fresh config to dest.
func runInitHardware(dest string) error {
	backend := &audio.PipeWireBackend{}
	ports, err := backend.ListSources()
	if err != nil {
		return fmt.Errorf("failed to list audio sources (is PipeWire running?): %w", err)
	}

	hwInputs, swSources := audio.CategorizeAndGroup(ports)
	devices, genericRawPorts := config.IdentifyDevices(hwInputs)
	running := config.MatchSoftwareSources(swSources)

	// Enrich unmatched hardware ports with device names from pw-dump
	descMap := audio.LookupPortDescriptions(genericRawPorts)
	namedPorts := make([]config.NamedPort, len(genericRawPorts))
	for i, g := range genericRawPorts {
		name := ""
		if len(g) > 0 {
			name = descMap[g[0]]
		}
		namedPorts[i] = config.NamedPort{Ports: g, DeviceName: name}
	}

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
	// Group unknown ports by device name
	unknownByDevice := make(map[string]int)
	var unknownOrder []string
	for _, p := range namedPorts {
		key := p.DeviceName
		if _, exists := unknownByDevice[key]; !exists {
			unknownOrder = append(unknownOrder, key)
		}
		unknownByDevice[key]++
	}
	for _, name := range unknownOrder {
		count := unknownByDevice[name]
		if name == "" {
			fmt.Printf("  Matériel inconnu : %d port(s)\n", count)
		} else {
			fmt.Printf("  Matériel : %s (%d port(s))\n", name, count)
		}
	}
	for _, sw := range running {
		fmt.Printf("  Logiciel  : %s (%s)\n", sw.AppName, sw.AudioMode)
	}
	if len(devices) == 0 && len(namedPorts) == 0 && len(running) == 0 {
		return fmt.Errorf("no audio sources detected — is PipeWire running and any device connected?")
	}

	root := config.GenerateConfig(devices, running, namedPorts)
	if root == nil {
		return fmt.Errorf("no audio sources detected — is PipeWire running and any device connected?")
	}

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
	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		return fmt.Errorf("cannot create config directory: %w", err)
	}
	if err := os.WriteFile(dest, data, 0o644); err != nil {
		return fmt.Errorf("cannot write config to %s: %w", dest, err)
	}
	return nil
}

// runInitSoftware detects currently-running software audio sources and merges them into dest.
func runInitSoftware(dest string, noWait bool, timeout time.Duration, profileFilter string) error {
	root, err := loadOrCreateRoot(dest)
	if err != nil {
		return err
	}

	existing := make(map[string]bool)
	if root.Definitions != nil {
		for _, ch := range root.Definitions.Channels {
			existing[ch.ID] = true
		}
	}
	if root.Definitions == nil {
		root.Definitions = &config.DefinitionsConfig{}
	}
	if root.Configs == nil {
		root.Configs = make(map[string]*config.ConfigProfile)
	}

	backend := &audio.PipeWireBackend{}
	accumulated := make(map[string]config.SoftwareSourceTemplate)

	detect := func() {
		ports, err := backend.ListSources()
		if err != nil {
			return
		}
		_, swSources := audio.CategorizeAndGroup(ports)
		for _, sw := range append(config.MatchSoftwareSources(swSources), config.GenericSoftwareSources(swSources)...) {
			accumulated[sw.ID] = sw
		}
	}

	if noWait {
		detect()
	} else {
		fmt.Printf("Lancez votre musique (Chrome, Spotify…) — détection pendant %.0fs", timeout.Seconds())
		ticker := time.NewTicker(500 * time.Millisecond)
		deadline := time.NewTimer(timeout)
	loop:
		for {
			select {
			case <-deadline.C:
				break loop
			case <-ticker.C:
				detect()
				fmt.Print(".")
			}
		}
		ticker.Stop()
		deadline.Stop()
		fmt.Println()
	}

	if len(accumulated) == 0 {
		fmt.Println("Aucune source logicielle détectée.")
		return nil
	}

	var newRefs []config.ChannelReference
	for _, sw := range accumulated {
		if existing[sw.ID] {
			fmt.Printf("  Déjà présent : %s\n", sw.AppName)
			continue
		}
		sources := []string{sw.PortFL}
		if sw.AudioMode == "stereo" && sw.PortFR != "" {
			sources = []string{sw.PortFL, sw.PortFR}
		}
		root.Definitions.Channels = append(root.Definitions.Channels, config.ChannelDefinition{
			ID:        sw.ID,
			Name:      sw.Name,
			Sources:   sources,
			AudioMode: sw.AudioMode,
			Type:      "software",
			Volume:    sw.Volume,
			Delay:     sw.Delay,
		})
		newRefs = append(newRefs, config.ChannelReference{Ref: sw.ID})
		fmt.Printf("  Ajouté : %s (%s)\n", sw.AppName, sw.AudioMode)
	}

	if len(newRefs) > 0 {
		updateWithMonitorProfiles(root, newRefs, profileFilter)
	}

	data, err := yaml.Marshal(root)
	if err != nil {
		return fmt.Errorf("cannot serialize config: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		return fmt.Errorf("cannot create config directory: %w", err)
	}
	if err := os.WriteFile(dest, data, 0o644); err != nil {
		return fmt.Errorf("cannot write config to %s: %w", dest, err)
	}
	fmt.Printf("\nConfiguration mise à jour : %s\n", dest)
	return nil
}

// updateWithMonitorProfiles creates or updates _with_monitor profiles with the new software refs.
// If profileFilter is set, only that profile's _with_monitor variant is updated.
func updateWithMonitorProfiles(root *config.RootConfig, newRefs []config.ChannelReference, profileFilter string) {
	if profileFilter != "" {
		base, ok := root.Configs[profileFilter]
		if !ok {
			fmt.Printf("  Profil '%s' introuvable, aucun profil mis à jour.\n", profileFilter)
			return
		}
		monitorName := strings.TrimSuffix(profileFilter, "_studio") + "_with_monitor"
		updateOrCreateMonitorProfile(root, monitorName, base.Channels, newRefs)
		return
	}

	for name, profile := range root.Configs {
		if !strings.HasSuffix(name, "_studio") {
			continue
		}
		monitorName := strings.TrimSuffix(name, "_studio") + "_with_monitor"
		updateOrCreateMonitorProfile(root, monitorName, profile.Channels, newRefs)
	}
}

func updateOrCreateMonitorProfile(root *config.RootConfig, monitorName string, baseChannels []config.ChannelReference, newRefs []config.ChannelReference) {
	if existing, ok := root.Configs[monitorName]; ok {
		existingSet := make(map[string]bool)
		for _, r := range existing.Channels {
			existingSet[r.Ref] = true
		}
		for _, r := range newRefs {
			if !existingSet[r.Ref] {
				existing.Channels = append(existing.Channels, r)
			}
		}
	} else {
		refs := append(append([]config.ChannelReference{}, baseChannels...), newRefs...)
		root.Configs[monitorName] = &config.ConfigProfile{
			AutoMix:  true,
			Channels: refs,
			Output:   config.OutputConfig{Format: "flac"},
		}
	}
	fmt.Printf("  Profil mis à jour : %s\n", monitorName)
}

// loadOrCreateRoot loads an existing config file, or returns a minimal skeleton if none exists.
func loadOrCreateRoot(dest string) (*config.RootConfig, error) {
	if _, err := os.Stat(dest); err == nil {
		root, err := config.ValidateConfigurationFormat(dest)
		if err != nil {
			return nil, fmt.Errorf("cannot read existing config %s: %w", dest, err)
		}
		return root, nil
	}
	return &config.RootConfig{
		ActiveConfig: "default",
		Audio: &config.AudioConfig{
			Backend:    "pipewire",
			SampleRate: 48000,
		},
		Globals: &config.GlobalsConfig{
			Output: config.GlobalOutputConfig{
				RecordingsDirectory:    "~/Audio/JamCapture/Recordings",
				BackingtracksDirectory: "~/Audio/JamCapture/BackingTracks",
			},
		},
		Definitions:              &config.DefinitionsConfig{},
		Configs:                  make(map[string]*config.ConfigProfile),
		SupportedAudioExtensions: []string{"flac", "wav", "mp3"},
	}, nil
}

func init() {
	configCmd.AddCommand(configShowCmd)
	configCmd.AddCommand(configEditCmd)
	configCmd.AddCommand(configInitCmd)
	configCmd.AddCommand(configInitHardwareCmd)
	configInitSoftwareCmd.Flags().BoolVar(&initSoftwareNoWait, "no-wait", false, "detect immediately without waiting")
	configInitSoftwareCmd.Flags().DurationVar(&initSoftwareTimeout, "timeout", 10*time.Second, "detection window duration")
	configInitSoftwareCmd.Flags().StringVar(&initSoftwareProfile, "profile", "", "restrict software sources to a single hardware profile (e.g. xr18_studio)")
	configCmd.AddCommand(configInitSoftwareCmd)
}
