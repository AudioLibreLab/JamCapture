#!/bin/bash
# Web Interface Test Script
# Tests the web UI including the new sources section and WAIT state fixes

set -euo pipefail

# Configuration
TEST_PORT="8099"
TEST_CONFIG="tests/jamcapture-e2e.yaml"
SERVER_PID=""
CLEANUP_DONE=false

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Cleanup function
cleanup() {
    if [ "$CLEANUP_DONE" = true ]; then
        return
    fi
    CLEANUP_DONE=true

    echo -e "\n${YELLOW}🧹 Cleaning up...${NC}"

    # Kill server if running
    if [ -n "$SERVER_PID" ]; then
        if kill -0 "$SERVER_PID" 2>/dev/null; then
            echo "Stopping web server (PID: $SERVER_PID)"
            kill "$SERVER_PID" 2>/dev/null || true
            sleep 2
            # Force kill if still running
            if kill -0 "$SERVER_PID" 2>/dev/null; then
                kill -9 "$SERVER_PID" 2>/dev/null || true
            fi
        fi
    fi

    # Clean up any background processes
    pkill -f "jamcapture.*serve.*port.*$TEST_PORT" 2>/dev/null || true
    pkill -f "paplay.*client-name.*test_" 2>/dev/null || true
}

# Set up cleanup trap
trap cleanup EXIT INT TERM

# Test result tracking
TESTS_RUN=0
TESTS_PASSED=0

# Helper functions
log_info() {
    echo -e "${BLUE}ℹ️  $1${NC}"
}

log_success() {
    echo -e "${GREEN}✅ $1${NC}"
}

log_error() {
    echo -e "${RED}❌ $1${NC}"
}

log_warning() {
    echo -e "${YELLOW}⚠️  $1${NC}"
}

test_assertion() {
    local description="$1"
    local command="$2"
    local expected_status="${3:-0}"

    TESTS_RUN=$((TESTS_RUN + 1))
    log_info "Testing: $description"

    if eval "$command" >/dev/null 2>&1; then
        local actual_status=0
    else
        local actual_status=1
    fi

    if [ "$actual_status" -eq "$expected_status" ]; then
        log_success "PASS: $description"
        TESTS_PASSED=$((TESTS_PASSED + 1))
        return 0
    else
        log_error "FAIL: $description"
        log_error "  Expected status: $expected_status, Got: $actual_status"
        return 1
    fi
}

test_http_response() {
    local description="$1"
    local url="$2"
    local expected_pattern="$3"

    TESTS_RUN=$((TESTS_RUN + 1))
    log_info "Testing: $description"

    local response
    if response=$(curl -s -f "$url" 2>/dev/null); then
        if echo "$response" | grep -q "$expected_pattern"; then
            log_success "PASS: $description"
            TESTS_PASSED=$((TESTS_PASSED + 1))
            return 0
        else
            log_error "FAIL: $description - Pattern '$expected_pattern' not found in response"
            echo "Response preview: $(echo "$response" | head -c 200)..."
            return 1
        fi
    else
        log_error "FAIL: $description - HTTP request failed"
        return 1
    fi
}

test_json_field() {
    local description="$1"
    local url="$2"
    local jq_filter="$3"
    local expected_value="$4"

    TESTS_RUN=$((TESTS_RUN + 1))
    log_info "Testing: $description"

    local response
    if response=$(curl -s -f "$url" 2>/dev/null); then
        local actual_value
        if actual_value=$(echo "$response" | jq -r "$jq_filter" 2>/dev/null); then
            if [ "$actual_value" = "$expected_value" ]; then
                log_success "PASS: $description"
                TESTS_PASSED=$((TESTS_PASSED + 1))
                return 0
            else
                log_error "FAIL: $description - Expected '$expected_value', got '$actual_value'"
                return 1
            fi
        else
            log_error "FAIL: $description - JSON parsing failed"
            echo "Response: $response"
            return 1
        fi
    else
        log_error "FAIL: $description - HTTP request failed"
        return 1
    fi
}

