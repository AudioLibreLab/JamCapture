#!/bin/bash

# Simple configuration test script
DIR="$(dirname "$0")"
TEMP_DIR="/tmp/jamcapture-simple-test"
mkdir -p "$TEMP_DIR"

echo "🧪 Testing configuration ID/name features..."

# Test 1: Name fallback
echo "1. Testing name fallback (ID -> name)"
cat > "$TEMP_DIR/fallback.yaml" << 'EOF'
active_config: test
definitions:
  channels:
    - id: guitar_input
      type: input
      sources: ["system:capture_1"]
      audiomode: mono
      volume: 2.0
      delay: 0
configs:
  test:
    channels:
      - ref: guitar_input
    output:
      directory: /tmp
EOF

if "$DIR/../jamcapture" --config "$TEMP_DIR/fallback.yaml" info test_song >/dev/null 2>&1; then
    echo "✅ Name fallback test passed"
else
    echo "❌ Name fallback test failed"
fi

# Test 2: Name override
echo "2. Testing name override"
cat > "$TEMP_DIR/override.yaml" << 'EOF'
active_config: test
definitions:
  channels:
    - id: guitar_input
      type: input
      sources: ["system:capture_1"]
      audiomode: mono
      volume: 2.0
      delay: 0
configs:
  test:
    channels:
      - ref: guitar_input
        name: "Lead Guitar"
    output:
      directory: /tmp
EOF

if "$DIR/../jamcapture" --config "$TEMP_DIR/override.yaml" info test_song >/dev/null 2>&1; then
    echo "✅ Name override test passed"
else
    echo "❌ Name override test failed"
fi

# Test 3: Duplicate names (should fail)
echo "3. Testing duplicate names detection"
cat > "$TEMP_DIR/duplicate.yaml" << 'EOF'
active_config: test
definitions:
  channels:
    - id: guitar_input
      type: input
      sources: ["system:capture_1"]
      audiomode: mono
      volume: 2.0
      delay: 0
    - id: bass_input
      type: input
      sources: ["system:capture_2"]
      audiomode: mono
      volume: 2.0
      delay: 0
configs:
  test:
    channels:
      - ref: guitar_input
        name: "guitar"
      - ref: bass_input
        name: "guitar"
    output:
      directory: /tmp
EOF

if "$DIR/../jamcapture" --config "$TEMP_DIR/duplicate.yaml" info test_song >/dev/null 2>&1; then
    echo "❌ Duplicate names test failed (should have been rejected)"
else
    echo "✅ Duplicate names test passed (correctly rejected)"
fi

# Test 4: Channel reuse with different names
echo "4. Testing channel reuse with different names"
cat > "$TEMP_DIR/reuse.yaml" << 'EOF'
active_config: test
definitions:
  channels:
    - id: guitar_input
      type: input
      sources: ["system:capture_1"]
      audiomode: mono
      volume: 2.0
      delay: 0
configs:
  test:
    channels:
      - ref: guitar_input
        name: "Guitar Lead"
        volume: 3.0
      - ref: guitar_input
        name: "Guitar Rhythm"
        volume: 2.0
        delay: 100
    output:
      directory: /tmp
EOF

if "$DIR/../jamcapture" --config "$TEMP_DIR/reuse.yaml" info test_song >/dev/null 2>&1; then
    echo "✅ Channel reuse test passed"
else
    echo "❌ Channel reuse test failed"
fi

echo "🎉 Configuration tests completed!"

# Cleanup
rm -rf "$TEMP_DIR"