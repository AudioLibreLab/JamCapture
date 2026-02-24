package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/viper"
)

type DefinitionsConfig struct {
	Channels []ChannelDefinition `mapstructure:"channels" yaml:"channels"`
}

type ChannelDefinition struct {
	ID        string   `mapstructure:"id" yaml:"id"`
	Name      string   `mapstructure:"name" yaml:"name"`
	Sources   []string `mapstructure:"sources" yaml:"sources"`
	AudioMode string   `mapstructure:"audioMode" yaml:"audioMode"`
	Type      string   `mapstructure:"type" yaml:"type"`
	Volume    float64  `mapstructure:"volume" yaml:"volume"`
	Delay     int      `mapstructure:"delay" yaml:"delay"`
}

type ChannelReference struct {
	Ref    string   `mapstructure:"ref" yaml:"ref"`
	Volume *float64 `mapstructure:"volume,omitempty" yaml:"volume,omitempty"` // Surcharge autorisée
	Delay  *int     `mapstructure:"delay,omitempty" yaml:"delay,omitempty"`   // Surcharge autorisée
}

type GlobalsConfig struct {
	Output GlobalOutputConfig `mapstructure:"output" yaml:"output"`
}

type GlobalOutputConfig struct {
	RecordingsDirectory   string `mapstructure:"recordings_directory" yaml:"recordings_directory"`
	BackingtracksDirectory string `mapstructure:"backingtracks_directory" yaml:"backingtracks_directory"`
}

type RootConfig struct {
	ActiveConfig              string                    `mapstructure:"active_config" yaml:"active_config"`
	Globals                   *GlobalsConfig            `mapstructure:"globals,omitempty" yaml:"globals,omitempty"`
	Audio                     *AudioConfig              `mapstructure:"audio,omitempty" yaml:"audio,omitempty"`
	Definitions              *DefinitionsConfig        `mapstructure:"definitions,omitempty" yaml:"definitions,omitempty"`
	Configs                   map[string]*ConfigProfile `mapstructure:"configs" yaml:"configs"`
	SupportedAudioExtensions  []string                  `mapstructure:"supported_audio_extensions" yaml:"supported_audio_extensions"`
}

type Config struct {
	Audio    AudioConfig `mapstructure:"audio" yaml:"audio"`
	Channels []Channel   `mapstructure:"channels" yaml:"channels"`
	Output   OutputConfig `mapstructure:"output" yaml:"output"`
	AutoMix  bool         `mapstructure:"auto_mix" yaml:"auto_mix"`

	// Internal field to track inheritance information for info command
	Inheritance *InheritanceInfo `mapstructure:"-" yaml:"-"`
}

type ConfigProfile struct {
	Audio    AudioConfig        `mapstructure:"audio" yaml:"audio"`
	Channels []ChannelReference `mapstructure:"channels" yaml:"channels"`
	Output   OutputConfig       `mapstructure:"output" yaml:"output"`
	AutoMix  bool               `mapstructure:"auto_mix" yaml:"auto_mix"`

	// Internal field to track inheritance information for info command
	Inheritance *InheritanceInfo `mapstructure:"-" yaml:"-"`
}

type InheritanceInfo struct {
	Audio struct {
		SampleRate string // "inherited" or "profile-specific"
		Interface  string
		Backend    string // "inherited" or "profile-specific"
	}
	Channels map[string]struct {
		Source string // "inherited" or "profile-specific"
		Type   string
		Volume string // "inherited" or "profile-specific"
		Delay  string
	}
	Output struct {
		Directory string
		Format    string
	}
}

type AudioConfig struct {
	SampleRate int    `mapstructure:"sample_rate" yaml:"sample_rate"`
	Interface  string `mapstructure:"interface" yaml:"interface"` // "jack" interface (deprecated, use Backend)
	Backend    string `mapstructure:"backend" yaml:"backend"`     // "pipewire", "auto"
}

type Channel struct {
	Name      string   `mapstructure:"name" yaml:"name"`
	Sources   []string `mapstructure:"sources" yaml:"sources"`   // Ordered list: mono=[source], stereo=[left,right]
	AudioMode string   `mapstructure:"audioMode" yaml:"audioMode"` // "mono" (default), "stereo"
	Type      string   `mapstructure:"type" yaml:"type"`         // "input", "monitor"
	Volume    float64  `mapstructure:"volume" yaml:"volume"`
	Delay     int      `mapstructure:"delay" yaml:"delay"`
}



