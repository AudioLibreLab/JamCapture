package server

import (
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"mime"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/audiolibrelab/jamcapture/internal/config"
	"github.com/audiolibrelab/jamcapture/internal/service"
	"github.com/spf13/viper"
)

// Server represents the web server for controlling JamCapture
type Server struct {
	service       service.Service
	cfg           *config.Config
	configFile    string
	port          string
	lastSongName  string
	activeProfile string

	// Profile locking mechanism
	profileLock   sync.RWMutex
	lockedProfile string
	lockTimestamp time.Time

	// Last local file storage
	lastLocalFile string
	fileLock      sync.RWMutex

}

// StatusResponse represents the JSON response for status endpoint
type StatusResponse struct {
	Status        string                    `json:"status"`
	Message       string                    `json:"message,omitempty"`
	Session       *service.RecordingSession `json:"session,omitempty"`
	Config        *ResolvedConfigInfo       `json:"resolved_config"`
	ActiveProfile string                    `json:"active_profile"`
}

// ResolvedConfigInfo contains configuration information for the UI
type ResolvedConfigInfo struct {
	ActiveProfile string        `json:"active_profile"`
	OutputDir     string        `json:"output_dir"`
	Channels      []ChannelInfo `json:"channels"`
	SampleRate    int           `json:"sample_rate"`
	Format        string        `json:"format"`
	AutoMix       bool          `json:"auto_mix"`
	Mix           MixInfo       `json:"mix"`
}

// SourceInfo contains information about an audio source
type SourceInfo struct {
	Name        string `json:"name"`
	Source      string `json:"source"`
	Type        string `json:"type"`
	Status      string `json:"status"` // "available", "unavailable", "unknown"
	LastChecked string `json:"last_checked"`
}

// SourcesResponse represents the JSON response for sources endpoint
type SourcesResponse struct {
	Sources []SourceInfo `json:"sources"`
}

// FileInfo contains information about an audio file
type FileInfo struct {
	Name         string    `json:"name"`
	Path         string    `json:"path"`
	Size         int64     `json:"size"`
	SizeHuman    string    `json:"size_human"`
	ModTime      time.Time `json:"mod_time"`
	ModTimeHuman string    `json:"mod_time_human"`
	Extension    string    `json:"extension"`
	StreamURL    string    `json:"stream_url"`
	DownloadURL  string    `json:"download_url"`
}

// FilesResponse represents the JSON response for files endpoint
type FilesResponse struct {
	Files               []FileInfo `json:"files"`
	TotalCount          int        `json:"total_count"`
	OutputDirectory     string     `json:"output_directory"`
	SupportedExtensions []string   `json:"supported_extensions"`
}

// JackPortInfo contains information about a JACK port
type JackPortInfo struct {
	Name      string `json:"name"`
	Type      string `json:"type"`      // "input", "output", "monitor"
	Device    string `json:"device"`    // device part before ":"
	Port      string `json:"port"`      // port part after ":"
	Available bool   `json:"available"` // whether port is currently available
}

// JackPortsResponse represents the JSON response for JACK ports endpoint
type JackPortsResponse struct {
	Ports []JackPortInfo `json:"ports"`
}

// BackingtracksResponse represents the JSON response for backing tracks endpoint
type BackingtracksResponse struct {
	Backingtracks           []service.BackingtrackInfo `json:"backingtracks"`
	TotalCount              int                         `json:"total_count"`
	BackingtracksDirectory  string                      `json:"backingtracks_directory"`
	SelectedBackingtrack    string                      `json:"selected_backingtrack,omitempty"`
	SupportedExtensions     []string                    `json:"supported_extensions"`
}

// BackingtrackConvertRequest represents a request to convert a recording to backing track
type BackingtrackConvertRequest struct {
	RecordingName string `json:"recording_name"`
}

// BackingtrackSelectRequest represents a request to select a backing track
type BackingtrackSelectRequest struct {
	Name string `json:"name"`
}

// ConfigCreateRequest represents a request to create a new configuration
type ConfigCreateRequest struct {
	Name        string         `json:"name"`
	BaseProfile string         `json:"base_profile"` // profile to clone from
	Config      *config.Config `json:"config"`       // complete config data
}

// ConfigUpdateRequest represents a request to update a configuration
type ConfigUpdateRequest struct {
	Config *config.Config `json:"config"`
}

// GenericResponse represents a generic API response
type GenericResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
	Error   string `json:"error,omitempty"`
}

// MixInfo contains mixing configuration for the UI
type MixInfo struct {
	Volumes map[string]float64 `json:"volumes"`
	DelayMs int                `json:"delay_ms"`
}

// ChannelInfo represents channel information for the UI
type ChannelInfo struct {
	Name        string   `json:"name"`
	Sources     []string `json:"sources"`
	AudioMode   string   `json:"audioMode"`
	Type        string   `json:"type"`
	Volume      float64  `json:"volume"`
	Delay       int      `json:"delay"`
	Inheritance string   `json:"inheritance"` // "inherited" or "profile-specific"
}

// New creates a new web server instance
func New(configFile string, port string) (*Server, error) {
	// Load configuration with active profile from config file
	cfg, err := config.LoadWithProfile(configFile, "")
	if err != nil {
		return nil, fmt.Errorf("failed to load config: %w", err)
	}

	// Determine the actual active profile name
	activeProfileName := getActiveProfileName(configFile)

	// Create service
	svc := service.New(cfg, configFile, nil)

	return &Server{
		service:       svc,
		cfg:           cfg,
		configFile:    configFile,
		port:          port,
		activeProfile: activeProfileName,
	}, nil
}

// Start starts the web server
func (s *Server) Start() error {
	http.HandleFunc("/", s.handleIndex)
	http.HandleFunc("/config", s.handleConfigPage)
	http.HandleFunc("/ready", s.handleStartReady)
	http.HandleFunc("/cancel", s.handleCancelReady)
	http.HandleFunc("/stop", s.handleStopRecording)
	http.HandleFunc("/status", s.handleStatus)
	http.HandleFunc("/config/profiles", s.handleProfiles)
	http.HandleFunc("/config/select", s.handleSelectProfile)
	http.HandleFunc("/config/active", s.handleActiveProfile)
	http.HandleFunc("/config/lock", s.handleLockProfile)
	http.HandleFunc("/config/unlock", s.handleUnlockProfile)
	http.HandleFunc("/config/details/", s.handleProfileDetails)
	http.HandleFunc("/sources", s.handleSources)
	http.HandleFunc("/api/files", s.handleFiles)
	http.HandleFunc("/api/files/stream/", s.handleFileStream)
	http.HandleFunc("/api/files/download/", s.handleFileDownload)
	http.HandleFunc("/api/config/create", s.handleCreateConfig)
	http.HandleFunc("/api/config/update/", s.handleUpdateConfig)
	http.HandleFunc("/api/config/delete/", s.handleDeleteConfig)
	http.HandleFunc("/api/config/clone/", s.handleCloneConfig)
	// Audio player endpoints
	http.HandleFunc("/api/latest-recording", s.handleLatestRecording)
	http.HandleFunc("/api/recording/", s.handleRecordingStream)
	http.HandleFunc("/api/set-local-file", s.handleSetLocalFile)
	http.HandleFunc("/api/get-local-file", s.handleGetLocalFile)
	http.HandleFunc("/api/upload-local-file", s.handleUploadLocalFile)
	http.HandleFunc("/api/backingtrack/", s.handleBackingtrackStream)
	// Backing tracks API
	http.HandleFunc("/api/backingtracks", s.handleBackingtracks)
	http.HandleFunc("/api/backingtracks/selected", s.handleSelectedBackingtrack)
	http.HandleFunc("/api/backingtracks/select", s.handleSelectBackingtrack)
	http.HandleFunc("/api/backingtracks/convert", s.handleConvertToBackingtrack)
	http.HandleFunc("/api/backingtracks/stream/", s.handleBackingtrackStreamNew)
	http.HandleFunc("/api/backingtracks/download/", s.handleBackingtrackDownload)

	// Get local IP address
	localIP := getLocalIP()

	slog.Info("Starting JamCapture Web Server",
		"port", s.port,
		"local_url", fmt.Sprintf("http://%s:%s", localIP, s.port),
		"localhost_url", fmt.Sprintf("http://localhost:%s", s.port))

	return http.ListenAndServe(":"+s.port, nil)
}

