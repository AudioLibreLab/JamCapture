package mix

import (
	"encoding/json"
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

	// Analyze the input file to determine available streams
	analysis, err := m.analyzeMKVFile(inputFile)
	if err != nil {
		return fmt.Errorf("failed to analyze input file: %w", err)
	}

	// Build FFmpeg filter based on actual file structure
	mixFilter, outputChannels := m.cfg.BuildMixFilterForFile(analysis)
	if mixFilter == "" {
		return fmt.Errorf("no valid mix configuration found for file with %d tracks", len(analysis.Tracks))
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

// MixWithChannelAndGlobalVolumes creates a mix with custom volume levels for specific channels and a global volume
func (m *Mixer) MixWithChannelAndGlobalVolumes(songName string, channelVolumes map[string]float64, globalVolume float64) error {
	// Store original values
	originalChannels := make([]config.Channel, len(m.cfg.Channels))
	copy(originalChannels, m.cfg.Channels)

	// Apply custom volumes to matching channels
	for i, channel := range m.cfg.Channels {
		if vol, exists := channelVolumes[channel.Name]; exists {
			m.cfg.Channels[i].Volume = vol
		}
	}

	slog.Debug("Mixing with custom channel volumes and global volume", "song", songName, "volumes", channelVolumes, "global_volume", globalVolume)

	// Use custom logic for mixing with global volume
	err := m.mixWithGlobalVolume(songName, globalVolume)

	// Restore original values
	m.cfg.Channels = originalChannels

	return err
}

// mixWithGlobalVolume performs the actual mixing with global volume control
func (m *Mixer) mixWithGlobalVolume(songName string, globalVolume float64) error {
	cleanName := m.cleanFileName(songName)

	// Build file paths
	inputFile := filepath.Join(m.cfg.Output.Directory, fmt.Sprintf("%s.mkv", cleanName))
	outputFile := filepath.Join(m.cfg.Output.Directory, fmt.Sprintf("%s.%s", cleanName, m.cfg.Output.Format))

	// Check if input file exists
	if _, err := os.Stat(inputFile); os.IsNotExist(err) {
		return fmt.Errorf("input file not found: %s", inputFile)
	}

	// Analyze the input file to determine available streams
	analysis, err := m.analyzeMKVFile(inputFile)
	if err != nil {
		return fmt.Errorf("failed to analyze input file: %w", err)
	}

	// Build FFmpeg filter with global volume based on actual file structure
	mixFilter, outputChannels := m.cfg.BuildMixFilterForFileWithGlobalVolume(analysis, globalVolume)
	if mixFilter == "" {
		return fmt.Errorf("no valid mix configuration found for file with %d tracks", len(analysis.Tracks))
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

// analyzeMKVFile extracts track information from an MKV file using ffprobe
func (m *Mixer) analyzeMKVFile(filePath string) (*config.MKVAnalysis, error) {
	// Validate file exists
	if _, err := os.Stat(filePath); err != nil {
		return nil, fmt.Errorf("MKV file not found: %s", filePath)
	}

	// Use ffprobe to extract stream information
	cmd := exec.Command("ffprobe",
		"-v", "quiet",
		"-print_format", "json",
		"-show_streams",
		filePath,
	)

	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("ffprobe failed for %s: %w", filePath, err)
	}

	// Parse ffprobe output
	var probeResult struct {
		Streams []struct {
			Index       int               `json:"index"`
			CodecType   string            `json:"codec_type"`
			Channels    int               `json:"channels"`
			Tags        map[string]string `json:"tags"`
		} `json:"streams"`
	}

	if err := json.Unmarshal(output, &probeResult); err != nil {
		return nil, fmt.Errorf("failed to parse ffprobe output for %s: %w", filePath, err)
	}

	// Extract audio tracks only
	var tracks []config.TrackInfo
	for _, stream := range probeResult.Streams {
		if stream.CodecType != "audio" {
			continue
		}

		// Extract title from metadata, fallback to index-based name
		title := fmt.Sprintf("Track %d", stream.Index)
		if streamTitle, exists := stream.Tags["title"]; exists && streamTitle != "" {
			title = streamTitle
		} else if streamTitle, exists := stream.Tags["TITLE"]; exists && streamTitle != "" {
			title = streamTitle
		}

		track := config.TrackInfo{
			Index:    stream.Index,
			Name:     fmt.Sprintf("track_%d", stream.Index),
			Title:    title,
			Channels: stream.Channels,
		}

		tracks = append(tracks, track)
	}

	filename := filepath.Base(filePath)
	analysis := &config.MKVAnalysis{
		Filename:   filename,
		TrackCount: len(tracks),
		Tracks:     tracks,
	}

	slog.Debug("MKV analysis completed", "filename", filename, "tracks", len(tracks))
	return analysis, nil
}

