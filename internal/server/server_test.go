package server

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/audiolibrelab/jamcapture/internal/config"
	"github.com/audiolibrelab/jamcapture/internal/service"
)

func newTestServer(t *testing.T) *Server {
	t.Helper()
	cfg := &config.Config{
		Output: config.OutputConfig{
			Directory: t.TempDir(),
			Format:    "flac",
		},
	}
	return &Server{
		service: service.New(cfg, "", nil),
		cfg:     cfg,
	}
}

func TestIsPathWithinDir(t *testing.T) {
	tests := []struct {
		name string
		path string
		dir  string
		want bool
	}{
		{"file directly inside dir", "/tmp/jc/Output/song.flac", "/tmp/jc/Output", true},
		{"dir itself", "/tmp/jc/Output", "/tmp/jc/Output", true},
		{"nested subdirectory", "/tmp/jc/Output/sub/song.flac", "/tmp/jc/Output", true},
		{"parent directory", "/tmp/jc", "/tmp/jc/Output", false},
		{"sibling directory sharing name prefix", "/tmp/jc/OutputEvil/song.flac", "/tmp/jc/Output", false},
		{"unrelated path", "/etc/passwd", "/tmp/jc/Output", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isPathWithinDir(tt.path, tt.dir); got != tt.want {
				t.Errorf("isPathWithinDir(%q, %q) = %v, want %v", tt.path, tt.dir, got, tt.want)
			}
		})
	}
}

func TestHandleFileStreamRejectsPathTraversal(t *testing.T) {
	srv := newTestServer(t)

	req := httptest.NewRequest(http.MethodGet, "/api/files/stream/..%2F..%2Fetc%2Fpasswd", nil)
	rr := httptest.NewRecorder()

	srv.handleFileStream(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected status %d, got %d: %s", http.StatusBadRequest, rr.Code, rr.Body.String())
	}
}

func TestHandleFileDeleteRejectsPathTraversal(t *testing.T) {
	srv := newTestServer(t)

	req := httptest.NewRequest(http.MethodDelete, "/api/files/delete/..%2F..%2Fetc%2Fpasswd", nil)
	rr := httptest.NewRecorder()

	srv.handleFileDelete(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected status %d, got %d: %s", http.StatusBadRequest, rr.Code, rr.Body.String())
	}
}

func TestHandleRecordingStreamRejectsPathTraversal(t *testing.T) {
	srv := newTestServer(t)

	req := httptest.NewRequest(http.MethodGet, "/api/recording/..%2F..%2Fetc%2Fpasswd", nil)
	rr := httptest.NewRecorder()

	srv.handleRecordingStream(rr, req)

	if rr.Code != http.StatusForbidden {
		t.Errorf("expected status %d, got %d: %s", http.StatusForbidden, rr.Code, rr.Body.String())
	}
}

func TestHandleRecordingStreamServesFileWithinDirectory(t *testing.T) {
	srv := newTestServer(t)

	if err := os.WriteFile(filepath.Join(srv.cfg.Output.Directory, "song.flac"), []byte("data"), 0644); err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/recording/song.flac", nil)
	rr := httptest.NewRecorder()

	srv.handleRecordingStream(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d: %s", http.StatusOK, rr.Code, rr.Body.String())
	}
	if got := rr.Body.String(); got != "data" {
		t.Errorf("expected body %q, got %q", "data", got)
	}
}

func TestHandleMixStreamRejectsPathTraversal(t *testing.T) {
	srv := newTestServer(t)

	req := httptest.NewRequest(http.MethodGet, "/api/mix/stream/..%2F..%2Fetc%2Fpasswd", nil)
	rr := httptest.NewRecorder()

	srv.handleMixStream(rr, req)

	if rr.Code != http.StatusForbidden {
		t.Errorf("expected status %d, got %d: %s", http.StatusForbidden, rr.Code, rr.Body.String())
	}
}

func TestHandleBackingtrackStreamNewRejectsPathTraversal(t *testing.T) {
	srv := newTestServer(t)

	req := httptest.NewRequest(http.MethodGet, "/api/backingtracks/stream/..%2F..%2Fetc%2Fpasswd", nil)
	rr := httptest.NewRecorder()

	srv.handleBackingtrackStreamNew(rr, req)

	if rr.Code != http.StatusForbidden {
		t.Errorf("expected status %d, got %d: %s", http.StatusForbidden, rr.Code, rr.Body.String())
	}
}
