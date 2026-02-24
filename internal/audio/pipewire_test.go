package audio

import (
	"fmt"
	"strings"
	"testing"
)

// TestableValidatePort tests ValidatePort logic without external commands
func TestableValidatePort(portName string, allPorts []string) error {
	if portName == "" || portName == "disabled" {
		return nil
	}

	// Check if port exists
	portExists := false
	for _, port := range allPorts {
		if port == portName {
			portExists = true
			break
		}
	}

	if !portExists {
		return fmt.Errorf("port not found: %s", portName)
	}

	// Check for duplicates using the same logic as PipeWire.findPortDuplicates
	pw := &PipeWire{}
	duplicates := pw.findPortDuplicatesInList(portName, allPorts)

	if len(duplicates) > 1 {
		return fmt.Errorf("duplicate sources detected for '%s': %v. Please close conflicting applications", portName, duplicates)
	}

	return nil
}

func TestValidatePort_Success(t *testing.T) {
	mockPorts := []string{"Chrome:output_FL", "system:capture_1"}

	err := TestableValidatePort("system:capture_1", mockPorts)
	if err != nil {
		t.Errorf("Expected no error for valid single port, got: %v", err)
	}
}

func TestValidatePort_NotFound(t *testing.T) {
	mockPorts := []string{"Chrome:output_FL"}

	err := TestableValidatePort("nonexistent:port", mockPorts)
	if err == nil {
		t.Error("Expected error for nonexistent port")
	}
	if !strings.Contains(err.Error(), "port not found") {
		t.Errorf("Expected 'port not found' error, got: %v", err)
	}
}

func TestValidatePort_DuplicateDetection(t *testing.T) {
	mockPorts := []string{
		"Chrome:output_FL",
		"Chrome:output_FL", // True duplicate - same name appears twice
		"Chrome-2:output_FL", // Different instance - NOT a duplicate
	}

	err := TestableValidatePort("Chrome:output_FL", mockPorts)
	if err == nil {
		t.Error("Expected error for duplicate sources")
	}
	if !strings.Contains(err.Error(), "duplicate sources detected") {
		t.Errorf("Expected 'duplicate sources detected' error, got: %v", err)
	}
}

func TestValidatePort_EmptyAndDisabled(t *testing.T) {
	// Test empty string
	err := TestableValidatePort("", []string{})
	if err != nil {
		t.Errorf("Expected no error for empty string, got: %v", err)
	}

	// Test "disabled"
	err = TestableValidatePort("disabled", []string{})
	if err != nil {
		t.Errorf("Expected no error for 'disabled', got: %v", err)
	}
}

func TestFindPortDuplicates_Chrome(t *testing.T) {
	pw := &PipeWire{}
	mockPorts := []string{
		"Chrome:output_FL",
		"Chrome:output_FL", // True duplicate - same name
		"Chrome-2:output_FL", // Different instance - NOT a duplicate
		"Firefox:output_FL",
		"system:capture_1",
	}

	duplicates := pw.findPortDuplicatesInList("Chrome:output_FL", mockPorts)

	expectedCount := 2 // Only the two identical "Chrome:output_FL" entries
	if len(duplicates) != expectedCount {
		t.Errorf("Expected %d duplicates, got %d: %v", expectedCount, len(duplicates), duplicates)
	}

	// Check that both identical entries are found
	for _, duplicate := range duplicates {
		if duplicate != "Chrome:output_FL" {
			t.Errorf("Expected only 'Chrome:output_FL', got: %s", duplicate)
		}
	}
}

func TestFindPortDuplicates_Firefox(t *testing.T) {
	pw := &PipeWire{}
	mockPorts := []string{
		"Firefox:output_FL",
		"Firefox:output_FL", // True duplicate - same name
		"Firefox (1):output_FL", // Different instance - NOT a duplicate
		"Chrome:output_FL",
	}

	duplicates := pw.findPortDuplicatesInList("Firefox:output_FL", mockPorts)

	expectedCount := 2 // Only the two identical "Firefox:output_FL" entries
	if len(duplicates) != expectedCount {
		t.Errorf("Expected %d duplicates, got %d: %v", expectedCount, len(duplicates), duplicates)
	}
}

func TestFindPortDuplicates_NoDuplicates(t *testing.T) {
	pw := &PipeWire{}
	mockPorts := []string{
		"Chrome:output_FL",
		"Firefox:output_FL",
		"system:capture_1",
	}

	duplicates := pw.findPortDuplicatesInList("Chrome:output_FL", mockPorts)

	expectedCount := 1
	if len(duplicates) != expectedCount {
		t.Errorf("Expected %d duplicate (itself), got %d: %v", expectedCount, len(duplicates), duplicates)
	}

	if duplicates[0] != "Chrome:output_FL" {
		t.Errorf("Expected Chrome:output_FL, got: %s", duplicates[0])
	}
}

func TestValidatePort_DifferentChromeInstances_NotDuplicates(t *testing.T) {
	// Test that Chrome and Chrome-2 are NOT considered duplicates
	mockPorts := []string{
		"Chrome:output_FL",
		"Chrome-2:output_FL",
		"Chrome-3:output_FL",
	}

	// Chrome:output_FL should validate successfully (no duplicates)
	err := TestableValidatePort("Chrome:output_FL", mockPorts)
	if err != nil {
		t.Errorf("Expected no error for Chrome:output_FL with different Chrome instances, got: %v", err)
	}

	// Chrome-2:output_FL should also validate successfully
	err = TestableValidatePort("Chrome-2:output_FL", mockPorts)
	if err != nil {
		t.Errorf("Expected no error for Chrome-2:output_FL with different Chrome instances, got: %v", err)
	}
}

