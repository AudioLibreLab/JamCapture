package service

import (
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/audiolibrelab/jamcapture/internal/audio"
	"github.com/audiolibrelab/jamcapture/internal/config"
	"github.com/audiolibrelab/jamcapture/internal/mix"
	"github.com/audiolibrelab/jamcapture/internal/play"
	"gopkg.in/yaml.v3"
)


// Service represents the core JamCapture service interface
type Service interface {
	// Recording operations
	StartReady(songName string) error
	CancelReady() error
	StopRecording() error
	GetRecordingStatus() (RecordingStatus, *RecordingSession)

	// Mixing operations
	Mix(songName string) error
	MixWithOptions(songName string, guitarVolume, backingVolume float64, delay int) error

	// Playback operations
	Play(songName string) error

	// Pipeline operations
	RunPipeline(songName string, steps string) error

	// Configuration operations
	LoadProfile(profile string) error
	GetConfig() *config.Config

	// Information operations
	GetSongInfo(songName string) (*SongInfo, error)
	GetChannelStatus() map[string]string
	GetLastError() string

	// Backing track operations
	ListBackingtracks() ([]BackingtrackInfo, error)
	GetSelectedBackingtrack() (*BackingtrackInfo, error)
	SetSelectedBackingtrack(name string) error
	ConvertRecordingToBackingtrack(recordingName string) error

	// MKV mixing operations
	ListMKVFiles() ([]MKVFileInfo, error)
	AnalyzeMKVFile(filename string) (*MKVAnalysis, error)
	MixWithTrackVolumes(filename string, trackVolumes map[string]float64) error
}

// RecordingStatus represents the current recording state
type RecordingStatus string

const (
	StatusStandby   RecordingStatus = "STANDBY"
	StatusReady     RecordingStatus = "READY"
	StatusRecording RecordingStatus = "RECORDING"
	StatusError     RecordingStatus = "ERROR"
)

// RecordingSession contains information about the current recording session
type RecordingSession struct {
	SongName     string    `json:"song_name"`
	StartTime    time.Time `json:"start_time"`
	OutputFile   string    `json:"output_file"`
	ChannelCount int       `json:"channel_count"`
	ChannelNames []string  `json:"channel_names"`
}

// SongInfo contains file path information for a song
type SongInfo struct {
	OutputMKV   string `json:"output_mkv"`
	OutputMixed string `json:"output_mixed"`
	CleanName   string `json:"clean_name"`
}

// BackingtrackInfo contains information about a backing track file
type BackingtrackInfo struct {
	Name         string    `json:"name"`
	Path         string    `json:"path"`
	Size         int64     `json:"size"`
	SizeHuman    string    `json:"size_human"`
	ModTime      time.Time `json:"mod_time"`
	ModTimeHuman string    `json:"mod_time_human"`
	Extension    string    `json:"extension"`
	IsSelected   bool      `json:"is_selected"`
	StreamURL    string    `json:"stream_url"`
	DownloadURL  string    `json:"download_url"`
}

// MKVFileInfo contains information about an MKV file for mixing
type MKVFileInfo struct {
	Name         string    `json:"name"`
	Path         string    `json:"path"`
	Size         int64     `json:"size"`
	SizeHuman    string    `json:"size_human"`
	ModTime      time.Time `json:"mod_time"`
	ModTimeHuman string    `json:"mod_time_human"`
	StreamURL    string    `json:"stream_url"`
	AnalyzeURL   string    `json:"analyze_url"`
}

// MKVAnalysis contains track information extracted from an MKV file
type MKVAnalysis struct {
	Filename    string      `json:"filename"`
	TrackCount  int         `json:"track_count"`
	Tracks      []TrackInfo `json:"tracks"`
}

// TrackInfo contains information about a single track within an MKV file
type TrackInfo struct {
	Index    int    `json:"index"`
	Name     string `json:"name"`
	Title    string `json:"title"`
	Channels int    `json:"channels"`
}

// MixOptions contains mixing configuration
type MixOptions struct {
	GuitarVolume  float64
	BackingVolume float64
	Delay         int
}

// BackingtrackConfig represents the backing track configuration stored in conf.yaml
type BackingtrackConfig struct {
	SelectedBackingtrack string `yaml:"selected_backingtrack"`
	LastUpdated          string `yaml:"last_updated"`
}