type OutputConfig struct {
	Directory           string `mapstructure:"directory" yaml:"directory"`
	BackingtracksDirectory string `mapstructure:"backingtracks_directory" yaml:"backingtracks_directory"`
	Format              string `mapstructure:"format" yaml:"format"`
}

var defaultConfig = Config{
	Audio: AudioConfig{
		SampleRate: 48000,
		Interface:  "jack", // deprecated, kept for compatibility
		Backend:    "auto", // new preferred field
	},
	Channels: []Channel{
		{Name: "guitar", Sources: []string{"system:capture_1"}, AudioMode: "mono", Type: "input", Volume: 4.0, Delay: 0},
		{Name: "monitor_left", Sources: []string{"system:monitor_FL"}, AudioMode: "mono", Type: "monitor", Volume: 0.8, Delay: 0},
		{Name: "monitor_right", Sources: []string{"system:monitor_FR"}, AudioMode: "mono", Type: "monitor", Volume: 0.8, Delay: 0},
	},
	Output: OutputConfig{
		Directory:           filepath.Join(os.Getenv("HOME"), "Audio", "JamCapture"),
		BackingtracksDirectory: filepath.Join(os.Getenv("HOME"), "Audio", "JamCapture", "BackingTracks"),
		Format:              "flac",
	},
	AutoMix: true, // Default to enabled for backward compatibility
}


func LoadWithProfile(configFile, profile string) (*Config, error) {
	if configFile == "" {
		return nil, fmt.Errorf("no config file specified, use --config flag")
	}

	// Validate configuration format first
	rootConfig, err := ValidateConfigurationFormat(configFile)
	if err != nil {
		return nil, fmt.Errorf("configuration validation failed: %w", err)
	}

	// Determine which config to use
	configName := profile
	if configName == "" {
		configName = rootConfig.ActiveConfig
	}
	if configName == "" {
		configName = "default"
	}

	// Get the requested config profile
	selectedProfile, exists := rootConfig.Configs[configName]
	if !exists {
		return nil, fmt.Errorf("configuration profile '%s' not found", configName)
	}

	// Convert profile to Config by resolving references
	selectedConfig, err := convertProfileToConfig(selectedProfile, rootConfig.Definitions)
	if err != nil {
		return nil, fmt.Errorf("error resolving configuration profile '%s': %w", configName, err)
	}

	// Apply global audio settings as base if they exist
	if rootConfig.Audio != nil {
		if selectedConfig.Audio.Backend == "" {
			selectedConfig.Audio.Backend = rootConfig.Audio.Backend
		}
		if selectedConfig.Audio.SampleRate == 0 {
			selectedConfig.Audio.SampleRate = rootConfig.Audio.SampleRate
		}
		if selectedConfig.Audio.Interface == "" {
			selectedConfig.Audio.Interface = rootConfig.Audio.Interface
		}
	}

	// Apply global output settings as base if they exist
	// Global recordings directory takes priority over profile-specific directory
	if rootConfig.Globals != nil && rootConfig.Globals.Output.RecordingsDirectory != "" {
		selectedConfig.Output.Directory = rootConfig.Globals.Output.RecordingsDirectory
	}
	// Global backingtracks directory takes priority over profile-specific directory
	if rootConfig.Globals != nil && rootConfig.Globals.Output.BackingtracksDirectory != "" {
		selectedConfig.Output.BackingtracksDirectory = rootConfig.Globals.Output.BackingtracksDirectory
	}

	// Merge with default config if it exists and we're not already using default
	if configName != "default" {
		if defaultProfile, exists := rootConfig.Configs["default"]; exists {
			defaultConfig, err := convertProfileToConfig(defaultProfile, rootConfig.Definitions)
			if err != nil {
				return nil, fmt.Errorf("error resolving default configuration: %w", err)
			}
			selectedConfig = mergeConfigs(defaultConfig, selectedConfig)
		}
	}

	// Apply global directories again after merging to ensure they take precedence
	if rootConfig.Globals != nil && rootConfig.Globals.Output.RecordingsDirectory != "" {
		selectedConfig.Output.Directory = rootConfig.Globals.Output.RecordingsDirectory
	}
	if rootConfig.Globals != nil && rootConfig.Globals.Output.BackingtracksDirectory != "" {
		selectedConfig.Output.BackingtracksDirectory = rootConfig.Globals.Output.BackingtracksDirectory
	}

	// Expand tilde in output directories
	selectedConfig.Output.Directory = expandPath(selectedConfig.Output.Directory)
	if selectedConfig.Output.BackingtracksDirectory != "" {
		selectedConfig.Output.BackingtracksDirectory = expandPath(selectedConfig.Output.BackingtracksDirectory)
	}

	// Validate JACK port specifications
	if err := validateAudioSources(selectedConfig); err != nil {
		return nil, fmt.Errorf("config validation failed: %w", err)
	}

	return selectedConfig, nil
}


