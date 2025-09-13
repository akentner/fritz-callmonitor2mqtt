package types

import (
	"testing"
	"time"
)

func TestNewCallManager(t *testing.T) {
	cm := NewCallManager(nil)
	defer cm.Cleanup()

	if cm == nil {
		t.Fatal("Expected CallManager to be created")
	}

	// Should start with no active lines
	if len(cm.GetActiveLines()) != 0 {
		t.Errorf("Expected no active lines initially, got %d", len(cm.GetActiveLines()))
	}
}

func TestCallManagerProcessEvent(t *testing.T) {
	var statusChanges []struct {
		line      int
		oldStatus CallStatus
		newStatus CallStatus
	}

	cm := NewCallManager(func(line int, oldStatus, newStatus CallStatus, event *CallEvent) {
		statusChanges = append(statusChanges, struct {
			line      int
			oldStatus CallStatus
			newStatus CallStatus
		}{line, oldStatus, newStatus})
	})
	defer cm.Cleanup()

	event := &CallEvent{
		Line: 1,
		Type: CallTypeRing,
	}

	result := cm.ProcessEvent(event)

	if result.Status != CallStatusRinging {
		t.Errorf("Expected event status to be ringing, got %v", result.Status)
	}

	if cm.GetLineStatus(1) != CallStatusRinging {
		t.Errorf("Expected line status to be ringing, got %v", cm.GetLineStatus(1))
	}

	// Check status change notification (CallManager has both FSM and CM callbacks)
	if len(statusChanges) < 1 {
		t.Errorf("Expected at least 1 status change notification, got %d", len(statusChanges))
	} else {
		// Find the change we're interested in (could be from either FSM or CM callback)
		found := false
		for _, change := range statusChanges {
			if change.line == 1 && change.oldStatus == CallStatusIdle && change.newStatus == CallStatusRinging {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Expected to find line 1, idle->ringing transition in %v", statusChanges)
		}
	}
}

func TestCallManagerValidation(t *testing.T) {
	cm := NewCallManager(nil)
	defer cm.Cleanup()

	tests := []struct {
		name        string
		event       *CallEvent
		shouldError bool
	}{
		{
			name:        "nil event",
			event:       nil,
			shouldError: true,
		},
		{
			name: "negative line number",
			event: &CallEvent{
				Line: -1,
				Type: CallTypeRing,
			},
			shouldError: true,
		},
		{
			name: "empty event type",
			event: &CallEvent{
				Line: 1,
				Type: "",
			},
			shouldError: true,
		},
		{
			name: "invalid transition",
			event: &CallEvent{
				Line: 1,
				Type: CallTypeConnect, // Can't connect from idle
			},
			shouldError: true,
		},
		{
			name: "valid event",
			event: &CallEvent{
				Line: 1,
				Type: CallTypeRing,
			},
			shouldError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := cm.validateEvent(tt.event)
			if tt.shouldError && err == nil {
				t.Errorf("Expected validation error, but got none")
			}
			if !tt.shouldError && err != nil {
				t.Errorf("Expected no validation error, but got: %v", err)
			}
		})
	}
}

func TestCallManagerGetAllLineStatuses(t *testing.T) {
	cm := NewCallManager(nil)
	defer cm.Cleanup()

	// Create events for multiple lines
	events := []*CallEvent{
		{Line: 1, Type: CallTypeRing},
		{Line: 2, Type: CallTypeCall},
	}

	for _, event := range events {
		cm.ProcessEvent(event)
	}

	statuses := cm.GetAllLineStatuses()
	expectedStatuses := map[int]CallStatus{
		1: CallStatusRinging,
		2: CallStatusCalling,
	}

	if len(statuses) != len(expectedStatuses) {
		t.Errorf("Expected %d line statuses, got %d", len(expectedStatuses), len(statuses))
	}

	for line, expectedStatus := range expectedStatuses {
		if statuses[line] != expectedStatus {
			t.Errorf("Expected line %d status %v, got %v", line, expectedStatus, statuses[line])
		}
	}
}

func TestCallManagerResetLine(t *testing.T) {
	var statusChanges []CallStatus

	cm := NewCallManager(func(line int, oldStatus, newStatus CallStatus, event *CallEvent) {
		if line == 1 {
			statusChanges = append(statusChanges, newStatus)
		}
	})
	defer cm.Cleanup()

	// Move line to talking state
	events := []*CallEvent{
		{Line: 1, Type: CallTypeRing},
		{Line: 1, Type: CallTypeConnect},
	}

	for _, event := range events {
		cm.ProcessEvent(event)
	}

	if cm.GetLineStatus(1) != CallStatusTalking {
		t.Errorf("Expected line to be talking, got %v", cm.GetLineStatus(1))
	}

	// Reset line
	cm.ResetLine(1)

	if cm.GetLineStatus(1) != CallStatusIdle {
		t.Errorf("Expected line to be idle after reset, got %v", cm.GetLineStatus(1))
	}

	// Should have: idle -> ringing -> talking -> idle
	if len(statusChanges) < 3 {
		t.Errorf("Expected at least 3 status changes, got %d: %v", len(statusChanges), statusChanges)
	}
}

func TestCallManagerGetStatusSummary(t *testing.T) {
	cm := NewCallManager(nil)
	defer cm.Cleanup()

	// Test with no lines
	summary := cm.GetStatusSummary()
	if summary != "No active lines" {
		t.Errorf("Expected 'No active lines', got %q", summary)
	}

	// Create a line
	event := &CallEvent{Line: 1, Type: CallTypeRing}
	cm.ProcessEvent(event)

	summary = cm.GetStatusSummary()
	if summary == "No active lines" {
		t.Errorf("Expected line summary with states, got %q", summary)
	}
}

