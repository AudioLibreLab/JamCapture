package config

import (
	"os"
	"path/filepath"
	"testing"
	"io/ioutil"
)

func TestMergeConfigs_SelectionAndFallback(t *testing.T) {
	// Create base (default) config
	base := &Config{
		Audio: AudioConfig{
			SampleRate: 48000,
			Interface:  "jack",
		},
		Channels: []Channel{
			{Name: "guitar", Sources: []string{"system:capture_1"}, AudioMode: "mono", Type: "input", Volume: 4.0, Delay: 0},
			{Name: "mic", Sources: []string{"system:capture_2"}, AudioMode: "mono", Type: "input", Volume: 4.0, Delay: 0},
			{Name: "monitor_left", Sources: []string{"system:monitor_FL"}, AudioMode: "mono", Type: "monitor", Volume: 0.8, Delay: 100},
			{Name: "monitor_right", Sources: []string{"system:monitor_FR"}, AudioMode: "mono", Type: "monitor", Volume: 0.8, Delay: 100},
		},
		Output: OutputConfig{
			Directory: "~/Audio/Default",
			Format:    "flac",
		},
	}

	// Create profile config that only lists some channels and overrides some settings
	profile := &Config{
		Audio: AudioConfig{
			SampleRate: 44100, // Override sample rate
		},
		Channels: []Channel{
			{Name: "guitar", Sources: []string{"scarlett:capture_1"}, Volume: 2.0, Delay: 200}, // Override source, volume and delay
			{Name: "monitor_left", Volume: 0, Delay: 0},                                        // Inherit source, type, explicit 0 for volume and delay
		},
		Output: OutputConfig{
			Directory: "~/Audio/Studio", // Override directory
		},
	}

	// Merge configs
	result := mergeConfigs(base, profile)

	// Test Selection & Fallback for Channels
	// Should only have 2 channels (those listed in profile), not all 4 from base
	if len(result.Channels) != 2 {
		t.Errorf("Expected 2 channels, got %d", len(result.Channels))
	}

	// First channel: guitar with overridden source, inherited type
	guitar := result.Channels[0]
	if guitar.Name != "guitar" || len(guitar.Sources) != 1 || guitar.Sources[0] != "scarlett:capture_1" || guitar.Type != "input" {
		t.Errorf("Guitar channel incorrect: got %+v", guitar)
	}

	// Second channel: monitor_left with inherited source and type
	monitor := result.Channels[1]
	if monitor.Name != "monitor_left" || len(monitor.Sources) != 1 || monitor.Sources[0] != "system:monitor_FL" || monitor.Type != "monitor" {
		t.Errorf("Monitor channel incorrect: got %+v", monitor)
	}

	// Test Global Settings Fallback
	// Audio: overridden sample rate, inherited interface
	if result.Audio.SampleRate != 44100 {
		t.Errorf("Expected sample rate 44100, got %d", result.Audio.SampleRate)
	}
	if result.Audio.Interface != "jack" {
		t.Errorf("Expected interface 'jack', got %s", result.Audio.Interface)
	}

	// Check unified channel structure
	if len(result.Channels) != 2 {
		t.Errorf("Expected 2 channels, got %d", len(result.Channels))
	}

	// Check guitar channel (profile-specific values)
	var guitarChannel *Channel
	for _, ch := range result.Channels {
		if ch.Name == "guitar" {
			guitarChannel = &ch
			break
		}
	}
	if guitarChannel == nil {
		t.Errorf("Guitar channel not found")
	} else {
		if len(guitarChannel.Sources) != 1 || guitarChannel.Sources[0] != "scarlett:capture_1" {
			t.Errorf("Expected guitar source 'scarlett:capture_1', got %v", guitarChannel.Sources)
		}
		if guitarChannel.Volume != 2.0 {
			t.Errorf("Expected guitar volume 2.0, got %.1f", guitarChannel.Volume)
		}
		if guitarChannel.Delay != 200 {
			t.Errorf("Expected guitar delay 200ms, got %d", guitarChannel.Delay)
		}
	}

	// Check monitor_left channel (inherited values)
	var monitorChannel *Channel
	for _, ch := range result.Channels {
		if ch.Name == "monitor_left" {
			monitorChannel = &ch
			break
		}
	}
	if monitorChannel == nil {
		t.Errorf("Monitor_left channel not found")
	} else {
		if len(monitorChannel.Sources) != 1 || monitorChannel.Sources[0] != "system:monitor_FL" {
			t.Errorf("Expected monitor_left source 'system:monitor_FL', got %v", monitorChannel.Sources)
		}
		if monitorChannel.Volume != 0 {
			t.Errorf("Expected monitor_left volume 0 (profile-specific), got %.1f", monitorChannel.Volume)
		}
		if monitorChannel.Delay != 0 {
			t.Errorf("Expected monitor_left delay 0 (profile-specific), got %d", monitorChannel.Delay)
		}
	}

	// Output: overridden directory, inherited format
	if result.Output.Directory != "~/Audio/Studio" {
		t.Errorf("Expected directory '~/Audio/Studio', got %s", result.Output.Directory)
	}
	if result.Output.Format != "flac" {
		t.Errorf("Expected format 'flac', got %s", result.Output.Format)
	}

	// Test inheritance tracking
	if result.Inheritance == nil {
		t.Fatal("Inheritance tracking not initialized")
	}

	// Audio inheritance
	if result.Inheritance.Audio.SampleRate != "profile-specific" {
		t.Errorf("Expected sample rate to be profile-specific, got %s", result.Inheritance.Audio.SampleRate)
	}
	if result.Inheritance.Audio.Interface != "inherited" {
		t.Errorf("Expected interface to be inherited, got %s", result.Inheritance.Audio.Interface)
	}

	// Channel inheritance
	guitarInheritance := result.Inheritance.Channels["guitar"]
	if guitarInheritance.Source != "profile-specific" {
		t.Errorf("Expected guitar source to be profile-specific, got %s", guitarInheritance.Source)
	}
	if guitarInheritance.Type != "inherited" {
		t.Errorf("Expected guitar type to be inherited, got %s", guitarInheritance.Type)
	}
	if guitarInheritance.Volume != "profile-specific" {
		t.Errorf("Expected guitar volume to be profile-specific, got %s", guitarInheritance.Volume)
	}
	if guitarInheritance.Delay != "profile-specific" {
		t.Errorf("Expected guitar delay to be profile-specific, got %s", guitarInheritance.Delay)
	}

	monitorInheritance := result.Inheritance.Channels["monitor_left"]
	if monitorInheritance.Source != "inherited" {
		t.Errorf("Expected monitor_left source to be inherited, got %s", monitorInheritance.Source)
	}
	if monitorInheritance.Type != "inherited" {
		t.Errorf("Expected monitor_left type to be inherited, got %s", monitorInheritance.Type)
	}
	if monitorInheritance.Volume != "profile-specific" {
		t.Errorf("Expected monitor_left volume to be profile-specific (explicitly set to 0), got %s", monitorInheritance.Volume)
	}
	if monitorInheritance.Delay != "profile-specific" {
		t.Errorf("Expected monitor_left delay to be profile-specific (explicitly set to 0), got %s", monitorInheritance.Delay)
	}
}