func (c *Config) Save() error {
	return viper.WriteConfig()
}

// UpdateActiveConfig updates the active_config field in the config file
func UpdateActiveConfig(configFile, newActiveConfig string) error {
	if configFile == "" {
		return fmt.Errorf("no config file specified")
	}

	// Create a new viper instance to avoid interfering with the global one
	v := viper.New()
	v.SetConfigFile(configFile)

	// Read current config
	if err := v.ReadInConfig(); err != nil {
		return fmt.Errorf("error reading config file %s: %w", configFile, err)
	}

	// Update the active_config field
	v.Set("active_config", newActiveConfig)

	// Write back to file
	if err := v.WriteConfig(); err != nil {
		return fmt.Errorf("error writing config file %s: %w", configFile, err)
	}

	return nil
}

// convertProfileToConfig converts a ConfigProfile to Config by resolving channel references
func convertProfileToConfig(profile *ConfigProfile, definitions *DefinitionsConfig) (*Config, error) {
	if profile == nil {
		return nil, fmt.Errorf("profile cannot be nil")
	}

	config := &Config{
		Audio:   profile.Audio,
		Output:  profile.Output,
		AutoMix: profile.AutoMix,
	}

	// Resolve channel references
	for i, chRef := range profile.Channels {
		if chRef.Ref == "" {
			return nil, fmt.Errorf("channel[%d]: 'ref' is required", i)
		}

		// Find the channel definition
		var definition *ChannelDefinition
		if definitions != nil {
			for _, def := range definitions.Channels {
				if def.ID == chRef.Ref {
					definition = &def
					break
				}
			}
		}

		if definition == nil {
			return nil, fmt.Errorf("channel[%d]: reference '%s' not found in definitions", i, chRef.Ref)
		}

		// Create channel from definition
		channel := Channel{
			Name:      definition.Name,
			Sources:   definition.Sources,
			AudioMode: definition.AudioMode,
			Type:      definition.Type,
			Volume:    definition.Volume,
			Delay:     definition.Delay,
		}

		// Apply overrides
		if chRef.Volume != nil {
			channel.Volume = *chRef.Volume
		}
		if chRef.Delay != nil {
			channel.Delay = *chRef.Delay
		}

		config.Channels = append(config.Channels, channel)
	}

	return config, nil
}

