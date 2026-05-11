package config

import (
	"fmt"
	"strings"
)

// GenerateDefault creates a RootConfig from auto-detected port groups.
//
// hwInputs: hardware capture ports — each inner slice is one mono channel (1 port).
// swSources: software output groups — 1 port = mono, 2 ports = stereo pair.
//
// Returns nil if no ports are provided.
func GenerateDefault(hwInputs [][]string, swSources [][]string) *RootConfig {
	if len(hwInputs) == 0 && len(swSources) == 0 {
		return nil
	}

	var defs []ChannelDefinition
	var refs []ChannelReference

	// Hardware inputs — one mono channel per capture port
	for i, ports := range hwInputs {
		id := fmt.Sprintf("hw_input_%d", i+1)
		defs = append(defs, ChannelDefinition{
			ID:        id,
			Sources:   ports,
			AudioMode: "mono",
			Type:      "input",
			Volume:    4.0,
			Delay:     0,
		})
		refs = append(refs, ChannelReference{Ref: id})
	}

	// Software sources — stereo pairs or mono
	seenNames := make(map[string]int)
	for _, ports := range swSources {
		baseName := portAppName(ports[0])
		count := seenNames[baseName]
		seenNames[baseName]++

		audioMode := "mono"
		suffix := "_mono"
		if len(ports) == 2 {
			audioMode = "stereo"
			suffix = "_stereo"
		}

		id := baseName + suffix
		if count > 0 {
			id = fmt.Sprintf("%s%s_%d", baseName, suffix, count+1)
		}

		defs = append(defs, ChannelDefinition{
			ID:        id,
			Sources:   ports,
			AudioMode: audioMode,
			Type:      "monitor",
			Volume:    0.8,
			Delay:     0,
		})
		refs = append(refs, ChannelReference{Ref: id})
	}

	return &RootConfig{
		ActiveConfig: "default",
		Audio: &AudioConfig{
			Backend:    "pipewire",
			SampleRate: 48000,
		},
		Globals: &GlobalsConfig{
			Output: GlobalOutputConfig{
				RecordingsDirectory:    "~/Audio/JamCapture/Recordings",
				BackingtracksDirectory: "~/Audio/JamCapture/BackingTracks",
			},
		},
		Definitions: &DefinitionsConfig{
			Channels: defs,
		},
		Configs: map[string]*ConfigProfile{
			"default": {
				AutoMix:  true,
				Channels: refs,
				Output:   OutputConfig{Format: "flac"},
			},
		},
		SupportedAudioExtensions: []string{"flac", "wav", "mp3"},
	}
}

// portAppName extracts a sanitized lowercase identifier from a JACK port string.
// "Chrome:output_FL" → "chrome", "Google Chrome:output_FL" → "google_chrome"
func portAppName(port string) string {
	idx := strings.Index(port, ":")
	if idx < 0 {
		return strings.ToLower(strings.ReplaceAll(port, " ", "_"))
	}
	return strings.ReplaceAll(strings.ToLower(port[:idx]), " ", "_")
}
