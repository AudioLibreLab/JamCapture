package audio

import (
	"bufio"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/audiolibrelab/jamcapture/internal/config"
)

// PipeWireRecorder implements the Recorder interface using PipeWire/JACK
type PipeWireRecorder struct {
	cfg       *config.Config
	logWriter io.Writer

	// PipeWire components
	pipewire *PipeWire

	// Recording state
	mutex       sync.RWMutex
	status      Status
	session     *SessionInfo
	isRecording bool
	stopChan    chan struct{}

	// FFmpeg process
	ffmpegCmd *exec.Cmd
	stdoutBuf strings.Builder
	stderrBuf strings.Builder

	// Source monitoring
	sourceMonitorStop chan struct{}
	sourceMonitorDone chan struct{}

	// Channel status cache
	channelStatusCache     map[string]string
	channelStatusCacheTime time.Time
}

// NewPipeWireRecorder creates a new PipeWire-based recorder
func NewPipeWireRecorder(cfg *config.Config, logWriter io.Writer) *PipeWireRecorder {
	if logWriter == nil {
		logWriter = io.Discard
	}

	return &PipeWireRecorder{
		cfg:       cfg,
		logWriter: logWriter,
		pipewire:  NewPipeWire(),
		status:    StatusStandby,
	}
}

// StartReady transitions from STANDBY to READY state
func (r *PipeWireRecorder) StartReady(songName string) error {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	if r.status != StatusStandby && r.status != StatusError {
		return fmt.Errorf("can only start ready from standby or error state, current: %s", r.status)
	}

	if songName == "" {
		return fmt.Errorf("song name is required")
	}

	// Create output directory
	if err := os.MkdirAll(r.cfg.Output.Directory, 0755); err != nil {
		r.status = StatusError
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	// Prepare session info
	cleanName := r.cleanFileName(songName)
	outputFile := filepath.Join(r.cfg.Output.Directory, cleanName+".mkv")

	enabledChannels := r.cfg.Channels
	channelNames := make([]string, len(enabledChannels))
	for i, ch := range enabledChannels {
		channelNames[i] = fmt.Sprintf("%s:[%s]", ch.Name, strings.Join(ch.Sources, ","))
	}

	// Validate sources before transitioning to READY state
	if r.hasDuplicateSources() {
		r.status = StatusError
		// Start monitoring even in error state so we can auto-recover when duplicates are resolved
		r.startSourceMonitoring()
		return fmt.Errorf("duplicate audio sources detected - please close conflicting applications before starting recording")
	}

	r.session = &SessionInfo{
		SongName:     songName,
		StartTime:    time.Now(),
		OutputFile:   outputFile,
		ChannelCount: len(enabledChannels),
		ChannelNames: channelNames,
	}

	r.status = StatusReady

	// Start automatic source monitoring
	r.startSourceMonitoring()

	slog.Info("PipeWire ready state activated", "song", songName, "channels", len(enabledChannels))
	return nil
}

// StartRecording begins recording from READY state
func (r *PipeWireRecorder) StartRecording() error {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	if r.status != StatusReady {
		return fmt.Errorf("can only start recording from ready state, current: %s", r.status)
	}

	if r.session == nil {
		return fmt.Errorf("no session prepared, call StartReady first")
	}

	// Update channel status cache before starting
	r.scanChannelStatus()

	// Remove existing output file
	os.Remove(r.session.OutputFile)

	// Build and start FFmpeg command
	enabledChannels := r.cfg.Channels
	if err := r.buildAndStartFFmpeg(enabledChannels, r.session.OutputFile); err != nil {
		r.status = StatusError
		return fmt.Errorf("failed to start FFmpeg: %w", err)
	}

	r.isRecording = true
	r.status = StatusRecording

	slog.Info("PipeWire recording started", "song", r.session.SongName, "channels", len(enabledChannels))

	// Start background goroutine to monitor FFmpeg and handle connections
	r.stopChan = make(chan struct{})
	go r.recordingWorker(enabledChannels)

	return nil
}

// recordingWorker handles the recording process in background
func (r *PipeWireRecorder) recordingWorker(enabledChannels []config.Channel) {
	// Wait for FFmpeg JACK ports to appear (1 second)
	time.Sleep(1 * time.Second)

	// Connect all channel sources to their corresponding FFmpeg inputs
	for _, channel := range enabledChannels {
		// Determine if this is mono or stereo based on sources
		if len(channel.Sources) <= 1 {
			// Mono channel - connect to input_1
			destPort := fmt.Sprintf("jamcapture_%s:input_1", channel.Name)

			// Wait for FFmpeg port to be available
			if err := r.waitForSpecificPort(destPort, 5*time.Second); err != nil {
				slog.Error("FFmpeg JACK port did not appear", "port", destPort, "error", err)
				continue
			}

			// Connect the source to the input
			if len(channel.Sources) > 0 {
				source := channel.Sources[0]
				if source != "" && source != "disabled" {
					if err := r.pipewire.ConnectPortsWithRetry(source, destPort); err != nil {
						slog.Error("Failed to connect mono source", "channel", channel.Name, "source", source, "dest", destPort, "error", err)
					} else {
						slog.Info("Connected mono source successfully", "channel", channel.Name, "source", source, "dest", destPort)
					}
				}
			}
		} else {
			// Stereo channel - connect to input_1 and input_2
			for i, source := range channel.Sources {
				if i >= 2 { // Only handle first 2 sources for stereo
					break
				}
				if source == "" || source == "disabled" {
					continue
				}

				destPort := fmt.Sprintf("jamcapture_%s:input_%d", channel.Name, i+1)

				// Wait for FFmpeg port to be available
				if err := r.waitForSpecificPort(destPort, 5*time.Second); err != nil {
					slog.Error("FFmpeg JACK port did not appear", "port", destPort, "error", err)
					continue
				}

				if err := r.pipewire.ConnectPortsWithRetry(source, destPort); err != nil {
					slog.Error("Failed to connect stereo source", "channel", channel.Name, "source", source, "dest", destPort, "error", err)
				} else {
					slog.Info("Connected stereo source successfully", "channel", channel.Name, "source", source, "dest", destPort)
				}
			}
		}
	}

	// Wait for recording to be stopped
	<-r.stopChan
}

// Stop ends the current recording session
func (r *PipeWireRecorder) Stop() error {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	if r.status != StatusRecording {
		return fmt.Errorf("no recording in progress")
	}

	slog.Debug("Stopping PipeWire recording...")

	// Stop source monitoring
	r.stopSourceMonitoring()

	r.isRecording = false
	if r.stopChan != nil {
		close(r.stopChan)
	}

	// Stop FFmpeg process
	if err := r.stopFFmpeg(); err != nil {
		r.status = StatusError
		return fmt.Errorf("failed to stop FFmpeg: %w", err)
	}

	// Validate output file
	if err := r.validateOutputFile(); err != nil {
		r.status = StatusError
		return err
	}

	r.status = StatusStandby
	slog.Debug("PipeWire recording completed successfully", "output", r.session.OutputFile)

	return nil
}

// CancelReady cancels ready state and returns to STANDBY
func (r *PipeWireRecorder) CancelReady() error {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	if r.status != StatusReady {
		return fmt.Errorf("can only cancel from ready state, current: %s", r.status)
	}

	r.stopSourceMonitoring()

	r.status = StatusStandby
	r.session = nil
	slog.Debug("PipeWire ready state cancelled, returned to standby")

	return nil
}

// GetStatus returns the current status and session info
func (r *PipeWireRecorder) GetStatus() (Status, *SessionInfo) {
	r.mutex.RLock()
	defer r.mutex.RUnlock()

	var sessionCopy *SessionInfo
	if r.session != nil {
		sessionCopy = &SessionInfo{
			SongName:     r.session.SongName,
			StartTime:    r.session.StartTime,
			OutputFile:   r.session.OutputFile,
			ChannelCount: r.session.ChannelCount,
			ChannelNames: make([]string, len(r.session.ChannelNames)),
		}
		copy(sessionCopy.ChannelNames, r.session.ChannelNames)
	}

	return r.status, sessionCopy
}

// GetChannelStatus returns the availability status of configured channels
func (r *PipeWireRecorder) GetChannelStatus() map[string]string {
	r.mutex.RLock()
	defer r.mutex.RUnlock()

	// If recording, return cached status
	if r.status == StatusRecording {
		if r.channelStatusCache != nil {
			return r.channelStatusCache
		}
	}

	return r.scanChannelStatus()
}

// scanChannelStatus scans and caches channel status
func (r *PipeWireRecorder) scanChannelStatus() map[string]string {
	status := make(map[string]string)

	for _, channel := range r.cfg.Channels {
		channelAvailable := true
		channelHasDuplicates := false

		for _, source := range channel.Sources {
			if source != "" && source != "disabled" {
				if err := r.pipewire.ValidatePort(source); err != nil {
					channelAvailable = false
					// Check if error is due to duplicates
					if strings.Contains(err.Error(), "duplicate sources detected") {
						channelHasDuplicates = true
						slog.Error("Channel source has duplicates", "channel", channel.Name, "source", source, "error", err)
					} else {
						slog.Debug("Channel source unavailable", "channel", channel.Name, "source", source, "error", err)
					}
					break
				}
			}
		}

		if channelHasDuplicates {
			status[channel.Name] = "duplicate"
		} else if channelAvailable {
			status[channel.Name] = "available"
		} else {
			status[channel.Name] = "unavailable"
		}
	}

	r.channelStatusCache = status
	r.channelStatusCacheTime = time.Now()

	return status
}

// Cleanup cleans up PipeWire resources
func (r *PipeWireRecorder) Cleanup() error {
	if r.ffmpegCmd != nil && r.ffmpegCmd.Process != nil {
		r.ffmpegCmd.Process.Kill()
		r.ffmpegCmd.Wait()
	}

	slog.Debug("PipeWire recorder cleaned up")
	return nil
}

// buildAndStartFFmpeg constructs and starts the FFmpeg command for PipeWire recording
func (r *PipeWireRecorder) buildAndStartFFmpeg(channels []config.Channel, outputFile string) error {
	// Set PipeWire environment variables
	env := os.Environ()
	env = append(env, "PIPEWIRE_QUANTUM=256/48000")
	env = append(env, "PIPEWIRE_LATENCY=256/48000")

	// Build FFmpeg command - create individual JACK clients per channel like main branch
	args := []string{
		"pw-jack",
		"ffmpeg",
	}

	// Add each channel as a separate JACK input
	for _, channel := range channels {
		// Determine number of channels for this input (mono=1, stereo=2)
		channelCount := len(channel.Sources)
		if channelCount == 0 {
			channelCount = 1 // Default to mono
		}
		if channelCount > 2 {
			channelCount = 2 // Cap at stereo
		}

		args = append(args,
			"-f", "jack",
			"-channels", fmt.Sprintf("%d", channelCount),
			"-i", fmt.Sprintf("jamcapture_%s", channel.Name),
		)
	}

	// Add sample rate
	args = append(args, "-ar", fmt.Sprintf("%d", r.cfg.Audio.SampleRate))

	// Map each input to a separate track with metadata
	for i, channel := range channels {
		args = append(args, "-map", fmt.Sprintf("%d:0", i))
		args = append(args, fmt.Sprintf("-metadata:s:a:%d", i), fmt.Sprintf("title=%s", channel.Name))
	}

	// Add codec and output
	args = append(args,
		"-c:a", r.cfg.Output.Format,
		"-y", // Overwrite output
		outputFile,
	)

	slog.Info("Starting PipeWire FFmpeg", "command", strings.Join(args, " "))

	// Create command
	cmd := exec.Command(args[0], args[1:]...)
	cmd.Env = env

	// Capture output for debugging
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("failed to create stdout pipe: %w", err)
	}

	stderr, err := cmd.StderrPipe()
	if err != nil {
		return fmt.Errorf("failed to create stderr pipe: %w", err)
	}

	// Start command
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start FFmpeg: %w", err)
	}

	r.ffmpegCmd = cmd

	// Start output readers
	go r.readOutput(stdout, &r.stdoutBuf, "stdout")
	go r.readOutput(stderr, &r.stderrBuf, "stderr")

	return nil
}