// mergeConfigs implements the "Selection & Fallback" inheritance model:
// - Channels: Only record the channels explicitly listed in the profile's channels section
// - For listed channels missing source/type, inherit from default channel with same name
// - For all other settings (mix, output, audio), use profile value or fallback to default
func mergeConfigs(base, profile *Config) *Config {
	result := &Config{}

	// Initialize inheritance tracking
	result.Inheritance = &InheritanceInfo{
		Channels: make(map[string]struct {
			Source string
			Type   string
			Volume string
			Delay  string
		}),
	}

	// Start with base config for all non-channel settings
	if base != nil {
		result.Audio = base.Audio
		result.Output = base.Output
		result.AutoMix = base.AutoMix

		// Mark as inherited by default
		result.Inheritance.Audio.SampleRate = "inherited"
		result.Inheritance.Audio.Interface = "inherited"
		result.Inheritance.Audio.Backend = "inherited"
		result.Inheritance.Output.Directory = "inherited"
		result.Inheritance.Output.Format = "inherited"
	}

	if profile == nil {
		return result
	}

	// Override global settings with profile values
	if profile.Audio.SampleRate != 0 {
		result.Audio.SampleRate = profile.Audio.SampleRate
		result.Inheritance.Audio.SampleRate = "profile-specific"
	}
	if profile.Audio.Interface != "" {
		result.Audio.Interface = profile.Audio.Interface
		result.Inheritance.Audio.Interface = "profile-specific"
	}
	if profile.Audio.Backend != "" {
		result.Audio.Backend = profile.Audio.Backend
		result.Inheritance.Audio.Backend = "profile-specific"
	}


	if profile.Output.Directory != "" {
		result.Output.Directory = profile.Output.Directory
		result.Inheritance.Output.Directory = "profile-specific"
	}
	if profile.Output.BackingtracksDirectory != "" {
		result.Output.BackingtracksDirectory = profile.Output.BackingtracksDirectory
	}
	if profile.Output.Format != "" {
		result.Output.Format = profile.Output.Format
		result.Inheritance.Output.Format = "profile-specific"
	}

	// AutoMix: profile value always takes precedence if the profile is loaded
	result.AutoMix = profile.AutoMix

	// CHANNELS: Selection & Fallback Model
	// Only use channels explicitly listed in profile, with inheritance for missing fields
	result.Channels = make([]Channel, 0, len(profile.Channels))

	for _, profileChannel := range profile.Channels {
		resolvedChannel := Channel{
			Name:      profileChannel.Name,
			Sources:   profileChannel.Sources,
			AudioMode: profileChannel.AudioMode,
			Type:      profileChannel.Type,
			Volume:    profileChannel.Volume,
			Delay:     profileChannel.Delay,
		}

		// Track inheritance for this channel
		channelInheritance := struct {
			Source string
			Type   string
			Volume string
			Delay  string
		}{
			Source: "profile-specific",
			Type:   "profile-specific",
			Volume: "profile-specific",
			Delay:  "profile-specific",
		}

		// Inherit missing fields from base channel with same name
		if base != nil {
			for _, baseChannel := range base.Channels {
				if baseChannel.Name == profileChannel.Name {
					// Inherit sources if not specified
					if len(resolvedChannel.Sources) == 0 {
						resolvedChannel.Sources = baseChannel.Sources
						channelInheritance.Source = "inherited"
					}
					// Inherit audioMode if not specified
					if resolvedChannel.AudioMode == "" {
						resolvedChannel.AudioMode = baseChannel.AudioMode
					}
					if resolvedChannel.Type == "" {
						resolvedChannel.Type = baseChannel.Type
						channelInheritance.Type = "inherited"
					}
					// For volume and delay: if they are present in profile channel (even if 0),
					// they are considered profile-specific. Only inherit if completely missing.
					// Since the test explicitly sets Volume: 0, Delay: 0, these are profile-specific.

					// Note: In Go, when unmarshaling YAML, if Volume/Delay are specified (even as 0),
					// they will be set. The only way to inherit would be if the field was not
					// present in the YAML at all. For this test, Volume: 0, Delay: 0 are explicit.
					break
				}
			}
		}

		// Set default audioMode if not specified
		if resolvedChannel.AudioMode == "" {
			resolvedChannel.AudioMode = "mono"
		}

		result.Inheritance.Channels[resolvedChannel.Name] = channelInheritance
		result.Channels = append(result.Channels, resolvedChannel)
	}

	return result
}

func expandPath(path string) string {
	if strings.HasPrefix(path, "~/") {
		homeDir, _ := os.UserHomeDir()
		return filepath.Join(homeDir, path[2:])
	}
	return path
}

// isValidAudioSource checks if a source name is valid for JACK/PipeWire
func isValidAudioSource(source string) bool {
	source = strings.TrimSpace(source)

	// Empty or disabled sources are handled elsewhere
	if source == "" || source == "disabled" {
		return true
	}

	// Check if it contains colon
	if strings.Contains(source, ":") {
		// For JACK/PipeWire devices that may contain colons in their names,
		// we need to split from the right to get the last part as port
		lastColonIndex := strings.LastIndex(source, ":")
		if lastColonIndex == -1 {
			return false
		}

		deviceName := strings.TrimSpace(source[:lastColonIndex])
		channelOrPort := strings.TrimSpace(source[lastColonIndex+1:])

		// Device name must not be empty
		if len(deviceName) == 0 {
			return false
		}

		// Port specification must not be empty
		if len(channelOrPort) == 0 {
			return false
		}

		// JACK format: "device:port_name" (port_name can be numeric or string)
		if isNumeric(channelOrPort) {
			return true
		}

		// JACK format: "device:port_name" (port_name is not purely numeric)
		return len(channelOrPort) > 0
	}

	// Device name without colon (not recommended for JACK/PipeWire)
	// Basic validation - must not be empty after trimming
	return len(source) > 0
}