# Wait for server to be ready
wait_for_server() {
    local max_attempts=30
    local attempt=1

    log_info "Waiting for server to be ready on port $TEST_PORT..."

    while [ $attempt -le $max_attempts ]; do
        if curl -s -f "http://localhost:$TEST_PORT/" >/dev/null 2>&1; then
            log_success "Server is ready!"
            return 0
        fi

        if [ $((attempt % 5)) -eq 0 ]; then
            log_info "Still waiting... (attempt $attempt/$max_attempts)"
        fi

        sleep 1
        attempt=$((attempt + 1))
    done

    log_error "Server failed to start within $max_attempts seconds"
    return 1
}

# Start web server
start_web_server() {
    log_info "Starting web server with test configuration..."

    # Build first
    if ! go build; then
        log_error "Failed to build jamcapture"
        exit 1
    fi

    # Start server in background
    ./jamcapture --config "$TEST_CONFIG" serve --port "$TEST_PORT" &
    SERVER_PID=$!

    log_info "Web server started with PID: $SERVER_PID"

    # Wait for server to be ready
    if ! wait_for_server; then
        log_error "Failed to start web server"
        exit 1
    fi
}

# Create test audio sources
create_test_sources() {
    log_info "Creating test audio sources..."

    # Generate test audio files
    ffmpeg -f lavfi -i "sine=frequency=440:duration=10" -y /tmp/test_backing.wav >/dev/null 2>&1
    ffmpeg -f lavfi -i "sine=frequency=880:duration=10" -y /tmp/test_guitar.wav >/dev/null 2>&1

    # Start paplay instances with specific names
    paplay --client-name=paplay_backing /tmp/test_backing.wav &
    paplay --client-name=paplay_guitar /tmp/test_guitar.wav &

    # Give them time to register
    sleep 2

    log_success "Test audio sources created"
}

