package config

import (
	"testing"
)

// --- stereoRightPort ---

func TestStereoRightPortExplicit(t *testing.T) {
	got := stereoRightPort("capture_AUX16", "capture_AUX17")
	if got != "capture_AUX17" {
		t.Errorf("expected capture_AUX17, got %s", got)
	}
}

func TestStereoRightPortAutoIncrement(t *testing.T) {
	got := stereoRightPort("capture_AUX16", "")
	if got != "capture_AUX17" {
		t.Errorf("expected capture_AUX17, got %s", got)
	}
}

func TestStereoRightPortSuffix(t *testing.T) {
	got := stereoRightPort("capture_FL", "")
	// "FL" ends with no digit — ReplaceAllStringFunc finds no match → unchanged
	// This case doesn't arise in practice; just document the behaviour.
	if got != "capture_FL" {
		t.Errorf("no trailing digit: expected capture_FL unchanged, got %s", got)
	}
}

// --- IdentifyDevices ---

func xr18Port(suffix string) string {
	return "alsa_input.usb-BEHRINGER_XR18_production_03-00.pro-input-0:" + suffix
}

func scarlettPort(suffix string) string {
	return "alsa_input.usb-Focusrite_Scarlett_2i2_USB_Y814JK8264026F-00.analog-stereo:" + suffix
}

func TestIdentifyDevicesXR18(t *testing.T) {
	hwInputs := [][]string{
		{xr18Port("capture_AUX0")},
		{xr18Port("capture_AUX1")},
		{xr18Port("capture_AUX2")},
		{xr18Port("capture_AUX16")},
		{xr18Port("capture_AUX17")},
	}

	devices, unmatched := IdentifyDevices(hwInputs)

	if len(unmatched) != 0 {
		t.Errorf("expected 0 unmatched, got %d", len(unmatched))
	}
	if len(devices) != 1 {
		t.Fatalf("expected 1 device, got %d", len(devices))
	}
	dev := devices[0]
	if dev.Template.Fingerprint != "BEHRINGER_XR18" {
		t.Errorf("expected BEHRINGER_XR18, got %s", dev.Template.Fingerprint)
	}
	if !dev.ActualPorts["capture_AUX0"] {
		t.Error("capture_AUX0 should be in ActualPorts")
	}
	if !dev.ActualPorts["capture_AUX16"] {
		t.Error("capture_AUX16 should be in ActualPorts")
	}
}

func TestIdentifyDevicesScarlett2i2(t *testing.T) {
	hwInputs := [][]string{
		{scarlettPort("capture_FL")},
		{scarlettPort("capture_FR")},
	}

	devices, unmatched := IdentifyDevices(hwInputs)

	if len(unmatched) != 0 {
		t.Errorf("expected 0 unmatched, got %d", len(unmatched))
	}
	if len(devices) != 1 {
		t.Fatalf("expected 1 device, got %d", len(devices))
	}
	if devices[0].Template.ProfilePrefix != "2i2" {
		t.Errorf("expected prefix 2i2, got %s", devices[0].Template.ProfilePrefix)
	}
	if !devices[0].ActualPorts["capture_FL"] {
		t.Error("capture_FL should be present")
	}
}

func TestIdentifyDevicesMixed(t *testing.T) {
	hwInputs := [][]string{
		{xr18Port("capture_AUX0")},
		{xr18Port("capture_AUX1")},
		{scarlettPort("capture_FL")},
		{scarlettPort("capture_FR")},
	}

	devices, unmatched := IdentifyDevices(hwInputs)

	if len(unmatched) != 0 {
		t.Errorf("expected 0 unmatched, got %d", len(unmatched))
	}
	if len(devices) != 2 {
		t.Fatalf("expected 2 devices, got %d", len(devices))
	}
}

func TestIdentifyDevicesGeneric(t *testing.T) {
	hwInputs := [][]string{
		{"alsa_input.pci-0000_00_1f.3:capture_1"},
		{"alsa_input.pci-0000_00_1f.3:capture_2"},
	}

	devices, unmatched := IdentifyDevices(hwInputs)

	if len(devices) != 0 {
		t.Errorf("expected 0 known devices, got %d", len(devices))
	}
	if len(unmatched) != 2 {
		t.Errorf("expected 2 unmatched, got %d", len(unmatched))
	}
}

// --- MatchSoftwareSources ---

func TestMatchSoftwareSourcesChromeRunning(t *testing.T) {
	swSources := [][]string{
		{"Google Chrome:output_FL", "Google Chrome:output_FR"},
	}
	running := MatchSoftwareSources(swSources)

	if len(running) != 1 {
		t.Fatalf("expected 1 running source, got %d", len(running))
	}
	if running[0].ID != "chrome_stereo" {
		t.Errorf("expected chrome_stereo, got %s", running[0].ID)
	}
}

func TestMatchSoftwareSourcesNoneRunning(t *testing.T) {
	running := MatchSoftwareSources(nil)
	if len(running) != 0 {
		t.Errorf("expected 0 running sources, got %d", len(running))
	}
}

func TestMatchSoftwareSourcesMultiple(t *testing.T) {
	swSources := [][]string{
		{"Firefox:output_FL", "Firefox:output_FR"},
		{"spotify:output_FL", "spotify:output_FR"},
	}
	running := MatchSoftwareSources(swSources)
	if len(running) != 2 {
		t.Fatalf("expected 2 running sources, got %d", len(running))
	}
}

