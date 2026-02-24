package audio

import (
	"time"
)

// Status represents the current state of the recorder
type Status string

const (
	StatusStandby   Status = "STANDBY"
	StatusReady     Status = "READY"
	StatusRecording Status = "RECORDING"
	StatusError     Status = "ERROR"
)

// SessionInfo contains information about the current recording session
type SessionInfo struct {
	SongName     string    `json:"song_name"`
	StartTime    time.Time `json:"start_time"`
	OutputFile   string    `json:"output_file"`
	ChannelCount int       `json:"channel_count"`
	ChannelNames []string  `json:"channel_names"`
}

// Recorder defines the interface that all audio recorders must implement
type Recorder interface {
	StartReady(songName string) error
	StartRecording() error
	CancelReady() error
	Stop() error

	// Status and information
	GetStatus() (Status, *SessionInfo)
	GetChannelStatus() map[string]string

	// Cleanup
	Cleanup() error
}

