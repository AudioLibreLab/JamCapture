package audio

import (
	"io"
	"strings"

	"github.com/audiolibrelab/jamcapture/internal/config"
)

// BackendType represents the type of audio backend
type BackendType string

const (
	BackendTypePipeWire BackendType = "pipewire"
	BackendTypeAuto     BackendType = "auto"
)

// AudioBackend defines the interface for audio backend implementations
type AudioBackend interface {
	// Create a new recorder instance
	NewRecorder(cfg *config.Config, logWriter io.Writer) Recorder

	// List available audio sources
	ListSources() ([]string, error)

	// Validate if a source is available
	ValidateSource(source string) error

	// Get the backend type
	GetType() BackendType
}

// NewRecorder creates a recorder using the appropriate backend based on configuration
func NewRecorder(cfg *config.Config, logWriter io.Writer) Recorder {
	backendType := determineBackend(cfg)

	switch backendType {
	case BackendTypePipeWire:
		backend := &PipeWireBackend{}
		return backend.NewRecorder(cfg, logWriter)
	default:
		// Default to PipeWire as the only available backend
		backend := &PipeWireBackend{}
		return backend.NewRecorder(cfg, logWriter)
	}
}

// determineBackend determines which backend to use based on configuration
func determineBackend(cfg *config.Config) BackendType {
	// If explicitly specified in config
	if cfg.Audio.Backend != "" {
		switch strings.ToLower(cfg.Audio.Backend) {
		case "pipewire":
			return BackendTypePipeWire
		case "auto":
			return BackendTypePipeWire // Only PipeWire is available now
		}
	}

	// Only PipeWire backend is available
	return BackendTypePipeWire
}


// GetAvailableBackends returns list of available backends on current system
func GetAvailableBackends() []BackendType {
	backends := []BackendType{}

	// Only PipeWire backend is available
	backends = append(backends, BackendTypePipeWire)

	return backends
}