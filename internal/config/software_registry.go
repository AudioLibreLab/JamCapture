package config

// SoftwareSourceTemplate describes a well-known software audio source.
// PortFL and PortFR are the exact JACK port names the app produces when running.
type SoftwareSourceTemplate struct {
	ID        string
	Name      string
	AppName   string // human-readable, for display only
	PortFL    string // left (or mono) port
	PortFR    string // right port; empty = mono
	AudioMode string // "stereo" or "mono"
	Type      string // always "monitor"
	Volume    float64
	Delay     int // ms
}

// WellKnownSoftwareSources is the software source registry.
// To add a new app: append a SoftwareSourceTemplate with the exact port names
// (check with: pw-link -io | grep <AppName>).
var WellKnownSoftwareSources = []SoftwareSourceTemplate{
	// Browsers — primary use case: YouTube, streaming backing tracks
	{ID: "chrome_stereo", Name: "chrome", AppName: "Google Chrome", PortFL: "Google Chrome:output_FL", PortFR: "Google Chrome:output_FR", AudioMode: "stereo", Type: "monitor", Volume: 0.8},
	{ID: "chromium_stereo", Name: "chromium", AppName: "Chromium", PortFL: "Chromium:output_FL", PortFR: "Chromium:output_FR", AudioMode: "stereo", Type: "monitor", Volume: 0.8},
	{ID: "brave_stereo", Name: "brave", AppName: "Brave", PortFL: "Brave:output_FL", PortFR: "Brave:output_FR", AudioMode: "stereo", Type: "monitor", Volume: 0.8},
	{ID: "firefox_stereo", Name: "firefox", AppName: "Firefox", PortFL: "Firefox:output_FL", PortFR: "Firefox:output_FR", AudioMode: "stereo", Type: "monitor", Volume: 0.8},
	{ID: "edge_stereo", Name: "edge", AppName: "Microsoft Edge", PortFL: "Microsoft Edge:output_FL", PortFR: "Microsoft Edge:output_FR", AudioMode: "stereo", Type: "monitor", Volume: 0.8},
	{ID: "opera_stereo", Name: "opera", AppName: "Opera", PortFL: "Opera:output_FL", PortFR: "Opera:output_FR", AudioMode: "stereo", Type: "monitor", Volume: 0.8},
	// Media players
	{ID: "vlc_stereo", Name: "vlc", AppName: "VLC", PortFL: "VLC media player:output_FL", PortFR: "VLC media player:output_FR", AudioMode: "stereo", Type: "monitor", Volume: 0.8},
	{ID: "spotify_stereo", Name: "spotify", AppName: "Spotify", PortFL: "spotify:output_FL", PortFR: "spotify:output_FR", AudioMode: "stereo", Type: "monitor", Volume: 0.8},
}
