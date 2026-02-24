package config

import (
	"io/ioutil"
	"os"
	"testing"
)

func TestValidateConfigurationFormat_ValidConfig(t *testing.T) {
	// Create temporary config file with valid definitions format
	validConfig := `
active_config: test

definitions:
  channels:
    - id: test_guitar
      name: guitar
      type: input
      sources:
        - system:capture_1
      audiomode: mono
      volume: 2.0
      delay: 0

    - id: test_monitor
      name: monitor
      type: monitor
      sources:
        - system:monitor_FL
        - system:monitor_FR
      audiomode: stereo
      volume: 0.8
      delay: 100

configs:
  test:
    channels:
      - ref: test_guitar
        volume: 3.0
      - ref: test_monitor
        delay: 200
    output:
      directory: ~/Audio/Test

supported_audio_extensions:
  - flac
  - wav
`

	configFile := createTempConfig(t, validConfig)
	defer os.Remove(configFile)

	rootConfig, err := ValidateConfigurationFormat(configFile)
	if err != nil {
		t.Errorf("Expected no error, got: %v", err)
	}

	if rootConfig == nil {
		t.Fatal("Expected non-nil root config")
	}

	// Validate definitions
	if rootConfig.Definitions == nil {
		t.Fatal("Expected definitions section")
	}

	if len(rootConfig.Definitions.Channels) != 2 {
		t.Errorf("Expected 2 channel definitions, got %d", len(rootConfig.Definitions.Channels))
	}

	// Check first definition
	def := rootConfig.Definitions.Channels[0]
	if def.ID != "test_guitar" || def.Name != "guitar" || def.Type != "input" {
		t.Errorf("Invalid first definition: %+v", def)
	}

	// Check config references
	testConfig := rootConfig.Configs["test"]
	if testConfig == nil {
		t.Fatal("Expected test config")
	}

	if len(testConfig.Channels) != 2 {
		t.Errorf("Expected 2 channel references, got %d", len(testConfig.Channels))
	}

	// Check references
	chRef := testConfig.Channels[0]
	if chRef.Ref != "test_guitar" {
		t.Errorf("Expected ref 'test_guitar', got '%s'", chRef.Ref)
	}

	if chRef.Volume == nil || *chRef.Volume != 3.0 {
		t.Errorf("Expected volume override 3.0, got %v", chRef.Volume)
	}
}

func TestValidateConfigurationFormat_MissingDefinitions(t *testing.T) {
	invalidConfig := `
active_config: test

configs:
  test:
    channels:
      - ref: missing_definition
`

	configFile := createTempConfig(t, invalidConfig)
	defer os.Remove(configFile)

	_, err := ValidateConfigurationFormat(configFile)
	if err == nil {
		t.Error("Expected error for missing definitions section")
	}

	if !containsSubstring(err.Error(), "definitions section is required") {
		t.Errorf("Expected error about missing definitions, got: %v", err)
	}
}

func TestValidateConfigurationFormat_EmptyDefinitions(t *testing.T) {
	invalidConfig := `
active_config: test

definitions:
  channels: []

configs:
  test:
    channels:
      - ref: missing_definition
`

	configFile := createTempConfig(t, invalidConfig)
	defer os.Remove(configFile)

	_, err := ValidateConfigurationFormat(configFile)
	if err == nil {
		t.Error("Expected error for empty definitions")
	}

	if !containsSubstring(err.Error(), "definitions.channels cannot be empty") {
		t.Errorf("Expected error about empty definitions, got: %v", err)
	}
}

func TestValidateConfigurationFormat_InvalidReference(t *testing.T) {
	invalidConfig := `
active_config: test

definitions:
  channels:
    - id: valid_definition
      name: guitar
      type: input
      sources:
        - system:capture_1
      audiomode: mono
      volume: 2.0
      delay: 0

configs:
  test:
    channels:
      - ref: invalid_reference
`

	configFile := createTempConfig(t, invalidConfig)
	defer os.Remove(configFile)

	_, err := ValidateConfigurationFormat(configFile)
	if err == nil {
		t.Error("Expected error for invalid reference")
	}

	if !containsSubstring(err.Error(), "references undefined channel definition 'invalid_reference'") {
		t.Errorf("Expected error about undefined reference, got: %v", err)
	}
}

