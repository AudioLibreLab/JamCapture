<div align="center">
  <img src="../images/logo.png" alt="JamCapture Logo" width="150" height="150">
  <h1>JamCapture Web Server Guide</h1>
  <p><strong>The complete guide to JamCapture's web interface for smartphone-controlled recording</strong></p>
</div>

## Quick Start

Launch the web server to control JamCapture from your smartphone:

```bash
# Start with default configuration
./jamcapture serve

# Start with specific config and port
./jamcapture --config examples/jamcapture.yaml serve --port 8080

# Both 'serve' and 'server' commands work
./jamcapture server --port 3000
```

**Network Access**: The server displays URLs for both local and network access:

```
INFO Starting JamCapture Web Server port=8080
     local_url=http://192.168.1.15:8080
     localhost_url=http://localhost:8080
```

## Mobile Access Setup

### 1. Connect to Same Network
Ensure your smartphone and computer are on the same WiFi network.

### 2. Access Web Interface
Open your smartphone browser and navigate to the URL displayed by the server:
- **Network access**: `http://192.168.1.15:8080` (use the IP shown in your terminal)
- **Local access**: `http://localhost:8080` (from the same computer)

### 3. Mobile-First Design
Optimized for use while playing instruments:
- **Extra-large touch controls** (60px+ height) for easy access while holding a guitar
- **Vintage tape recorder styling** with authentic START/STOP buttons
- **One-handed operation** with logical control placement
- **Real-time updates** via HTMX (no page reloads)
- **Smart state management** with visual feedback for all recording states

## Web Interface Features

### Control Panel
- **Status Display**: Real-time state indicator (STANDBY/READY/RECORDING/ERROR)
- **Recording Controls**:
  - **READY**: Large blue button to prepare recording (validates audio sources)
  - **START**: Activated automatically when audio is detected in READY state
  - **STOP**: Large red button to end recording (with optional auto-mix)
  - **CANCEL**: Exit READY state without recording
- **Configuration Panel**:
  - **Profile Selector**: Dropdown with all available configurations
  - **Song Name Input**: Text field for recording name
  - **Auto-mix Toggle**: Automatic FLAC generation after recording
  - **Profile Locking**: Prevents changes during active sessions

### Real-time Information
- **Session Monitor**: Live recording details (song name, duration, channels, file size)
- **Audio Source Status**: Real-time validation of JACK ports and connections
- **Configuration Inspector**: Detailed view of active profile with inheritance
  - 🔵 **Profile-specific** settings and overrides
  - 🔶 **Inherited** values from base configuration
- **System Logs**: Live FFmpeg output, connection status, and debug information
- **File Browser**: Stream and download recordings directly from the interface

### Mobile Optimizations
- **Responsive design** adapts from mobile to desktop
- **Dark mode support** via Pico.css
- **Auto-updating status** every 2 seconds
- **Expandable sections** for detailed information

### Advanced Features

#### Auto-Mix Processing
- **Smart Workflow**: Automatically generates mixed FLAC after recording
- **Dual Output**: Raw multi-track MKV + final mixed FLAC
- **Profile Integration**: Uses volumes and delays from active configuration
- **Progress Tracking**: Real-time mixing status in system logs

#### Audio Management
- **File Browser**: Browse, stream, and download all recordings
- **Audio Player**: HTML5 player with support for FLAC, WAV, MP3
- **Backing Tracks**: Upload, manage, and play along with backing audio
- **Format Support**: Handles MKV (multi-track), FLAC, WAV, MP3 formats

#### Profile System
- **Dynamic Loading**: Switch profiles without restarting server
- **Session Locking**: Prevents profile changes during recording
- **Inheritance Visualization**: Clear display of inherited vs. profile-specific settings
- **Validation**: Real-time audio source checking for each profile

## REST API Reference

Complete HTTP API for custom integrations and external tools:

### Recording Control

**Prepare Recording (STANDBY → READY)**
```bash
# Enter READY state with source validation
curl -X POST http://localhost:8080/ready \
  -d "song=my-awesome-song" \
  -d "profile=studio" \
  -d "auto_mix=true"
```

**Stop Recording**
```bash
curl -X POST http://localhost:8080/stop
```

**Cancel Ready State**
```bash
curl -X POST http://localhost:8080/cancel
```