func TestCallManagerSimulateCall(t *testing.T) {
	var statusChanges []CallStatus
	var eventCount int

	cm := NewCallManager(func(line int, oldStatus, newStatus CallStatus, event *CallEvent) {
		if line == 1 {
			statusChanges = append(statusChanges, newStatus)
			eventCount++
		}
	})
	defer cm.Cleanup()

	// Test inbound call simulation
	cm.SimulateCall(1, CallDirectionInbound, "01234567890", "987654321")

	// Should have gone through: idle -> ringing -> talking -> finished
	expectedSequence := []CallStatus{CallStatusRinging, CallStatusTalking, CallStatusFinished}

	if len(statusChanges) < len(expectedSequence) {
		t.Errorf("Expected at least %d status changes, got %d: %v", len(expectedSequence), len(statusChanges), statusChanges)
	}

	// Check that all expected states appear in the sequence (order may vary due to duplicate callbacks)
	for _, expectedStatus := range expectedSequence {
		found := false
		for _, actualStatus := range statusChanges {
			if actualStatus == expectedStatus {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Expected to find status %v in sequence %v", expectedStatus, statusChanges)
		}
	}
}

func TestCallManagerSimulateMissedCall(t *testing.T) {
	var statusChanges []CallStatus

	cm := NewCallManager(func(line int, oldStatus, newStatus CallStatus, event *CallEvent) {
		if line == 1 {
			statusChanges = append(statusChanges, newStatus)
		}
	})
	defer cm.Cleanup()

	// Simulate missed call (this will include timeout wait)
	go cm.SimulateMissedCall(1)

	// Wait a bit for the simulation to start
	time.Sleep(200 * time.Millisecond)

	// Should be in missedCall state
	if cm.GetLineStatus(1) != CallStatusMissedCall {
		t.Errorf("Expected line to be in missedCall state, got %v", cm.GetLineStatus(1))
	}

	// Wait for timeout
	time.Sleep(1200 * time.Millisecond)

	// Should be back to idle
	if cm.GetLineStatus(1) != CallStatusIdle {
		t.Errorf("Expected line to be idle after timeout, got %v", cm.GetLineStatus(1))
	}
}

func TestCallManagerSimulateNotReachedCall(t *testing.T) {
	var statusChanges []CallStatus

	cm := NewCallManager(func(line int, oldStatus, newStatus CallStatus, event *CallEvent) {
		if line == 1 {
			statusChanges = append(statusChanges, newStatus)
		}
	})
	defer cm.Cleanup()

	// Simulate not reached call (this will include timeout wait)
	go cm.SimulateNotReachedCall(1)

	// Wait a bit for the simulation to start
	time.Sleep(200 * time.Millisecond)

	// Should be in notReached state
	if cm.GetLineStatus(1) != CallStatusNotReached {
		t.Errorf("Expected line to be in notReached state, got %v", cm.GetLineStatus(1))
	}

	// Wait for timeout
	time.Sleep(1200 * time.Millisecond)

	// Should be back to idle
	if cm.GetLineStatus(1) != CallStatusIdle {
		t.Errorf("Expected line to be idle after timeout, got %v", cm.GetLineStatus(1))
	}
}

func TestCallManagerComplexScenario(t *testing.T) {
	cm := NewCallManager(nil)
	defer cm.Cleanup()

	// Simulate multiple concurrent calls on different lines
	events := []*CallEvent{
		// Line 1: Incoming call that gets answered
		{Line: 1, Type: CallTypeRing},
		// Line 2: Outgoing call
		{Line: 2, Type: CallTypeCall},
		// Line 1: Answer incoming call
		{Line: 1, Type: CallTypeConnect},
		// Line 2: Outgoing call connects
		{Line: 2, Type: CallTypeConnect},
		// Line 3: Another incoming call that will be missed
		{Line: 3, Type: CallTypeRing},
		// Line 1: End first call
		{Line: 1, Type: CallTypeDisconnect},
		// Line 3: Missed call
		{Line: 3, Type: CallTypeDisconnect},
		// Line 2: End second call
		{Line: 2, Type: CallTypeDisconnect},
	}

	for _, event := range events {
		cm.ProcessEvent(event)
	}

	// Check final states
	expectedStates := map[int]CallStatus{
		1: CallStatusFinished,
		2: CallStatusFinished,
		3: CallStatusMissedCall,
	}

	for line, expectedState := range expectedStates {
		actualState := cm.GetLineStatus(line)
		if actualState != expectedState {
			t.Errorf("Expected line %d to be in %v state, got %v", line, expectedState, actualState)
		}
	}

	// Check active lines
	activeLines := cm.GetActiveLines()
	if len(activeLines) != 3 {
		t.Errorf("Expected 3 active lines, got %d", len(activeLines))
	}
}

func TestCallManagerInvalidTransitionHandling(t *testing.T) {
	cm := NewCallManager(nil)
	defer cm.Cleanup()

	// Start with a ring
	event1 := &CallEvent{Line: 1, Type: CallTypeRing}
	cm.ProcessEvent(event1)

	// Try invalid transition (ring again while already ringing)
	event2 := &CallEvent{Line: 1, Type: CallTypeRing}
	err := cm.validateEvent(event2)

	if err == nil {
		t.Errorf("Expected validation error for invalid transition, but got none")
	}

	// Status should remain unchanged
	if cm.GetLineStatus(1) != CallStatusRinging {
		t.Errorf("Expected line to remain ringing, got %v", cm.GetLineStatus(1))
	}
}