func TestValidateConfigurationFormat_DuplicateDefinitionIDs(t *testing.T) {
	invalidConfig := `
active_config: test

definitions:
  channels:
    - id: duplicate_id
      name: guitar
      type: input
      sources:
        - system:capture_1
      audiomode: mono
      volume: 2.0
      delay: 0

    - id: duplicate_id
      name: mic
      type: input
      sources:
        - system:capture_2
      audiomode: mono
      volume: 3.0
      delay: 0

configs:
  test:
    channels:
      - ref: duplicate_id
`

	configFile := createTempConfig(t, invalidConfig)
	defer os.Remove(configFile)

	_, err := ValidateConfigurationFormat(configFile)
	if err == nil {
		t.Error("Expected error for duplicate IDs")
	}

	if !containsSubstring(err.Error(), "duplicate ID 'duplicate_id'") {
		t.Errorf("Expected error about duplicate ID, got: %v", err)
	}
}

func TestValidateConfigurationFormat_InvalidChannelDefinition(t *testing.T) {
	tests := []struct {
		name        string
		config      string
		expectedErr string
	}{
		{
			name: "missing ID",
			config: `
definitions:
  channels:
    - name: guitar
      type: input
      sources:
        - system:capture_1
      audiomode: mono
      volume: 2.0
`,
			expectedErr: "'id' is required",
		},
		{
			name: "missing name",
			config: `
definitions:
  channels:
    - id: test_guitar
      type: input
      sources:
        - system:capture_1
      audiomode: mono
      volume: 2.0
`,
			expectedErr: "'name' is required",
		},
		{
			name: "invalid type",
			config: `
definitions:
  channels:
    - id: test_guitar
      name: guitar
      type: invalid
      sources:
        - system:capture_1
      audiomode: mono
      volume: 2.0
`,
			expectedErr: "'type' must be 'input' or 'monitor', got: invalid",
		},
		{
			name: "invalid audioMode",
			config: `
definitions:
  channels:
    - id: test_guitar
      name: guitar
      type: input
      sources:
        - system:capture_1
      audiomode: invalid
      volume: 2.0
`,
			expectedErr: "'audioMode' must be 'mono' or 'stereo', got: invalid",
		},
		{
			name: "zero volume",
			config: `
definitions:
  channels:
    - id: test_guitar
      name: guitar
      type: input
      sources:
        - system:capture_1
      audiomode: mono
      volume: 0
`,
			expectedErr: "'volume' must be > 0, got: 0.00",
		},
		{
			name: "negative delay",
			config: `
definitions:
  channels:
    - id: test_guitar
      name: guitar
      type: input
      sources:
        - system:capture_1
      audiomode: mono
      volume: 2.0
      delay: -100
`,
			expectedErr: "'delay' must be >= 0, got: -100",
		},
		{
			name: "stereo with one source",
			config: `
definitions:
  channels:
    - id: test_chrome
      name: chrome
      type: monitor
      sources:
        - Chrome:output_FL
      audiomode: stereo
      volume: 0.8
`,
			expectedErr: "audioMode 'stereo' requires exactly 2 source(s)",
		},
		{
			name: "mono with two sources",
			config: `
definitions:
  channels:
    - id: test_guitar
      name: guitar
      type: input
      sources:
        - system:capture_1
        - system:capture_2
      audiomode: mono
      volume: 2.0
`,
			expectedErr: "audioMode 'mono' requires exactly 1 source(s)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fullConfig := `
active_config: test
` + tt.config + `
configs:
  test:
    channels:
      - ref: test_guitar
`

			configFile := createTempConfig(t, fullConfig)
			defer os.Remove(configFile)

			_, err := ValidateConfigurationFormat(configFile)
			if err == nil {
				t.Error("Expected error but got none")
			}

			if !containsSubstring(err.Error(), tt.expectedErr) {
				t.Errorf("Expected error containing '%s', got: %v", tt.expectedErr, err)
			}
		})
	}
}

func TestValidateConfigurationFormat_InvalidChannelReference(t *testing.T) {
	tests := []struct {
		name        string
		refConfig   string
		expectedErr string
	}{
		{
			name: "missing ref",
			refConfig: `
    channels:
      - volume: 2.0
`,
			expectedErr: "'ref' is required",
		},
		{
			name: "invalid volume override",
			refConfig: `
    channels:
      - ref: test_guitar
        volume: 0
`,
			expectedErr: "volume override must be > 0",
		},
		{
			name: "invalid delay override",
			refConfig: `
    channels:
      - ref: test_guitar
        delay: -100
`,
			expectedErr: "delay override must be >= 0",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fullConfig := `
active_config: test

definitions:
  channels:
    - id: test_guitar
      name: guitar
      type: input
      sources:
        - system:capture_1
      audiomode: mono
      volume: 2.0
      delay: 0

configs:
  test:
` + tt.refConfig

			configFile := createTempConfig(t, fullConfig)
			defer os.Remove(configFile)

			_, err := ValidateConfigurationFormat(configFile)
			if err == nil {
				t.Error("Expected error but got none")
			}

			if !containsSubstring(err.Error(), tt.expectedErr) {
				t.Errorf("Expected error containing '%s', got: %v", tt.expectedErr, err)
			}
		})
	}
}

func TestConvertProfileToConfig_ValidProfile(t *testing.T) {
	// Setup definitions
	definitions := &DefinitionsConfig{
		Channels: []ChannelDefinition{
			{
				ID:        "test_guitar",
				Name:      "guitar",
				Type:      "input",
				Sources:   []string{"system:capture_1"},
				AudioMode: "mono",
				Volume:    2.0,
				Delay:     0,
			},
			{
				ID:        "test_monitor",
				Name:      "monitor",
				Type:      "monitor",
				Sources:   []string{"system:monitor_FL", "system:monitor_FR"},
				AudioMode: "stereo",
				Volume:    0.8,
				Delay:     100,
			},
		},
	}

	// Setup profile with references and overrides
	profile := &ConfigProfile{
		Audio: AudioConfig{
			SampleRate: 48000,
		},
		Channels: []ChannelReference{
			{
				Ref:    "test_guitar",
				Volume: &[]float64{3.5}[0], // Override volume
			},
			{
				Ref:   "test_monitor",
				Delay: &[]int{200}[0], // Override delay
			},
		},
		Output: OutputConfig{
			Directory: "~/Audio/Test",
		},
	}

	config, err := convertProfileToConfig(profile, definitions)
	if err != nil {
		t.Errorf("Expected no error, got: %v", err)
	}

	if config == nil {
		t.Fatal("Expected non-nil config")
	}

	// Check that we have 2 channels
	if len(config.Channels) != 2 {
		t.Errorf("Expected 2 channels, got %d", len(config.Channels))
	}

	// Check first channel (guitar with volume override)
	guitar := config.Channels[0]
	if guitar.Name != "guitar" || guitar.Type != "input" {
		t.Errorf("Invalid guitar channel: %+v", guitar)
	}
	if guitar.Volume != 3.5 {
		t.Errorf("Expected volume override 3.5, got %.1f", guitar.Volume)
	}
	if guitar.Delay != 0 { // Should inherit original delay
		t.Errorf("Expected inherited delay 0, got %d", guitar.Delay)
	}

	// Check second channel (monitor with delay override)
	monitor := config.Channels[1]
	if monitor.Name != "monitor" || monitor.Type != "monitor" {
		t.Errorf("Invalid monitor channel: %+v", monitor)
	}
	if monitor.Volume != 0.8 { // Should inherit original volume
		t.Errorf("Expected inherited volume 0.8, got %.1f", monitor.Volume)
	}
	if monitor.Delay != 200 {
		t.Errorf("Expected delay override 200, got %d", monitor.Delay)
	}
}

func TestConvertProfileToConfig_MissingReference(t *testing.T) {
	definitions := &DefinitionsConfig{
		Channels: []ChannelDefinition{
			{
				ID:   "existing_channel",
				Name: "guitar",
			},
		},
	}

	profile := &ConfigProfile{
		Channels: []ChannelReference{
			{
				Ref: "missing_channel",
			},
		},
	}

	_, err := convertProfileToConfig(profile, definitions)
	if err == nil {
		t.Error("Expected error for missing reference")
	}

	if !containsSubstring(err.Error(), "reference 'missing_channel' not found in definitions") {
		t.Errorf("Expected error about missing reference, got: %v", err)
	}
}

func TestConvertProfileToConfig_EmptyRef(t *testing.T) {
	profile := &ConfigProfile{
		Channels: []ChannelReference{
			{
				Ref: "", // Empty reference
			},
		},
	}

	_, err := convertProfileToConfig(profile, nil)
	if err == nil {
		t.Error("Expected error for empty reference")
	}

	if !containsSubstring(err.Error(), "'ref' is required") {
		t.Errorf("Expected error about required ref, got: %v", err)
	}
}

// Helper function to create temporary config file for testing
func createTempConfig(t *testing.T, content string) string {
	tmpfile, err := ioutil.TempFile("", "jamcapture-test-*.yaml")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}

	if _, err := tmpfile.Write([]byte(content)); err != nil {
		t.Fatalf("Failed to write temp file: %v", err)
	}

	if err := tmpfile.Close(); err != nil {
		t.Fatalf("Failed to close temp file: %v", err)
	}

	return tmpfile.Name()
}