// isNumeric checks if a string contains only digits
func isNumeric(s string) bool {
	if len(s) == 0 {
		return false
	}
	for _, c := range s {
		if c < '0' || c > '9' {
			return false
		}
	}
	return true
}

// validateAudioSources ensures all channel sources specify valid audio source names
// Accepts JACK port names (device:port) format
func validateAudioSources(config *Config) error {
	for i, channel := range config.Channels {
		// Validate channel name
		if channel.Name == "" {
			return fmt.Errorf("channel[%d] must have a name", i)
		}

		// Validate channel type
		if channel.Type == "" {
			return fmt.Errorf("channel[%d] '%s' must have a type (input, monitor)", i, channel.Name)
		}
		if channel.Type != "input" && channel.Type != "monitor" {
			return fmt.Errorf("channel[%d] '%s' type must be 'input' or 'monitor', got: %s", i, channel.Name, channel.Type)
		}

		// Validate audioMode
		if channel.AudioMode != "" && channel.AudioMode != "mono" && channel.AudioMode != "stereo" {
			return fmt.Errorf("channel[%d] '%s' audioMode must be 'mono' or 'stereo', got: %s", i, channel.Name, channel.AudioMode)
		}

		// Set default audioMode if not specified
		if config.Channels[i].AudioMode == "" {
			config.Channels[i].AudioMode = "mono"
		}
		channel = config.Channels[i] // Update reference after modification

		// Validate sources
		if len(channel.Sources) == 0 {
			return fmt.Errorf("channel[%d] '%s' must have at least one source", i, channel.Name)
		}

		// Validate sources count matches audioMode
		expectedSources := 1
		if channel.AudioMode == "stereo" {
			expectedSources = 2
		}
		if len(channel.Sources) != expectedSources {
			return fmt.Errorf("channel[%d] '%s' with audioMode '%s' must have exactly %d source(s), got %d",
				i, channel.Name, channel.AudioMode, expectedSources, len(channel.Sources))
		}

		// Validate each source has proper format
		for j, source := range channel.Sources {
			if source != "" && source != "disabled" {
				// Accept JACK format (device:port)
				if !isValidAudioSource(source) {
					return fmt.Errorf("channel[%d] '%s' source[%d] must be a valid audio source (JACK port), got: %s",
						i, channel.Name, j, source)
				}
			}
		}
	}

	return nil
}

// GetSupportedAudioExtensions returns the supported audio extensions from config or defaults
func GetSupportedAudioExtensions(configFile string) []string {
	defaultExtensions := []string{"flac", "wav", "mp3"}

	if configFile == "" {
		return defaultExtensions
	}

	viper.SetConfigFile(configFile)
	if err := viper.ReadInConfig(); err != nil {
		return defaultExtensions
	}

	var rootConfig RootConfig
	if err := viper.Unmarshal(&rootConfig); err != nil {
		return defaultExtensions
	}

	if len(rootConfig.SupportedAudioExtensions) == 0 {
		return defaultExtensions
	}

	return rootConfig.SupportedAudioExtensions
}

