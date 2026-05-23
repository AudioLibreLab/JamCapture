package config

import (
	"fmt"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
)

var trailingDigits = regexp.MustCompile(`(\d+)$`)

// NamedPort is a hardware port group paired with an optional device description from pw-dump.
type NamedPort struct {
	Ports      []string // port name(s): 1 = mono, 2 = stereo
	DeviceName string   // node.description from pw-dump; "" = fall back to hw_input_N
}

// sanitizeID converts a string to a lowercase underscore-separated slug suitable for use as an ID.
func sanitizeID(s string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(s) {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
		} else {
			b.WriteRune('_')
		}
	}
	parts := strings.FieldsFunc(b.String(), func(r rune) bool { return r == '_' })
	return strings.Join(parts, "_")
}

// stereoRightPort returns the right-channel port suffix for a stereo pair.
// If suffix2 is non-empty it is used directly; otherwise the last digit
// group in suffix is incremented (e.g. "capture_AUX16" → "capture_AUX17").
func stereoRightPort(suffix, suffix2 string) string {
	if suffix2 != "" {
		return suffix2
	}
	return trailingDigits.ReplaceAllStringFunc(suffix, func(s string) string {
		n, _ := strconv.Atoi(s)
		return strconv.Itoa(n + 1)
	})
}

// IdentifyDevices matches hwInputs against KnownDevices.
// Recognised ports are collected into DetectedDevice structs; the rest go into unmatched.
func IdentifyDevices(hwInputs [][]string) (devices []DetectedDevice, unmatched [][]string) {
	consumed := make([]bool, len(hwInputs))

	for di := range KnownDevices {
		tmpl := &KnownDevices[di]
		var dev *DetectedDevice

		for i, group := range hwInputs {
			if consumed[i] {
				continue
			}
			for _, port := range group {
				if !strings.Contains(port, tmpl.Fingerprint) {
					continue
				}
				if dev == nil {
					prefix := port[:strings.LastIndex(port, ":")]
					dev = &DetectedDevice{
						Template:    tmpl,
						PortPrefix:  prefix,
						ActualPorts: make(map[string]bool),
					}
				}
				suffix := port[strings.LastIndex(port, ":")+1:]
				dev.ActualPorts[suffix] = true
				consumed[i] = true
				break
			}
		}

		if dev != nil {
			devices = append(devices, *dev)
		}
	}

	for i, group := range hwInputs {
		if !consumed[i] {
			unmatched = append(unmatched, group)
		}
	}
	return devices, unmatched
}

// MatchSoftwareSources returns the WellKnownSoftwareSources entries whose
// ports are currently present in swSources (i.e. the app is running).
func MatchSoftwareSources(swSources [][]string) []SoftwareSourceTemplate {
	portSet := make(map[string]bool)
	for _, group := range swSources {
		for _, port := range group {
			portSet[port] = true
		}
	}

	var running []SoftwareSourceTemplate
	for _, sw := range WellKnownSoftwareSources {
		if portSet[sw.PortFL] {
			running = append(running, sw)
		}
	}
	return running
}

