// Package web bundles the static pages of the web UI into the binary, so
// JamCapture serves them whatever its working directory (systemd unit,
// launcher, `go install`ed binary…).
package web

import "embed"

// Static holds the pages under static/.
//
//go:embed static/*.html
var Static embed.FS
