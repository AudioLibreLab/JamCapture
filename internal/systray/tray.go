//go:build !nosystray

package systray

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"time"

	"fyne.io/systray"
	"github.com/audiolibrelab/jamcapture/internal/service"
)

type SystemTray struct {
	service service.Service
	webPort int
	quit    chan struct{}

	// Menu items
	menuRecord   *systray.MenuItem
	menuStop     *systray.MenuItem
	menuWebUI    *systray.MenuItem
	menuStatus   *systray.MenuItem
	menuPlayLatest *systray.MenuItem
	menuQuit     *systray.MenuItem

	// Context for cleanup
	ctx    context.Context
	cancel context.CancelFunc
}

// New creates a new SystemTray instance
func New(svc service.Service, webPort int) *SystemTray {
	ctx, cancel := context.WithCancel(context.Background())
	return &SystemTray{
		service: svc,
		webPort: webPort,
		quit:    make(chan struct{}),
		ctx:     ctx,
		cancel:  cancel,
	}
}

// Run starts the system tray application
func (st *SystemTray) Run() {
	systray.Run(st.onReady, st.onExit)
}

// onReady sets up the system tray when it's ready
func (st *SystemTray) onReady() {
	slog.Info("System tray initialized")

	// Set initial status
	st.updateIcon("STANDBY")

	// Create menu items
	st.setupMenu()

	// Start status monitoring
	go st.monitorStatus()

	// Handle menu item clicks
	go st.handleMenuEvents()
}

// onExit cleans up when the system tray exits
func (st *SystemTray) onExit() {
	slog.Info("System tray shutting down")
	st.cancel()
	close(st.quit)
}

// setupMenu creates the system tray menu
func (st *SystemTray) setupMenu() {
	st.menuRecord = systray.AddMenuItem("🔴 Record", "Start recording with a song name")
	st.menuStop = systray.AddMenuItem("⏹️ Stop Recording", "Stop current recording")
	st.menuStop.Hide() // Hidden by default

	systray.AddSeparator()

	st.menuWebUI = systray.AddMenuItem("🌐 Open Web UI", fmt.Sprintf("Open web interface in browser (port %d)", st.webPort))
	st.menuStatus = systray.AddMenuItem("📊 Status: STANDBY", "Current recording status")
	st.menuStatus.Disable()

	systray.AddSeparator()

	st.menuPlayLatest = systray.AddMenuItem("🎵 Play Latest", "Play latest recording with default audio player")

	systray.AddSeparator()
	st.menuQuit = systray.AddMenuItem("❌ Quit", "Quit JamCapture")
}

// handleMenuEvents processes menu item clicks
func (st *SystemTray) handleMenuEvents() {
	for {
		select {
		case <-st.ctx.Done():
			return

		case <-st.menuRecord.ClickedCh:
			st.handleRecord()

		case <-st.menuStop.ClickedCh:
			st.handleStop()

		case <-st.menuWebUI.ClickedCh:
			st.handleWebUI()

		case <-st.menuPlayLatest.ClickedCh:
			st.handlePlayLatest()

		case <-st.menuQuit.ClickedCh:
			st.handleQuit()
			return
		}
	}
}

// handleRecord starts recording with a generated default song name
func (st *SystemTray) handleRecord() {
	songName := service.GenerateDefaultSongName()
	slog.Info("Starting recording", "song_name", songName)

	if err := st.service.StartReady(songName); err != nil {
		slog.Error("Failed to start recording", "error", err)
		st.showNotification("Recording Failed", fmt.Sprintf("Failed to start recording: %v", err))
		return
	}

	st.showNotification("Recording Started", fmt.Sprintf("Recording '%s' has started", songName))
}