### Status and Configuration

**Get System Status**
```bash
curl http://localhost:8080/status
```

**List Available Profiles**
```bash
curl http://localhost:8080/config/profiles
```

**Change Active Profile**
```bash
curl -X POST http://localhost:8080/config/select \
  -d "profile=guitar-solo"
```

**Audio Source Status**
```bash
curl http://localhost:8080/sources
```

**Status Response Structure**:
```json
{
  "status": "READY|RECORDING|STANDBY|ERROR",
  "message": "Human-readable status description",
  "session": {
    "song_name": "my-song",
    "start_time": "2026-02-23T10:00:00Z",
    "output_file": "/path/to/output.mkv",
    "channel_count": 4,
    "channel_names": ["guitar", "mic", "monitor_left", "monitor_right"]
  },
  "resolved_config": {
    "active_profile": "studio",
    "output_dir": "/home/user/Audio/JamCapture",
    "auto_mix": true,
    "channels": [...],
    "sample_rate": 48000
  },
  "active_profile": "studio"
}
```

### File Management API

**List Recordings**
```bash
curl http://localhost:8080/api/files
```

**Stream Audio File**
```bash
# Direct streaming URL
http://localhost:8080/api/files/stream/my-song.flac
```

**Latest Recording**
```bash
curl http://localhost:8080/api/latest-recording
```

**Backing Tracks**
```bash
# List backing tracks
curl http://localhost:8080/api/backingtracks

# Select backing track
curl -X POST http://localhost:8080/api/backingtracks/select \
  -H "Content-Type: application/json" \
  -d '{"name": "backing-track-name"}'
```

## Configuration

### Server Options
```bash
./jamcapture serve --help
```

Available flags:
- `--port string`: Port for web server (default "8080")
- `--config string`: Configuration file path
- `--profile string`: Default profile to use
- `--verbose int`: Logging verbosity (0-3)

### Profile Management
The web interface automatically loads available profiles from your configuration file. Example configuration structure:

```yaml
active_config: "default"
configs:
  default:
    audio:
      sample_rate: 48000
    channels:
      - name: "guitar"
        source: "system:capture_1"
        type: "input"
    output:
      directory: "~/Audio/JamCapture"
      format: "flac"

  guitar:
    # Inherits from default, overrides specific settings
    mix:
      volumes:
        guitar: 2.0
```

## Troubleshooting

### Network Connection Issues

**Problem**: Cannot access from smartphone
**Solutions**:
1. Check firewall settings - ensure port 8080 is open
2. Verify both devices are on same network
3. Use the exact IP address shown in terminal output
4. Try temporarily disabling firewall for testing

### Audio Issues

**Problem**: Recording fails or no audio captured
**Solutions**:
1. Ensure PipeWire-JACK is running: `pw-jack jack_lsp`
2. Check JACK port connections with verbose logging: `--verbose 1`
3. Validate configuration: `./jamcapture info test-song`
4. Test CLI recording first: `./jamcapture record test-song`

### Performance Issues

**Problem**: Web interface feels slow
**Solutions**:
1. Check network quality between devices
2. Reduce logging verbosity if using `--verbose`
3. Use local browser for testing: `http://localhost:8080`

## Advanced Usage

### Custom Integration
Build custom applications using the HTTP API:
- Mobile apps can integrate with the REST endpoints
- Home automation systems can trigger recordings
- External monitoring tools can poll status

### Multiple Profiles
Create different profiles for various recording scenarios:
- **guitar**: Solo guitar with high input gain
- **full-band**: Multi-channel setup for band practice
- **podcast**: Optimized for voice recording

### Development Mode
For development or testing:
```bash
# Run with verbose logging and custom port
./jamcapture serve --verbose 2 --port 3000
```

## Security Considerations

- The web server is designed for **local network use only**
- No authentication is implemented - suitable for trusted networks
- Consider firewall rules if exposing to wider networks
- All communication is over HTTP (not HTTPS) for simplicity

## Next Steps

1. **Test the interface** on your smartphone while connected to your guitar
2. **Create custom profiles** for different recording scenarios
3. **Bookmark the URL** on your mobile device for quick access
4. **Explore the API** for custom integrations

The web server maintains full compatibility with the CLI - you can use both interfaces interchangeably based on your workflow preferences.