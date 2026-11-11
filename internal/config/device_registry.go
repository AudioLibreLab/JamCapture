package config

import "fmt"

// ChannelTemplate describes one logical channel on a hardware device.
// PortSuffix is the part after ":" in the JACK port name, e.g. "capture_AUX0".
// For stereo channels, PortSuffix is the left port; PortSuffix2 is the right port
// (if empty, the right port is derived by incrementing the last digit of PortSuffix).
type ChannelTemplate struct {
	ID          string
	Name        string
	PortSuffix  string
	PortSuffix2 string // explicit right port for stereo; empty = auto-derive
	AudioMode   string // "mono" or "stereo"
	Type        string // "hardware" or "software"
	Volume      float64
}

// DeviceTemplate maps a known hardware device to its default channel layout.
// Fingerprint is matched as a substring of JACK port strings from pw-link.
type DeviceTemplate struct {
	Fingerprint   string // e.g. "BEHRINGER_XR18"
	ProfilePrefix string // e.g. "xr18"
	DisplayName   string // e.g. "Behringer XR18"
	Channels      []ChannelTemplate
}

// DetectedDevice is returned by IdentifyDevices and carries the matched
// template plus the actual port suffixes seen on this system.
type DetectedDevice struct {
	Template    *DeviceTemplate
	PortPrefix  string          // ALSA node prefix, e.g. "alsa_input.usb-BEHRINGER_XR18_production_03-00.pro-input-0"
	ActualPorts map[string]bool // port suffix → present on this system
}

// KnownDevices is the hardware device registry.
// To add a new device: append a DeviceTemplate with the right Fingerprint and Channels.
var KnownDevices = []DeviceTemplate{
	{
		Fingerprint:   "BEHRINGER_XR18",
		ProfilePrefix: "xr18",
		DisplayName:   "Behringer XR18",
		Channels:      xAir18Channels("xr18"),
	},
	{
		Fingerprint:   "Focusrite_Scarlett_2i2",
		ProfilePrefix: "2i2",
		DisplayName:   "Focusrite Scarlett 2i2",
		Channels: []ChannelTemplate{
			{ID: "2i2_guitar", Name: "guitar", PortSuffix: "capture_FR", AudioMode: "mono", Type: "hardware", Volume: 4.0},
			{ID: "2i2_mic", Name: "mic", PortSuffix: "capture_FL", AudioMode: "mono", Type: "hardware", Volume: 3.0},
		},
	},
	{
		Fingerprint:   "Focusrite_Scarlett_4i4",
		ProfilePrefix: "4i4",
		DisplayName:   "Focusrite Scarlett 4i4",
		Channels: []ChannelTemplate{
			{ID: "4i4_input1", Name: "input1", PortSuffix: "capture_FL", AudioMode: "mono", Type: "hardware", Volume: 4.0},
			{ID: "4i4_input2", Name: "input2", PortSuffix: "capture_FR", AudioMode: "mono", Type: "hardware", Volume: 4.0},
			{ID: "4i4_input3", Name: "input3", PortSuffix: "capture_RL", AudioMode: "mono", Type: "hardware", Volume: 4.0},
			{ID: "4i4_input4", Name: "input4", PortSuffix: "capture_RR", AudioMode: "mono", Type: "hardware", Volume: 4.0},
		},
	},
	{
		Fingerprint:   "HOTONE_AUDIO_Ampero",
		ProfilePrefix: "ampero",
		DisplayName:   "Hotone Ampero",
		Channels: []ChannelTemplate{
			// Stereo guitar processor — L+R as a single stereo channel
			{ID: "ampero", Name: "ampero", PortSuffix: "capture_FL", PortSuffix2: "capture_FR", AudioMode: "stereo", Type: "hardware", Volume: 4.0},
		},
	},
	{
		// Same hardware as the XR18; USB must be set to 18x18 on the mixer
		Fingerprint:   "MIDAS_MR18",
		ProfilePrefix: "mr18",
		DisplayName:   "Midas MR18",
		Channels:      xAir18Channels("mr18"),
	},
}

// xAir18Channels returns the 18x18 USB layout shared by the Behringer XR18 and
// Midas MR18: inputs 1-16 on AUX0-AUX15, stereo main bus on AUX16+AUX17.
func xAir18Channels(prefix string) []ChannelTemplate {
	channels := make([]ChannelTemplate, 0, 17)
	for i := 1; i <= 16; i++ {
		channels = append(channels, ChannelTemplate{
			ID:         fmt.Sprintf("%s_ch%d", prefix, i),
			Name:       fmt.Sprintf("ch%d", i),
			PortSuffix: fmt.Sprintf("capture_AUX%d", i-1),
			AudioMode:  "mono",
			Type:       "hardware",
			Volume:     4.0,
		})
	}
	return append(channels, ChannelTemplate{
		ID: prefix + "_main", Name: "main", PortSuffix: "capture_AUX16", PortSuffix2: "capture_AUX17",
		AudioMode: "stereo", Type: "hardware", Volume: 4.0,
	})
}