// handleStop stops the current recording
func (st *SystemTray) handleStop() {
	// Capture song name before stopping (service clears it on stop)
	songName := st.service.CurrentSongName()
	slog.Info("Stopping recording", "song_name", songName)

	if err := st.service.StopRecording(); err != nil {
		slog.Error("Failed to stop recording", "error", err)
		st.showNotification("Stop Failed", fmt.Sprintf("Failed to stop recording: %v", err))
		return
	}

	st.showNotification("Recording Stopped", fmt.Sprintf("Recording '%s' has been stopped", songName))

	// Auto-mix the recording to create FLAC file
	slog.Info("Auto-mixing recording", "song_name", songName)
	if err := st.service.Mix(songName); err != nil {
		slog.Error("Failed to auto-mix recording", "error", err)
		st.showNotification("Mix Failed", fmt.Sprintf("Recording stopped but auto-mix failed: %v", err))
	} else {
		slog.Info("Auto-mix completed", "song_name", songName)
		st.showNotification("Mix Complete", fmt.Sprintf("'%s' has been mixed and is ready", songName))
	}
}

// handleWebUI opens the web interface in the default browser
func (st *SystemTray) handleWebUI() {
	url := fmt.Sprintf("http://localhost:%d", st.webPort)
	slog.Info("Opening web UI", "url", url)

	// Check if web server is running
	resp, err := http.Get(url)
	if err != nil {
		st.showNotification("Web UI Error", "Web server is not running. Start it with: jamcapture serve")
		return
	}
	resp.Body.Close()

	// Open in browser
	if err := st.openURL(url); err != nil {
		slog.Error("Failed to open web UI", "error", err)
		st.showNotification("Browser Error", fmt.Sprintf("Failed to open browser: %v", err))
	}
}

// handlePlayLatest plays the latest recorded file with the default audio player
func (st *SystemTray) handlePlayLatest() {
	lastFile := st.service.GetLatestMixedFile()
	if lastFile == "" {
		st.showNotification("No Recording", "No recording available to play")
		return
	}

	slog.Info("Playing latest recording", "file", lastFile)

	// Create a player and play the file
	// Extract song name from the file (remove extension)
	songName := lastFile
	if dotIndex := strings.LastIndex(lastFile, "."); dotIndex > 0 {
		songName = lastFile[:dotIndex]
	}

	if err := st.service.Play(songName); err != nil {
		slog.Error("Failed to play last recording", "error", err)
		st.showNotification("Playback Failed", fmt.Sprintf("Failed to play recording: %v", err))
		return
	}

	st.showNotification("Playing", fmt.Sprintf("Playing '%s'", lastFile))
}

// handleQuit properly shuts down the entire application
func (st *SystemTray) handleQuit() {
	slog.Info("User requested shutdown via systray")

	// Stop any active recording first
	status, _ := st.service.GetRecordingStatus()
	if status == service.StatusRecording || status == service.StatusReady {
		songName := st.service.CurrentSongName()
		slog.Info("Stopping active recording before shutdown", "song_name", songName)
		if err := st.service.StopRecording(); err != nil {
			slog.Error("Failed to stop recording during shutdown", "error", err)
		}
	}

	st.cancel()
	systray.Quit()
}

// monitorStatus periodically checks and updates the recording status
func (st *SystemTray) monitorStatus() {
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-st.ctx.Done():
			return
		case <-ticker.C:
			st.updateStatus()
		}
	}
}

// updateStatus checks the current status and updates the UI
func (st *SystemTray) updateStatus() {
	status, session := st.service.GetRecordingStatus()

	// Update icon
	st.updateIcon(string(status))

	// Update status menu item
	statusText := fmt.Sprintf("📊 Status: %s", status)
	if session != nil {
		statusText += fmt.Sprintf(" (%s)", session.SongName)
	}
	st.menuStatus.SetTitle(statusText)

	// Show/hide menu items based on status
	switch status {
	case service.StatusRecording, service.StatusReady:
		st.menuRecord.Hide()
		st.menuStop.Show()
	default: // STANDBY, ERROR
		st.menuRecord.Show()
		st.menuStop.Hide()
	}
}

