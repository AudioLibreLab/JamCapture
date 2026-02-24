<div align="center">
  <img src="images/logo.png" alt="JamCapture Logo" width="200" height="200">
  <h1>JamCapture</h1>
  <p><em>A professional audio recording and mixing tool for musicians, with both CLI and web interfaces.</em></p>
</div>

## Features

- **Multi-channel Recording**: Capture guitar, microphone, and system audio simultaneously via JACK/PipeWire
- **Smart Mixing**: Combine tracks with volume control and latency compensation
- **Web Interface**: Mobile-optimized responsive UI for smartphone control during recording
- **Profile System**: YAML-based configuration with inheritance and multiple recording setups
- **Real-time Monitoring**: Live status updates, audio source detection, and log streaming
- **File Management**: Built-in audio player, file browser, and backing track conversion

## Quick Start

```bash
# Build the application
go build

# Test your setup
./jamcapture sources

# Start web interface (recommended)
./jamcapture serve --port 8080
# Open http://your-ip:8080 on your smartphone

# Or use CLI for quick recording
./jamcapture --config examples/jamcapture.yaml -p rmp "my-song"
```

## Configuration

Configuration is stored in `~/.config/jamcapture.yaml`:

```yaml
audio:
  sample_rate: 48000
  channels: 2

record:
  guitar_input: "alsa_input.usb-Focusrite_Scarlett_2i2_USB_Y814JK8264026F-00.analog-stereo"
  monitor_input: ""  # Auto-detect if empty

mix:
  guitar_volume: 4.0
  backing_volume: 0.8
  delay_ms: 200  # Bluetooth compensation delay

output:
  directory: "~/Audio/JamCapture"
  format: "flac"
```

### Configuration Templates

Example configurations are provided in `examples/`:

- **`jamcapture.yaml`**: Complete configuration with multiple profiles
- **Profile-based setup**: Inherit from default, override specific settings
- **Multi-channel support**: Guitar, microphone, and system audio routing

## Usage

### Simplified Syntax (Recommended)

Use `-p` to specify pipeline steps directly on song name:

```bash
# Record, mix, and play in one command
jamcapture -p rmp "My Song"

# Mix and play with custom delay
jamcapture -p mp -d 180 "My Song"

# Just mix with custom settings
jamcapture -p m -g 3.0 -b 0.6 -d 150 "My Song"

# Record and mix only (no playback)
jamcapture -p rm "My Song"
```

Pipeline steps:
- `r` = record
- `m` = mix
- `p` = play

### Traditional Commands (Still Available)

```bash
# List available audio sources
jamcapture sources

# Individual commands
jamcapture record "My Song"
jamcapture mix "My Song" -d 150
jamcapture play "My Song"

# With pipeline chaining
jamcapture -p mp mix "My Song"
```

### Configuration Management

```bash
# View current configuration
jamcapture config show

# Edit configuration (opens in $EDITOR)
jamcapture config edit
```

## Bluetooth Latency Compensation

When using Bluetooth speakers/headphones:

1. You hear the backing track with ~200ms delay
2. You naturally play guitar late to match what you hear
3. In the recording, guitar is behind the backing track
4. The `delay_ms` setting delays the backing track in the mix to sync with your guitar

Common Bluetooth delays:
- **Standard A2DP**: 180-250ms
- **aptX Low Latency**: 40-80ms
- **aptX Standard**: 100-150ms

## Examples

```bash
# Quick jam session with full pipeline
jamcapture -p rmp "blues_jam"

# Re-mix existing recording with different settings
jamcapture -p m -d 180 -g 3.5 "blues_jam"

# Test different delay values with immediate playback
jamcapture -p mp -d 150 "blues_jam"
jamcapture -p mp -d 200 "blues_jam"
jamcapture -p mp -d 250 "blues_jam"
```

## Web Interface (Recommended)

### Smartphone Control

Control JamCapture from your smartphone - perfect for recording while playing guitar:

```bash
# Start web server
./jamcapture serve --port 8080

# With custom configuration
./jamcapture --config examples/jamcapture.yaml serve
```

**Access**: Use the displayed network URL (e.g., `http://192.168.1.15:8080`) on your smartphone

### Key Features

- **Large Touch Controls**: Start/stop recording with guitar-friendly buttons
- **Real-time Status**: Live recording progress and audio source monitoring
- **Profile Selection**: Switch between recording setups (solo, band, podcast)
- **Auto-mix**: Automatically generate mixed FLAC files after recording
- **File Browser**: Stream, download, and manage your recordings
- **Backing Tracks**: Upload and play along with backing tracks
- **Mobile-optimized**: Dark/light theme, responsive design

**→ [Complete Web Server Guide](docs/web-server-guide.md)**

## Configuration Profiles

JamCapture supports multiple configuration profiles with inheritance:

```yaml
active_config: "guitar"
configs:
  default:
    audio:
      sample_rate: 48000
    channels:
      - name: "guitar"
        source: "system:capture_1"
        type: "input"
      - name: "monitor_left"
        source: "system:monitor_FL"
        type: "monitor"
    output:
      directory: "~/Audio/JamCapture"
      format: "flac"

  guitar:
    # Inherits from default, overrides specific settings
    mix:
      volumes:
        guitar: 2.0
        monitor: 0.8

  full-band:
    # Multi-channel setup for band practice
    channels:
      - name: "guitar"
        source: "system:capture_1"
        type: "input"
      - name: "bass"
        source: "system:capture_2"
        type: "input"
      - name: "monitor_left"
        source: "system:monitor_FL"
        type: "monitor"
      - name: "monitor_right"
        source: "system:monitor_FR"
        type: "monitor"
```

Use profiles with:
```bash
# CLI
./jamcapture --profile guitar record "my-song"

# Web interface automatically shows all available profiles
```

## Files

- Input recordings: `~/Audio/JamCapture/{song}.mkv`
- Mixed output: `~/Audio/JamCapture/{song}.flac`
- Configuration: `~/.config/jamcapture.yaml` or `examples/jamcapture.yaml`

## Requirements

- **FFmpeg**: For audio recording and mixing
- **PipeWire with JACK support**: `pw-jack` command must be available
- **Modern web browser**: For mobile interface (Chrome, Firefox, Safari)
- **VLC or other audio player**: For playback (optional)