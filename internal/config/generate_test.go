package config

import (
	"testing"
)

func TestGenerateDefault(t *testing.T) {
	hwInputs := [][]string{
		{"alsa_input.usb-Focusrite_Scarlett_2i2_USB_Y814JK8264026F-00.analog-stereo:capture_FL"},
		{"alsa_input.usb-Focusrite_Scarlett_2i2_USB_Y814JK8264026F-00.analog-stereo:capture_FR"},
	}
	swSources := [][]string{
		{"Chrome:output_FL", "Chrome:output_FR"},
	}

	root := GenerateDefault(hwInputs, swSources)

	if root == nil {
		t.Fatal("GenerateDefault returned nil")
	}
	if root.ActiveConfig != "default" {
		t.Errorf("expected active_config=default, got %s", root.ActiveConfig)
	}
	if root.Definitions == nil || len(root.Definitions.Channels) != 3 {
		t.Errorf("expected 3 channel definitions, got %d", len(root.Definitions.Channels))
	}
	profile, ok := root.Configs["default"]
	if !ok || len(profile.Channels) != 3 {
		t.Errorf("expected 3 refs in default profile")
	}

	// Check hardware channels are mono input
	hw1 := root.Definitions.Channels[0]
	if hw1.ID != "hw_input_1" || hw1.Type != "input" || hw1.AudioMode != "mono" {
		t.Errorf("unexpected hw_input_1: %+v", hw1)
	}

	// Check software channel is stereo monitor
	chrome := root.Definitions.Channels[2]
	if chrome.ID != "chrome_stereo" || chrome.Type != "monitor" || chrome.AudioMode != "stereo" {
		t.Errorf("unexpected chrome def: %+v", chrome)
	}
	if len(chrome.Sources) != 2 {
		t.Errorf("expected 2 sources for stereo chrome, got %d", len(chrome.Sources))
	}
}

func TestGenerateDefaultEmpty(t *testing.T) {
	root := GenerateDefault(nil, nil)
	if root != nil {
		t.Error("expected nil for empty inputs")
	}
}

// TestGenerateDefaultGoogleChrome covers newer kernels where Chrome reports
// as "Google Chrome" instead of "Chrome".
func TestGenerateDefaultGoogleChrome(t *testing.T) {
	swSources := [][]string{
		{"Google Chrome:output_FL", "Google Chrome:output_FR"},
	}

	root := GenerateDefault(nil, swSources)

	if root == nil {
		t.Fatal("GenerateDefault returned nil")
	}
	if len(root.Definitions.Channels) != 1 {
		t.Fatalf("expected 1 channel definition, got %d", len(root.Definitions.Channels))
	}
	ch := root.Definitions.Channels[0]
	if ch.ID != "google_chrome_stereo" {
		t.Errorf("expected ID google_chrome_stereo, got %s", ch.ID)
	}
	if ch.AudioMode != "stereo" {
		t.Errorf("expected stereo, got %s", ch.AudioMode)
	}
}
