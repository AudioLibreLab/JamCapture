//go:build nosystray

package systray

import "github.com/audiolibrelab/jamcapture/internal/service"

type SystemTray struct{}

func New(svc service.Service, webPort int) *SystemTray {
	return &SystemTray{}
}

func (st *SystemTray) Run() {}

func (st *SystemTray) Shutdown() {}

func IsSupported() bool        { return false }
func IsSupportedVerbose() bool { return false }

func GetIcon(status string) []byte { return nil }
