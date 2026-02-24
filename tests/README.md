<div align="center">
  <img src="../images/logo.png" alt="JamCapture Logo" width="100" height="100">
  <h1>JamCapture Tests</h1>
</div>

## End-to-End Test

### Files

- `e2e-test.sh` - Main end-to-end test script
- `jamcapture-e2e.yaml` - Test configuration for e2e testing

### Running the Test

```bash
# From project root
./tests/e2e-test.sh
```

### Test Process

The e2e test validates the complete JamCapture pipeline:

1. **Audio Generation**: Creates test tones (440Hz backing, 880Hz guitar)
2. **Named Audio Sources**:
   - `paplay_backing` - Simulates backing track
   - `paplay_guitar` - Simulates guitar input
3. **Recording**: Records both sources using dedicated test config
4. **Mixing**: Automatically mixes the recorded tracks
5. **Validation**: Verifies file integrity and audio content

### Test Configuration

The test uses `jamcapture-e2e.yaml` which:
- Routes `paplay_guitar:output_FL` as guitar input
- Routes `paplay_backing:output_FL/FR` as stereo backing monitors
- Outputs to `/tmp/jamcapture-test/`
- Uses FLAC format for validation

### Expected Output

```
/tmp/jamcapture-test/
├── e2etest.mkv  # Raw recording (2 audio tracks)
└── e2etest.flac # Mixed final output
```

### Usage from Project Root

```bash
# Build first
go build

# Run e2e test
./tests/e2e-test.sh
```