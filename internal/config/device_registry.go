package config

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
	Type        string // "input" or "monitor"
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
		Channels: []ChannelTemplate{
			{ID: "xr18_ch1", Name: "ch1", PortSuffix: "capture_AUX0", AudioMode: "mono", Type: "input", Volume: 4.0},
			{ID: "xr18_ch2", Name: "ch2", PortSuffix: "capture_AUX1", AudioMode: "mono", Type: "input", Volume: 4.0},
			{ID: "xr18_ch3", Name: "ch3", PortSuffix: "capture_AUX2", AudioMode: "mono", Type: "input", Volume: 4.0},
			{ID: "xr18_ch4", Name: "ch4", PortSuffix: "capture_AUX3", AudioMode: "mono", Type: "input", Volume: 4.0},
			{ID: "xr18_ch5", Name: "ch5", PortSuffix: "capture_AUX4", AudioMode: "mono", Type: "input", Volume: 4.0},
			{ID: "xr18_ch6", Name: "ch6", PortSuffix: "capture_AUX5", AudioMode: "mono", Type: "input", Volume: 4.0},
			{ID: "xr18_ch7", Name: "ch7", PortSuffix: "capture_AUX6", AudioMode: "mono", Type: "input", Volume: 4.0},
			{ID: "xr18_ch8", Name: "ch8", PortSuffix: "capture_AUX7", AudioMode: "mono", Type: "input", Volume: 4.0},
			{ID: "xr18_ch9", Name: "ch9", PortSuffix: "capture_AUX8", AudioMode: "mono", Type: "input", Volume: 4.0},
			{ID: "xr18_ch10", Name: "ch10", PortSuffix: "capture_AUX9", AudioMode: "mono", Type: "input", Volume: 4.0},
			{ID: "xr18_ch11", Name: "ch11", PortSuffix: "capture_AUX10", AudioMode: "mono", Type: "input", Volume: 4.0},
			{ID: "xr18_ch12", Name: "ch12", PortSuffix: "capture_AUX11", AudioMode: "mono", Type: "input", Volume: 4.0},
			{ID: "xr18_ch13", Name: "ch13", PortSuffix: "capture_AUX12", AudioMode: "mono", Type: "input", Volume: 4.0},
			{ID: "xr18_ch14", Name: "ch14", PortSuffix: "capture_AUX13", AudioMode: "mono", Type: "input", Volume: 4.0},
			{ID: "xr18_ch15", Name: "ch15", PortSuffix: "capture_AUX14", AudioMode: "mono", Type: "input", Volume: 4.0},
			{ID: "xr18_ch16", Name: "ch16", PortSuffix: "capture_AUX15", AudioMode: "mono", Type: "input", Volume: 4.0},
			// AUX16+AUX17 are the stereo main bus mix output
			{ID: "xr18_main", Name: "main", PortSuffix: "capture_AUX16", PortSuffix2: "capture_AUX17", AudioMode: "stereo", Type: "input", Volume: 4.0},
		},
	},
	{
		Fingerprint:   "Focusrite_Scarlett_2i2",
		ProfilePrefix: "2i2",
		DisplayName:   "Focusrite Scarlett 2i2",
		Channels: []ChannelTemplate{
			{ID: "2i2_guitar", Name: "guitar", PortSuffix: "capture_FR", AudioMode: "mono", Type: "input", Volume: 4.0},
			{ID: "2i2_mic", Name: "mic", PortSuffix: "capture_FL", AudioMode: "mono", Type: "input", Volume: 3.0},
		},
	},
	{
		Fingerprint:   "Focusrite_Scarlett_4i4",
		ProfilePrefix: "4i4",
		DisplayName:   "Focusrite Scarlett 4i4",
		Channels: []ChannelTemplate{
			{ID: "4i4_input1", Name: "input1", PortSuffix: "capture_FL", AudioMode: "mono", Type: "input", Volume: 4.0},
			{ID: "4i4_input2", Name: "input2", PortSuffix: "capture_FR", AudioMode: "mono", Type: "input", Volume: 4.0},
			{ID: "4i4_input3", Name: "input3", PortSuffix: "capture_RL", AudioMode: "mono", Type: "input", Volume: 4.0},
			{ID: "4i4_input4", Name: "input4", PortSuffix: "capture_RR", AudioMode: "mono", Type: "input", Volume: 4.0},
		},
	},
}
