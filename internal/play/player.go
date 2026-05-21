package play

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/audiolibrelab/jamcapture/internal/config"
)

type Player struct {
	cfg *config.Config
}

func New(cfg *config.Config) *Player {
	return &Player{cfg: cfg}
}

// nonBlockingPlayers open files in a GUI application and should not be waited on.
var nonBlockingPlayers = map[string]bool{
	"audacity": true,
	"xdg-open": true,
	"vlc":      true,
}

func (p *Player) Play(songName string) error {
	cleanName := p.cleanFileName(songName)
	audioFile := filepath.Join(p.cfg.Output.Directory, cleanName+"."+p.cfg.Output.Format)

	if _, err := os.Stat(audioFile); err != nil {
		return fmt.Errorf("audio file not found: %s", audioFile)
	}

	fmt.Printf("Playing: %s\n", audioFile)

	player, err := p.resolvePlayer()
	if err != nil {
		return err
	}

	cmd := p.buildCommand(player, audioFile)
	if cmd == nil {
		return fmt.Errorf("unsupported player: %s", player)
	}

	if nonBlockingPlayers[player] {
		// GUI editors/openers: launch and return immediately
		if err := cmd.Start(); err != nil {
			return fmt.Errorf("failed to open %s with %s: %w", audioFile, player, err)
		}
		return nil
	}

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("playback failed with %s: %w", player, err)
	}
	fmt.Println("Playback completed")
	return nil
}

// resolvePlayer returns the configured player if set and available, else falls back to auto-detect.
func (p *Player) resolvePlayer() (string, error) {
	if p.cfg.AudioPlayer != "" {
		if _, err := exec.LookPath(p.cfg.AudioPlayer); err == nil {
			return p.cfg.AudioPlayer, nil
		}
		return "", fmt.Errorf("configured audio player %q not found in PATH", p.cfg.AudioPlayer)
	}
	return p.autoDetectPlayer()
}

// autoDetectPlayer probes a fallback list when no player is configured.
func (p *Player) autoDetectPlayer() (string, error) {
	candidates := []string{"audacity", "vlc", "mpv", "ffplay", "aplay"}
	for _, player := range candidates {
		if _, err := exec.LookPath(player); err == nil {
			return player, nil
		}
	}
	return "", fmt.Errorf("no audio player found (tried: %s)", strings.Join(candidates, ", "))
}

// buildCommand constructs the exec.Cmd for the given player and file.
func (p *Player) buildCommand(player, audioFile string) *exec.Cmd {
	switch player {
	case "audacity":
		return exec.Command("audacity", audioFile)
	case "xdg-open":
		return exec.Command("xdg-open", audioFile)
	case "vlc":
		return exec.Command("vlc", audioFile)
	case "mpv":
		return exec.Command("mpv", "--no-video", audioFile)
	case "ffplay":
		return exec.Command("ffplay", "-nodisp", "-autoexit", audioFile)
	case "aplay":
		return exec.Command("aplay", audioFile)
	default:
		return nil
	}
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