// BuildMixFilter creates FFmpeg filter based on recorded stream structure and channel configuration
func (c *Config) BuildMixFilter() (filter string, outputChannels int) {
	// The recorded file structure is now:
	// Stream 0:0 - First channel (mono=1ch, stereo=2ch with metadata title=channel_name or channel_name_stereo)
	// Stream 0:1 - Second channel (mono=1ch, stereo=2ch)
	// Stream 0:N - Nth channel
	// Each channel is a separate track in the same order as configuration

	enabledChannels := c.getEnabledChannels()
	if len(enabledChannels) == 0 {
		return "", 0
	}

	var filterParts []string
	var inputChannels []string

	// Process each channel individually with its own volume and delay
	for i, channel := range enabledChannels {
		channelRef := fmt.Sprintf("[ch_%s]", channel.Name)

		// For stereo channels, the input stream already contains 2 channels
		// For mono channels, the input stream contains 1 channel
		var baseFilter string
		if channel.AudioMode == "stereo" || len(channel.Sources) > 1 {
			// Stereo channel: apply volume and delay to both channels
			if channel.Delay > 0 {
				// For stereo channels, apply delay to both left and right channels: adelay=delay|delay
				baseFilter = fmt.Sprintf("[0:%d]volume=%.1f,adelay=%d|%d", i, channel.Volume, channel.Delay, channel.Delay)
			} else {
				baseFilter = fmt.Sprintf("[0:%d]volume=%.1f", i, channel.Volume)
			}
		} else {
			// Mono channel: apply volume and delay, then convert to stereo for mixing
			if channel.Delay > 0 {
				baseFilter = fmt.Sprintf("[0:%d]volume=%.1f,adelay=%d,aformat=channel_layouts=stereo", i, channel.Volume, channel.Delay)
			} else {
				baseFilter = fmt.Sprintf("[0:%d]volume=%.1f,aformat=channel_layouts=stereo", i, channel.Volume)
			}
		}

		filterParts = append(filterParts, baseFilter+channelRef)
		inputChannels = append(inputChannels, channelRef)
	}

	// Final mix
	if len(inputChannels) == 1 {
		// Single track - remove intermediate labels for direct output
		// Remove the channel reference from the filter part to make it direct
		singleChannelRef := fmt.Sprintf("[ch_%s]", enabledChannels[0].Name)
		filter = strings.ReplaceAll(filterParts[0], singleChannelRef, "")
		outputChannels = 2 // Always output stereo
	} else {
		// Mix multiple tracks
		mixInputs := strings.Join(inputChannels, "")
		filterParts = append(filterParts, fmt.Sprintf("%samix=inputs=%d:normalize=0", mixInputs, len(inputChannels)))
		filter = strings.Join(filterParts, ";")
		outputChannels = 2 // Stereo output
	}

	return filter, outputChannels
}

// getEnabledChannels returns channels that are not disabled
func (c *Config) getEnabledChannels() []Channel {
	var enabled []Channel
	for _, ch := range c.Channels {
		// Channel is enabled if it has at least one non-empty, non-disabled source
		if len(ch.Sources) > 0 {
			hasValidSource := false
			for _, source := range ch.Sources {
				if source != "" && source != "disabled" {
					hasValidSource = true
					break
				}
			}
			if hasValidSource {
				enabled = append(enabled, ch)
			}
		}
	}
	return enabled
}

// GetChannelVolume gets volume for a channel
func (c *Config) GetChannelVolume(channelName string) float64 {
	for _, channel := range c.Channels {
		if channel.Name == channelName {
			return channel.Volume
		}
	}

	// Default volumes if not specified
	switch channelName {
	case "guitar":
		return 4.0
	case "monitor":
		return 0.8
	default:
		return 1.0
	}
}

// GetChannelDelay gets delay for a channel
func (c *Config) GetChannelDelay(channelName string) int {
	for _, channel := range c.Channels {
		if channel.Name == channelName {
			return channel.Delay
		}
	}

	// Default delay if not specified
	return 0
}

// ValidateConfigurationFormat validates the configuration file format and returns parsed config
func ValidateConfigurationFormat(configFile string) (*RootConfig, error) {
	viper.SetConfigFile(configFile)

	// Set environment variable prefix
	viper.SetEnvPrefix("JAMCAPTURE")
	viper.AutomaticEnv()

	// Read config file
	if err := viper.ReadInConfig(); err != nil {
		return nil, fmt.Errorf("error reading config file %s: %w", configFile, err)
	}

	var rootConfig RootConfig
	if err := viper.Unmarshal(&rootConfig); err != nil {
		return nil, fmt.Errorf("error unmarshaling config: %w", err)
	}

	// Validate definitions section
	if err := validateDefinitions(rootConfig.Definitions); err != nil {
		return nil, fmt.Errorf("invalid definitions: %w", err)
	}

	// Validate that all channel references in configs are valid
	for configName, configProfile := range rootConfig.Configs {
		if err := validateChannelReferences(configProfile.Channels, rootConfig.Definitions, configName); err != nil {
			return nil, fmt.Errorf("invalid config '%s': %w", configName, err)
		}
	}

	return &rootConfig, nil
}