// readOutput reads from a pipe and buffers output
func (r *PipeWireRecorder) readOutput(pipe io.ReadCloser, buffer *strings.Builder, label string) {
	scanner := bufio.NewScanner(pipe)
	for scanner.Scan() {
		line := scanner.Text()
		buffer.WriteString(line + "\n")
		slog.Debug("FFmpeg output", "stream", label, "line", line)
	}
	pipe.Close()
}

// stopFFmpeg stops the FFmpeg process
func (r *PipeWireRecorder) stopFFmpeg() error {
	if r.ffmpegCmd == nil {
		return nil
	}

	// Send termination signal
	if r.ffmpegCmd.Process != nil {
		slog.Debug("Sending SIGINT to FFmpeg process")
		if err := r.ffmpegCmd.Process.Signal(os.Interrupt); err != nil {
			slog.Debug("Failed to send interrupt to FFmpeg", "error", err)
			// Try kill as fallback
			slog.Debug("Falling back to SIGKILL")
			r.ffmpegCmd.Process.Kill()
		}
	}

	// Wait for process to exit with timeout
	done := make(chan error, 1)
	go func() {
		done <- r.ffmpegCmd.Wait()
	}()

	select {
	case err := <-done:
		r.ffmpegCmd = nil
		if err != nil {
			// Check if it's a normal exit due to signal
			if exitErr, ok := err.(*exec.ExitError); ok {
				// Exit code 255 often means the process was interrupted gracefully
				if exitErr.ExitCode() == 255 {
					slog.Debug("FFmpeg exited normally after interrupt signal")
					return nil
				}
				// Check for signal-based termination
				if exitErr.ProcessState != nil {
					stateStr := exitErr.ProcessState.String()
					if stateStr == "signal: interrupt" || stateStr == "signal: killed" {
						slog.Debug("FFmpeg exited normally due to signal", "state", stateStr)
						return nil
					}
				}
			}
			slog.Debug("FFmpeg stderr", "output", r.stderrBuf.String())
			return fmt.Errorf("FFmpeg process failed: %w", err)
		}
		slog.Debug("FFmpeg exited successfully")
		return nil

	case <-time.After(5 * time.Second):
		// Timeout - force kill
		slog.Warn("FFmpeg did not exit within timeout, force killing")
		if r.ffmpegCmd.Process != nil {
			r.ffmpegCmd.Process.Kill()
		}
		<-done // Wait for the kill to complete
		r.ffmpegCmd = nil
		return nil
	}
}

