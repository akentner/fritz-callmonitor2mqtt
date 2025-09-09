package mqtt

import (
	"testing"
	"time"

	"fritz-callmonitor2mqtt/pkg/types"
)

func TestNewClient(t *testing.T) {
	client := NewClient(
		"localhost",
		1883,
		"user",
		"pass",
		"test-client",
		"test/topic",
		1,
		true,
		60*time.Second,
		30*time.Second,
	)

	if client.broker != "localhost" {
		t.Errorf("Expected broker 'localhost', got %s", client.broker)
	}

	if client.port != 1883 {
		t.Errorf("Expected port 1883, got %d", client.port)
	}

	if client.clientID != "test-client" {
		t.Errorf("Expected clientID 'test-client', got %s", client.clientID)
	}

	if client.connected {
		t.Error("Expected client to be disconnected initially")
	}
}

func TestLineStatusManagement(t *testing.T) {
	client := NewClient(
		"localhost", 1883, "", "", "test", "test", 1, true,
		60*time.Second, 30*time.Second,
	)

	// Create test event
	event := types.CallEvent{
		Timestamp: time.Now(),
		Type:      types.CallTypeRing,
		Line:      1,
		Extension: "1",
		Trunk:     "SIP0",
		Caller:    "123456789",
		Called:    "987654321",
	}

	// Test line status creation
	lineKey := "SIP0_1"
	status := client.getOrCreateLineStatus(lineKey, event)

	if status.Line != 1 {
		t.Errorf("Expected Line '1', got %d", status.Line)
	}

	if status.Extension != "1" {
		t.Errorf("Expected Extension '1', got %s", status.Extension)
	}

	if status.Status != types.CallStatusIdle {
		t.Errorf("Expected initial status idle, got %s", status.Status)
	}

	// Test call history
	client.callHistory.AddCall(event)

	if len(client.callHistory.Calls) != 1 {
		t.Errorf("Expected 1 call in history, got %d", len(client.callHistory.Calls))
	}

	if client.callHistory.Calls[0].Line != 1 {
		t.Errorf("Expected call ID '1', got %d", client.callHistory.Calls[0].Line)
	}
}

func TestCallHistoryLimit(t *testing.T) {
	client := NewClient(
		"localhost", 1883, "", "", "test", "test", 1, true,
		60*time.Second, 30*time.Second,
	)

	// Set smaller history size for testing
	client.callHistory.MaxSize = 3

	// Add more calls than the limit
	for i := 1; i <= 5; i++ {
		event := types.CallEvent{
			Timestamp: time.Now(),
			Type:      types.CallTypeRing,
			Line:      i,
			Extension: "1",
			Trunk:     "SIP0",
		}
		client.callHistory.AddCall(event)
	}

	// Should only keep the last 3 calls
	if len(client.callHistory.Calls) != 3 {
		t.Errorf("Expected 3 calls in history, got %d", len(client.callHistory.Calls))
	}

	// Should be in reverse order (newest first)
	if client.callHistory.Calls[0].Line != 5 {
		t.Errorf("Expected newest call ID '5', got %d", client.callHistory.Calls[0].Line)
	}

	if client.callHistory.Calls[2].Line != 3 {
		t.Errorf("Expected oldest call ID '3', got %d", client.callHistory.Calls[2].Line)
	}
}

func TestIsConnected(t *testing.T) {
	client := NewClient(
		"localhost", 1883, "", "", "test", "test", 1, true,
		60*time.Second, 30*time.Second,
	)

	if client.IsConnected() {
		t.Error("Expected client to be disconnected initially")
	}

	// Simulate connection
	client.connected = true

	if !client.IsConnected() {
		t.Error("Expected client to be connected")
	}
}
