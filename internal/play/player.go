package play

import (
	"fmt"
	"github.com/audiolibrelab/jamcapture/internal/config"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

type Player struct {
	cfg *config.Config
}

func New(cfg *config.Config) *Player {
	return &Player{cfg: cfg}
}

func (p *Player) Play(songName string) error {
	cleanName := p.cleanFileName(songName)
	audioFile := filepath.Join(p.cfg.Output.Directory, cleanName+"."+p.cfg.Output.Format)

	// Check if file exists
	if _, err := os.Stat(audioFile); err != nil {
		return fmt.Errorf("audio file not found: %s", audioFile)
	}

	fmt.Printf("Playing: %s\n", audioFile)

	// Try to find available audio player
	player, err := p.findAudioPlayer()
	if err != nil {
		return fmt.Errorf("no suitable audio player found: %w", err)
	}

	var cmd *exec.Cmd
	switch player {
	case "vlc":
		cmd = exec.Command("vlc", "--play-and-exit", audioFile)
	case "mpv":
		cmd = exec.Command("mpv", "--no-video", audioFile)
	case "ffplay":
		cmd = exec.Command("ffplay", "-nodisp", "-autoexit", audioFile)
	case "aplay":
		// aplay only works with WAV files, so we need to convert first
		if p.cfg.Output.Format != "wav" {
			return fmt.Errorf("aplay requires WAV format, current format is %s", p.cfg.Output.Format)
		}
		cmd = exec.Command("aplay", audioFile)
	default:
		return fmt.Errorf("unsupported player: %s", player)
	}

	// Run the player
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("playback failed with %s: %w", player, err)
	}

	fmt.Println("Playback completed")
	return nil
}

func (p *Player) findAudioPlayer() (string, error) {
	// List of preferred audio players in order of preference
	players := []string{"vlc", "mpv", "ffplay", "aplay"}

	for _, player := range players {
		if _, err := exec.LookPath(player); err == nil {
			return player, nil
		}
	}

	return "", fmt.Errorf("no audio player found (tried: %s)", strings.Join(players, ", "))
}

func (p *Player) cleanFileName(name string) string {
	// Remove special characters and replace spaces with underscores
	// Allows: letters, numbers, spaces, hyphens, underscores
	var result strings.Builder
	for _, r := range name {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == ' ' || r == '-' || r == '_' {
			result.WriteRune(r)
		}
	}
	return strings.ReplaceAll(strings.TrimSpace(result.String()), " ", "_")
}