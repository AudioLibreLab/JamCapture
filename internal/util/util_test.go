package util

import "testing"

func TestCleanFileName(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{"simple", "MySong", "MySong"},
		{"spaces become underscores", "My Song Name", "My_Song_Name"},
		{"strips special characters", "My/Song:Name!", "MySongName"},
		{"trims surrounding whitespace", "  Song  ", "Song"},
		{"keeps hyphens and underscores", "My-Song_Name", "My-Song_Name"},
		{"path traversal stripped", "../../etc/passwd", "etcpasswd"},
		{"empty string", "", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := CleanFileName(tt.in); got != tt.want {
				t.Errorf("CleanFileName(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

func TestFormatBytes(t *testing.T) {
	tests := []struct {
		bytes int64
		want  string
	}{
		{0, "0 B"},
		{512, "512 B"},
		{1023, "1023 B"},
		{1024, "1.0 KB"},
		{1536, "1.5 KB"},
		{1024 * 1024, "1.0 MB"},
		{1024 * 1024 * 1024, "1.0 GB"},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			if got := FormatBytes(tt.bytes); got != tt.want {
				t.Errorf("FormatBytes(%d) = %q, want %q", tt.bytes, got, tt.want)
			}
		})
	}
}