// handleIndex serves the main web UI
func (s *Server) handleIndex(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Try to read the HTML file directly
	htmlPath := "web/static/index.html"
	htmlContent, err := os.ReadFile(htmlPath)
	if err != nil {
		// Fallback to inline HTML if file not found
		htmlContent = []byte(getDefaultHTML())
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache")
	w.Write(htmlContent)
}

// getDefaultHTML provides a fallback HTML interface
func getDefaultHTML() string {
	return `<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>JamCapture</title>
    <link rel="stylesheet" href="https://cdn.jsdelivr.net/npm/@picocss/pico@2/css/pico.min.css">
    <script src="https://unpkg.com/htmx.org@1.9.10"></script>
</head>
<body>
    <div class="container">
        <h1>🎸 JamCapture</h1>
        <p>Web UI loaded successfully! The HTML file could not be read from disk, but the server is working.</p>
        <h2>API Endpoints:</h2>
        <ul>
            <li>POST /record - Start recording</li>
            <li>POST /stop - Stop recording</li>
            <li>GET /status - Get status</li>
            <li>GET /config/profiles - List profiles</li>
        </ul>
    </div>
</body>
</html>`
}

// handleStartReady transitions to READY state (STANDBY -> READY)
func (s *Server) handleStartReady(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusMethodNotAllowed)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"error":   "Method not allowed",
		})
		return
	}

	// Parse form data
	if err := r.ParseForm(); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"error":   "Failed to parse form",
		})
		return
	}

	songName := r.FormValue("song")
	profile := r.FormValue("profile")
	autoMixValue := r.FormValue("auto_mix")

	slog.Debug("Ready request received", "song", songName, "profile", profile, "auto_mix", autoMixValue)

	if songName == "" {
		s.sendErrorResponse(w, http.StatusBadRequest, "Song name is required", "operation", "start_ready")
		return
	}

	// Auto mix will be read from configuration, but we can still accept override from web UI
	// For backward compatibility, we could allow temporary override here
	// But for now, we'll use the configuration value
	s.lastSongName = songName

	// Reload config if profile is specified and different
	if profile != "" {
		newCfg, err := config.LoadWithProfile(s.configFile, profile)
		if err != nil {
			s.sendErrorResponse(w, http.StatusBadRequest,
				fmt.Sprintf("Failed to load profile '%s': %v", profile, err),
				"profile", profile, "operation", "profile_load_for_ready")
			return
		}
		s.cfg = newCfg
		s.activeProfile = profile
		// Create new service with updated config
		s.service = service.New(s.cfg, s.configFile, nil)
	}

	// Transition to READY state
	slog.Info("Server: Starting READY state", "song_name", songName, "profile", profile)
	if err := s.service.StartReady(songName); err != nil {
		slog.Error("Server: StartReady failed", "error", err, "song_name", songName)
		s.sendErrorResponse(w, http.StatusInternalServerError,
			fmt.Sprintf("Failed to start ready: %v", err),
			"song_name", songName, "profile", profile, "operation", "start_ready")
		return
	}
	slog.Info("Server: StartReady completed successfully", "song_name", songName)

	// Return success
	w.Header().Set("Content-Type", "application/json")
	response := map[string]interface{}{
		"success": true,
		"message": "Ready state activated",
		"song":    songName,
		"profile": profile,
	}
	json.NewEncoder(w).Encode(response)
}

// handleCancelReady cancels READY state (READY -> STANDBY)
func (s *Server) handleCancelReady(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusMethodNotAllowed)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"error":   "Method not allowed",
		})
		return
	}

	// Cancel ready state
	if err := s.service.CancelReady(); err != nil {
		s.sendErrorResponse(w, http.StatusInternalServerError,
			fmt.Sprintf("Failed to cancel ready: %v", err),
			"operation", "cancel_ready")
		return
	}

	// Return success
	w.Header().Set("Content-Type", "application/json")
	response := map[string]interface{}{
		"success": true,
		"message": "Ready state cancelled",
	}
	json.NewEncoder(w).Encode(response)
}

// handleStopRecording stops the current recording session
func (s *Server) handleStopRecording(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusMethodNotAllowed)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"error":   "Method not allowed",
		})
		return
	}

	// Stop recording
	if err := s.service.StopRecording(); err != nil {
		s.sendErrorResponse(w, http.StatusInternalServerError,
			fmt.Sprintf("Failed to stop recording: %v", err),
			"operation", "stop_recording")
		return
	}

	message := "Recording stopped"
	var mixError string

	// Auto-mix if enabled in configuration
	if s.cfg.AutoMix && s.lastSongName != "" {
		slog.Info("Starting automatic mixing", "song", s.lastSongName)
		if err := s.service.Mix(s.lastSongName); err != nil {
			mixError = fmt.Sprintf("Mixing failed: %v", err)
			slog.Error("Mixing failed", "error", err)
		} else {
			message = "Recording stopped and mixed successfully"
			slog.Info("Mixing completed successfully")
		}
	}

	// Return success
	w.Header().Set("Content-Type", "application/json")
	response := map[string]interface{}{
		"success": true,
		"message": message,
	}

	if mixError != "" {
		response["mix_error"] = mixError
	}

	json.NewEncoder(w).Encode(response)
}

// handleStatus returns the current status and session info
func (s *Server) handleStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusMethodNotAllowed)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"error":   "Method not allowed",
		})
		return
	}

	// Get current status
	status, session := s.service.GetRecordingStatus()

	// Auto-unlock profile if status returned to STANDBY but profile is still locked
	// This handles cases where the recorder automatically transitions to STANDBY
	// (e.g., duplicate sources detected) without going through the normal unlock process
	if status == service.StatusStandby {
		if isLocked, lockedProfile := s.isProfileLocked(); isLocked {
			slog.Info("Auto-unlocking profile after automatic return to STANDBY", "profile", lockedProfile, "reason", "status_became_standby")
			s.profileLock.Lock()
			s.lockedProfile = ""
			s.lockTimestamp = time.Time{}
			s.profileLock.Unlock()
			slog.Debug("Profile auto-unlocked", "profile", lockedProfile)
		}
	}

	// Get resolved config info
	resolvedConfig := s.getResolvedConfigInfo()

	// Generate status message
	message := s.generateStatusMessage(status, session)

	// Prepare response
	response := StatusResponse{
		Status:        string(status),
		Message:       message,
		Session:       session,
		Config:        resolvedConfig,
		ActiveProfile: s.activeProfile,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// handleProfiles returns available configuration profiles
func (s *Server) handleProfiles(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusMethodNotAllowed)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"error":   "Method not allowed",
		})
		return
	}

	// Get profiles from config file (simplified implementation)
	profiles := s.getAvailableProfiles()

	w.Header().Set("Content-Type", "application/json")
	response := map[string]interface{}{
		"profiles": profiles,
	}
	json.NewEncoder(w).Encode(response)
}

