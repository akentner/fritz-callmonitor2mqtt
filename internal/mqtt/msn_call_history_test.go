package mqtt

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"

	"fritz-callmonitor2mqtt/pkg/types"
)

func TestMSNCallHistoryUpdates(t *testing.T) {
	client := createTestClient()

	// Create a test disconnect event involving MSN "12345"
	event := types.CallEvent{
		ID:        "test-id",
		Timestamp: time.Now(),
		Type:      types.CallTypeDisconnect,
		Direction: types.CallDirectionInbound,
		Caller:    "+491234567890",
		Called:    "+4961812345",
		CallerMSN: "",
		CalledMSN: "12345", // One of the configured test MSNs
		Status:    types.CallStatusFinished,
	}

	// Update MSN call histories
	client.updateMSNCallHistories(event)

	// Check that the MSN call history was updated
	msnHistory, exists := client.msnCallHistories["12345"]
	if !exists {
		t.Fatal("MSN call history for '12345' should exist")
	}

	if len(msnHistory.Calls) != 1 {
		t.Errorf("Expected 1 call in MSN history, got %d", len(msnHistory.Calls))
	}

	if msnHistory.Calls[0].ID != "test-id" {
		t.Errorf("Expected call ID 'test-id', got %s", msnHistory.Calls[0].ID)
	}
}

func TestMSNCallHistoryOnlyAddsDisconnectEvents(t *testing.T) {
	client := createTestClient()

	// Create different types of events
	ringEvent := types.CallEvent{
		ID:        "ring-id",
		Type:      types.CallTypeRing,
		CallerMSN: "12345",
	}

	callEvent := types.CallEvent{
		ID:        "call-id",
		Type:      types.CallTypeCall,
		CalledMSN: "12345",
	}

	connectEvent := types.CallEvent{
		ID:        "connect-id",
		Type:      types.CallTypeConnect,
		CallerMSN: "12345",
	}

	disconnectEvent := types.CallEvent{
		ID:        "disconnect-id",
		Type:      types.CallTypeDisconnect,
		CalledMSN: "12345",
	}

	// Process all events
	client.updateMSNCallHistories(ringEvent)
	client.updateMSNCallHistories(callEvent)
	client.updateMSNCallHistories(connectEvent)
	client.updateMSNCallHistories(disconnectEvent)

	// Only the disconnect event should be added
	msnHistory := client.msnCallHistories["12345"]
	if len(msnHistory.Calls) != 1 {
		t.Errorf("Expected 1 call in MSN history (only disconnect), got %d", len(msnHistory.Calls))
	}

	if msnHistory.Calls[0].ID != "disconnect-id" {
		t.Errorf("Expected disconnect event ID, got %s", msnHistory.Calls[0].ID)
	}
}

func TestMSNCallHistoryMaxSize(t *testing.T) {
	// Create client with small MSN call history size for testing
	client := NewClient(
		"localhost", 1883, "", "", "test", "test", 1, false,
		60*time.Second, 30*time.Second, "info", nil,
		[]string{"12345"}, 2, // Only keep 2 calls
	)

	// Add 3 disconnect events
	for i := 1; i <= 3; i++ {
		event := types.CallEvent{
			ID:        fmt.Sprintf("call-%d", i),
			Type:      types.CallTypeDisconnect,
			CalledMSN: "12345",
			Timestamp: time.Now().Add(time.Duration(i) * time.Minute),
		}
		client.updateMSNCallHistories(event)
	}

	// Should only keep the 2 most recent calls
	msnHistory := client.msnCallHistories["12345"]
	if len(msnHistory.Calls) != 2 {
		t.Errorf("Expected 2 calls (max size), got %d", len(msnHistory.Calls))
	}

	// Newest call should be first
	if msnHistory.Calls[0].ID != "call-3" {
		t.Errorf("Expected newest call first, got %s", msnHistory.Calls[0].ID)
	}

	if msnHistory.Calls[1].ID != "call-2" {
		t.Errorf("Expected second newest call second, got %s", msnHistory.Calls[1].ID)
	}
}

func TestMSNCallHistoryEmptyArraySerialization(t *testing.T) {
	// Create client with empty MSN call history
	client := NewClient(
		"localhost", 1883, "", "", "test", "test", 1, false,
		60*time.Second, 30*time.Second, "info", nil,
		[]string{"12345"}, 30,
	)

	// Get the empty MSN history
	msnHistory := client.msnCallHistories["12345"]
	if msnHistory == nil {
		t.Fatalf("MSN history for '12345' should exist")
	}

	// Marshal to JSON
	jsonData, err := json.Marshal(msnHistory)
	if err != nil {
		t.Fatalf("Failed to marshal MSN history: %v", err)
	}

	// Verify that calls is an empty array, not null
	jsonStr := string(jsonData)
	if !strings.Contains(jsonStr, `"calls":[]`) {
		t.Errorf("Expected 'calls':[] in JSON, got: %s", jsonStr)
	}

	if strings.Contains(jsonStr, `"calls":null`) {
		t.Errorf("Found 'calls':null in JSON (should be empty array): %s", jsonStr)
	}
}