func TestMergeConfigs_ProfileOnly(t *testing.T) {
	// Test when profile has all required fields - no inheritance needed
	profile := &Config{
		Audio: AudioConfig{
			SampleRate: 48000,
			Interface:  "alsa",
		},
		Channels: []Channel{
			{Name: "guitar", Sources: []string{"hw:1,0:capture_1"}, AudioMode: "mono", Type: "input", Volume: 3.0, Delay: 0},
		},
		Output: OutputConfig{
			Directory: "/tmp/recordings",
			Format:    "wav",
		},
	}

	result := mergeConfigs(nil, profile)

	// Should have exactly the profile values
	if result.Audio.SampleRate != 48000 || result.Audio.Interface != "alsa" {
		t.Errorf("Audio config not preserved: %+v", result.Audio)
	}
	if len(result.Channels) != 1 || result.Channels[0].Name != "guitar" {
		t.Errorf("Channels not preserved: %+v", result.Channels)
	}
	if len(result.Channels) != 1 || result.GetChannelVolume("guitar") != 3.0 || result.GetChannelDelay("guitar") != 0 {
		t.Errorf("Channel config not preserved: %+v", result.Channels)
	}
	if result.Output.Directory != "/tmp/recordings" || result.Output.Format != "wav" {
		t.Errorf("Output config not preserved: %+v", result.Output)
	}
}

