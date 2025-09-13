package types

import (
	"fmt"
	"strings"
	"testing"
	"time"
)

// MockMQTTPublisher implements MQTTPublisher for testing
type MockMQTTPublisher struct {
	PublishedChanges []LineStatusChangeMessage
	ShouldError      bool
	ErrorMessage     string
}

func (m *MockMQTTPublisher) PublishLineStatusChange(line int, oldStatus, newStatus CallStatus, event *CallEvent) error {
	if m.ShouldError {
		return fmt.Errorf("%s", m.ErrorMessage)
	}

	msg := LineStatusChangeMessage{
		Line:      line,
		OldStatus: oldStatus,
		NewStatus: newStatus,
		Timestamp: time.Now().Format(time.RFC3339),
		Event:     event,
	}

	if event != nil {
		msg.Reason = "event"
	} else {
		msg.Reason = "timeout"
	}

	m.PublishedChanges = append(m.PublishedChanges, msg)
	return nil
}

func (m *MockMQTTPublisher) PublishTimeoutStatusUpdate(line int, newStatus CallStatus) error {
	if m.ShouldError {
		return fmt.Errorf("%s", m.ErrorMessage)
	}
	// For testing purposes, we don't need to track timeout updates separately
	return nil
}

func (m *MockMQTTPublisher) Reset() {
	m.PublishedChanges = nil
	m.ShouldError = false
	m.ErrorMessage = ""
}

func TestCallStateMachineWithMQTT(t *testing.T) {
	mockPublisher := &MockMQTTPublisher{}

	fsm := NewCallStateMachineWithMQTT(1, mockPublisher, nil)
	defer fsm.Cleanup()

	// Test transition with MQTT publishing
	event := &CallEvent{
		Line: 1,
		Type: CallTypeRing,
	}
	fsm.ProcessEventWithContext(CallTypeRing, event)

	// Wait for async goroutine to complete
	time.Sleep(50 * time.Millisecond)

	if len(mockPublisher.PublishedChanges) != 1 {
		t.Errorf("Expected 1 published change, got %d", len(mockPublisher.PublishedChanges))
		return
	}

	change := mockPublisher.PublishedChanges[0]
	if change.Line != 1 {
		t.Errorf("Expected line 1, got %d", change.Line)
	}
	if change.OldStatus != CallStatusIdle {
		t.Errorf("Expected old status idle, got %s", change.OldStatus)
	}
	if change.NewStatus != CallStatusRinging {
		t.Errorf("Expected new status ringing, got %s", change.NewStatus)
	}
	if change.Reason != "event" {
		t.Errorf("Expected reason 'event', got %s", change.Reason)
	}
}

func TestCallStateMachineWithMQTTTimeout(t *testing.T) {
	mockPublisher := &MockMQTTPublisher{}

	fsm := NewCallStateMachineWithMQTT(1, mockPublisher, nil)
	defer fsm.Cleanup()

	// Create transition to finished state (which has timeout)
	fsm.ProcessEvent(CallTypeRing)
	fsm.ProcessEvent(CallTypeConnect)
	fsm.ProcessEvent(CallTypeDisconnect) // Should go to finished

	// Reset published changes to focus on timeout
	mockPublisher.PublishedChanges = nil

	// Wait for timeout
	time.Sleep(1200 * time.Millisecond)

	// Should have timeout transition published
	if len(mockPublisher.PublishedChanges) == 0 {
		t.Error("Expected timeout transition to be published")
		return
	}

	change := mockPublisher.PublishedChanges[len(mockPublisher.PublishedChanges)-1]
	if change.NewStatus != CallStatusIdle {
		t.Errorf("Expected timeout transition to idle, got %s", change.NewStatus)
	}
	if change.Reason != "timeout" {
		t.Errorf("Expected reason 'timeout', got %s", change.Reason)
	}
}

func TestCallStateMachineSetMQTTPublisher(t *testing.T) {
	fsm := NewCallStateMachine(nil)
	defer fsm.Cleanup()

	mockPublisher := &MockMQTTPublisher{}
	fsm.SetMQTTPublisher(mockPublisher, 2)

	// Test that MQTT publishing works after setting publisher
	event := &CallEvent{
		Line: 2,
		Type: CallTypeCall,
	}
	fsm.ProcessEventWithContext(CallTypeCall, event)

	// Wait for async goroutine to complete
	time.Sleep(50 * time.Millisecond)

	if len(mockPublisher.PublishedChanges) != 1 {
		t.Errorf("Expected 1 published change after setting publisher, got %d", len(mockPublisher.PublishedChanges))
		return
	}

	change := mockPublisher.PublishedChanges[0]
	if change.Line != 2 {
		t.Errorf("Expected line 2, got %d", change.Line)
	}
}

func TestCallStateMachineProcessEventWithContext(t *testing.T) {
	mockPublisher := &MockMQTTPublisher{}
	fsm := NewCallStateMachineWithMQTT(1, mockPublisher, nil)
	defer fsm.Cleanup()

	event := &CallEvent{
		Line:      1,
		Type:      CallTypeRing,
		Timestamp: time.Now(),
		Caller:    "123456789",
		Called:    "987654321",
	}

	fsm.ProcessEventWithContext(CallTypeRing, event)

	// Wait for async goroutine to complete
	time.Sleep(50 * time.Millisecond)

	if len(mockPublisher.PublishedChanges) != 1 {
		t.Errorf("Expected 1 published change, got %d", len(mockPublisher.PublishedChanges))
		return
	}

	change := mockPublisher.PublishedChanges[0]
	if change.Event == nil {
		t.Error("Expected event to be included in status change")
	} else if change.Event.Caller != event.Caller {
		t.Errorf("Expected caller %s, got %s", event.Caller, change.Event.Caller)
	}
}