// getResolvedConfigInfo builds configuration information for the UI
func (s *Server) getResolvedConfigInfo() *ResolvedConfigInfo {
	// Build channel info with inheritance information
	channels := make([]ChannelInfo, len(s.cfg.Channels))
	for i, ch := range s.cfg.Channels {
		inheritance := "profile-specific"
		if s.cfg.Inheritance != nil {
			if chInheritance, exists := s.cfg.Inheritance.Channels[ch.Name]; exists {
				if chInheritance.Source == "inherited" {
					inheritance = "inherited"
				}
			}
		}

		channels[i] = ChannelInfo{
			Name:        ch.Name,
			Sources:     ch.Sources,
			AudioMode:   ch.AudioMode,
			Type:        ch.Type,
			Volume:      ch.Volume,
			Delay:       ch.Delay,
			Inheritance: inheritance,
		}
	}

	return &ResolvedConfigInfo{
		ActiveProfile: "current", // Could be enhanced to track actual active profile
		OutputDir:     s.cfg.Output.Directory,
		Channels:      channels,
		SampleRate:    s.cfg.Audio.SampleRate,
		Format:        s.cfg.Output.Format,
		AutoMix:       s.cfg.AutoMix,
		Mix:           s.buildMixInfo(),
	}
}

// buildMixInfo builds MixInfo from current server config for API compatibility
func (s *Server) buildMixInfo() MixInfo {
	return s.buildMixInfoFromConfig(s.cfg)
}

// buildMixInfoFromConfig builds MixInfo from any config for API compatibility
func (s *Server) buildMixInfoFromConfig(cfg *config.Config) MixInfo {
	volumes := make(map[string]float64)

	// Convert unified channel config to legacy volumes map for API
	for _, channel := range cfg.Channels {
		if channel.Volume > 0 {
			volumes[channel.Name] = channel.Volume
		}
	}

	// Get the first delay value as DelayMs (for backward compatibility)
	delayMs := 0
	if len(cfg.Channels) > 0 {
		for _, channel := range cfg.Channels {
			if channel.Delay > 0 {
				delayMs = channel.Delay
				break
			}
		}
	}

	return MixInfo{
		Volumes: volumes,
		DelayMs: delayMs,
	}
}

// getAvailableProfiles returns a list of available configuration profiles
func (s *Server) getAvailableProfiles() []string {
	profiles := []string{}

	// Read actual profiles from config file
	if s.configFile != "" {
		if _, err := os.Stat(s.configFile); err == nil {
			// Create a new viper instance to avoid interfering with global config
			v := viper.New()
			v.SetConfigFile(s.configFile)

			if err := v.ReadInConfig(); err == nil {
				// Parse the root config to get available profiles
				var rootConfig config.RootConfig
				if err := v.Unmarshal(&rootConfig); err == nil {
					// Add all available profile names from the configs section
					for profileName := range rootConfig.Configs {
						profiles = append(profiles, profileName)
					}
				} else {
					slog.Debug("Failed to unmarshal config for profiles", "error", err)
				}
			} else {
				slog.Debug("Failed to read config file for profiles", "error", err)
			}
		}
	}

	slog.Debug("Available profiles loaded", "profiles", profiles, "config_file", s.configFile)
	return profiles
}

// handleSources returns the status of all configured audio sources
func (s *Server) handleSources(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusMethodNotAllowed)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"error":   "Method not allowed",
		})
		return
	}

	sources := make([]SourceInfo, 0)

	// Get channel status from service (uses recorder's shared cache)
	channelStatus := s.service.GetChannelStatus()

	// Build sources response from configured channels
	if s.cfg != nil && s.cfg.Channels != nil {
		for _, ch := range s.cfg.Channels {
			status, exists := channelStatus[ch.Name]
			if !exists {
				status = "unknown"
			}

			slog.Debug("Channel status", "channel", ch.Name, "sources", ch.Sources, "status", status)

			// Create a source info entry for this channel
			sourceDesc := strings.Join(ch.Sources, ", ")
			sources = append(sources, SourceInfo{
				Name:        ch.Name,
				Source:      sourceDesc,
				Type:        ch.Type,
				Status:      status,
				LastChecked: time.Now().Format(time.RFC3339),
			})
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(SourcesResponse{Sources: sources})
}

// getLocalIP returns the local IP address for network access
// handleConfigPage serves the configuration page
func (s *Server) handleConfigPage(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Try to read the config HTML file
	htmlPath := "web/static/config.html"
	htmlContent, err := os.ReadFile(htmlPath)
	if err != nil {
		// Fallback to basic HTML if file not found
		htmlContent = []byte(getDefaultConfigHTML())
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache")
	w.Write(htmlContent)
}

// getDefaultConfigHTML provides a fallback configuration interface
func getDefaultConfigHTML() string {
	return `<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>JamCapture - Configuration</title>
    <link rel="stylesheet" href="https://cdn.jsdelivr.net/npm/@picocss/pico@2/css/pico.min.css">
</head>
<body>
    <div class="container">
        <h1>🎸 JamCapture - Configuration</h1>
        <p>Config page loaded! The HTML file could not be read from disk, but the server is working.</p>
        <a href="/">Back to main page</a>
    </div>
</body>
</html>`
}

// handleSelectProfile handles profile selection
func (s *Server) handleSelectProfile(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusMethodNotAllowed)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"error":   "Method not allowed",
		})
		return
	}

	// Parse form data
	if err := r.ParseForm(); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"error":   "Failed to parse form",
		})
		return
	}

	profile := r.FormValue("profile")
	slog.Debug("Profile selection request", "profile", profile)

	// Check if current profile is locked
	if isLocked, lockedProfile := s.isProfileLocked(); isLocked {
		s.sendErrorResponse(w, http.StatusConflict,
			fmt.Sprintf("Profile '%s' is locked by another recording session", lockedProfile),
			"profile", lockedProfile, "operation", "profile_selection")
		return
	}

	// Load new configuration
	newCfg, err := config.LoadWithProfile(s.configFile, profile)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"error":   fmt.Sprintf("Failed to load profile '%s': %v", profile, err),
		})
		return
	}

	// Update server configuration
	s.cfg = newCfg
	s.activeProfile = profile

	// Update the active_config in the config file
	if err := config.UpdateActiveConfig(s.configFile, s.activeProfile); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"error":   fmt.Sprintf("Failed to save profile selection to config file: %v", err),
		})
		return
	}

	// Create new service with updated config
	s.service = service.New(s.cfg, s.configFile, nil)

	slog.Info("Profile changed", "profile", s.activeProfile)

	// Return success
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"message": fmt.Sprintf("Profile changed to %s", s.activeProfile),
		"profile": s.activeProfile,
	})
}

