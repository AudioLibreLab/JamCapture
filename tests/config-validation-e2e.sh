#!/bin/bash

set -euo pipefail

# Configuration validation E2E tests for ID/name system
DIR="$(dirname "$0")"
TEMP_DIR="/tmp/jamcapture-config-tests"
mkdir -p "$TEMP_DIR"

echo "--- 🧪 Starting Configuration Validation E2E Tests ---"

# Test counter
PASSED=0
FAILED=0

# Test helper functions
test_config() {
    local test_name="$1"
    local config_content="$2"
    local expected_result="$3"  # "success" or "fail"
    local expected_error="${4:-}"

    echo "🔍 Testing: $test_name"

    local config_file="$TEMP_DIR/test-$test_name.yaml"
    echo "$config_content" > "$config_file"

    # Try to load the config using the info command
    local output
    local exit_code
    if output=$(timeout 5s "$DIR/../jamcapture" --config "$config_file" info test_song 2>&1); then
        exit_code=0
    else
        exit_code=$?
    fi

    if [ "$expected_result" = "success" ]; then
        if [ $exit_code -eq 0 ]; then
            echo "✅ PASS: $test_name"
            ((PASSED++))
        else
            echo "❌ FAIL: $test_name (expected success but got error)"
            echo "   Output: $output"
            ((FAILED++))
        fi
    else
        if [ $exit_code -ne 0 ]; then
            if [ -n "$expected_error" ] && echo "$output" | grep -q "$expected_error"; then
                echo "✅ PASS: $test_name (correctly failed with expected error)"
                ((PASSED++))
            elif [ -z "$expected_error" ]; then
                echo "✅ PASS: $test_name (correctly failed)"
                ((PASSED++))
            else
                echo "❌ FAIL: $test_name (failed but with wrong error message)"
                echo "   Expected: $expected_error"
                echo "   Got: $output"
                ((FAILED++))
            fi
        else
            echo "❌ FAIL: $test_name (expected failure but succeeded)"
            ((FAILED++))
        fi
    fi
}

# Test 1: Valid config with name fallback
test_config "name_fallback" '
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
        # No name provided - should fallback to guitar_input
    output:
      directory: /tmp
' "success"

# Test 2: Valid config with name override
test_config "name_override" '
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
' "success"

# Test 3: Duplicate names in same config should fail
test_config "duplicate_names" '
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
        name: "guitar"  # Duplicate name
    output:
      directory: /tmp
' "fail" "already used by"

# Test 4: Duplicate IDs in definitions should fail
test_config "duplicate_ids" '
active_config: test

definitions:
  channels:
    - id: duplicate_id
      type: input
      sources: ["system:capture_1"]
      audiomode: mono
      volume: 2.0
      delay: 0
    - id: duplicate_id
      type: input
      sources: ["system:capture_2"]
      audiomode: mono
      volume: 2.0
      delay: 0

configs:
  test:
    channels:
      - ref: duplicate_id
    output:
      directory: /tmp
' "fail" "duplicate ID"

# Test 5: Channel reuse with different names should work
test_config "channel_reuse" '
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
' "success"

# Test 6: Missing reference should fail
test_config "missing_reference" '
active_config: test

definitions:
  channels:
    - id: existing_channel
      type: input
      sources: ["system:capture_1"]
      audiomode: mono
      volume: 2.0
      delay: 0

configs:
  test:
    channels:
      - ref: nonexistent_channel
    output:
      directory: /tmp
' "fail" "not found in definitions"

# Test 7: Empty ref should fail
test_config "empty_ref" '
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
      - ref: ""
    output:
      directory: /tmp
' "fail" "required"

echo ""
echo "--- 📊 Test Results ---"
echo "✅ Passed: $PASSED"
echo "❌ Failed: $FAILED"
echo "Total: $((PASSED + FAILED))"

# Cleanup
rm -rf "$TEMP_DIR"

if [ $FAILED -eq 0 ]; then
    echo "🎉 All configuration validation tests passed!"
    exit 0
else
    echo "💥 Some tests failed!"
    exit 1
fi