// JamCaptureService is the main service implementation
type JamCaptureService struct {
	cfg        *config.Config
	configFile string
	recorder   audio.Recorder
	logWriter  io.Writer

	// Backing track management
	backingtrackMutex sync.RWMutex

	// Error tracking
	lastError      string
	lastErrorMutex sync.RWMutex
}

// New creates a new JamCapture service instance
func New(cfg *config.Config, configFile string, logWriter io.Writer) Service {
	if logWriter == nil {
		logWriter = io.Discard
	}

	return &JamCaptureService{
		cfg:        cfg,
		configFile: configFile,
		recorder:   audio.NewRecorder(cfg, logWriter),
		logWriter:  logWriter,
	}
}

// StartReady prepares for recording (STANDBY -> READY)
func (s *JamCaptureService) StartReady(songName string) error {
	slog.Debug("Service.StartReady called", "song_name", songName)
	s.clearLastError() // Clear any previous errors when starting a new operation
	err := s.recorder.StartReady(songName)
	if err != nil {
		slog.Error("Service.StartReady failed", "error", err)
		s.setLastError(fmt.Sprintf("Failed to start recording: %v", err))
	} else {
		slog.Debug("Service.StartReady completed successfully")
	}
	return err
}

// CancelReady cancels ready state (READY -> STANDBY)
func (s *JamCaptureService) CancelReady() error {
	return s.recorder.CancelReady()
}

// StopRecording stops the current recording session
func (s *JamCaptureService) StopRecording() error {
	err := s.recorder.Stop()
	if err != nil {
		s.setLastError(fmt.Sprintf("Failed to stop recording: %v", err))
	} else {
		s.clearLastError() // Clear error on successful stop
	}
	return err
}

// GetRecordingStatus returns the current recording status and session info
func (s *JamCaptureService) GetRecordingStatus() (RecordingStatus, *RecordingSession) {
	status, session := s.recorder.GetStatus()

	// Convert from audio.Status to service.RecordingStatus
	var svcStatus RecordingStatus
	switch status {
	case audio.StatusStandby:
		svcStatus = StatusStandby
		// Auto-clear any previous errors when returning to STANDBY
		s.clearLastError()
	case audio.StatusReady:
		svcStatus = StatusReady
		// Auto-clear any previous errors when successfully reaching READY
		s.clearLastError()
	case audio.StatusRecording:
		svcStatus = StatusRecording
		// Auto-clear any previous errors when successfully recording
		s.clearLastError()
	case audio.StatusError:
		svcStatus = StatusError
	}

	// Convert session if present
	var svcSession *RecordingSession
	if session != nil {
		svcSession = &RecordingSession{
			SongName:     session.SongName,
			StartTime:    session.StartTime,
			OutputFile:   session.OutputFile,
			ChannelCount: session.ChannelCount,
			ChannelNames: session.ChannelNames,
		}
	}

	return svcStatus, svcSession
}

// Mix mixes recorded tracks using configuration defaults
func (s *JamCaptureService) Mix(songName string) error {
	mixer := mix.New(s.cfg)
	return mixer.Mix(songName)
}

// MixWithOptions mixes recorded tracks with custom options
func (s *JamCaptureService) MixWithOptions(songName string, guitarVolume, backingVolume float64, delay int) error {
	mixer := mix.New(s.cfg)
	return mixer.MixWithOptions(songName, guitarVolume, backingVolume, delay)
}

// Play plays the mixed audio file
func (s *JamCaptureService) Play(songName string) error {
	player := play.New(s.cfg)
	return player.Play(songName)
}

// RunPipeline executes a sequence of operations (r=record, m=mix, p=play)
func (s *JamCaptureService) RunPipeline(songName string, steps string) error {
	for _, step := range steps {
		switch step {
		case 'r':
			// Start ready - recording will start automatically when sources are available
			if err := s.StartReady(songName); err != nil {
				return fmt.Errorf("pipeline ready start failed: %w", err)
			}
			// Note: Recording will start automatically when all sources are available.
			// In pipeline mode, the caller should wait for recording to start and then
			// handle the recording duration and call StopRecording() when appropriate
		case 'm':
			if err := s.Mix(songName); err != nil {
				return fmt.Errorf("pipeline mix failed: %w", err)
			}
		case 'p':
			if err := s.Play(songName); err != nil {
				return fmt.Errorf("pipeline play failed: %w", err)
			}
		default:
			return fmt.Errorf("unknown pipeline step: '%c' (valid: r=record, m=mix, p=play)", step)
		}
	}
	return nil
}