// handleActiveProfile returns the currently active profile
func (s *Server) handleActiveProfile(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusMethodNotAllowed)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"error":   "Method not allowed",
		})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"active_profile": s.activeProfile,
		"success":        true,
	})
}

// handleLockProfile locks the current active profile
func (s *Server) handleLockProfile(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusMethodNotAllowed)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"error":   "Method not allowed",
		})
		return
	}

	s.profileLock.Lock()
	defer s.profileLock.Unlock()

	// Check if profile is already locked
	if s.lockedProfile != "" {
		s.sendErrorResponse(w, http.StatusConflict,
			fmt.Sprintf("Profile '%s' is locked by another recording session", s.lockedProfile),
			"current_locked_profile", s.lockedProfile, "operation", "lock_profile")
		return
	}

	// Lock the current active profile
	s.lockedProfile = s.activeProfile
	s.lockTimestamp = time.Now()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success":        true,
		"locked_profile": s.lockedProfile,
		"message":        fmt.Sprintf("Profile '%s' locked successfully", s.lockedProfile),
	})

	slog.Debug("Profile locked", "profile", s.lockedProfile)
}

// handleUnlockProfile unlocks the current profile
func (s *Server) handleUnlockProfile(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusMethodNotAllowed)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"error":   "Method not allowed",
		})
		return
	}

	s.profileLock.Lock()
	defer s.profileLock.Unlock()

	if s.lockedProfile == "" {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": true,
			"message": "No profile was locked",
		})
		return
	}

	unlockedProfile := s.lockedProfile
	s.lockedProfile = ""
	s.lockTimestamp = time.Time{}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"message": fmt.Sprintf("Profile '%s' unlocked successfully", unlockedProfile),
	})

	slog.Debug("Profile unlocked", "profile", unlockedProfile)
}

// isProfileLocked checks if the current profile is locked (thread-safe)
func (s *Server) isProfileLocked() (bool, string) {
	s.profileLock.RLock()
	defer s.profileLock.RUnlock()
	return s.lockedProfile != "", s.lockedProfile
}

// handleProfileDetails returns details about a specific profile
func (s *Server) handleProfileDetails(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusMethodNotAllowed)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"error":   "Method not allowed",
		})
		return
	}

	// Extract profile name from URL path
	profileName := r.URL.Path[len("/config/details/"):]
	if profileName == "" {
		// Return 400 Bad Request if no profile name specified
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"error":   "Profile name is required",
		})
		return
	}

	slog.Debug("Profile details request", "profile", profileName)

	// Load configuration for the specified profile
	cfg, err := config.LoadWithProfile(s.configFile, profileName)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"error":   fmt.Sprintf("Failed to load profile '%s': %v", profileName, err),
		})
		return
	}

	// Build channel info
	channels := make([]ChannelInfo, len(cfg.Channels))
	for i, ch := range cfg.Channels {
		inheritance := "profile-specific"
		if cfg.Inheritance != nil {
			if chInheritance, exists := cfg.Inheritance.Channels[ch.Name]; exists {
				if chInheritance.Source == "inherited" {
					inheritance = "inherited"
				}
			}
		}

		channels[i] = ChannelInfo{
			Name:        ch.Name,
			Sources:     ch.Sources,
			AudioMode:   ch.AudioMode,
			Type:        ch.Type,
			Volume:      ch.Volume,
			Delay:       ch.Delay,
			Inheritance: inheritance,
		}
	}

	profileInfo := &ResolvedConfigInfo{
		OutputDir:  cfg.Output.Directory,
		Channels:   channels,
		SampleRate: cfg.Audio.SampleRate,
		Format:     cfg.Output.Format,
		AutoMix:    cfg.AutoMix,
		Mix:        s.buildMixInfoFromConfig(cfg),
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"profile": profileName,
		"config":  profileInfo,
	})
}


// handleFiles returns the list of audio files
func (s *Server) handleFiles(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusMethodNotAllowed)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"error":   "Method not allowed",
		})
		return
	}

	// Get output directory from current config
	outputDir := s.cfg.Output.Directory
	if outputDir == "" {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"error":   "No output directory configured",
		})
		return
	}

	// Get supported extensions
	supportedExts := config.GetSupportedAudioExtensions(s.configFile)

	// Create extensions map for quick lookup
	extMap := make(map[string]bool)
	for _, ext := range supportedExts {
		extMap[strings.ToLower(ext)] = true
	}

	// Read directory
	files, err := os.ReadDir(outputDir)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"error":   fmt.Sprintf("Failed to read output directory: %v", err),
		})
		return
	}

	var audioFiles []FileInfo
	for _, file := range files {
		if file.IsDir() {
			continue
		}

		// Check if file has supported extension
		ext := strings.TrimPrefix(strings.ToLower(filepath.Ext(file.Name())), ".")
		if !extMap[ext] {
			continue
		}

		// Get file info
		filePath := filepath.Join(outputDir, file.Name())
		info, err := file.Info()
		if err != nil {
			slog.Warn("Failed to get file info", "file", file.Name(), "error", err)
			continue
		}

		// Format size in human readable format
		sizeHuman := formatBytes(info.Size())

		// Format modification time
		modTimeHuman := info.ModTime().Format("2006-01-02 15:04:05")

		fileInfo := FileInfo{
			Name:         file.Name(),
			Path:         filePath,
			Size:         info.Size(),
			SizeHuman:    sizeHuman,
			ModTime:      info.ModTime(),
			ModTimeHuman: modTimeHuman,
			Extension:    ext,
			StreamURL:    fmt.Sprintf("/api/files/stream/%s", file.Name()),
			DownloadURL:  fmt.Sprintf("/api/files/download/%s", file.Name()),
		}

		audioFiles = append(audioFiles, fileInfo)
	}

	// Sort files by modification time (newest first)
	sort.Slice(audioFiles, func(i, j int) bool {
		return audioFiles[i].ModTime.After(audioFiles[j].ModTime)
	})

	response := FilesResponse{
		Files:               audioFiles,
		TotalCount:          len(audioFiles),
		OutputDirectory:     outputDir,
		SupportedExtensions: supportedExts,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// handleFileStream streams an audio file
func (s *Server) handleFileStream(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Extract filename from URL
	filename := r.URL.Path[len("/api/files/stream/"):]
	if filename == "" {
		http.Error(w, "Filename required", http.StatusBadRequest)
		return
	}

	// Validate filename (prevent path traversal)
	if strings.Contains(filename, "..") || strings.Contains(filename, "/") || strings.Contains(filename, "\\") {
		http.Error(w, "Invalid filename", http.StatusBadRequest)
		return
	}

	// Construct file path
	filePath := filepath.Join(s.cfg.Output.Directory, filename)

	// Check if file exists
	info, err := os.Stat(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			http.Error(w, "File not found", http.StatusNotFound)
		} else {
			http.Error(w, "Error accessing file", http.StatusInternalServerError)
		}
		return
	}

	// Verify file extension is supported
	ext := strings.TrimPrefix(strings.ToLower(filepath.Ext(filename)), ".")
	supportedExts := config.GetSupportedAudioExtensions(s.configFile)
	supported := false
	for _, supportedExt := range supportedExts {
		if ext == strings.ToLower(supportedExt) {
			supported = true
			break
		}
	}

	if !supported {
		http.Error(w, "File type not supported", http.StatusForbidden)
		return
	}

	// Set headers for audio streaming
	contentType := mime.TypeByExtension("." + ext)
	if contentType == "" {
		contentType = "audio/mpeg" // Default fallback
	}

	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Accept-Ranges", "bytes")
	w.Header().Set("Content-Length", fmt.Sprintf("%d", info.Size()))

	// Open and serve the file
	file, err := os.Open(filePath)
	if err != nil {
		http.Error(w, "Error opening file", http.StatusInternalServerError)
		return
	}
	defer file.Close()

	// Stream the file
	http.ServeContent(w, r, filename, info.ModTime(), file)
}