func TestMergeConfigs_ChannelOverride(t *testing.T) {
	// Test channel override behavior
	base := &Config{
		Channels: []Channel{
			{Name: "guitar", Sources: []string{"base:capture"}, AudioMode: "mono", Type: "input", Volume: 1.0, Delay: 150},
			{Name: "monitor", Sources: []string{"base:monitor"}, AudioMode: "mono", Type: "monitor", Volume: 0.5, Delay: 100},
		},
	}

	profile := &Config{
		Channels: []Channel{
			{Name: "guitar", Volume: 2.0, Delay: 0}, // Override guitar volume/delay, inherit source/type
		},
	}

	result := mergeConfigs(base, profile)

	// Profile channels should completely replace base channels
	if len(result.Channels) != 1 {
		t.Errorf("Expected 1 channel, got %d", len(result.Channels))
	}

	guitarVol := result.GetChannelVolume("guitar")
	if guitarVol != 2.0 {
		t.Errorf("Expected guitar volume 2.0, got %.1f", guitarVol)
	}

	guitarDelay := result.GetChannelDelay("guitar")
	if guitarDelay != 0 {
		t.Errorf("Expected guitar delay 0, got %d", guitarDelay)
	}

	// Check that source and type were inherited
	if len(result.Channels) > 0 && (len(result.Channels[0].Sources) != 1 || result.Channels[0].Sources[0] != "base:capture") {
		t.Errorf("Expected guitar source to be inherited as 'base:capture', got %v", result.Channels[0].Sources)
	}

	// Monitor should not be present (not in profile)
	monitorVol := result.GetChannelVolume("monitor")
	if monitorVol != 0.8 { // Should get default
		t.Errorf("Expected monitor to get default volume 0.8, got %.1f", monitorVol)
	}
}

func TestMergeConfigs_EmptyProfile(t *testing.T) {
	// Test when profile has empty channels list - should result in no channels
	base := &Config{
		Channels: []Channel{
			{Name: "guitar", Sources: []string{"system:capture_1"}, AudioMode: "mono", Type: "input"},
			{Name: "mic", Sources: []string{"system:capture_2"}, AudioMode: "mono", Type: "input"},
		},
	}

	profile := &Config{
		Channels: []Channel{}, // Empty list
	}

	result := mergeConfigs(base, profile)

	// Should have no channels (selection model: only profile channels are used)
	if len(result.Channels) != 0 {
		t.Errorf("Expected 0 channels, got %d", len(result.Channels))
	}
}

func TestExpandPath(t *testing.T) {
	// Test tilde expansion
	homeDir, _ := os.UserHomeDir()

	tests := []struct {
		input    string
		expected string
	}{
		{"~/Audio/JamCapture", filepath.Join(homeDir, "Audio", "JamCapture")},
		{"/absolute/path", "/absolute/path"},
		{"relative/path", "relative/path"},
		{"~", "~"}, // Should not expand bare tilde
	}

	for _, test := range tests {
		result := expandPath(test.input)
		if result != test.expected {
			t.Errorf("expandPath(%q) = %q, expected %q", test.input, result, test.expected)
		}
	}
}

// Helper function to check if string contains substring
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 ||
		(len(s) > len(substr) && (s[:len(substr)] == substr || s[len(s)-len(substr):] == substr ||
			containsSubstring(s, substr))))
}

func containsSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func TestBuildMixFilter(t *testing.T) {
	tests := []struct {
		name            string
		channels        []Channel
		expectedFilter  string
		expectedOutputs int
	}{
		{
			name:            "no channels",
			channels:        []Channel{},
			expectedFilter:  "",
			expectedOutputs: 0,
		},
		{
			name: "single mono channel",
			channels: []Channel{
				{Name: "guitar", Sources: []string{"system:capture_1"}, AudioMode: "mono", Type: "input", Volume: 2.0, Delay: 0},
			},
			expectedFilter:  "[0:0]volume=2.0,aformat=channel_layouts=stereo",
			expectedOutputs: 2,
		},
		{
			name: "single stereo channel",
			channels: []Channel{
				{Name: "chrome", Sources: []string{"Chrome:output_FL", "Chrome:output_FR"}, AudioMode: "stereo", Type: "monitor", Volume: 0.8, Delay: 250},
			},
			expectedFilter:  "[0:0]volume=0.8,adelay=250|250",
			expectedOutputs: 2,
		},
		{
			name: "mono with delay",
			channels: []Channel{
				{Name: "guitar", Sources: []string{"system:capture_1"}, AudioMode: "mono", Type: "input", Volume: 1.5, Delay: 100},
			},
			expectedFilter:  "[0:0]volume=1.5,adelay=100,aformat=channel_layouts=stereo",
			expectedOutputs: 2,
		},
		{
			name: "mixed mono and stereo channels",
			channels: []Channel{
				{Name: "guitar", Sources: []string{"system:capture_1"}, AudioMode: "mono", Type: "input", Volume: 2.0, Delay: 0},
				{Name: "chrome", Sources: []string{"Chrome:output_FL", "Chrome:output_FR"}, AudioMode: "stereo", Type: "monitor", Volume: 0.8, Delay: 0},
			},
			expectedFilter:  "[0:0]volume=2.0,aformat=channel_layouts=stereo[ch_guitar];[0:1]volume=0.8[ch_chrome];[ch_guitar][ch_chrome]amix=inputs=2:normalize=0[mixed];[mixed]alimiter=limit=0.9:attack=7:release=150",
			expectedOutputs: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &Config{Channels: tt.channels}
			filter, outputs := cfg.BuildMixFilter()

			if filter != tt.expectedFilter {
				t.Errorf("Expected filter '%s', got '%s'", tt.expectedFilter, filter)
			}

			if outputs != tt.expectedOutputs {
				t.Errorf("Expected %d output channels, got %d", tt.expectedOutputs, outputs)
			}
		})
	}
}

func TestGlobalsRecordingsDirectory(t *testing.T) {
	// Create a temporary config file with globals section
	configContent := `
active_config: test
globals:
    output:
        recordings_directory: /global/recordings
definitions:
    channels:
        - id: guitar
          name: guitar
          sources: ["system:capture_1"]
          type: input
          volume: 4.0
          delay: 0
configs:
    test:
        channels:
            - ref: guitar
        output:
            directory: /profile/recordings
            format: flac
`

	// Create temporary file
	tmpfile, err := ioutil.TempFile("", "jamcapture_test_*.yaml")
	if err != nil {
		t.Fatalf("Failed to create temporary config file: %v", err)
	}
	defer os.Remove(tmpfile.Name())

	if _, err := tmpfile.WriteString(configContent); err != nil {
		t.Fatalf("Failed to write to temporary config file: %v", err)
	}
	if err := tmpfile.Close(); err != nil {
		t.Fatalf("Failed to close temporary config file: %v", err)
	}

	// Load configuration
	cfg, err := LoadWithProfile(tmpfile.Name(), "test")
	if err != nil {
		t.Fatalf("Failed to load configuration: %v", err)
	}

	// Verify that global recordings directory overrides profile directory
	expectedDir := "/global/recordings"
	if cfg.Output.Directory != expectedDir {
		t.Errorf("Expected directory '%s' from globals, got '%s'", expectedDir, cfg.Output.Directory)
	}

	// Verify other output settings still come from profile
	if cfg.Output.Format != "flac" {
		t.Errorf("Expected format 'flac' from profile, got '%s'", cfg.Output.Format)
	}
}

func TestGlobalsRecordingsDirectoryWithoutProfileDirectory(t *testing.T) {
	// Create a temporary config file with globals section but no profile directory
	configContent := `
active_config: test
globals:
    output:
        recordings_directory: /global/recordings
definitions:
    channels:
        - id: guitar
          name: guitar
          sources: ["system:capture_1"]
          type: input
          volume: 4.0
          delay: 0
configs:
    test:
        channels:
            - ref: guitar
        output:
            format: wav
`

	// Create temporary file
	tmpfile, err := ioutil.TempFile("", "jamcapture_test_*.yaml")
	if err != nil {
		t.Fatalf("Failed to create temporary config file: %v", err)
	}
	defer os.Remove(tmpfile.Name())

	if _, err := tmpfile.WriteString(configContent); err != nil {
		t.Fatalf("Failed to write to temporary config file: %v", err)
	}
	if err := tmpfile.Close(); err != nil {
		t.Fatalf("Failed to close temporary config file: %v", err)
	}

	// Load configuration
	cfg, err := LoadWithProfile(tmpfile.Name(), "test")
	if err != nil {
		t.Fatalf("Failed to load configuration: %v", err)
	}

	// Verify that global recordings directory is used
	expectedDir := "/global/recordings"
	if cfg.Output.Directory != expectedDir {
		t.Errorf("Expected directory '%s' from globals, got '%s'", expectedDir, cfg.Output.Directory)
	}
}
