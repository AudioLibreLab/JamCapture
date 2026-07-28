package service

import (
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/audiolibrelab/jamcapture/internal/audio"
	"github.com/audiolibrelab/jamcapture/internal/config"
	"github.com/audiolibrelab/jamcapture/internal/mix"
	"github.com/audiolibrelab/jamcapture/internal/play"
	"github.com/audiolibrelab/jamcapture/internal/util"
	"gopkg.in/yaml.v3"
)


// GenerateDefaultSongName returns a unique song name based on the current time.
func GenerateDefaultSongName() string {
	return "MySong_" + time.Now().Format("20060102_150405")
}

// Service represents the core JamCapture service interface
type Service interface {
	// Recording operations
	// StartReady returns the song name actually used, which differs from the
	// requested one when a recording already exists under that name.
	StartReady(songName string) (string, error)
	CancelReady() error
	StopRecording() error
	GetRecordingStatus() (RecordingStatus, *RecordingSession)
	CurrentSongName() string

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
	MixWithTrackAndGlobalVolumes(filename string, trackVolumes map[string]float64, globalVolume float64) error
	GetLatestMixedFile() string
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
type MKVAnalysis = config.MKVAnalysis

// TrackInfo contains information about a single track within an MKV file
type TrackInfo = config.TrackInfo

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

	// Configuration management
	configMutex sync.RWMutex

	// Backing track management
	backingtrackMutex sync.RWMutex

	// Error tracking
	lastError      string
	lastErrorMutex sync.RWMutex

	// Current song name shared between server and systray
	songMu      sync.RWMutex
	currentSong string
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

// CurrentSongName returns the active song name, or a freshly generated default if none is set.
func (s *JamCaptureService) CurrentSongName() string {
	s.songMu.RLock()
	name := s.currentSong
	s.songMu.RUnlock()
	if name != "" {
		return name
	}
	return GenerateDefaultSongName()
}

// recordingExists reports whether a recording already exists for songName.
func (s *JamCaptureService) recordingExists(songName string) bool {
	path := filepath.Join(s.cfg.Output.Directory, util.CleanFileName(songName)+".mkv")
	_, err := os.Stat(path)
	return err == nil
}

// uniqueSongName returns songName when no recording uses it yet, otherwise the
// first free "<songName>_takeN" variant. FFmpeg records with -y, so without this
// a second READY on an already-used name would silently destroy the previous take.
func (s *JamCaptureService) uniqueSongName(songName string) string {
	if !s.recordingExists(songName) {
		return songName
	}

	for take := 2; take <= 99; take++ {
		candidate := fmt.Sprintf("%s_take%d", songName, take)
		if !s.recordingExists(candidate) {
			slog.Info("Recording already exists, renaming to preserve the previous take",
				"requested", songName, "effective", candidate)
			return candidate
		}
	}

	// 99 takes of the same name: fall back to a name unique by construction.
	candidate := fmt.Sprintf("%s_%s", songName, time.Now().Format("150405"))
	slog.Info("Recording already exists, renaming to preserve the previous take",
		"requested", songName, "effective", candidate)
	return candidate
}

// StartReady prepares for recording (STANDBY -> READY).
// It returns the song name actually used, which differs from songName when a
// recording already exists under that name.
func (s *JamCaptureService) StartReady(songName string) (string, error) {
	slog.Debug("Service.StartReady called", "song_name", songName)
	s.clearLastError() // Clear any previous errors when starting a new operation

	// Validate song name
	if errMsg := validateFileName(songName); errMsg != "" {
		err := fmt.Errorf("invalid song name: %s", errMsg)
		slog.Error("Service.StartReady validation failed", "error", err)
		s.setLastError(err.Error())
		return "", err
	}

	effectiveName := s.uniqueSongName(songName)

	if err := s.recorder.StartReady(effectiveName); err != nil {
		slog.Error("Service.StartReady failed", "error", err)
		s.setLastError(fmt.Sprintf("Failed to start recording: %v", err))
		return "", err
	}

	s.songMu.Lock()
	s.currentSong = effectiveName
	s.songMu.Unlock()
	slog.Debug("Service.StartReady completed successfully", "song_name", effectiveName)
	return effectiveName, nil
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
		s.clearLastError()
		s.songMu.Lock()
		s.currentSong = ""
		s.songMu.Unlock()
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
	if err := mixer.Mix(songName); err != nil {
		return err
	}

	// Update last mixed file with the generated output filename (FLAC/WAV)
	outputExtension := s.getOutputExtension()
	outputFilename := songName + "." + outputExtension
	if err := s.updateLastMixedFile(outputFilename); err != nil {
		slog.Error("Failed to update last mixed file", "error", err, "filename", outputFilename)
	}

	return nil
}

// MixWithOptions mixes recorded tracks with custom options
func (s *JamCaptureService) MixWithOptions(songName string, guitarVolume, backingVolume float64, delay int) error {
	mixer := mix.New(s.cfg)
	if err := mixer.MixWithOptions(songName, guitarVolume, backingVolume, delay); err != nil {
		return err
	}

	// Update last mixed file with the generated output filename (FLAC/WAV)
	outputExtension := s.getOutputExtension()
	outputFilename := songName + "." + outputExtension
	if err := s.updateLastMixedFile(outputFilename); err != nil {
		slog.Error("Failed to update last mixed file", "error", err, "filename", outputFilename)
	}

	return nil
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
			effectiveName, err := s.StartReady(songName)
			if err != nil {
				return fmt.Errorf("pipeline ready start failed: %w", err)
			}
			// Follow the rename when the requested name was already taken, so the
			// later mix/play steps operate on the file we just recorded.
			songName = effectiveName
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
	cleanName := util.CleanFileName(songName)

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

// validateSafeFilename rejects filenames that could escape their base directory
// via path traversal (e.g. "..", "/", "\\").
func validateSafeFilename(name string) error {
	if strings.Contains(name, "..") || strings.Contains(name, "/") || strings.Contains(name, "\\") {
		return fmt.Errorf("invalid filename: %s", name)
	}
	return nil
}

// validateFileName checks if a filename contains only allowed characters
// Returns an error message if invalid, empty string if valid
func validateFileName(name string) string {
	if strings.TrimSpace(name) == "" {
		return "Song name cannot be empty"
	}

	for _, r := range name {
		if !((r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == ' ' || r == '-' || r == '_') {
			return fmt.Sprintf("Song name contains invalid character '%c'. Only letters, numbers, spaces, hyphens (-) and underscores (_) are allowed.", r)
		}
	}

	if len(name) > 100 {
		return "Song name must be 100 characters or less"
	}

	return ""
}

// updateLastMixedFile updates the last mixed file in configuration
func (s *JamCaptureService) updateLastMixedFile(filename string) error {
	s.configMutex.Lock()
	defer s.configMutex.Unlock()

	// Update the configuration
	s.cfg.Output.LastMixedFile = filename
	slog.Debug("Updated last mixed file", "filename", filename)
	return nil
}

// GetLatestMixedFile scans the recordings directory and returns the basename of
// the most recently modified audio file. Always scans so it reflects recordings
// made via the web GUI or any other client since the last restart.
func (s *JamCaptureService) GetLatestMixedFile() string {
	s.configMutex.RLock()
	dir := s.cfg.Output.Directory
	s.configMutex.RUnlock()

	if strings.HasPrefix(dir, "~/") {
		if home, err := os.UserHomeDir(); err == nil {
			dir = filepath.Join(home, dir[2:])
		}
	}
	if dir == "" {
		return ""
	}
	latest, err := latestAudioFile(dir)
	if err != nil {
		return ""
	}
	return filepath.Base(latest)
}

// latestAudioFile returns the path of the most recently modified audio file
// (flac/wav/mp3 preferred over mkv) in dir, or an error if none found.
func latestAudioFile(dir string) (string, error) {
	priorityExts := []string{".flac", ".wav", ".mp3"}
	fallbackExts := []string{".mkv"}
	allExts := append(priorityExts, fallbackExts...)

	var latestFile string
	var latestTime time.Time
	var latestPriority = len(allExts)

	err := filepath.Walk(dir, func(path string, info os.FileInfo, walkErr error) error {
		if walkErr != nil || info.IsDir() {
			return nil
		}
		ext := strings.ToLower(filepath.Ext(path))
		priority := -1
		for i, e := range allExts {
			if ext == e {
				priority = i
				break
			}
		}
		if priority == -1 {
			return nil
		}
		if priority < latestPriority || (priority == latestPriority && info.ModTime().After(latestTime)) || latestFile == "" {
			latestTime = info.ModTime()
			latestFile = path
			latestPriority = priority
		}
		return nil
	})
	if err != nil || latestFile == "" {
		return "", fmt.Errorf("no audio files found in %s", dir)
	}
	return latestFile, nil
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
	return s.cfg.BackingtracksDir()
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
			SizeHuman:    util.FormatBytes(info.Size()),
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
	if err := validateSafeFilename(name); err != nil {
		return err
	}

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
	if err := validateSafeFilename(recordingName); err != nil {
		return err
	}

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
			SizeHuman:    util.FormatBytes(info.Size()),
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
	if err := validateSafeFilename(filename); err != nil {
		return nil, err
	}

	// Look for MKV files in the recordings directory where they are created
	recordingDir := s.cfg.Output.Directory
	filePath := filepath.Join(recordingDir, filename)

	return mix.AnalyzeMKVFile(filePath)
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

	// Update last mixed file with the generated output filename (FLAC/WAV)
	outputExtension := s.getOutputExtension()
	outputFilename := songName + "." + outputExtension
	if err := s.updateLastMixedFile(outputFilename); err != nil {
		slog.Error("Failed to update last mixed file", "error", err, "filename", outputFilename)
	}

	return nil
}

// MixWithTrackAndGlobalVolumes creates a custom mix using the specified track volumes and global volume
func (s *JamCaptureService) MixWithTrackAndGlobalVolumes(filename string, trackVolumes map[string]float64, globalVolume float64) error {
	// Remove .mkv extension to get the song name
	songName := strings.TrimSuffix(filename, ".mkv")

	// Create mixer with current config
	mixer := mix.New(s.cfg)

	slog.Info("Starting custom mix with global volume", "filename", filename, "song_name", songName, "volumes", trackVolumes, "global_volume", globalVolume)

	// Use the new MixWithChannelAndGlobalVolumes method
	if err := mixer.MixWithChannelAndGlobalVolumes(songName, trackVolumes, globalVolume); err != nil {
		s.setLastError(fmt.Sprintf("Custom mix with global volume failed for %s: %v", filename, err))
		return fmt.Errorf("custom mix with global volume failed for %s: %w", filename, err)
	}

	slog.Info("Custom mix with global volume completed successfully", "filename", filename, "song_name", songName)

	// Update last mixed file with the generated output filename (FLAC/WAV)
	outputExtension := s.getOutputExtension()
	outputFilename := songName + "." + outputExtension
	if err := s.updateLastMixedFile(outputFilename); err != nil {
		slog.Error("Failed to update last mixed file", "error", err, "filename", outputFilename)
	}

	return nil
}

