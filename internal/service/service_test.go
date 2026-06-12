package service

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/audiolibrelab/jamcapture/internal/config"
)

func newTestConfig(t *testing.T) *config.Config {
	t.Helper()
	return &config.Config{
		Output: config.OutputConfig{
			Directory: t.TempDir(),
			Format:    "flac",
		},
	}
}

func TestValidateSafeFilename(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{"simple name", "song.mkv", false},
		{"with spaces and dashes", "My Song-1.flac", false},
		{"parent dir traversal", "../etc/passwd", true},
		{"nested traversal", "foo/../../etc/passwd", true},
		{"forward slash", "sub/song.flac", true},
		{"backslash", "sub\\song.flac", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateSafeFilename(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateSafeFilename(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
			}
		})
	}
}

func TestSetSelectedBackingtrack(t *testing.T) {
	cfg := newTestConfig(t)
	svc := New(cfg, "", nil)

	backingDir := cfg.BackingtracksDir()
	if err := os.MkdirAll(backingDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(backingDir, "track.flac"), []byte("data"), 0644); err != nil {
		t.Fatal(err)
	}

	if err := svc.SetSelectedBackingtrack("track.flac"); err != nil {
		t.Fatalf("SetSelectedBackingtrack failed: %v", err)
	}

	selected, err := svc.GetSelectedBackingtrack()
	if err != nil {
		t.Fatalf("GetSelectedBackingtrack failed: %v", err)
	}
	if selected == nil || selected.Name != "track.flac" {
		t.Errorf("expected track.flac to be selected, got %+v", selected)
	}
}

func TestSetSelectedBackingtrackRejectsPathTraversal(t *testing.T) {
	cfg := newTestConfig(t)
	svc := New(cfg, "", nil)

	if err := svc.SetSelectedBackingtrack("../evil.flac"); err == nil {
		t.Error("expected error for path traversal filename, got nil")
	}
}

func TestConvertRecordingToBackingtrack(t *testing.T) {
	cfg := newTestConfig(t)
	svc := New(cfg, "", nil)

	recordingPath := filepath.Join(cfg.Output.Directory, "session.mkv")
	if err := os.WriteFile(recordingPath, []byte("data"), 0644); err != nil {
		t.Fatal(err)
	}

	if err := svc.ConvertRecordingToBackingtrack("session.mkv"); err != nil {
		t.Fatalf("ConvertRecordingToBackingtrack failed: %v", err)
	}

	destPath := filepath.Join(cfg.BackingtracksDir(), "session.mkv")
	if _, err := os.Stat(destPath); err != nil {
		t.Errorf("expected converted file at %s: %v", destPath, err)
	}
	if _, err := os.Stat(recordingPath); !os.IsNotExist(err) {
		t.Errorf("expected source recording to be moved away, stat err: %v", err)
	}

	// .mkv is not a "supported" backing track extension, so it won't show up
	// via ListBackingtracks/GetSelectedBackingtrack, but it should still be
	// recorded as the selection in conf.yaml.
	jcs := svc.(*JamCaptureService)
	selectedName, err := jcs.getSelectedBackingtrackName()
	if err != nil {
		t.Fatalf("getSelectedBackingtrackName failed: %v", err)
	}
	if selectedName != "session.mkv" {
		t.Errorf("expected session.mkv to be recorded as selected, got %q", selectedName)
	}
}

func TestConvertRecordingToBackingtrackRejectsPathTraversal(t *testing.T) {
	cfg := newTestConfig(t)
	svc := New(cfg, "", nil)

	if err := svc.ConvertRecordingToBackingtrack("../../etc/passwd"); err == nil {
		t.Error("expected error for path traversal recording name, got nil")
	}
}

func TestLatestAudioFilePrefersPriorityExtension(t *testing.T) {
	dir := t.TempDir()

	if err := os.WriteFile(filepath.Join(dir, "old.mkv"), []byte("mkv"), 0644); err != nil {
		t.Fatal(err)
	}
	time.Sleep(10 * time.Millisecond)
	if err := os.WriteFile(filepath.Join(dir, "new.flac"), []byte("flac"), 0644); err != nil {
		t.Fatal(err)
	}

	got, err := latestAudioFile(dir)
	if err != nil {
		t.Fatalf("latestAudioFile failed: %v", err)
	}
	if filepath.Base(got) != "new.flac" {
		t.Errorf("expected new.flac (higher priority extension), got %s", got)
	}
}

func TestLatestAudioFilePrefersNewerWithinSamePriority(t *testing.T) {
	dir := t.TempDir()

	if err := os.WriteFile(filepath.Join(dir, "a.flac"), []byte("a"), 0644); err != nil {
		t.Fatal(err)
	}
	time.Sleep(10 * time.Millisecond)
	if err := os.WriteFile(filepath.Join(dir, "b.flac"), []byte("b"), 0644); err != nil {
		t.Fatal(err)
	}

	got, err := latestAudioFile(dir)
	if err != nil {
		t.Fatalf("latestAudioFile failed: %v", err)
	}
	if filepath.Base(got) != "b.flac" {
		t.Errorf("expected b.flac (most recently modified), got %s", got)
	}
}

func TestLatestAudioFileNoFiles(t *testing.T) {
	dir := t.TempDir()
	if _, err := latestAudioFile(dir); err == nil {
		t.Error("expected error for directory with no audio files")
	}
}

func TestGetLatestMixedFile(t *testing.T) {
	cfg := newTestConfig(t)
	svc := New(cfg, "", nil)

	if err := os.WriteFile(filepath.Join(cfg.Output.Directory, "song.flac"), []byte("data"), 0644); err != nil {
		t.Fatal(err)
	}

	if got := svc.GetLatestMixedFile(); got != "song.flac" {
		t.Errorf("GetLatestMixedFile() = %q, want %q", got, "song.flac")
	}
}

func TestAnalyzeMKVFile(t *testing.T) {
	if _, err := exec.LookPath("ffprobe"); err != nil {
		t.Skip("ffprobe not available")
	}
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		t.Skip("ffmpeg not available")
	}

	cfg := newTestConfig(t)
	svc := New(cfg, "", nil)

	mkvPath := filepath.Join(cfg.Output.Directory, "session.mkv")
	cmd := exec.Command("ffmpeg",
		"-hide_banner", "-loglevel", "error",
		"-f", "lavfi", "-i", "anullsrc=r=48000:cl=mono",
		"-t", "0.1",
		"-c:a", "flac",
		"-metadata:s:a:0", "title=guitar",
		"-y", mkvPath,
	)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("ffmpeg failed: %v\n%s", err, out)
	}

	analysis, err := svc.AnalyzeMKVFile("session.mkv")
	if err != nil {
		t.Fatalf("AnalyzeMKVFile failed: %v", err)
	}

	if analysis.TrackCount != 1 {
		t.Fatalf("expected 1 track, got %d", analysis.TrackCount)
	}
	if analysis.Tracks[0].Title != "guitar" {
		t.Errorf("expected track title 'guitar', got %q", analysis.Tracks[0].Title)
	}
	if analysis.Tracks[0].Channels != 1 {
		t.Errorf("expected 1 channel, got %d", analysis.Tracks[0].Channels)
	}
}

func TestAnalyzeMKVFileRejectsPathTraversal(t *testing.T) {
	cfg := newTestConfig(t)
	svc := New(cfg, "", nil)

	if _, err := svc.AnalyzeMKVFile("../../etc/passwd"); err == nil {
		t.Error("expected error for path traversal filename, got nil")
	}
}
