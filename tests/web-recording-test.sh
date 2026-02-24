#!/bin/bash
# Web Interface Recording Test
# Tests the WAIT state behavior and recording workflow

set -euo pipefail

# Configuration
TEST_PORT="8098"
TEST_CONFIG="tests/jamcapture-e2e.yaml"
SERVER_PID=""

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

cleanup() {
    echo -e "\n${YELLOW}🧹 Cleaning up...${NC}"

    # Stop recording if active
    curl -s -X POST "http://localhost:$TEST_PORT/stop" >/dev/null 2>&1 || true

    # Kill server
    if [ -n "$SERVER_PID" ]; then
        kill "$SERVER_PID" 2>/dev/null || true
        sleep 2
        kill -9 "$SERVER_PID" 2>/dev/null || true
    fi

    # Clean up background processes
    pkill -f "jamcapture.*serve.*port.*$TEST_PORT" 2>/dev/null || true
    pkill -f "paplay.*client-name.*test_chrome" 2>/dev/null || true

    # Clean up test files
    rm -f /tmp/test_chrome_*.wav
}

trap cleanup EXIT INT TERM

log_info() { echo -e "${BLUE}ℹ️  $1${NC}"; }
log_success() { echo -e "${GREEN}✅ $1${NC}"; }
log_error() { echo -e "${RED}❌ $1${NC}"; }
log_warning() { echo -e "${YELLOW}⚠️  $1${NC}"; }

# Start server
start_server() {
    log_info "Starting web server for recording tests..."

    go build || exit 1
    ./jamcapture --config "$TEST_CONFIG" serve --port "$TEST_PORT" &
    SERVER_PID=$!

    # Wait for server
    local attempts=0
    while [ $attempts -lt 30 ]; do
        if curl -s -f "http://localhost:$TEST_PORT/" >/dev/null 2>&1; then
            log_success "Server ready on port $TEST_PORT"
            return 0
        fi
        sleep 1
        attempts=$((attempts + 1))
    done

    log_error "Server failed to start"
    exit 1
}

# Test the WAIT state behavior
test_wait_state() {
    log_info "Testing WAIT state behavior..."

    echo -e "\n${BLUE}📋 Step 1: Check initial sources status${NC}"
    local sources_response
    sources_response=$(curl -s "http://localhost:$TEST_PORT/sources")
    echo "Sources: $sources_response"

    # Count available vs unavailable sources
    local available_count unavailable_count
    available_count=$(echo "$sources_response" | jq '[.sources[] | select(.status=="available")] | length' 2>/dev/null || echo "0")
    unavailable_count=$(echo "$sources_response" | jq '[.sources[] | select(.status=="unavailable")] | length' 2>/dev/null || echo "0")

    log_info "Available sources: $available_count"
    log_info "Unavailable sources: $unavailable_count"

    if [ "$unavailable_count" -eq 0 ]; then
        log_warning "All sources are available - WAIT state won't be triggered"
        log_info "This means Chrome (or equivalent) is already running"
        return 0
    fi

    echo -e "\n${BLUE}📋 Step 2: Start recording (should enter WAIT state)${NC}"

    # Start recording
    local record_response
    record_response=$(curl -s -X POST "http://localhost:$TEST_PORT/record" \
        -H "Content-Type: application/x-www-form-urlencoded" \
        -d "song=test-wait-state&profile=e2e&auto_mix=false")

    echo "Record response: $record_response"

    if echo "$record_response" | jq -e '.success' >/dev/null 2>&1; then
        log_success "Recording started successfully"
    else
        log_error "Failed to start recording"
        echo "$record_response"
        return 1
    fi

    echo -e "\n${BLUE}📋 Step 3: Monitor status during WAIT${NC}"

    # Monitor status for up to 15 seconds
    local wait_time=0
    local max_wait=15
    local found_recording=false

    while [ $wait_time -lt $max_wait ]; do
        local status_response
        status_response=$(curl -s "http://localhost:$TEST_PORT/status")
        local current_status
        current_status=$(echo "$status_response" | jq -r '.status' 2>/dev/null || echo "UNKNOWN")

        log_info "Status at ${wait_time}s: $current_status"

        if [ "$current_status" = "RECORDING" ]; then
            found_recording=true
            log_success "Status transitioned to RECORDING"
            break
        elif [ "$current_status" = "IDLE" ]; then
            log_warning "Status returned to IDLE (possible timeout or error)"
            break
        fi

        sleep 2
        wait_time=$((wait_time + 2))
    done

    echo -e "\n${BLUE}📋 Step 4: Simulate Chrome startup${NC}"

    if [ "$found_recording" = false ]; then
        log_info "Recording hasn't started yet - simulating Chrome startup"

        # Create and play Chrome audio to simulate Chrome starting
        ffmpeg -f lavfi -i "sine=frequency=1000:duration=5" -y /tmp/test_chrome_audio.wav >/dev/null 2>&1
        paplay --client-name="Google Chrome" /tmp/test_chrome_audio.wav &

        log_info "Chrome audio simulation started"
        sleep 3

        # Check if this triggers transition to RECORDING
        local post_chrome_status
        post_chrome_status=$(curl -s "http://localhost:$TEST_PORT/status" | jq -r '.status' 2>/dev/null)
        log_info "Status after Chrome simulation: $post_chrome_status"

        if [ "$post_chrome_status" = "RECORDING" ]; then
            log_success "Successfully transitioned to RECORDING after Chrome startup"
        else
            log_warning "Still not recording - this may be expected if other sources are missing"
        fi
    fi

    echo -e "\n${BLUE}📋 Step 5: Stop recording${NC}"

    # Stop recording
    local stop_response
    stop_response=$(curl -s -X POST "http://localhost:$TEST_PORT/stop")
    echo "Stop response: $stop_response"

    if echo "$stop_response" | jq -e '.success' >/dev/null 2>&1; then
        log_success "Recording stopped successfully"
    else
        log_info "Stop response (may be normal if not recording): $stop_response"
    fi

    sleep 2

    # Final status check
    local final_status
    final_status=$(curl -s "http://localhost:$TEST_PORT/status" | jq -r '.status' 2>/dev/null)
    log_info "Final status: $final_status"

    if [ "$final_status" = "IDLE" ]; then
        log_success "Successfully returned to IDLE state"
    fi
}