// LoadProfile loads a new configuration profile
func (s *JamCaptureService) LoadProfile(profile string) error {
	newCfg, err := config.LoadWithProfile(s.configFile, profile)
	if err != nil {
		return fmt.Errorf("failed to load profile '%s': %w", profile, err)
	}

	// Clean up old recorder
	if s.recorder != nil {
		s.recorder.Cleanup()
	}

	s.cfg = newCfg
	s.recorder = audio.NewRecorder(s.cfg, s.logWriter)
	return nil
}

// GetConfig returns the current configuration
func (s *JamCaptureService) GetConfig() *config.Config {
	return s.cfg
}

// GetSongInfo returns file path information for a song
func (s *JamCaptureService) GetSongInfo(songName string) (*SongInfo, error) {
	// This is a simplified implementation - you might want to move
	// the actual path resolution logic from cmd/info.go here
	cleanName := cleanFileName(songName)

	return &SongInfo{
		OutputMKV:   fmt.Sprintf("%s/%s.mkv", s.cfg.Output.Directory, cleanName),
		OutputMixed: fmt.Sprintf("%s/%s.%s", s.cfg.Output.Directory, cleanName, s.getOutputExtension()),
		CleanName:   cleanName,
	}, nil
}

// GetChannelStatus returns the availability status of configured channels
func (s *JamCaptureService) GetChannelStatus() map[string]string {
	return s.recorder.GetChannelStatus()
}


// Helper functions

func cleanFileName(name string) string {
	// Remove special characters and replace spaces with underscores
	// This matches the implementation in the original recorder
	var result strings.Builder
	for _, r := range name {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == ' ' {
			result.WriteRune(r)
		}
	}
	return strings.ReplaceAll(strings.TrimSpace(result.String()), " ", "_")
}

func (s *JamCaptureService) getOutputExtension() string {
	switch s.cfg.Output.Format {
	case "flac":
		return "flac"
	case "wav":
		return "wav"
	case "mp3":
		return "mp3"
	default:
		return "flac"
	}
}

// ===== BACKING TRACK SERVICE METHODS =====

// getBackingtracksDirectory returns the resolved backing tracks directory path
func (s *JamCaptureService) getBackingtracksDirectory() string {
	backingDir := s.cfg.Output.BackingtracksDirectory
	if backingDir == "" {
		backingDir = filepath.Join(s.cfg.Output.Directory, "BackingTracks")
	}
	return backingDir
}

// ListBackingtracks returns all backing tracks in the backingtracks directory
func (s *JamCaptureService) ListBackingtracks() ([]BackingtrackInfo, error) {
	s.backingtrackMutex.RLock()
	defer s.backingtrackMutex.RUnlock()

	backingDir := s.getBackingtracksDirectory()

	// Create directory if it doesn't exist
	if err := os.MkdirAll(backingDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create backingtracks directory: %w", err)
	}

	// Read directory contents
	files, err := os.ReadDir(backingDir)
	if err != nil {
		return nil, fmt.Errorf("failed to read backingtracks directory: %w", err)
	}

	// Get current selected backing track
	selected, _ := s.getSelectedBackingtrackName()

	var backingtracks []BackingtrackInfo
	supportedExts := map[string]bool{
		".flac": true,
		".wav":  true,
		".mp3":  true,
	}

	for _, file := range files {
		if file.IsDir() {
			continue
		}

		ext := strings.ToLower(filepath.Ext(file.Name()))
		if !supportedExts[ext] {
			continue
		}

		filePath := filepath.Join(backingDir, file.Name())
		info, err := file.Info()
		if err != nil {
			slog.Warn("Failed to get file info", "file", file.Name(), "error", err)
			continue
		}

		backing := BackingtrackInfo{
			Name:         file.Name(),
			Path:         filePath,
			Size:         info.Size(),
			SizeHuman:    formatBytes(info.Size()),
			ModTime:      info.ModTime(),
			ModTimeHuman: info.ModTime().Format("2006-01-02 15:04:05"),
			Extension:    strings.TrimPrefix(ext, "."),
			IsSelected:   file.Name() == selected,
			StreamURL:    fmt.Sprintf("/api/backingtracks/stream/%s", file.Name()),
			DownloadURL:  fmt.Sprintf("/api/backingtracks/download/%s", file.Name()),
		}

		backingtracks = append(backingtracks, backing)
	}

	// Sort by modification time (newest first), but selected one goes to top
	sort.Slice(backingtracks, func(i, j int) bool {
		if backingtracks[i].IsSelected {
			return true
		}
		if backingtracks[j].IsSelected {
			return false
		}
		return backingtracks[i].ModTime.After(backingtracks[j].ModTime)
	})

	return backingtracks, nil
}