func TestGetFSMStatus(t *testing.T) {
	fsm := NewCallStateMachineWithMQTT(3, nil, nil)
	defer fsm.Cleanup()

	// Move to ringing state
	fsm.ProcessEvent(CallTypeRing)

	status := fsm.GetFSMStatus()

	if status.Line != 3 {
		t.Errorf("Expected line 3, got %d", status.Line)
	}
	if status.Status != CallStatusRinging {
		t.Errorf("Expected status ringing, got %s", status.Status)
	}
	if len(status.ValidTransitions) == 0 {
		t.Error("Expected valid transitions to be populated")
	}

	// Check that valid transitions include expected values
	expectedTransitions := []CallType{CallTypeConnect, CallTypeDisconnect}
	for _, expected := range expectedTransitions {
		found := false
		for _, actual := range status.ValidTransitions {
			if actual == expected {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Expected transition %s not found in %v", expected, status.ValidTransitions)
		}
	}
}

func TestLineStatusChangeMessageJSON(t *testing.T) {
	msg := LineStatusChangeMessage{
		Line:      1,
		OldStatus: CallStatusIdle,
		NewStatus: CallStatusRinging,
		Timestamp: "2023-01-01T00:00:00Z",
		Reason:    "event",
	}

	json, err := msg.ToJSON()
	if err != nil {
		t.Errorf("Failed to convert to JSON: %v", err)
	}

	if json == "" {
		t.Error("Expected non-empty JSON string")
	}

	// Basic validation that it contains expected fields
	expectedFields := []string{`"line":1`, `"old_status":"idle"`, `"new_status":"ringing"`, `"reason":"event"`}
	for _, field := range expectedFields {
		if !strings.Contains(json, field) {
			t.Errorf("Expected JSON to contain %s, got: %s", field, json)
		}
	}
}

func TestFSMStatusMessageJSON(t *testing.T) {
	msg := FSMStatusMessage{
		Line:               2,
		Status:             CallStatusTalking,
		Timestamp:          "2023-01-01T00:00:00Z",
		ValidTransitions:   []CallType{CallTypeDisconnect},
		IsTimeoutActive:    false,
		LastEventType:      CallTypeConnect,
		LastEventTimestamp: "2023-01-01T00:00:00Z",
	}

	json, err := msg.ToJSON()
	if err != nil {
		t.Errorf("Failed to convert to JSON: %v", err)
	}

	if json == "" {
		t.Error("Expected non-empty JSON string")
	}

	expectedFields := []string{`"line":2`, `"status":"talking"`, `"is_timeout_active":false`}
	for _, field := range expectedFields {
		if !strings.Contains(json, field) {
			t.Errorf("Expected JSON to contain %s, got: %s", field, json)
		}
	}
}

func TestLineStateMachineWithMQTT(t *testing.T) {
	mockPublisher := &MockMQTTPublisher{}

	lsm := NewLineStateMachineWithMQTT(mockPublisher, nil)
	defer lsm.Cleanup()

	event := &CallEvent{
		Line: 1,
		Type: CallTypeRing,
	}

	lsm.ProcessCallEvent(event)

	// Wait for async goroutine to complete
	time.Sleep(50 * time.Millisecond)

	if len(mockPublisher.PublishedChanges) != 1 {
		t.Errorf("Expected 1 published change, got %d", len(mockPublisher.PublishedChanges))
	}
}

func TestLineStateMachineSetMQTTPublisher(t *testing.T) {
	lsm := NewLineStateMachine(nil)
	defer lsm.Cleanup()

	// Create some lines first
	event1 := &CallEvent{Line: 1, Type: CallTypeRing}
	event2 := &CallEvent{Line: 2, Type: CallTypeCall}
	lsm.ProcessCallEvent(event1)
	lsm.ProcessCallEvent(event2)

	// Set MQTT publisher
	mockPublisher := &MockMQTTPublisher{}
	lsm.SetMQTTPublisher(mockPublisher)

	// Process new event - should now publish via MQTT
	event3 := &CallEvent{Line: 1, Type: CallTypeConnect}
	lsm.ProcessCallEvent(event3)

	// Wait for async goroutine to complete
	time.Sleep(50 * time.Millisecond)

	if len(mockPublisher.PublishedChanges) != 1 {
		t.Errorf("Expected 1 published change after setting publisher, got %d", len(mockPublisher.PublishedChanges))
	}
}

func TestGetAllFSMStatuses(t *testing.T) {
	lsm := NewLineStateMachine(nil)
	defer lsm.Cleanup()

	// Create multiple lines
	events := []*CallEvent{
		{Line: 1, Type: CallTypeRing},
		{Line: 2, Type: CallTypeCall},
	}

	for _, event := range events {
		lsm.ProcessCallEvent(event)
	}

	statuses := lsm.GetAllFSMStatuses()

	if len(statuses) != 2 {
		t.Errorf("Expected 2 FSM statuses, got %d", len(statuses))
		return
	}

	// Verify each status has correct data
	lineStatuses := make(map[int]FSMStatusMessage)
	for _, status := range statuses {
		lineStatuses[status.Line] = status
	}

	if status, ok := lineStatuses[1]; !ok {
		t.Error("Expected status for line 1")
	} else if status.Status != CallStatusRinging {
		t.Errorf("Expected line 1 status ringing, got %s", status.Status)
	}

	if status, ok := lineStatuses[2]; !ok {
		t.Error("Expected status for line 2")
	} else if status.Status != CallStatusCalling {
		t.Errorf("Expected line 2 status calling, got %s", status.Status)
	}
}
