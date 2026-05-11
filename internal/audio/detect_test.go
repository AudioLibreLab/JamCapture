package audio

import (
	"testing"
)

func TestCategorizeAndGroup(t *testing.T) {
	ports := []string{
		"alsa_input.usb-Focusrite_Scarlett_2i2_USB_Y814JK8264026F-00.analog-stereo:capture_FL",
		"alsa_input.usb-Focusrite_Scarlett_2i2_USB_Y814JK8264026F-00.analog-stereo:capture_FR",
		"alsa_output.usb-Focusrite_Scarlett_2i2_USB_Y814JK8264026F-00.analog-stereo:playback_FL",
		"alsa_output.usb-Focusrite_Scarlett_2i2_USB_Y814JK8264026F-00.analog-stereo:playback_FR",
		"Chrome:output_FL",
		"Chrome:output_FR",
		"system:capture_1",
		"system:playback_1",
	}

	hw, sw := CategorizeAndGroup(ports)

	if len(hw) != 2 {
		t.Errorf("expected 2 hardware inputs, got %d: %v", len(hw), hw)
	}
	if len(sw) != 1 {
		t.Errorf("expected 1 software source (stereo Chrome), got %d: %v", len(sw), sw)
	}
	if len(sw) == 1 && len(sw[0]) != 2 {
		t.Errorf("expected stereo pair for Chrome, got %v", sw[0])
	}
}

// TestCategorizeAndGroupGoogleChrome covers newer kernels where Chrome reports
// as "Google Chrome" instead of "Chrome".
func TestCategorizeAndGroupGoogleChrome(t *testing.T) {
	ports := []string{
		"alsa_input.usb-Focusrite_Scarlett_2i2_USB_Y814JK8264026F-00.analog-stereo:capture_FL",
		"Google Chrome:output_FL",
		"Google Chrome:output_FR",
		"system:playback_1",
	}

	hw, sw := CategorizeAndGroup(ports)

	if len(hw) != 1 {
		t.Errorf("expected 1 hardware input, got %d: %v", len(hw), hw)
	}
	if len(sw) != 1 {
		t.Errorf("expected 1 software source (stereo Google Chrome), got %d: %v", len(sw), sw)
	}
	if len(sw) == 1 && len(sw[0]) != 2 {
		t.Errorf("expected stereo pair for Google Chrome, got %v", sw[0])
	}
}

func TestCategorizeAndGroupXR18(t *testing.T) {
	ports := []string{
		"alsa_input.usb-BEHRINGER_XR18_production_03-00.pro-input-0:capture_AUX0",
		"alsa_input.usb-BEHRINGER_XR18_production_03-00.pro-input-0:capture_AUX1",
		"Firefox:output_FL",
		"Firefox:output_FR",
		"Spotify:output_FL",
		"Spotify:output_FR",
	}

	hw, sw := CategorizeAndGroup(ports)

	if len(hw) != 2 {
		t.Errorf("expected 2 XR18 capture ports, got %d", len(hw))
	}
	if len(sw) != 2 {
		t.Errorf("expected 2 software groups (Firefox+Spotify), got %d: %v", len(sw), sw)
	}
}