// --- GenerateConfig ---

func TestGenerateConfigXR18WithChrome(t *testing.T) {
	devices := []DetectedDevice{
		{
			Template:    &KnownDevices[0], // XR18
			PortPrefix:  "alsa_input.usb-BEHRINGER_XR18_production_03-00.pro-input-0",
			ActualPorts: map[string]bool{"capture_AUX0": true, "capture_AUX1": true},
		},
	}
	running := []SoftwareSourceTemplate{WellKnownSoftwareSources[0]} // chrome

	root := GenerateConfig(devices, running, nil)

	if root == nil {
		t.Fatal("GenerateConfig returned nil")
	}
	if _, ok := root.Configs["xr18_studio"]; !ok {
		t.Error("expected xr18_studio profile")
	}
	if _, ok := root.Configs["xr18_with_monitor"]; !ok {
		t.Error("expected xr18_with_monitor profile")
	}
	if root.ActiveConfig != "xr18_with_monitor" {
		t.Errorf("expected active_config=xr18_with_monitor, got %s", root.ActiveConfig)
	}

	// chrome_stereo definition must have 2 sources
	var chromeDef *ChannelDefinition
	for i, d := range root.Definitions.Channels {
		if d.ID == "chrome_stereo" {
			chromeDef = &root.Definitions.Channels[i]
			break
		}
	}
	if chromeDef == nil {
		t.Fatal("chrome_stereo definition missing")
	}
	if len(chromeDef.Sources) != 2 {
		t.Errorf("expected 2 sources for chrome_stereo, got %d", len(chromeDef.Sources))
	}
}

func TestGenerateConfigScarlettOnly(t *testing.T) {
	devices := []DetectedDevice{
		{
			Template:    &KnownDevices[1], // Scarlett 2i2
			PortPrefix:  "alsa_input.usb-Focusrite_Scarlett_2i2_USB_Y814JK8264026F-00.analog-stereo",
			ActualPorts: map[string]bool{"capture_FL": true, "capture_FR": true},
		},
	}

	root := GenerateConfig(devices, nil, nil)

	if root == nil {
		t.Fatal("GenerateConfig returned nil")
	}
	if _, ok := root.Configs["2i2_studio"]; !ok {
		t.Error("expected 2i2_studio profile")
	}
	if _, ok := root.Configs["2i2_with_monitor"]; ok {
		t.Error("unexpected 2i2_with_monitor profile (no software sources)")
	}
	if root.ActiveConfig != "2i2_studio" {
		t.Errorf("expected active_config=2i2_studio, got %s", root.ActiveConfig)
	}

	// Check channel names come from definition.Name
	var guitarDef *ChannelDefinition
	for i, d := range root.Definitions.Channels {
		if d.ID == "2i2_guitar" {
			guitarDef = &root.Definitions.Channels[i]
			break
		}
	}
	if guitarDef == nil {
		t.Fatal("2i2_guitar definition missing")
	}
	if guitarDef.Name != "guitar" {
		t.Errorf("expected Name=guitar, got %s", guitarDef.Name)
	}
}

func TestGenerateConfigMultipleDevices(t *testing.T) {
	devices := []DetectedDevice{
		{
			Template:    &KnownDevices[0], // XR18
			PortPrefix:  "alsa_input.usb-BEHRINGER_XR18_production_03-00.pro-input-0",
			ActualPorts: map[string]bool{"capture_AUX0": true},
		},
		{
			Template:    &KnownDevices[1], // Scarlett 2i2
			PortPrefix:  "alsa_input.usb-Focusrite_Scarlett_2i2_USB_Y814JK8264026F-00.analog-stereo",
			ActualPorts: map[string]bool{"capture_FL": true, "capture_FR": true},
		},
	}

	root := GenerateConfig(devices, nil, nil)

	if _, ok := root.Configs["all_devices"]; !ok {
		t.Error("expected all_devices profile for multiple devices")
	}
}

func TestGenerateConfigGenericFallback(t *testing.T) {
	genericPorts := []NamedPort{
		{Ports: []string{"alsa_input.pci-0000:capture_1"}},
		{Ports: []string{"alsa_input.pci-0000:capture_2"}},
	}

	root := GenerateConfig(nil, nil, genericPorts)

	if root == nil {
		t.Fatal("GenerateConfig returned nil")
	}
	if _, ok := root.Configs["default"]; !ok {
		t.Error("expected default profile for generic fallback")
	}
	if root.ActiveConfig != "default" {
		t.Errorf("expected active_config=default, got %s", root.ActiveConfig)
	}

	// Check generic IDs
	ids := make(map[string]bool)
	for _, d := range root.Definitions.Channels {
		ids[d.ID] = true
	}
	if !ids["hw_input_1"] || !ids["hw_input_2"] {
		t.Errorf("expected hw_input_1 and hw_input_2, got %v", ids)
	}
}

func TestGenerateConfigEmpty(t *testing.T) {
	root := GenerateConfig(nil, nil, nil)
	if root != nil {
		t.Error("expected nil for empty inputs")
	}
}
