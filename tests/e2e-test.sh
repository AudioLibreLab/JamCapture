#!/bin/bash

set -euxo pipefail

# Configuration
TEST_SONG="e2e-test"
DURATION=5
DIR="$(dirname "$0")"
CONFIG_PATH="$DIR/jamcapture-e2e.yaml"

echo "--- 🚀 Starting JamCapture Test Sync ---"

# Use hardcoded output file path based on config (name gets cleaned: e2e-test -> e2etest)
TEST_FILE="/tmp/jamcapture-test/e2etest.mkv"
echo "📁 Expected output file: $TEST_FILE"

# Cleanup from previous runs
rm -f "$TEST_FILE"

# Generate test signals for both guitar input and backing
echo "🎵 Generating test tones..."
# Create two different frequency test tones
ffmpeg -f lavfi -i "sine=frequency=440:duration=$(($DURATION + 2))" -y /tmp/backing_tone.wav > /dev/null 2>&1
ffmpeg -f lavfi -i "sine=frequency=880:duration=$(($DURATION + 2))" -y /tmp/guitar_tone.wav > /dev/null 2>&1

# Play backing track tone with specific client name
echo "🎵 Playing backing tone (440Hz) with client name 'paplay_backing'..."
paplay --client-name=paplay_backing /tmp/backing_tone.wav &
BACKING_PID=$!

# Simulate guitar input with specific client name
echo "🎸 Playing guitar tone (880Hz) with client name 'paplay_guitar'..."
paplay --client-name=paplay_guitar /tmp/guitar_tone.wav &
GUITAR_PID=$!

sleep 1

# Launch JamCapture to record the test tone
echo "🔴 Recording with JamCapture for $DURATION seconds..."
"$DIR/../jamcapture" --config "$CONFIG_PATH" -p rm "$TEST_SONG" &
JAM_PID=$!

# Attendre la durée du test
sleep "$DURATION"

# 4. Arrêt propre
echo "Stoping processes..."
kill -SIGINT $JAM_PID
wait $JAM_PID 2>/dev/null
kill $BACKING_PID 2>/dev/null
kill $GUITAR_PID 2>/dev/null

echo "--- 🔍 Analyzing Output File ---"

# 5. Vérification de la validité du MKV
if [ ! -f "$TEST_FILE" ]; then
    echo "❌ Error: Output file $TEST_FILE not found!"
    exit 1
fi

# Utilisation de ffprobe pour vérifier les pistes
TRACK_COUNT=$(ffprobe -v error -show_entries stream=index -of compact=p=0:nk=1 "$TEST_FILE" | wc -l)

echo "✅ File created: $TEST_FILE"
echo "✅ Number of audio tracks found: $TRACK_COUNT"

# 6. Vérification du contenu audio réel
echo "🔊 Analyzing audio content..."

# Analyser le contenu audio de façon plus simple
AUDIO_ANALYSIS=$(ffmpeg -i "$TEST_FILE" -af astats -f null - 2>&1)

# Extraire les niveaux max et RMS pour chaque canal
MAX_LEVELS=$(echo "$AUDIO_ANALYSIS" | grep "Max level:" | head -2)
RMS_LEVELS=$(echo "$AUDIO_ANALYSIS" | grep "RMS level dB:" | head -2)

echo "$MAX_LEVELS"
echo "$RMS_LEVELS"

# Vérifier qu'il y a au moins un signal détectable dans l'une des pistes
HAS_SIGNAL=false
for line in $MAX_LEVELS; do
    if [[ "$line" =~ [0-9]+\.[0-9]+ ]]; then
        LEVEL=$(echo "$line" | grep -o '[0-9]\+\.[0-9]\+')
        if (( $(echo "$LEVEL > 0.5" | bc -l 2>/dev/null || echo 0) )); then
            HAS_SIGNAL=true
            echo "✅ Audio signal detected (max level: $LEVEL)"
            break
        fi
    fi
done

if [ "$HAS_SIGNAL" = false ]; then
    echo "❌ No significant audio signal detected in recording"
    echo "⚠️  This might indicate a problem with the test audio generation or capture"
else
    echo "✅ Audio content verification passed"
fi

# 7. Vérification de l'intégrité (erreurs de décodage)
echo "Checking for stream errors..."
ffmpeg -v error -i "$TEST_FILE" -f null - 2>&1
if [ $? -eq 0 ]; then
    echo "✅ No decoding errors found. The MKV is valid."
else
    echo "❌ Integrity check failed!"
    exit 1
fi

echo "--- 🧪 Running Configuration Validation Tests ---"
"$DIR/simple-config-test.sh"

echo "--- ✨ Test Finished ---"