# Test sources endpoint during different states
test_sources_during_states() {
    log_info "Testing sources endpoint behavior..."

    echo -e "\n${BLUE}📋 Testing sources API responses${NC}"

    # Test initial state
    local initial_sources
    initial_sources=$(curl -s "http://localhost:$TEST_PORT/sources")

    if echo "$initial_sources" | jq -e '.sources' >/dev/null 2>&1; then
        log_success "Sources API returns valid JSON"

        local source_count
        source_count=$(echo "$initial_sources" | jq '.sources | length' 2>/dev/null)
        log_info "Found $source_count configured sources"

        # Show each source
        echo "$initial_sources" | jq -r '.sources[] | "  - \(.name): \(.status) (\(.source))"' 2>/dev/null || true
    else
        log_error "Sources API returned invalid JSON"
        echo "$initial_sources"
    fi
}

# Test error handling
test_error_handling() {
    log_info "Testing error handling..."

    # Test invalid record request
    local invalid_response
    invalid_response=$(curl -s -X POST "http://localhost:$TEST_PORT/record" \
        -H "Content-Type: application/x-www-form-urlencoded" \
        -d "song=&profile=" || echo "failed")

    if echo "$invalid_response" | grep -q "error\|failed"; then
        log_success "Invalid record request properly rejected"
    else
        log_warning "Invalid record request handling unclear"
    fi

    # Test invalid endpoint
    local not_found
    not_found=$(curl -s -w "%{http_code}" "http://localhost:$TEST_PORT/nonexistent" | tail -c 3)

    if [ "$not_found" = "404" ]; then
        log_success "404 handling works correctly"
    else
        log_warning "404 handling may not be working (got: $not_found)"
    fi
}

# Main test execution
main() {
    echo -e "${BLUE}🎬 Web Recording Test Suite${NC}"
    echo -e "${BLUE}============================${NC}\n"

    # Check prerequisites
    if ! command -v jq >/dev/null; then
        log_warning "jq not found - some tests will be limited"
    fi

    # Start server
    start_server

    # Run tests
    test_sources_during_states
    test_wait_state
    test_error_handling

    echo -e "\n${GREEN}🎉 Web recording tests completed${NC}"

    log_info "Manual testing suggestions:"
    echo "  1. Open http://localhost:$TEST_PORT in browser"
    echo "  2. Check 'Audio Sources Status' section"
    echo "  3. Try recording without Chrome running"
    echo "  4. Start Chrome with audio and observe transition"

    log_info "The server will remain running for manual testing..."
    log_info "Press Ctrl+C to stop"

    # Keep server running for manual testing
    wait $SERVER_PID || true
}

# Run if called directly
if [ "${BASH_SOURCE[0]}" = "${0}" ]; then
    main "$@"
fi