// handleFileDownload serves a file for download
func (s *Server) handleFileDownload(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Extract filename from URL
	filename := r.URL.Path[len("/api/files/download/"):]
	if filename == "" {
		http.Error(w, "Filename required", http.StatusBadRequest)
		return
	}

	// Validate filename (prevent path traversal)
	if strings.Contains(filename, "..") || strings.Contains(filename, "/") || strings.Contains(filename, "\\") {
		http.Error(w, "Invalid filename", http.StatusBadRequest)
		return
	}

	// Construct file path
	filePath := filepath.Join(s.cfg.Output.Directory, filename)

	// Check if file exists
	info, err := os.Stat(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			http.Error(w, "File not found", http.StatusNotFound)
		} else {
			http.Error(w, "Error accessing file", http.StatusInternalServerError)
		}
		return
	}

	// Verify file extension is supported
	ext := strings.TrimPrefix(strings.ToLower(filepath.Ext(filename)), ".")
	supportedExts := config.GetSupportedAudioExtensions(s.configFile)
	supported := false
	for _, supportedExt := range supportedExts {
		if ext == strings.ToLower(supportedExt) {
			supported = true
			break
		}
	}

	if !supported {
		http.Error(w, "File type not supported", http.StatusForbidden)
		return
	}

	// Set headers for download
	contentType := mime.TypeByExtension("." + ext)
	if contentType == "" {
		contentType = "application/octet-stream"
	}

	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", filename))
	w.Header().Set("Content-Length", fmt.Sprintf("%d", info.Size()))

	// Open and serve the file
	file, err := os.Open(filePath)
	if err != nil {
		http.Error(w, "Error opening file", http.StatusInternalServerError)
		return
	}
	defer file.Close()

	// Copy file to response
	_, err = io.Copy(w, file)
	if err != nil {
		slog.Error("Error serving file download", "file", filename, "error", err)
	}
}

// getActiveProfileName returns the active profile name from config file
func getActiveProfileName(configFile string) string {
	if configFile == "" {
		return ""
	}

	viper.SetConfigFile(configFile)
	if err := viper.ReadInConfig(); err != nil {
		slog.Warn("Failed to read config file for active profile", "error", err)
		return ""
	}

	var rootConfig config.RootConfig
	if err := viper.Unmarshal(&rootConfig); err != nil {
		slog.Warn("Failed to unmarshal config for active profile", "error", err)
		return ""
	}

	if rootConfig.ActiveConfig == "" {
		// If no active config is set, try to return the first available config
		for configName := range rootConfig.Configs {
			return configName
		}
		return ""
	}

	return rootConfig.ActiveConfig
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

// handleCreateConfig creates a new configuration profile
func (s *Server) handleCreateConfig(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusMethodNotAllowed)
		json.NewEncoder(w).Encode(GenericResponse{
			Success: false,
			Error:   "Method not allowed",
		})
		return
	}

	var req ConfigCreateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(GenericResponse{
			Success: false,
			Error:   "Invalid JSON payload",
		})
		return
	}

	// Validate request
	if req.Name == "" {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(GenericResponse{
			Success: false,
			Error:   "Configuration name is required",
		})
		return
	}

	// TODO: Implement YAML manipulation
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusNotImplemented)
	json.NewEncoder(w).Encode(GenericResponse{
		Success: false,
		Error:   "Configuration creation not yet implemented",
	})
}

// handleUpdateConfig updates an existing configuration profile
func (s *Server) handleUpdateConfig(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPut {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusMethodNotAllowed)
		json.NewEncoder(w).Encode(GenericResponse{
			Success: false,
			Error:   "Method not allowed",
		})
		return
	}

	// Extract profile name from URL
	profileName := r.URL.Path[len("/api/config/update/"):]
	if profileName == "" {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(GenericResponse{
			Success: false,
			Error:   "Profile name is required",
		})
		return
	}

	var req ConfigUpdateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(GenericResponse{
			Success: false,
			Error:   "Invalid JSON payload",
		})
		return
	}

	// TODO: Implement YAML manipulation
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusNotImplemented)
	json.NewEncoder(w).Encode(GenericResponse{
		Success: false,
		Error:   "Configuration update not yet implemented",
	})
}

// handleDeleteConfig deletes a configuration profile
func (s *Server) handleDeleteConfig(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusMethodNotAllowed)
		json.NewEncoder(w).Encode(GenericResponse{
			Success: false,
			Error:   "Method not allowed",
		})
		return
	}

	// Extract profile name from URL
	profileName := r.URL.Path[len("/api/config/delete/"):]
	if profileName == "" {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(GenericResponse{
			Success: false,
			Error:   "Profile name is required",
		})
		return
	}

	// Prevent deletion of active profile
	if profileName == s.activeProfile {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(GenericResponse{
			Success: false,
			Error:   "Cannot delete the currently active profile",
		})
		return
	}

	// TODO: Implement YAML manipulation
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusNotImplemented)
	json.NewEncoder(w).Encode(GenericResponse{
		Success: false,
		Error:   "Configuration deletion not yet implemented",
	})
}

