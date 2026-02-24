package audio

import (
	"fmt"
	"log/slog"
	"os/exec"
	"strings"
	"time"
)

// PipeWire manages PipeWire/JACK port operations
type PipeWire struct{}

// NewPipeWire creates a new PipeWire instance
func NewPipeWire() *PipeWire {
	return &PipeWire{}
}

// ListPorts returns all available JACK ports via PipeWire
func (pw *PipeWire) ListPorts() ([]string, error) {
	cmd := exec.Command("pw-link", "-io")
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to list PipeWire ports: %w", err)
	}

	lines := strings.Split(string(output), "\n")
	var ports []string

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line != "" && !strings.HasPrefix(line, "Input ports:") && !strings.HasPrefix(line, "Output ports:") {
			ports = append(ports, line)
		}
	}

	return ports, nil
}

// ValidatePort checks if a specific port exists and has no duplicates
func (pw *PipeWire) ValidatePort(portName string) error {
	if portName == "" || portName == "disabled" {
		return nil
	}

	if !pw.portExists(portName) {
		return fmt.Errorf("port not found: %s", portName)
	}

	// Check for duplicate sources
	duplicates, err := pw.findPortDuplicates(portName)
	if err != nil {
		return fmt.Errorf("failed to check port duplicates: %w", err)
	}

	if len(duplicates) > 1 {
		return fmt.Errorf("duplicate sources detected for '%s': %v. Please close conflicting applications", portName, duplicates)
	}

	return nil
}

// findPortDuplicates finds all ports with exactly the same name
func (pw *PipeWire) findPortDuplicates(portName string) ([]string, error) {
	allPorts, err := pw.ListPorts()
	if err != nil {
		return nil, fmt.Errorf("failed to list ports: %w", err)
	}

	var duplicates []string
	for _, port := range allPorts {
		if port == portName {
			duplicates = append(duplicates, port)
		}
	}

	return duplicates, nil
}

// findPortDuplicatesInList finds duplicates in a provided port list (for testing)
func (pw *PipeWire) findPortDuplicatesInList(portName string, allPorts []string) []string {
	var duplicates []string
	for _, port := range allPorts {
		if port == portName {
			duplicates = append(duplicates, port)
		}
	}

	return duplicates
}

// portExists checks if a port exists in the current JACK graph
func (pw *PipeWire) portExists(portName string) bool {
	cmd := exec.Command("pw-link", "-io")
	output, err := cmd.Output()
	if err != nil {
		slog.Debug("Failed to check port existence", "port", portName, "error", err)
		return false
	}

	lines := strings.Split(string(output), "\n")
	for _, line := range lines {
		if strings.TrimSpace(line) == portName {
			return true
		}
	}

	return false
}

// ConnectPortsWithRetry connects two JACK ports with intelligent retry logic
func (pw *PipeWire) ConnectPortsWithRetry(sourcePort, destPort string) error {
	// Determine if this is an ephemeral port (browser, etc.)
	isEphemeral := pw.isEphemeralPort(sourcePort)

	var maxRetries int
	var retryDelay time.Duration

	if isEphemeral {
		// Browsers and streaming apps may take longer to appear
		maxRetries = 15
		retryDelay = 1 * time.Second
		slog.Debug("Using ephemeral port retry strategy", "source", sourcePort, "retries", maxRetries)
	} else {
		// Hardware devices should be available quickly
		maxRetries = 5
		retryDelay = 500 * time.Millisecond
		slog.Debug("Using hardware port retry strategy", "source", sourcePort, "retries", maxRetries)
	}

	for attempt := 1; attempt <= maxRetries; attempt++ {
		if pw.portExists(sourcePort) {
			// Port exists, try to connect
			err := pw.connectPorts(sourcePort, destPort)
			if err == nil {
				slog.Debug("Successfully connected ports", "source", sourcePort, "dest", destPort, "attempt", attempt)
				return nil
			}
			slog.Debug("Connection attempt failed", "source", sourcePort, "dest", destPort, "attempt", attempt, "error", err)
		} else {
			slog.Debug("Source port not yet available", "source", sourcePort, "attempt", attempt)
		}

		if attempt < maxRetries {
			time.Sleep(retryDelay)
		}
	}

	return fmt.Errorf("failed to connect %s to %s after %d attempts", sourcePort, destPort, maxRetries)
}

// connectPorts performs the actual port connection
func (pw *PipeWire) connectPorts(sourcePort, destPort string) error {
	cmd := exec.Command("pw-link", sourcePort, destPort)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to connect ports: %w (output: %s)", err, string(output))
	}

	slog.Debug("Connected ports successfully", "source", sourcePort, "dest", destPort)
	return nil
}

// isEphemeralPort determines if a port is ephemeral (may appear/disappear)
func (pw *PipeWire) isEphemeralPort(portName string) bool {
	lowerPort := strings.ToLower(portName)

	ephemeralApps := []string{
		"chrome", "firefox", "spotify", "discord", "steam",
		"vlc", "mpv", "zoom", "teams", "slack", "wire",
	}

	for _, app := range ephemeralApps {
		if strings.Contains(lowerPort, app) {
			return true
		}
	}

	return false
}

// DisconnectPorts disconnects two JACK ports
func (pw *PipeWire) DisconnectPorts(sourcePort, destPort string) error {
	cmd := exec.Command("pw-link", "-d", sourcePort, destPort)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to disconnect ports: %w (output: %s)", err, string(output))
	}

	slog.Debug("Disconnected ports successfully", "source", sourcePort, "dest", destPort)
	return nil
}