// validateOutputFile validates the created output file
func (r *PipeWireRecorder) validateOutputFile() error {
	if r.session == nil {
		return fmt.Errorf("no session info available")
	}

	fileInfo, err := os.Stat(r.session.OutputFile)
	if err != nil {
		return fmt.Errorf("recording file not found: %s", r.session.OutputFile)
	}

	if fileInfo.Size() < 1024 {
		return fmt.Errorf("recording failed: file too small (%d bytes)", fileInfo.Size())
	}

	slog.Debug("PipeWire output file validated", "size", fileInfo.Size())
	return nil
}

// startSourceMonitoring monitors source availability and auto-transitions to RECORDING
func (r *PipeWireRecorder) startSourceMonitoring() {
	r.sourceMonitorStop = make(chan struct{})
	r.sourceMonitorDone = make(chan struct{})

	go func() {
		defer close(r.sourceMonitorDone)

		r.mutex.RLock()
		currentStatus := r.status
		r.mutex.RUnlock()
		slog.Debug("PipeWire source monitoring started", "current_status", currentStatus)

		ticker := time.NewTicker(500 * time.Millisecond)
		defer ticker.Stop()

		timeout := time.After(30 * time.Second)

		for {
			select {
			case <-r.sourceMonitorStop:
				slog.Info("PipeWire source monitoring stopped")
				return

			case <-timeout:
				slog.Info("PipeWire source monitoring timeout - returning to STANDBY")
				r.mutex.Lock()
				if r.status == StatusReady || r.status == StatusError {
					r.status = StatusStandby
					r.session = nil
				}
				r.mutex.Unlock()
				return

			case <-ticker.C:
				r.mutex.Lock()
				currentStatus := r.status
				r.mutex.Unlock()

				hasDuplicates := r.hasDuplicateSources()

				// Handle different states
				if currentStatus == StatusReady {
					if hasDuplicates {
						slog.Info("PipeWire duplicate sources detected - returning to STANDBY")
						r.mutex.Lock()
						r.status = StatusStandby
						r.session = nil
						r.mutex.Unlock()
						return
					}
				} else if currentStatus == StatusError {
					if hasDuplicates {
						slog.Debug("PipeWire duplicates still detected while in ERROR state - continuing monitoring")
					} else {
						slog.Info("PipeWire duplicate sources resolved - recovering from ERROR to STANDBY")
						r.mutex.Lock()
						r.status = StatusStandby
						r.session = nil
						r.mutex.Unlock()
					}
				}

				if r.checkAllSourcesAvailable() {
					r.mutex.Lock()
					if r.status == StatusReady {
						r.mutex.Unlock()

						slog.Info("All PipeWire sources available - starting recording automatically")
						if err := r.StartRecording(); err != nil {
							slog.Error("Failed to auto-start PipeWire recording", "error", err)
							r.mutex.Lock()
							r.status = StatusError
							r.mutex.Unlock()
						}
						return
					} else {
						r.mutex.Unlock()
					}
				}
			}
		}
	}()
}