// GenerateConfig builds a RootConfig from identified hardware devices,
// running software sources, and any unmatched (generic) hardware ports.
// Returns nil if no sources are provided.
func GenerateConfig(devices []DetectedDevice, running []SoftwareSourceTemplate, genericPorts []NamedPort) *RootConfig {
	if len(devices) == 0 && len(running) == 0 && len(genericPorts) == 0 {
		return nil
	}

	var defs []ChannelDefinition
	profiles := make(map[string]*ConfigProfile)

	// Known hardware devices
	for _, dev := range devices {
		var deviceRefs []ChannelReference

		for _, ch := range dev.Template.Channels {
			if !dev.ActualPorts[ch.PortSuffix] {
				continue
			}
			leftPort := dev.PortPrefix + ":" + ch.PortSuffix

			def := ChannelDefinition{
				ID:        ch.ID,
				Name:      ch.Name,
				AudioMode: ch.AudioMode,
				Type:      ch.Type,
				Volume:    ch.Volume,
			}

			if ch.AudioMode == "stereo" {
				rightSuffix := stereoRightPort(ch.PortSuffix, ch.PortSuffix2)
				if !dev.ActualPorts[rightSuffix] {
					continue // incomplete stereo pair
				}
				def.Sources = []string{leftPort, dev.PortPrefix + ":" + rightSuffix}
			} else {
				def.Sources = []string{leftPort}
			}

			defs = append(defs, def)
			deviceRefs = append(deviceRefs, ChannelReference{Ref: ch.ID})
		}

		if len(deviceRefs) > 0 {
			profiles[dev.Template.ProfilePrefix+"_studio"] = &ConfigProfile{
				AutoMix:  true,
				Channels: deviceRefs,
				Output:   OutputConfig{Format: "flac"},
			}
		}
	}

	// Running software sources
	var swRefs []ChannelReference
	for _, sw := range running {
		sources := []string{sw.PortFL}
		if sw.AudioMode == "stereo" && sw.PortFR != "" {
			sources = []string{sw.PortFL, sw.PortFR}
		}
		defs = append(defs, ChannelDefinition{
			ID:        sw.ID,
			Name:      sw.Name,
			Sources:   sources,
			AudioMode: sw.AudioMode,
			Type:      sw.Type,
			Volume:    sw.Volume,
			Delay:     sw.Delay,
		})
		swRefs = append(swRefs, ChannelReference{Ref: sw.ID})
	}

	// Generic fallback for unrecognised hardware — use pw-dump device names when available
	genericIDs := make([]string, len(genericPorts))
	for i, p := range genericPorts {
		id := fmt.Sprintf("hw_input_%d", i+1)
		name := id
		if p.DeviceName != "" {
			slug := sanitizeID(p.DeviceName)
			id = fmt.Sprintf("%s_%d", slug, i+1)
			name = fmt.Sprintf("%s (input %d)", p.DeviceName, i+1)
		}
		genericIDs[i] = id
		defs = append(defs, ChannelDefinition{
			ID:        id,
			Name:      name,
			Sources:   p.Ports,
			AudioMode: "mono",
			Type:      "hardware",
			Volume:    4.0,
		})
	}

	// _with_monitor profiles (hardware + active software sources)
	if len(swRefs) > 0 {
		for _, dev := range devices {
			prefix := dev.Template.ProfilePrefix
			studio, ok := profiles[prefix+"_studio"]
			if !ok {
				continue
			}
			refs := append(append([]ChannelReference{}, studio.Channels...), swRefs...)
			profiles[prefix+"_with_monitor"] = &ConfigProfile{
				AutoMix:  true,
				Channels: refs,
				Output:   OutputConfig{Format: "flac"},
			}
		}
	}

	// all_devices profile when multiple hardware devices are present
	if len(devices) > 1 {
		var allRefs []ChannelReference
		for _, dev := range devices {
			if p, ok := profiles[dev.Template.ProfilePrefix+"_studio"]; ok {
				allRefs = append(allRefs, p.Channels...)
			}
		}
		if len(allRefs) > 0 {
			profiles["all_devices"] = &ConfigProfile{
				AutoMix:  true,
				Channels: allRefs,
				Output:   OutputConfig{Format: "flac"},
			}
		}
	}

	// Default fallback when no known device matched
	if len(devices) == 0 {
		var refs []ChannelReference
		for _, id := range genericIDs {
			refs = append(refs, ChannelReference{Ref: id})
		}
		for _, sw := range running {
			refs = append(refs, ChannelReference{Ref: sw.ID})
		}
		if len(refs) > 0 {
			profiles["default"] = &ConfigProfile{
				AutoMix:  true,
				Channels: refs,
				Output:   OutputConfig{Format: "flac"},
			}
		}
	}

	activeConfig := selectActiveConfig(devices, running)

	return &RootConfig{
		ActiveConfig: activeConfig,
		Audio: &AudioConfig{
			Backend:    "pipewire",
			SampleRate: 48000,
		},
		Globals: &GlobalsConfig{
			Output: GlobalOutputConfig{
				RecordingsDirectory:    "~/Audio/JamCapture/Recordings",
				BackingtracksDirectory: "~/Audio/JamCapture/BackingTracks",
			},
			AudioPlayer: detectAudioPlayer(),
		},
		Definitions:              &DefinitionsConfig{Channels: defs},
		Configs:                  profiles,
		SupportedAudioExtensions: []string{"flac", "wav", "mp3"},
	}
}

func selectActiveConfig(devices []DetectedDevice, running []SoftwareSourceTemplate) string {
	if len(devices) > 0 {
		if len(running) > 0 {
			return devices[0].Template.ProfilePrefix + "_with_monitor"
		}
		return devices[0].Template.ProfilePrefix + "_studio"
	}
	return "default"
}

func detectAudioPlayer() string {
	for _, app := range []string{"audacity", "vlc", "rhythmbox", "xdg-open"} {
		if _, err := exec.LookPath(app); err == nil {
			return app
		}
	}
	return ""
}
