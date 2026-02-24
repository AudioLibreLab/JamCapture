package audio

import (
	"io"

	"github.com/audiolibrelab/jamcapture/internal/config"
)

// PipeWireBackend implements the AudioBackend interface for PipeWire
type PipeWireBackend struct{}

// NewRecorder creates a new PipeWire recorder
func (p *PipeWireBackend) NewRecorder(cfg *config.Config, logWriter io.Writer) Recorder {
	return NewPipeWireRecorder(cfg, logWriter)
}

// ListSources returns available PipeWire/JACK sources
func (p *PipeWireBackend) ListSources() ([]string, error) {
	pw := NewPipeWire()
	return pw.ListPorts()
}

// ValidateSource validates a PipeWire/JACK source
func (p *PipeWireBackend) ValidateSource(source string) error {
	if source == "" || source == "disabled" {
		return nil
	}

	pw := NewPipeWire()
	return pw.ValidatePort(source)
}

// GetType returns the backend type
func (p *PipeWireBackend) GetType() BackendType {
	return BackendTypePipeWire
}