// stopSourceMonitoring stops the source monitoring goroutine
func (r *PipeWireRecorder) stopSourceMonitoring() {
	if r.sourceMonitorStop != nil {
		close(r.sourceMonitorStop)
		if r.sourceMonitorDone != nil {
			<-r.sourceMonitorDone
		}
	}
}

// checkAllSourcesAvailable validates all configured sources
func (r *PipeWireRecorder) checkAllSourcesAvailable() bool {
	validChannels := 0
	totalChannelsToCheck := 0
	hasDuplicates := false

	for _, channel := range r.cfg.Channels {
		channelHasAllSources := true
		channelHasAnySources := false

		for _, source := range channel.Sources {
			if source != "" && source != "disabled" {
				channelHasAnySources = true
				totalChannelsToCheck++

				if err := r.pipewire.ValidatePort(source); err != nil {
					if strings.Contains(err.Error(), "duplicate sources detected") {
						hasDuplicates = true
						slog.Debug("PipeWire duplicate sources detected", "channel", channel.Name, "source", source, "error", err)
					} else {
						slog.Debug("PipeWire source validation failed", "channel", channel.Name, "source", source, "error", err)
					}
					channelHasAllSources = false
				}
			}
		}

		if channelHasAnySources && channelHasAllSources {
			validChannels++
		}
	}

	// Block transition if duplicates are detected
	if hasDuplicates {
		slog.Info("PipeWire duplicate sources detected - blocking READY→RECORDING transition")
		return false
	}

	// Count total channels that have sources configured (not disabled)
	totalChannelsWithSources := 0
	for _, channel := range r.cfg.Channels {
		hasAnySources := false
		for _, source := range channel.Sources {
			if source != "" && source != "disabled" {
				hasAnySources = true
				break
			}
		}
		if hasAnySources {
			totalChannelsWithSources++
		}
	}

	// ALL channels with sources must be valid, not just some
	result := validChannels > 0 && validChannels == totalChannelsWithSources
	slog.Debug("PipeWire source availability check", "validChannels", validChannels, "totalChannelsWithSources", totalChannelsWithSources, "totalChecked", totalChannelsToCheck, "result", result)

	return result
}

