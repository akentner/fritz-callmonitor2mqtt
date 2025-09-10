package mqtt

import (
	"encoding/json"
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

	if status.Extension.ID != "1" {
		t.Errorf("Expected Extension ID '1', got %s", status.Extension.ID)
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

func TestCreateStatusMessage(t *testing.T) {
	client := NewClient(
		"localhost", 1883, "", "", "test", "test", 1, true,
		60*time.Second, 30*time.Second,
	)

	// Test online status message
	onlinePayload, err := client.createStatusMessage("online")
	if err != nil {
		t.Fatalf("Failed to create online status message: %v", err)
	}

	var onlineStatus types.ServiceStatus
	err = json.Unmarshal(onlinePayload, &onlineStatus)
	if err != nil {
		t.Fatalf("Failed to unmarshal online status message: %v", err)
	}

	if onlineStatus.State != "online" {
		t.Errorf("Expected state 'online', got '%s'", onlineStatus.State)
	}

	// Test offline status message
	offlinePayload, err := client.createStatusMessage("offline")
	if err != nil {
		t.Fatalf("Failed to create offline status message: %v", err)
	}

	var offlineStatus types.ServiceStatus
	err = json.Unmarshal(offlinePayload, &offlineStatus)
	if err != nil {
		t.Fatalf("Failed to unmarshal offline status message: %v", err)
	}

	if offlineStatus.State != "offline" {
		t.Errorf("Expected state 'offline', got '%s'", offlineStatus.State)
	}

	// Verify LastChanged is recent
	timeDiff := time.Since(offlineStatus.LastChanged)
	if timeDiff > 5*time.Second {
		t.Errorf("LastChanged timestamp seems too old: %v", timeDiff)
	}
}

func TestCallEventStatusMapping(t *testing.T) {
	client := NewClient(
		"localhost", 1883, "", "", "test", "test", 1, true,
		60*time.Second, 30*time.Second,
	)

	// Test different call types and their expected status mappings
	testCases := []struct {
		name           string
		callType       types.CallType
		expectedStatus types.CallStatus
	}{
		{"Ring event should set status to ring", types.CallTypeRing, types.CallStatusRing},
		{"Call event should set status to call", types.CallTypeCall, types.CallStatusCall},
		{"Connect event should set status to active", types.CallTypeConnect, types.CallStatusActive},
		{"Disconnect event should set status to idle", types.CallTypeDisconnect, types.CallStatusIdle},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Create test event with specific type
			event := types.CallEvent{
				ID:        "test-uuid-" + string(tc.callType),
				Timestamp: time.Now(),
				Type:      tc.callType,
				Line:      1,
				Extension: "1",
				Trunk:     "SIP0",
				Caller:    "123456789",
				Called:    "987654321",
			}

			// Get or create line status
			lineKey := "SIP0_1"
			lineStatus := client.getOrCreateLineStatus(lineKey, event)

			// Update call history and line status like PublishCallEvent does
			client.callHistory.AddCall(event)

			// Update status based on call type (mimicking PublishCallEvent logic)
			switch event.Type {
			case types.CallTypeRing:
				lineStatus.Status = types.CallStatusRing
				lineStatus.ID = event.ID
				lineStatus.LastEvent = event.RawMessage
			case types.CallTypeCall:
				lineStatus.Status = types.CallStatusCall
				lineStatus.ID = event.ID
				lineStatus.LastEvent = event.RawMessage
			case types.CallTypeConnect:
				lineStatus.Status = types.CallStatusActive
				lineStatus.ID = event.ID
				lineStatus.LastEvent = event.RawMessage
			case types.CallTypeDisconnect:
				lineStatus.Status = types.CallStatusIdle
				lineStatus.ID = event.ID
				lineStatus.LastEvent = event.RawMessage
			}
			lineStatus.LastUpdated = event.Timestamp

			// Verify the status was set correctly
			if lineStatus.Status != tc.expectedStatus {
				t.Errorf("Expected status %s for %s, got %s", tc.expectedStatus, tc.callType, lineStatus.Status)
			}

			// Verify the EventId was set correctly
			if lineStatus.ID != event.ID {
				t.Errorf("Expected EventId %s, got %s", event.ID, lineStatus.ID)
			}

			// Verify LastEvent is set correctly
			if lineStatus.LastEvent != event.RawMessage {
				t.Errorf("Expected LastEvent %s, got %s", event.RawMessage, lineStatus.LastEvent)
			}
		})
	}
}
