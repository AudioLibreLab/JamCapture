# Contributing to JamCapture

Thanks for your interest! JamCapture is in early beta — feedback and contributions are very welcome.

## Reporting Bugs

Open an issue with:
- Your Linux distribution and PipeWire version (`pipewire --version`)
- Your audio interface (e.g. Focusrite Scarlett 2i2)
- The command you ran and the full error output
- The contents of `~/.config/jamcapture.yaml` (remove sensitive paths if needed)

## Feature Requests

Open an issue describing your use case. Since this is a niche tool, explaining your recording setup helps a lot.

## Building from Source

```bash
# Prerequisites: Go 1.21+, PipeWire, FFmpeg, pw-jack
git clone https://github.com/AudioLibreLab/JamCapture.git
cd JamCapture
go build
```

## Running Tests

```bash
# Unit tests (no audio hardware needed)
go test ./...

# End-to-end test (requires PipeWire + FFmpeg running)
./tests/e2e-test.sh
```

## Submitting a Pull Request

1. Fork the repo and create a branch from `main`
2. Make your changes — keep them focused (one PR per concern)
3. Run `go test ./...` and `go build` — both must pass
4. Open a PR with a short description of what and why

## Architecture

The codebase uses a layered service architecture:

```
cmd/           → CLI entry points
internal/
  service/     → unified business logic (used by both CLI and web)
  server/      → HTTP server and web UI
  audio/       → PipeWire/JACK recording
  mix/         → FFmpeg mixing
  config/      → YAML config loading and validation
  play/        → audio playback
```

Start with `internal/service/service.go` to understand the main flow.

## Code Style

Standard Go conventions apply. Run `go vet ./...` before submitting.