// handleCloneConfig clones an existing configuration profile
func (s *Server) handleCloneConfig(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusMethodNotAllowed)
		json.NewEncoder(w).Encode(GenericResponse{
			Success: false,
			Error:   "Method not allowed",
		})
		return
	}

	// Extract profile name from URL
	sourceProfile := r.URL.Path[len("/api/config/clone/"):]
	if sourceProfile == "" {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(GenericResponse{
			Success: false,
			Error:   "Source profile name is required",
		})
		return
	}

	var req struct {
		NewName string `json:"new_name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(GenericResponse{
			Success: false,
			Error:   "Invalid JSON payload",
		})
		return
	}

	if req.NewName == "" {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(GenericResponse{
			Success: false,
			Error:   "New profile name is required",
		})
		return
	}

	// TODO: Implement YAML manipulation
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusNotImplemented)
	json.NewEncoder(w).Encode(GenericResponse{
		Success: false,
		Error:   "Configuration cloning not yet implemented",
	})
}

// RecordingInfo represents information about a recording file
type RecordingInfo struct {
	Success   bool    `json:"success"`
	FileName  string  `json:"file_name"`
	FilePath  string  `json:"file_path"`
	FileSize  int64   `json:"file_size"`
	Duration  float64 `json:"duration,omitempty"`
	CreatedAt string  `json:"created_at"`
	Error     string  `json:"error,omitempty"`
}

// LocalFileInfo represents information about the last local file
type LocalFileInfo struct {
	Success  bool   `json:"success"`
	FileName string `json:"file_name"`
	Message  string `json:"message,omitempty"`
	Error    string `json:"error,omitempty"`
}

// UploadFileResponse represents the response for file upload
type UploadFileResponse struct {
	Success  bool   `json:"success"`
	FileName string `json:"file_name"`
	FileSize int64  `json:"file_size"`
	Message  string `json:"message,omitempty"`
	Error    string `json:"error,omitempty"`
}

// handleLatestRecording returns information about the latest recording
func (s *Server) handleLatestRecording(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Get the output directory from current config
	outputDir := s.cfg.Output.Directory

	// Expand tilde
	if strings.HasPrefix(outputDir, "~/") {
		home, err := os.UserHomeDir()
		if err == nil {
			outputDir = filepath.Join(home, outputDir[2:])
		}
	}

	// Find the latest recording file
	latestFile, err := findLatestRecording(outputDir)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(RecordingInfo{
			Success: false,
			Error:   "No recordings found",
		})
		return
	}

	// Get file info
	info, err := os.Stat(latestFile)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(RecordingInfo{
			Success: false,
			Error:   "Failed to get file info",
		})
		return
	}

	fileName := filepath.Base(latestFile)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(RecordingInfo{
		Success:   true,
		FileName:  fileName,
		FilePath:  latestFile,
		FileSize:  info.Size(),
		CreatedAt: info.ModTime().Format(time.RFC3339),
	})
}

// handleRecordingStream serves audio files for streaming
func (s *Server) handleRecordingStream(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Extract filename from URL path
	path := strings.TrimPrefix(r.URL.Path, "/api/recording/")
	if path == "" {
		http.Error(w, "Filename required", http.StatusBadRequest)
		return
	}

	// URL decode the filename
	fileName, err := url.QueryUnescape(path)
	if err != nil {
		http.Error(w, "Invalid filename encoding", http.StatusBadRequest)
		return
	}

	// Get the output directory from current config
	outputDir := s.cfg.Output.Directory

	// Expand tilde
	if strings.HasPrefix(outputDir, "~/") {
		home, err := os.UserHomeDir()
		if err == nil {
			outputDir = filepath.Join(home, outputDir[2:])
		}
	}

	filePath := filepath.Join(outputDir, fileName)

	// Security check: ensure the file is within the output directory
	cleanPath, err := filepath.Abs(filePath)
	if err != nil {
		http.Error(w, "Invalid file path", http.StatusBadRequest)
		return
	}

	cleanOutputDir, err := filepath.Abs(outputDir)
	if err != nil {
		http.Error(w, "Invalid output directory", http.StatusInternalServerError)
		return
	}

	if !strings.HasPrefix(cleanPath, cleanOutputDir) {
		http.Error(w, "Access denied", http.StatusForbidden)
		return
	}

	// Check if file exists
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		http.Error(w, "File not found", http.StatusNotFound)
		return
	}

	// Determine content type
	ext := strings.ToLower(filepath.Ext(fileName))
	contentType := mime.TypeByExtension(ext)

	// Handle FLAC files specifically since some systems don't have the MIME type registered
	switch ext {
	case ".flac":
		contentType = "audio/flac"
	case ".wav":
		contentType = "audio/wav"
	case ".mp3":
		contentType = "audio/mpeg"
	case ".mkv":
		contentType = "video/x-matroska"
	}

	if contentType == "" {
		contentType = "application/octet-stream"
	}

	// Set headers for audio streaming
	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Accept-Ranges", "bytes")
	w.Header().Set("Cache-Control", "public, max-age=31536000")

	// Serve the file
	http.ServeFile(w, r, filePath)
}

// findLatestRecording finds the most recent recording file in the output directory
// Prioritizes HTML5-compatible formats (FLAC, WAV, MP3) over MKV
func findLatestRecording(outputDir string) (string, error) {
	// Priority order: HTML5-compatible formats first
	priorityExts := []string{".flac", ".wav", ".mp3"}
	fallbackExts := []string{".mkv"}

	allExts := append(priorityExts, fallbackExts...)

	var latestFile string
	var latestTime time.Time
	var latestPriority int = -1

	err := filepath.Walk(outputDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // Skip files with errors
		}

		if info.IsDir() {
			return nil // Skip directories
		}

		ext := strings.ToLower(filepath.Ext(path))

		// Find extension priority
		priority := -1
		for i, validExt := range allExts {
			if ext == validExt {
				priority = i
				break
			}
		}

		if priority == -1 {
			return nil // Skip non-audio files
		}

		// Prefer files with higher priority (lower index), or newer files with same priority
		if priority < latestPriority || (priority == latestPriority && info.ModTime().After(latestTime)) || latestFile == "" {
			// Special case: if we find a FLAC file that might be the mixed version of an MKV
			if ext == ".flac" {
				baseName := strings.TrimSuffix(filepath.Base(path), ".flac")
				mkvFile := filepath.Join(outputDir, baseName+".mkv")
				if _, err := os.Stat(mkvFile); err == nil {
					// This FLAC file has a corresponding MKV, it's likely the mixed version
					latestTime = info.ModTime()
					latestFile = path
					latestPriority = priority
					return nil
				}
			}

			// For other cases, check if this is newer or higher priority
			if priority < latestPriority || info.ModTime().After(latestTime) || latestFile == "" {
				latestTime = info.ModTime()
				latestFile = path
				latestPriority = priority
			}
		}

		return nil
	})

	if err != nil {
		return "", err
	}

	if latestFile == "" {
		return "", fmt.Errorf("no audio files found")
	}

	return latestFile, nil
}

// handleSetLocalFile saves the name of the last local file selected by the user
func (s *Server) handleSetLocalFile(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusMethodNotAllowed)
		json.NewEncoder(w).Encode(LocalFileInfo{
			Success: false,
			Error:   "Method not allowed",
		})
		return
	}

	// Parse form data
	if err := r.ParseForm(); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(LocalFileInfo{
			Success: false,
			Error:   "Failed to parse form",
		})
		return
	}

	fileName := r.FormValue("filename")
	if fileName == "" {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(LocalFileInfo{
			Success: false,
			Error:   "Filename is required",
		})
		return
	}

	// Store the filename (thread-safe)
	s.fileLock.Lock()
	s.lastLocalFile = fileName
	s.fileLock.Unlock()

	slog.Debug("Saved last local file", "filename", fileName)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(LocalFileInfo{
		Success:  true,
		FileName: fileName,
		Message:  "Local file preference saved",
	})
}

// handleGetLocalFile returns the name of the last local file selected by the user
func (s *Server) handleGetLocalFile(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusMethodNotAllowed)
		json.NewEncoder(w).Encode(LocalFileInfo{
			Success: false,
			Error:   "Method not allowed",
		})
		return
	}

	// Get the filename (thread-safe)
	s.fileLock.RLock()
	fileName := s.lastLocalFile
	s.fileLock.RUnlock()

	if fileName == "" {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(LocalFileInfo{
			Success: false,
			Error:   "No local file preference saved",
		})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(LocalFileInfo{
		Success:  true,
		FileName: fileName,
		Message:  "Local file preference retrieved",
	})
}

// handleUploadLocalFile handles file upload for local audio files
func (s *Server) handleUploadLocalFile(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusMethodNotAllowed)
		json.NewEncoder(w).Encode(UploadFileResponse{
			Success: false,
			Error:   "Method not allowed",
		})
		return
	}

	// Parse multipart form (32MB max)
	err := r.ParseMultipartForm(32 << 20)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(UploadFileResponse{
			Success: false,
			Error:   "Failed to parse multipart form",
		})
		return
	}

	// Get the uploaded file
	file, handler, err := r.FormFile("audio_file")
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(UploadFileResponse{
			Success: false,
			Error:   "No audio file provided",
		})
		return
	}
	defer file.Close()

	// Validate file type by extension
	ext := strings.ToLower(filepath.Ext(handler.Filename))
	validExts := []string{".flac", ".wav", ".mp3", ".m4a", ".ogg"}
	isValid := false
	for _, validExt := range validExts {
		if ext == validExt {
			isValid = true
			break
		}
	}

	if !isValid {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(UploadFileResponse{
			Success: false,
			Error:   "Invalid audio file format. Supported: FLAC, WAV, MP3, M4A, OGG",
		})
		return
	}

	// Use temporary uploads directory for user-uploaded files
	tempDir := filepath.Join(os.TempDir(), "jamcapture_uploads")
	if err := os.MkdirAll(tempDir, 0755); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(UploadFileResponse{
			Success: false,
			Error:   "Failed to create upload directory",
		})
		return
	}

	// Create safe filename to prevent path traversal
	safeFilename := filepath.Base(handler.Filename)
	destPath := filepath.Join(tempDir, safeFilename)

	// If file already exists, add timestamp to make it unique
	if _, err := os.Stat(destPath); err == nil {
		name := strings.TrimSuffix(safeFilename, ext)
		timestamp := time.Now().Format("20060102_150405")
		safeFilename = fmt.Sprintf("%s_%s%s", name, timestamp, ext)
		destPath = filepath.Join(tempDir, safeFilename)
	}

	// Create destination file
	destFile, err := os.Create(destPath)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(UploadFileResponse{
			Success: false,
			Error:   "Failed to create destination file",
		})
		return
	}
	defer destFile.Close()

	// Copy file content
	fileSize, err := io.Copy(destFile, file)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(UploadFileResponse{
			Success: false,
			Error:   "Failed to save file",
		})
		return
	}

	// Save this as the last local file
	s.fileLock.Lock()
	s.lastLocalFile = safeFilename
	s.fileLock.Unlock()

	slog.Info("File uploaded successfully", "filename", safeFilename, "size", fileSize)

	// Return success response
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(UploadFileResponse{
		Success:  true,
		FileName: safeFilename,
		FileSize: fileSize,
		Message:  "File uploaded successfully to temporary directory",
	})
}


// handleBackingtrackStream serves temporary uploaded audio files (backing tracks)
func (s *Server) handleBackingtrackStream(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Extract filename from URL path
	path := strings.TrimPrefix(r.URL.Path, "/api/backingtrack/")
	if path == "" {
		http.Error(w, "Filename required", http.StatusBadRequest)
		return
	}

	// URL decode the filename
	fileName, err := url.QueryUnescape(path)
	if err != nil {
		http.Error(w, "Invalid filename encoding", http.StatusBadRequest)
		return
	}

	// Use temporary uploads directory
	tempDir := filepath.Join(os.TempDir(), "jamcapture_uploads")
	filePath := filepath.Join(tempDir, fileName)

	// Security check: ensure the file is within the temp directory
	cleanPath, err := filepath.Abs(filePath)
	if err != nil {
		http.Error(w, "Invalid file path", http.StatusBadRequest)
		return
	}

	cleanTempDir, err := filepath.Abs(tempDir)
	if err != nil {
		http.Error(w, "Invalid temp directory", http.StatusInternalServerError)
		return
	}

	if !strings.HasPrefix(cleanPath, cleanTempDir) {
		http.Error(w, "Access denied", http.StatusForbidden)
		return
	}

	// Check if file exists
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		http.Error(w, "Backingtrack file not found", http.StatusNotFound)
		return
	}

	// Determine content type
	ext := strings.ToLower(filepath.Ext(fileName))
	contentType := mime.TypeByExtension(ext)

	// Handle specific audio formats
	switch ext {
	case ".flac":
		contentType = "audio/flac"
	case ".wav":
		contentType = "audio/wav"
	case ".mp3":
		contentType = "audio/mpeg"
	case ".m4a":
		contentType = "audio/mp4"
	case ".ogg":
		contentType = "audio/ogg"
	case ".mkv":
		contentType = "video/x-matroska"
	}

	if contentType == "" {
		contentType = "application/octet-stream"
	}

	// Set headers for audio streaming
	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Accept-Ranges", "bytes")
	w.Header().Set("Cache-Control", "public, max-age=3600") // 1 hour cache for backing tracks

	// Serve the file
	http.ServeFile(w, r, filePath)
}

// ===== BACKING TRACKS API HANDLERS =====

// handleBackingtracks returns the list of backing tracks
func (s *Server) handleBackingtracks(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusMethodNotAllowed)
		json.NewEncoder(w).Encode(GenericResponse{
			Success: false,
			Error:   "Method not allowed",
		})
		return
	}

	// Get backing tracks from service
	backingtracks, err := s.service.ListBackingtracks()
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(GenericResponse{
			Success: false,
			Error:   fmt.Sprintf("Failed to load backing tracks: %v", err),
		})
		return
	}

	// Get directory path
	backingDir := s.getBackingtracksDirectory()

	// Get selected backing track
	selectedName := ""
	if selected, err := s.service.GetSelectedBackingtrack(); err == nil && selected != nil {
		selectedName = selected.Name
	}

	// Supported extensions
	supportedExts := []string{"flac", "wav", "mp3"}

	response := BackingtracksResponse{
		Backingtracks:          backingtracks,
		TotalCount:             len(backingtracks),
		BackingtracksDirectory: backingDir,
		SelectedBackingtrack:   selectedName,
		SupportedExtensions:    supportedExts,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// handleSelectedBackingtrack returns the currently selected backing track
func (s *Server) handleSelectedBackingtrack(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusMethodNotAllowed)
		json.NewEncoder(w).Encode(GenericResponse{
			Success: false,
			Error:   "Method not allowed",
		})
		return
	}

	selected, err := s.service.GetSelectedBackingtrack()
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(GenericResponse{
			Success: false,
			Error:   fmt.Sprintf("Failed to get selected backing track: %v", err),
		})
		return
	}

	if selected == nil {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success":              true,
			"selected_backingtrack": nil,
			"message":              "No backing track selected",
		})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success":              true,
		"selected_backingtrack": selected,
	})
}

// handleSelectBackingtrack sets the selected backing track
func (s *Server) handleSelectBackingtrack(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusMethodNotAllowed)
		json.NewEncoder(w).Encode(GenericResponse{
			Success: false,
			Error:   "Method not allowed",
		})
		return
	}

	var req BackingtrackSelectRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(GenericResponse{
			Success: false,
			Error:   "Invalid JSON payload",
		})
		return
	}

	if req.Name == "" {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(GenericResponse{
			Success: false,
			Error:   "Backing track name is required",
		})
		return
	}

	if err := s.service.SetSelectedBackingtrack(req.Name); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(GenericResponse{
			Success: false,
			Error:   fmt.Sprintf("Failed to select backing track: %v", err),
		})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(GenericResponse{
		Success: true,
		Message: fmt.Sprintf("Selected backing track: %s", req.Name),
	})
}

// handleConvertToBackingtrack converts a recording to a backing track
func (s *Server) handleConvertToBackingtrack(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusMethodNotAllowed)
		json.NewEncoder(w).Encode(GenericResponse{
			Success: false,
			Error:   "Method not allowed",
		})
		return
	}

	var req BackingtrackConvertRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(GenericResponse{
			Success: false,
			Error:   "Invalid JSON payload",
		})
		return
	}

	if req.RecordingName == "" {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(GenericResponse{
			Success: false,
			Error:   "Recording name is required",
		})
		return
	}

	if err := s.service.ConvertRecordingToBackingtrack(req.RecordingName); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(GenericResponse{
			Success: false,
			Error:   fmt.Sprintf("Failed to convert recording: %v", err),
		})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(GenericResponse{
		Success: true,
		Message: fmt.Sprintf("Successfully converted '%s' to backing track", req.RecordingName),
	})
}

// handleBackingtrackStreamNew streams a backing track file from the backingtracks directory
func (s *Server) handleBackingtrackStreamNew(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Extract filename from URL path
	path := strings.TrimPrefix(r.URL.Path, "/api/backingtracks/stream/")
	if path == "" {
		http.Error(w, "Filename required", http.StatusBadRequest)
		return
	}

	// URL decode the filename
	fileName, err := url.QueryUnescape(path)
	if err != nil {
		http.Error(w, "Invalid filename encoding", http.StatusBadRequest)
		return
	}

	// Get the backingtracks directory
	backingDir := s.getBackingtracksDirectory()

	filePath := filepath.Join(backingDir, fileName)

	// Security check: ensure the file is within the backingtracks directory
	cleanPath, err := filepath.Abs(filePath)
	if err != nil {
		http.Error(w, "Invalid file path", http.StatusBadRequest)
		return
	}

	cleanBackingDir, err := filepath.Abs(backingDir)
	if err != nil {
		http.Error(w, "Invalid backingtracks directory", http.StatusInternalServerError)
		return
	}

	if !strings.HasPrefix(cleanPath, cleanBackingDir) {
		http.Error(w, "Access denied", http.StatusForbidden)
		return
	}

	// Check if file exists
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		http.Error(w, "Backing track not found", http.StatusNotFound)
		return
	}

	// Determine content type
	ext := strings.ToLower(filepath.Ext(fileName))
	contentType := mime.TypeByExtension(ext)

	// Handle specific audio formats
	switch ext {
	case ".flac":
		contentType = "audio/flac"
	case ".wav":
		contentType = "audio/wav"
	case ".mp3":
		contentType = "audio/mpeg"
	}

	if contentType == "" {
		contentType = "application/octet-stream"
	}

	// Set headers for audio streaming
	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Accept-Ranges", "bytes")
	w.Header().Set("Cache-Control", "public, max-age=3600") // 1 hour cache for backing tracks

	// Serve the file
	http.ServeFile(w, r, filePath)
}

// handleBackingtrackDownload serves a backing track file for download
func (s *Server) handleBackingtrackDownload(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Extract filename from URL
	filename := r.URL.Path[len("/api/backingtracks/download/"):]
	if filename == "" {
		http.Error(w, "Filename required", http.StatusBadRequest)
		return
	}

	// URL decode the filename
	fileName, err := url.QueryUnescape(filename)
	if err != nil {
		http.Error(w, "Invalid filename encoding", http.StatusBadRequest)
		return
	}

	// Validate filename (prevent path traversal)
	if strings.Contains(fileName, "..") || strings.Contains(fileName, "/") || strings.Contains(fileName, "\\") {
		http.Error(w, "Invalid filename", http.StatusBadRequest)
		return
	}

	// Get the backingtracks directory
	backingDir := s.getBackingtracksDirectory()

	// Construct file path
	filePath := filepath.Join(backingDir, fileName)

	// Check if file exists
	info, err := os.Stat(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			http.Error(w, "File not found", http.StatusNotFound)
		} else {
			http.Error(w, "Error accessing file", http.StatusInternalServerError)
		}
		return
	}

	// Verify file extension is supported
	ext := strings.TrimPrefix(strings.ToLower(filepath.Ext(fileName)), ".")
	supportedExts := []string{"flac", "wav", "mp3"}
	supported := false
	for _, supportedExt := range supportedExts {
		if ext == strings.ToLower(supportedExt) {
			supported = true
			break
		}
	}

	if !supported {
		http.Error(w, "File type not supported", http.StatusForbidden)
		return
	}

	// Set headers for download
	contentType := mime.TypeByExtension("." + ext)
	if contentType == "" {
		contentType = "application/octet-stream"
	}

	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", fileName))
	w.Header().Set("Content-Length", fmt.Sprintf("%d", info.Size()))

	// Open and serve the file
	file, err := os.Open(filePath)
	if err != nil {
		http.Error(w, "Error opening file", http.StatusInternalServerError)
		return
	}
	defer file.Close()

	// Copy file to response
	_, err = io.Copy(w, file)
	if err != nil {
		slog.Error("Error serving backing track download", "file", fileName, "error", err)
	}
}

// getBackingtracksDirectory returns the resolved backing tracks directory path
func (s *Server) getBackingtracksDirectory() string {
	backingDir := s.cfg.Output.BackingtracksDirectory
	if backingDir == "" {
		backingDir = filepath.Join(s.cfg.Output.Directory, "BackingTracks")
	}
	return backingDir
}

// generateStatusMessage creates appropriate status messages based on current state
func (s *Server) generateStatusMessage(status service.RecordingStatus, session *service.RecordingSession) string {
	switch status {
	case service.StatusStandby:
		return ""
	case service.StatusReady:
		return "Waiting for audio sources - Please start audio playback"
	case service.StatusRecording:
		if session != nil {
			return fmt.Sprintf("Recording in progress - %s", session.SongName)
		}
		return "Recording in progress"
	case service.StatusError:
		// Get detailed error information from service
		if errorDetails := s.service.GetLastError(); errorDetails != "" {
			slog.Error("Displaying error status to user", "error_details", errorDetails)
			return errorDetails
		}
		genericError := "An error occurred during the operation"
		slog.Error("Displaying generic error status to user", "message", genericError)
		return genericError
	default:
		return ""
	}
}

// sendErrorResponse logs the error and sends a JSON error response to the client
func (s *Server) sendErrorResponse(w http.ResponseWriter, statusCode int, errorMsg string, logContext ...interface{}) {
	// Log the error with structured context
	logFields := []interface{}{"error_message", errorMsg, "status_code", statusCode}
	if len(logContext) > 0 {
		logFields = append(logFields, logContext...)
	}
	slog.Error("Sending error response to client", logFields...)

	// Send JSON error response
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": false,
		"error":   errorMsg,
	})
}

func getLocalIP() string {
	// Try to connect to a remote address to determine local IP
	conn, err := net.Dial("udp", "8.8.8.8:80")
	if err != nil {
		return "localhost"
	}
	defer conn.Close()

	localAddr := conn.LocalAddr().(*net.UDPAddr)
	return localAddr.IP.String()
}