// hasDuplicateSources checks if any configured sources have duplicates
func (r *PipeWireRecorder) hasDuplicateSources() bool {
	for _, channel := range r.cfg.Channels {
		for _, source := range channel.Sources {
			if source != "" && source != "disabled" {
				if err := r.pipewire.ValidatePort(source); err != nil {
					if strings.Contains(err.Error(), "duplicate sources detected") {
						return true
					}
				}
			}
		}
	}
	return false
}

// waitForSpecificPort waits for a specific JACK port to appear
func (r *PipeWireRecorder) waitForSpecificPort(portName string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)

	for time.Now().Before(deadline) {
		if err := r.pipewire.ValidatePort(portName); err == nil {
			slog.Debug("JACK port found", "port", portName)
			return nil
		}
		time.Sleep(100 * time.Millisecond)
	}

	return fmt.Errorf("timeout waiting for JACK port: %s", portName)
}

// cleanFileName sanitizes a filename
// Allows: letters, numbers, spaces, hyphens, underscores
func (r *PipeWireRecorder) cleanFileName(name string) string {
	var result strings.Builder
	for _, r := range name {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == ' ' || r == '-' || r == '_' {
			result.WriteRune(r)
		}
	}
	return strings.ReplaceAll(strings.TrimSpace(result.String()), " ", "_")
}