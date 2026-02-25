package mix

import (
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/audiolibrelab/jamcapture/internal/config"
)

type Mixer struct {
	cfg *config.Config
}

func New(cfg *config.Config) *Mixer {
	return &Mixer{cfg: cfg}
}

func (m *Mixer) Mix(songName string) error {
	cleanName := m.cleanFileName(songName)
	inputFile := filepath.Join(m.cfg.Output.Directory, cleanName+".mkv")
	outputFile := filepath.Join(m.cfg.Output.Directory, cleanName+"."+m.cfg.Output.Format)

	// Check if input file exists
	if _, err := os.Stat(inputFile); err != nil {
		return fmt.Errorf("input file not found: %s", inputFile)
	}

	// Remove existing output file
	os.Remove(outputFile)

	// Build FFmpeg filter based on channel configuration
	mixFilter, outputChannels := m.cfg.BuildMixFilter()
	if mixFilter == "" {
		return fmt.Errorf("no valid mix configuration found")
	}

	// Prepare FFmpeg command
	cmd := exec.Command("ffmpeg",
		"-i", inputFile,
		"-filter_complex", mixFilter,
		"-ac", fmt.Sprintf("%d", outputChannels),
		"-ar", fmt.Sprintf("%d", m.cfg.Audio.SampleRate),
		"-c:a", m.cfg.Output.Format,
		"-y", // Overwrite output file
		outputFile,
	)

	slog.Debug("Running FFmpeg for mixing", "command", strings.Join(cmd.Args, " "))

	// Run FFmpeg
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("FFmpeg mixing failed: %w\nOutput: %s", err, string(output))
	}

	// Verify output file was created
	if _, err := os.Stat(outputFile); err != nil {
		return fmt.Errorf("output file not created: %s", outputFile)
	}

	slog.Info("Mixed audio file saved to", "file", outputFile)
	return nil
}

func (m *Mixer) MixWithOptions(songName string, guitarVol, backingVol float64, delayMs int) error {
	// Store original values
	originalChannels := make([]config.Channel, len(m.cfg.Channels))
	copy(originalChannels, m.cfg.Channels)

	// Apply temporary values to channel config
	for i, channel := range m.cfg.Channels {
		if channel.Name == "guitar" && guitarVol > 0 {
			m.cfg.Channels[i].Volume = guitarVol
		}
		if (channel.Name == "monitor" || channel.Type == "monitor") && backingVol > 0 {
			m.cfg.Channels[i].Volume = backingVol
		}
		if delayMs >= 0 {
			m.cfg.Channels[i].Delay = delayMs
		}
	}

	err := m.Mix(songName)

	// Restore original values
	m.cfg.Channels = originalChannels

	return err
}

// MixWithChannelVolumes creates a mix with custom volume levels for specific channels
func (m *Mixer) MixWithChannelVolumes(songName string, channelVolumes map[string]float64) error {
	// Store original values
	originalChannels := make([]config.Channel, len(m.cfg.Channels))
	copy(originalChannels, m.cfg.Channels)

	// Apply custom volumes to matching channels
	for i, channel := range m.cfg.Channels {
		if vol, exists := channelVolumes[channel.Name]; exists {
			m.cfg.Channels[i].Volume = vol
		}
	}

	slog.Debug("Mixing with custom channel volumes", "song", songName, "volumes", channelVolumes)

	// Use existing Mix() method - reuses all existing logic
	err := m.Mix(songName)

	// Restore original values
	m.cfg.Channels = originalChannels

	return err
}

func (m *Mixer) cleanFileName(name string) string {
	// Remove special characters and replace spaces with underscores
	// Allows: letters, numbers, spaces, hyphens, underscores
	var result strings.Builder
	for _, r := range name {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == ' ' || r == '-' || r == '_' {
			result.WriteRune(r)
		}
	}
	return strings.ReplaceAll(strings.TrimSpace(result.String()), " ", "_")
}

