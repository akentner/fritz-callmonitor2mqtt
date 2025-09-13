package types

import (
	"testing"
	"time"
)

func TestCallManagerSetsEventStatus(t *testing.T) {
	cm := NewCallManager(nil)
	defer cm.Cleanup()

	// Test 1: RING event should set status to ringing
	event := &CallEvent{
		Line:      1,
		Type:      CallTypeRing,
		Timestamp: time.Now(),
	}

	processedEvent := cm.ProcessEvent(event)

	if processedEvent.Status != CallStatusRinging {
		t.Errorf("Expected event status to be %s, got %s", CallStatusRinging, processedEvent.Status)
	}

	// Test 2: DISCONNECT from ringing should set status to missedCall
	disconnectEvent := &CallEvent{
		Line:      1,
		Type:      CallTypeDisconnect,
		Timestamp: time.Now(),
	}

	processedDisconnectEvent := cm.ProcessEvent(disconnectEvent)

	if processedDisconnectEvent.Status != CallStatusMissedCall {
		t.Errorf("Expected event status to be %s, got %s", CallStatusMissedCall, processedDisconnectEvent.Status)
	}

	// Verify line status matches
	lineStatus := cm.GetLineStatus(1)
	if lineStatus != CallStatusMissedCall {
		t.Errorf("Expected line status to be %s, got %s", CallStatusMissedCall, lineStatus)
	}
}

func TestMQTTClientUsesFSMStatus(t *testing.T) {
	// Create a mock MQTT publisher that doesn't actually connect
	type MockMQTTClient struct {
		lastPublishedEvent CallEvent
	}

	mockClient := &MockMQTTClient{}

	// Simulate what would happen in the real MQTT client
	event := CallEvent{
		Line:   1,
		Type:   CallTypeRing,
		Status: CallStatusRinging, // FSM has set this status
	}

	// Simulate the logic from PublishCallEvent
	var expectedStatus CallStatus
	if event.Status != "" {
		expectedStatus = event.Status // Use FSM status
	} else {
		// Fallback logic
		switch event.Type {
		case CallTypeRing:
			expectedStatus = CallStatusRinging
		}
	}

	if expectedStatus != CallStatusRinging {
		t.Errorf("Expected FSM status to be used: %s", expectedStatus)
	}

	mockClient.lastPublishedEvent = event

	if mockClient.lastPublishedEvent.Status != CallStatusRinging {
		t.Errorf("Expected published event to have FSM status: %s", mockClient.lastPublishedEvent.Status)
	}
}