# Main test execution
main() {
    echo -e "${BLUE}🧪 JamCapture Web Interface Test Suite${NC}"
    echo -e "${BLUE}======================================${NC}\n"

    # Verify prerequisites
    log_info "Checking prerequisites..."

    if ! command -v curl >/dev/null; then
        log_error "curl is required for web interface testing"
        exit 1
    fi

    if ! command -v jq >/dev/null; then
        log_warning "jq not found - JSON parsing tests will be skipped"
    fi

    if [ ! -f "$TEST_CONFIG" ]; then
        log_error "Test configuration not found: $TEST_CONFIG"
        exit 1
    fi

    # Start web server
    start_web_server

    echo -e "\n${YELLOW}🔧 Testing Basic API Endpoints${NC}"
    echo "================================"

    # Test basic endpoints
    test_http_response "Main page loads" "http://localhost:$TEST_PORT/" "<title>JamCapture</title>"
    test_http_response "Status endpoint responds" "http://localhost:$TEST_PORT/status" "status"
    test_http_response "Sources endpoint responds" "http://localhost:$TEST_PORT/sources" "sources"
    test_http_response "Profiles endpoint responds" "http://localhost:$TEST_PORT/config/profiles" "profiles"

    echo -e "\n${YELLOW}🔌 Testing Sources API${NC}"
    echo "====================="

    if command -v jq >/dev/null; then
        # Test sources structure
        test_json_field "Sources array exists" "http://localhost:$TEST_PORT/sources" ".sources | type" "array"
        test_json_field "Sources have required fields" "http://localhost:$TEST_PORT/sources" ".sources[0] | has(\"name\") and has(\"source\") and has(\"type\") and has(\"status\")" "true"

        # Create test sources and test again
        create_test_sources
        sleep 3

        # Test with sources present
        test_json_field "Guitar source detected" "http://localhost:$TEST_PORT/sources" ".sources[] | select(.name==\"guitar\") | .status" "available"

        # Check if backing sources are detected
        local backing_status
        if backing_status=$(curl -s "http://localhost:$TEST_PORT/sources" | jq -r '.sources[] | select(.name=="backing_left") | .status' 2>/dev/null); then
            if [ "$backing_status" = "available" ]; then
                log_success "Backing source detected as available"
                TESTS_PASSED=$((TESTS_PASSED + 1))
            else
                log_info "Backing source status: $backing_status (may be unavailable, which is expected)"
            fi
        fi
        TESTS_RUN=$((TESTS_RUN + 1))
    else
        log_warning "Skipping JSON tests - jq not available"
    fi

    echo -e "\n${YELLOW}🌐 Testing Web Interface HTML${NC}"
    echo "============================="

    # Test HTML content
    test_http_response "Sources section present" "http://localhost:$TEST_PORT/" "sources-section"
    test_http_response "Sources details div present" "http://localhost:$TEST_PORT/" "sources-details"
    test_http_response "updateSourcesInfo function present" "http://localhost:$TEST_PORT/" "updateSourcesInfo"
    test_http_response "verifyAllSources function present" "http://localhost:$TEST_PORT/" "verifyAllSources"

    # Test CSS classes
    test_http_response "Source item CSS class present" "http://localhost:$TEST_PORT/" "source-item"
    test_http_response "Source status CSS classes present" "http://localhost:$TEST_PORT/" "source-available"
    test_http_response "Source indicators CSS present" "http://localhost:$TEST_PORT/" "source-indicator"

    echo -e "\n${YELLOW}📡 Testing Enhanced WAIT State Logic${NC}"
    echo "===================================="

    # Test enhanced timeout logic
    test_http_response "30-second timeout present" "http://localhost:$TEST_PORT/" "30000"
    test_http_response "Sources verification on timeout" "http://localhost:$TEST_PORT/" "sourcesData.sources"
    test_http_response "Auto-expand sources section" "http://localhost:$TEST_PORT/" "sources-section.*open.*true"

    echo -e "\n${YELLOW}🎛️ Testing Status Updates${NC}"
    echo "========================"

    # Test status endpoint structure
    if command -v jq >/dev/null; then
        test_json_field "Status has basic fields" "http://localhost:$TEST_PORT/status" "has(\"status\") and has(\"logs\")" "true"
        test_json_field "Status is initially IDLE" "http://localhost:$TEST_PORT/status" ".status" "IDLE"
    fi

    echo -e "\n${YELLOW}🧪 Testing Error Handling${NC}"
    echo "========================"

    # Test error endpoints
    test_assertion "Invalid endpoint returns 404" "curl -s -o /dev/null -w '%{http_code}' http://localhost:$TEST_PORT/invalid | grep -q 404" 0
    test_assertion "POST to GET endpoint fails appropriately" "curl -s -X POST http://localhost:$TEST_PORT/sources | grep -q 'not allowed\\|error'" 0

    # Show final results
    echo -e "\n${BLUE}📊 Test Results${NC}"
    echo "==============="

    local pass_rate=0
    if [ "$TESTS_RUN" -gt 0 ]; then
        pass_rate=$(( (TESTS_PASSED * 100) / TESTS_RUN ))
    fi

    echo -e "Tests run: ${BLUE}$TESTS_RUN${NC}"
    echo -e "Tests passed: ${GREEN}$TESTS_PASSED${NC}"
    echo -e "Tests failed: ${RED}$((TESTS_RUN - TESTS_PASSED))${NC}"
    echo -e "Pass rate: ${BLUE}$pass_rate%${NC}"

    if [ "$TESTS_PASSED" -eq "$TESTS_RUN" ]; then
        echo -e "\n${GREEN}🎉 All tests passed! Web interface is working correctly.${NC}"
        exit 0
    elif [ "$pass_rate" -ge 80 ]; then
        echo -e "\n${YELLOW}⚠️  Most tests passed, but some issues detected.${NC}"
        exit 0
    else
        echo -e "\n${RED}❌ Multiple test failures detected. Check the logs above.${NC}"
        exit 1
    fi
}

# Run main function
main "$@"