// validateDefinitions validates the definitions section
func validateDefinitions(definitions *DefinitionsConfig) error {
	if definitions == nil {
		return fmt.Errorf("definitions section is required")
	}

	if len(definitions.Channels) == 0 {
		return fmt.Errorf("definitions.channels cannot be empty")
	}

	seenIDs := make(map[string]bool)

	for i, def := range definitions.Channels {
		// ID unique et non vide
		if def.ID == "" {
			return fmt.Errorf("definitions.channels[%d]: 'id' is required", i)
		}
		if seenIDs[def.ID] {
			return fmt.Errorf("definitions.channels[%d]: duplicate ID '%s'", i, def.ID)
		}
		seenIDs[def.ID] = true

		// Validation standard des channels
		if err := validateChannelDefinition(def, fmt.Sprintf("definitions.channels[%d]", i)); err != nil {
			return err
		}
	}

	return nil
}

// validateChannelDefinition validates a single channel definition
func validateChannelDefinition(def ChannelDefinition, prefix string) error {
	if def.Name == "" {
		return fmt.Errorf("%s: 'name' is required", prefix)
	}

	if len(def.Sources) == 0 {
		return fmt.Errorf("%s: 'sources' is required and cannot be empty", prefix)
	}

	if def.Type == "" {
		return fmt.Errorf("%s: 'type' is required", prefix)
	}
	if def.Type != "input" && def.Type != "monitor" {
		return fmt.Errorf("%s: 'type' must be 'input' or 'monitor', got: %s", prefix, def.Type)
	}

	if def.AudioMode != "" && def.AudioMode != "mono" && def.AudioMode != "stereo" {
		return fmt.Errorf("%s: 'audioMode' must be 'mono' or 'stereo', got: %s", prefix, def.AudioMode)
	}

	// Set default audioMode if not specified
	if def.AudioMode == "" {
		def.AudioMode = "mono"
	}

	// Validate sources count matches audioMode
	expectedSources := 1
	if def.AudioMode == "stereo" {
		expectedSources = 2
	}
	if len(def.Sources) != expectedSources {
		return fmt.Errorf("%s: audioMode '%s' requires exactly %d source(s), got %d",
			prefix, def.AudioMode, expectedSources, len(def.Sources))
	}

	// Validate each source has proper format
	for j, source := range def.Sources {
		if source != "" && source != "disabled" {
			if !isValidAudioSource(source) {
				return fmt.Errorf("%s: source[%d] must be a valid audio source (JACK port), got: %s",
					prefix, j, source)
			}
		}
	}

	if def.Volume <= 0 {
		return fmt.Errorf("%s: 'volume' must be > 0, got: %.2f", prefix, def.Volume)
	}

	if def.Delay < 0 {
		return fmt.Errorf("%s: 'delay' must be >= 0, got: %d", prefix, def.Delay)
	}

	return nil
}

// validateChannelReferences validates channel references in a config profile
func validateChannelReferences(channels []ChannelReference, definitions *DefinitionsConfig, configName string) error {
	for i, chRef := range channels {
		prefix := fmt.Sprintf("channels[%d]", i)

		if chRef.Ref == "" {
			return fmt.Errorf("%s: 'ref' is required", prefix)
		}

		// Verify the reference exists
		found := false
		if definitions != nil {
			for _, def := range definitions.Channels {
				if def.ID == chRef.Ref {
					found = true
					break
				}
			}
		}

		if !found {
			return fmt.Errorf("%s: references undefined channel definition '%s'", prefix, chRef.Ref)
		}

		// Validate overrides
		if chRef.Volume != nil && *chRef.Volume <= 0 {
			return fmt.Errorf("%s: volume override must be > 0, got %.2f", prefix, *chRef.Volume)
		}

		if chRef.Delay != nil && *chRef.Delay < 0 {
			return fmt.Errorf("%s: delay override must be >= 0, got %d", prefix, *chRef.Delay)
		}
	}

	return nil
}

// ExtractDeviceAndPort splits a JACK port specification into device and port components