// GetSelectedBackingtrack returns the currently selected backing track
func (s *JamCaptureService) GetSelectedBackingtrack() (*BackingtrackInfo, error) {
	backingtracks, err := s.ListBackingtracks()
	if err != nil {
		return nil, err
	}

	for _, bt := range backingtracks {
		if bt.IsSelected {
			return &bt, nil
		}
	}

	return nil, nil // No backing track selected
}

// SetSelectedBackingtrack sets the selected backing track
func (s *JamCaptureService) SetSelectedBackingtrack(name string) error {
	s.backingtrackMutex.Lock()
	defer s.backingtrackMutex.Unlock()

	backingDir := s.getBackingtracksDirectory()

	// Verify the file exists
	filePath := filepath.Join(backingDir, name)
	if _, err := os.Stat(filePath); err != nil {
		return fmt.Errorf("backing track file not found: %s", name)
	}

	// Update configuration
	config := &BackingtrackConfig{
		SelectedBackingtrack: name,
		LastUpdated:          time.Now().Format(time.RFC3339),
	}

	return s.saveBackingtrackConfig(config)
}

// ConvertRecordingToBackingtrack moves a recording file to the backingtracks directory
func (s *JamCaptureService) ConvertRecordingToBackingtrack(recordingName string) error {
	s.backingtrackMutex.Lock()
	defer s.backingtrackMutex.Unlock()

	// Source path (recording)
	srcPath := filepath.Join(s.cfg.Output.Directory, recordingName)

	// Verify source exists
	if _, err := os.Stat(srcPath); err != nil {
		return fmt.Errorf("recording file not found: %s", recordingName)
	}

	// Destination directory
	backingDir := s.getBackingtracksDirectory()

	// Create directory if it doesn't exist
	if err := os.MkdirAll(backingDir, 0755); err != nil {
		return fmt.Errorf("failed to create backingtracks directory: %w", err)
	}

	// Destination path (keep original filename)
	destPath := filepath.Join(backingDir, recordingName)

	// Move the file
	if err := os.Rename(srcPath, destPath); err != nil {
		return fmt.Errorf("failed to move recording to backingtracks: %w", err)
	}

	slog.Info("Converted recording to backing track", "recording", recordingName, "dest", destPath)

	// Set as selected backing track (without additional locking)
	config := &BackingtrackConfig{
		SelectedBackingtrack: recordingName,
		LastUpdated:          time.Now().Format(time.RFC3339),
	}

	return s.saveBackingtrackConfig(config)
}

// Helper methods for backing track configuration

func (s *JamCaptureService) getBackingtrackConfigPath() string {
	backingDir := s.getBackingtracksDirectory()
	return filepath.Join(backingDir, "conf.yaml")
}

func (s *JamCaptureService) getSelectedBackingtrackName() (string, error) {
	configPath := s.getBackingtrackConfigPath()

	data, err := os.ReadFile(configPath)
	if err != nil {
		if os.IsNotExist(err) {
			return "", nil // No config file = no selection
		}
		return "", fmt.Errorf("failed to read backing track config: %w", err)
	}

	var config BackingtrackConfig
	if err := yaml.Unmarshal(data, &config); err != nil {
		return "", fmt.Errorf("failed to parse backing track config: %w", err)
	}

	return config.SelectedBackingtrack, nil
}

func (s *JamCaptureService) saveBackingtrackConfig(config *BackingtrackConfig) error {
	configPath := s.getBackingtrackConfigPath()

	// Ensure directory exists
	if err := os.MkdirAll(filepath.Dir(configPath), 0755); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	data, err := yaml.Marshal(config)
	if err != nil {
		return fmt.Errorf("failed to marshal backing track config: %w", err)
	}

	if err := os.WriteFile(configPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write backing track config: %w", err)
	}

	return nil
}

// GetLastError returns the last error message (thread-safe)
func (s *JamCaptureService) GetLastError() string {
	s.lastErrorMutex.RLock()
	defer s.lastErrorMutex.RUnlock()
	return s.lastError
}

// setLastError sets the last error message (thread-safe)
func (s *JamCaptureService) setLastError(err string) {
	s.lastErrorMutex.Lock()
	defer s.lastErrorMutex.Unlock()
	s.lastError = err

	// Log all errors for debugging and monitoring
	slog.Error("Service error occurred", "error_message", err)
}

