package audio

import "strings"

// knownSoftwareApps are application names that produce audio output in PipeWire/JACK.
// Used for retry strategy (isEphemeralPort) and source categorization (CategorizeAndGroup).
var knownSoftwareApps = []string{
	"chrome", "google chrome", "chromium", "firefox", "spotify", "discord", "steam",
	"vlc", "mpv", "zoom", "teams", "slack", "wire", "obs",
}

// CategorizeAndGroup categorizes raw pw-link -io ports into:
//   - hwInputs: hardware capture ports, one mono group per port
//   - swSources: software output groups (stereo pairs or mono)
//
// Each inner slice represents one channel: 1 port = mono, 2 ports = stereo.
func CategorizeAndGroup(ports []string) (hwInputs [][]string, swSources [][]string) {
	portSet := make(map[string]bool, len(ports))
	for _, p := range ports {
		portSet[p] = true
	}

	swHandled := make(map[string]bool)

	for _, port := range ports {
		lower := strings.ToLower(port)

		// Skip: hardware playback, JACK system ports, jamcapture's own recorder ports, v4l2 video nodes
		if strings.Contains(lower, ":playback_") ||
			strings.HasPrefix(lower, "system:") ||
			strings.HasPrefix(lower, "jamcapture") ||
			strings.HasPrefix(lower, "v4l2_") {
			continue
		}

		// Hardware capture port → one mono group per port
		if strings.Contains(port, ":capture_") {
			hwInputs = append(hwInputs, []string{port})
			continue
		}

		// Software output port
		if isSoftwareOutputPort(port) {
			if swHandled[port] {
				continue
			}
			partner := flToFrPartner(port)
			if partner != "" && portSet[partner] && !swHandled[partner] {
				swSources = append(swSources, []string{port, partner})
				swHandled[port] = true
				swHandled[partner] = true
			} else if !strings.HasSuffix(port, "_FR") && !strings.HasSuffix(port, "_R") {
				swSources = append(swSources, []string{port})
				swHandled[port] = true
			}
		}
	}

	return hwInputs, swSources
}

// isSoftwareOutputPort returns true for ports from known software applications or
// non-ALSA ports that follow the "Device:output_*" naming pattern.
func isSoftwareOutputPort(port string) bool {
	lower := strings.ToLower(port)
	for _, app := range knownSoftwareApps {
		if strings.HasPrefix(lower, app+":") {
			return true
		}
	}
	// Non-ALSA port with output_ pattern (e.g. PipeWire virtual sinks)
	return !strings.HasPrefix(lower, "alsa_") && strings.Contains(port, ":output_")
}

// flToFrPartner returns the _FR or _R counterpart port name for a _FL or _L port.
// Returns "" if the port does not end with _FL or _L.
func flToFrPartner(port string) string {
	switch {
	case strings.HasSuffix(port, "_FL"):
		return port[:len(port)-3] + "_FR"
	case strings.HasSuffix(port, "_L"):
		return port[:len(port)-2] + "_R"
	default:
		return ""
	}
}