// updateIcon updates the system tray display based on status
func (st *SystemTray) updateIcon(status string) {
	// Try setting both icon and title for maximum compatibility
	icon := GetIcon(status)
	systray.SetIcon(icon)

	// Set title for better status visibility - use clear indicators
	var title string
	switch status {
	case "RECORDING":
		title = "◉ JC REC" // Filled circle for recording
	case "READY":
		title = "◐ JC RDY" // Half circle for ready
	case "ERROR":
		title = "✗ JC ERR" // X for error
	default:
		title = "○ JC" // Empty circle for standby
	}
	systray.SetTitle(title)

	// Update tooltip
	var tooltip string
	switch status {
	case "RECORDING":
		tooltip = fmt.Sprintf("JamCapture - Recording: %s", st.service.CurrentSongName())
	case "READY":
		tooltip = "JamCapture - Ready to record"
	case "ERROR":
		tooltip = "JamCapture - Error occurred"
	default:
		tooltip = "JamCapture - Standby"
	}
	systray.SetTooltip(tooltip)
}

// openURL opens a URL in the default browser
func (st *SystemTray) openURL(url string) error {
	var cmd *exec.Cmd

	switch runtime.GOOS {
	case "linux":
		cmd = exec.Command("xdg-open", url)
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	case "darwin":
		cmd = exec.Command("open", url)
	default:
		return fmt.Errorf("unsupported platform: %s", runtime.GOOS)
	}

	return cmd.Start()
}

// showNotification shows a desktop notification (placeholder)
func (st *SystemTray) showNotification(title, message string) {
	slog.Info("Notification", "title", title, "message", message)
	// TODO: Add proper desktop notifications using a library like github.com/gen2brain/beeep
}

// Shutdown gracefully shuts down the system tray
func (st *SystemTray) Shutdown() {
	systray.Quit()
}

// IsSupported checks if system tray is supported on the current platform
func IsSupported() bool {
	if runtime.GOOS != "linux" {
		// Only support Linux for now
		return false
	}

	// Check if we have a display server (X11 or Wayland)
	if os.Getenv("DISPLAY") == "" && os.Getenv("WAYLAND_DISPLAY") == "" {
		return false
	}

	// Check if we have XDG session (indicates desktop environment)
	if os.Getenv("XDG_CURRENT_DESKTOP") == "" {
		return false
	}

	// Try to ping DBus session bus (required for system tray)
	cmd := exec.Command("busctl", "--user", "status")
	if err := cmd.Run(); err != nil {
		slog.Debug("DBus session bus not available, system tray not supported", "error", err)
		return false
	}

	return true
}

// IsSupportedVerbose checks if system tray is supported and logs detailed reasons
func IsSupportedVerbose() bool {
	if runtime.GOOS != "linux" {
		slog.Info("System tray only supported on Linux", "platform", runtime.GOOS)
		return false
	}

	slog.Debug("Checking Linux system tray support")

	// Check display server
	display := os.Getenv("DISPLAY")
	waylandDisplay := os.Getenv("WAYLAND_DISPLAY")
	if display == "" && waylandDisplay == "" {
		slog.Info("System tray not supported: no display server detected (DISPLAY or WAYLAND_DISPLAY not set)")
		return false
	}
	slog.Debug("Display server detected", "DISPLAY", display, "WAYLAND_DISPLAY", waylandDisplay)

	// Check desktop environment
	desktop := os.Getenv("XDG_CURRENT_DESKTOP")
	if desktop == "" {
		slog.Info("System tray not supported: no desktop environment detected (XDG_CURRENT_DESKTOP not set)")
		return false
	}
	slog.Debug("Desktop environment detected", "XDG_CURRENT_DESKTOP", desktop)

	// Check DBus
	cmd := exec.Command("busctl", "--user", "status")
	if err := cmd.Run(); err != nil {
		slog.Info("System tray not supported: DBus session bus not available", "error", err)
		return false
	}
	slog.Debug("DBus session bus is available")

	slog.Info("System tray is supported on this Linux system", "desktop", desktop)
	return true
}