// clearLastError clears the last error message (thread-safe)
func (s *JamCaptureService) clearLastError() {
	s.lastErrorMutex.Lock()
	defer s.lastErrorMutex.Unlock()
	s.lastError = ""
}

// ===== MKV MIXING SERVICE METHODS =====

// ListMKVFiles returns a list of MKV files available for mixing
func (s *JamCaptureService) ListMKVFiles() ([]MKVFileInfo, error) {
	// Look for MKV files in the recordings directory where they are created
	recordingDir := s.cfg.Output.Directory

	// Create directory if it doesn't exist
	if err := os.MkdirAll(recordingDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create recordings directory: %w", err)
	}

	// Read directory
	files, err := os.ReadDir(recordingDir)
	if err != nil {
		return nil, fmt.Errorf("failed to read recordings directory: %w", err)
	}

	var mkvFiles []MKVFileInfo
	for _, file := range files {
		if file.IsDir() {
			continue
		}

		// Only include MKV files
		ext := strings.ToLower(filepath.Ext(file.Name()))
		if ext != ".mkv" {
			continue
		}

		// Get file info
		filePath := filepath.Join(recordingDir, file.Name())
		info, err := file.Info()
		if err != nil {
			slog.Warn("Failed to get file info for MKV", "file", file.Name(), "error", err)
			continue
		}

		mkvInfo := MKVFileInfo{
			Name:         file.Name(),
			Path:         filePath,
			Size:         info.Size(),
			SizeHuman:    formatBytes(info.Size()),
			ModTime:      info.ModTime(),
			ModTimeHuman: info.ModTime().Format("2006-01-02 15:04:05"),
			StreamURL:    fmt.Sprintf("/api/backingtracks/stream/%s", file.Name()),
			AnalyzeURL:   fmt.Sprintf("/api/mix/analyze/%s", file.Name()),
		}

		mkvFiles = append(mkvFiles, mkvInfo)
	}

	// Sort by modification time (newest first)
	sort.Slice(mkvFiles, func(i, j int) bool {
		return mkvFiles[i].ModTime.After(mkvFiles[j].ModTime)
	})

	return mkvFiles, nil
}

// AnalyzeMKVFile extracts track information from an MKV file using ffprobe
func (s *JamCaptureService) AnalyzeMKVFile(filename string) (*MKVAnalysis, error) {
	// Look for MKV files in the recordings directory where they are created
	recordingDir := s.cfg.Output.Directory
	filePath := filepath.Join(recordingDir, filename)

	// Validate file exists
	if _, err := os.Stat(filePath); err != nil {
		return nil, fmt.Errorf("MKV file not found: %s", filename)
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
		return nil, fmt.Errorf("ffprobe failed for %s: %w", filename, err)
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
		return nil, fmt.Errorf("failed to parse ffprobe output for %s: %w", filename, err)
	}

	// Extract audio tracks only
	var tracks []TrackInfo
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

		track := TrackInfo{
			Index:    stream.Index,
			Name:     fmt.Sprintf("track_%d", stream.Index),
			Title:    title,
			Channels: stream.Channels,
		}

		tracks = append(tracks, track)
	}

	analysis := &MKVAnalysis{
		Filename:   filename,
		TrackCount: len(tracks),
		Tracks:     tracks,
	}

	slog.Debug("MKV analysis completed", "filename", filename, "tracks", len(tracks))
	return analysis, nil
}

// MixWithTrackVolumes creates a custom mix using the specified track volumes
func (s *JamCaptureService) MixWithTrackVolumes(filename string, trackVolumes map[string]float64) error {
	// Remove .mkv extension to get the song name
	songName := strings.TrimSuffix(filename, ".mkv")

	// Create mixer with current config
	mixer := mix.New(s.cfg)

	slog.Info("Starting custom mix", "filename", filename, "song_name", songName, "volumes", trackVolumes)

	// Use the new MixWithChannelVolumes method
	if err := mixer.MixWithChannelVolumes(songName, trackVolumes); err != nil {
		s.setLastError(fmt.Sprintf("Custom mix failed for %s: %v", filename, err))
		return fmt.Errorf("custom mix failed for %s: %w", filename, err)
	}

	slog.Info("Custom mix completed successfully", "filename", filename, "song_name", songName)
	return nil
}

// formatBytes formats bytes in human readable format
func formatBytes(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}
