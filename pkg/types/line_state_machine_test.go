package types

import (
	"sync"
	"testing"
	"time"
)

func TestNewLineStateMachine(t *testing.T) {
	lsm := NewLineStateMachine(nil)
	if lsm.GetLineCount() != 0 {
		t.Errorf("Expected 0 lines initially, got %d", lsm.GetLineCount())
	}
}

func TestProcessCallEvent(t *testing.T) {
	var stateChanges []struct {
		line     int
		oldState CallStatus
		newState CallStatus
	}
	var mu sync.Mutex

	lsm := NewLineStateMachine(func(line int, oldState, newState CallStatus) {
		mu.Lock()
		stateChanges = append(stateChanges, struct {
			line     int
			oldState CallStatus
			newState CallStatus
		}{line, oldState, newState})
		mu.Unlock()
	})

	// Test event processing
	event := &CallEvent{
		Line: 1,
		Type: CallTypeRing,
	}

	status := lsm.ProcessCallEvent(event)

	if status != CallStatusRinging {
		t.Errorf("Expected status ringing, got %v", status)
	}

	if event.Status != CallStatusRinging {
		t.Errorf("Expected event status to be updated to ringing, got %v", event.Status)
	}

	if lsm.GetLineCount() != 1 {
		t.Errorf("Expected 1 active line, got %d", lsm.GetLineCount())
	}

	// Check state change callback
	mu.Lock()
	changes := stateChanges
	mu.Unlock()

	if len(changes) != 1 {
		t.Errorf("Expected 1 state change, got %d", len(changes))
	} else {
		change := changes[0]
		if change.line != 1 || change.oldState != CallStatusIdle || change.newState != CallStatusRinging {
			t.Errorf("Expected line 1, idle->ringing, got line %d, %v->%v",
				change.line, change.oldState, change.newState)
		}
	}

	lsm.Cleanup()
}

func TestMultipleLines(t *testing.T) {
	lsm := NewLineStateMachine(nil)

	// Create events for different lines
	event1 := &CallEvent{Line: 1, Type: CallTypeRing}
	event2 := &CallEvent{Line: 2, Type: CallTypeCall}

	lsm.ProcessCallEvent(event1)
	lsm.ProcessCallEvent(event2)

	if lsm.GetLineCount() != 2 {
		t.Errorf("Expected 2 active lines, got %d", lsm.GetLineCount())
	}

	if lsm.GetLineState(1) != CallStatusRinging {
		t.Errorf("Expected line 1 to be ringing, got %v", lsm.GetLineState(1))
	}

	if lsm.GetLineState(2) != CallStatusCalling {
		t.Errorf("Expected line 2 to be calling, got %v", lsm.GetLineState(2))
	}

	// Test GetAllLineStates
	states := lsm.GetAllLineStates()
	expectedStates := map[int]CallStatus{
		1: CallStatusRinging,
		2: CallStatusCalling,
	}

	for line, expectedState := range expectedStates {
		if states[line] != expectedState {
			t.Errorf("Expected line %d state %v, got %v", line, expectedState, states[line])
		}
	}

	lsm.Cleanup()
}

func TestGetLineState(t *testing.T) {
	lsm := NewLineStateMachine(nil)

	// Non-existent line should return idle
	if lsm.GetLineState(99) != CallStatusIdle {
		t.Errorf("Expected non-existent line to return idle, got %v", lsm.GetLineState(99))
	}

	// Create a line and test state
	event := &CallEvent{Line: 1, Type: CallTypeRing}
	lsm.ProcessCallEvent(event)

	if lsm.GetLineState(1) != CallStatusRinging {
		t.Errorf("Expected line 1 to be ringing, got %v", lsm.GetLineState(1))
	}

	lsm.Cleanup()
}

func TestResetLine(t *testing.T) {
	var stateChanges []CallStatus
	var mu sync.Mutex

	lsm := NewLineStateMachine(func(line int, oldState, newState CallStatus) {
		if line == 1 {
			mu.Lock()
			stateChanges = append(stateChanges, newState)
			mu.Unlock()
		}
	})

	// Move line to talking state
	event := &CallEvent{Line: 1, Type: CallTypeRing}
	lsm.ProcessCallEvent(event)
	event.Type = CallTypeConnect
	lsm.ProcessCallEvent(event)

	if lsm.GetLineState(1) != CallStatusTalking {
		t.Errorf("Expected line 1 to be talking, got %v", lsm.GetLineState(1))
	}

	// Reset line
	lsm.ResetLine(1)

	if lsm.GetLineState(1) != CallStatusIdle {
		t.Errorf("Expected line 1 to be idle after reset, got %v", lsm.GetLineState(1))
	}

	// Check that reset triggered state change callback
	mu.Lock()
	changes := stateChanges
	mu.Unlock()

	// Should have: idle -> ringing -> talking -> idle
	if len(changes) < 3 {
		t.Errorf("Expected at least 3 state changes, got %d: %v", len(changes), changes)
	} else if changes[len(changes)-1] != CallStatusIdle {
		t.Errorf("Expected last state change to be idle, got %v", changes[len(changes)-1])
	}

	lsm.Cleanup()
}

func TestResetAllLines(t *testing.T) {
	lsm := NewLineStateMachine(nil)

	// Create multiple lines with different states
	events := []*CallEvent{
		{Line: 1, Type: CallTypeRing},
		{Line: 2, Type: CallTypeCall},
		{Line: 3, Type: CallTypeRing},
	}

	for _, event := range events {
		lsm.ProcessCallEvent(event)
	}

	// Reset all lines
	lsm.ResetAllLines()

	// Check all lines are idle
	states := lsm.GetAllLineStates()
	for line, state := range states {
		if state != CallStatusIdle {
			t.Errorf("Expected line %d to be idle after reset all, got %v", line, state)
		}
	}

	lsm.Cleanup()
}

func TestLineIsValidTransition(t *testing.T) {
	lsm := NewLineStateMachine(nil)

	// Test for non-existent line (should allow initial transitions)
	if !lsm.IsValidTransition(1, CallTypeRing) {
		t.Errorf("Expected RING to be valid for new line")
	}

	if !lsm.IsValidTransition(1, CallTypeCall) {
		t.Errorf("Expected CALL to be valid for new line")
	}

	if lsm.IsValidTransition(1, CallTypeConnect) {
		t.Errorf("Expected CONNECT to be invalid for new line")
	}

	// Create line and test transitions
	event := &CallEvent{Line: 1, Type: CallTypeRing}
	lsm.ProcessCallEvent(event)

	if !lsm.IsValidTransition(1, CallTypeConnect) {
		t.Errorf("Expected CONNECT to be valid from ringing state")
	}

	if lsm.IsValidTransition(1, CallTypeRing) {
		t.Errorf("Expected RING to be invalid from ringing state")
	}

	lsm.Cleanup()
}

func TestLineGetValidTransitions(t *testing.T) {
	lsm := NewLineStateMachine(nil)

	// Test for non-existent line
	transitions := lsm.GetValidTransitions(99)
	expectedTransitions := []CallType{CallTypeRing, CallTypeCall}

	if len(transitions) != len(expectedTransitions) {
		t.Errorf("Expected %d transitions for new line, got %d", len(expectedTransitions), len(transitions))
	}

	// Create line and test transitions
	event := &CallEvent{Line: 1, Type: CallTypeRing}
	lsm.ProcessCallEvent(event)

	transitions = lsm.GetValidTransitions(1)
	// From ringing state, should allow CONNECT and DISCONNECT
	expectedCount := 2
	if len(transitions) != expectedCount {
		t.Errorf("Expected %d transitions from ringing state, got %d: %v", expectedCount, len(transitions), transitions)
	}

	lsm.Cleanup()
}

func TestRemoveLine(t *testing.T) {
	lsm := NewLineStateMachine(nil)

	// Create multiple lines
	events := []*CallEvent{
		{Line: 1, Type: CallTypeRing},
		{Line: 2, Type: CallTypeCall},
	}

	for _, event := range events {
		lsm.ProcessCallEvent(event)
	}

	if lsm.GetLineCount() != 2 {
		t.Errorf("Expected 2 lines, got %d", lsm.GetLineCount())
	}

	// Remove one line
	lsm.RemoveLine(1)

	if lsm.GetLineCount() != 1 {
		t.Errorf("Expected 1 line after removal, got %d", lsm.GetLineCount())
	}

	if lsm.GetLineState(1) != CallStatusIdle {
		t.Errorf("Expected removed line to return idle, got %v", lsm.GetLineState(1))
	}

	if lsm.GetLineState(2) != CallStatusCalling {
		t.Errorf("Expected remaining line to keep its state, got %v", lsm.GetLineState(2))
	}

	// Remove non-existent line (should not cause issues)
	lsm.RemoveLine(99)

	if lsm.GetLineCount() != 1 {
		t.Errorf("Expected 1 line after removing non-existent line, got %d", lsm.GetLineCount())
	}

	lsm.Cleanup()
}

func TestGetActiveLines(t *testing.T) {
	lsm := NewLineStateMachine(nil)

	// Initially no active lines
	lines := lsm.GetActiveLines()
	if len(lines) != 0 {
		t.Errorf("Expected no active lines initially, got %d", len(lines))
	}

	// Create some lines
	lineNumbers := []int{1, 3, 5}
	for _, lineNum := range lineNumbers {
		event := &CallEvent{Line: lineNum, Type: CallTypeRing}
		lsm.ProcessCallEvent(event)
	}

	lines = lsm.GetActiveLines()
	if len(lines) != len(lineNumbers) {
		t.Errorf("Expected %d active lines, got %d", len(lineNumbers), len(lines))
	}

	// Check all expected lines are present
	for _, expectedLine := range lineNumbers {
		found := false
		for _, activeLine := range lines {
			if activeLine == expectedLine {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Expected line %d to be active, but not found in %v", expectedLine, lines)
		}
	}

	lsm.Cleanup()
}

func TestGetLineStateSummary(t *testing.T) {
	lsm := NewLineStateMachine(nil)

	// Test with no lines
	summary := lsm.GetLineStateSummary()
	if summary != "No active lines" {
		t.Errorf("Expected 'No active lines', got %q", summary)
	}

	// Create some lines
	events := []*CallEvent{
		{Line: 1, Type: CallTypeRing},
		{Line: 2, Type: CallTypeCall},
	}

	for _, event := range events {
		lsm.ProcessCallEvent(event)
	}

	summary = lsm.GetLineStateSummary()
	if summary == "No active lines" {
		t.Errorf("Expected line summary with states, got %q", summary)
	}

	// Check that summary contains expected information
	if !contains(summary, "Line 1") || !contains(summary, "Line 2") {
		t.Errorf("Expected summary to contain line information, got %q", summary)
	}

	lsm.Cleanup()
}

func TestLineConcurrentAccess(t *testing.T) {
	lsm := NewLineStateMachine(nil)
	done := make(chan bool)

	// Start multiple goroutines that access the LSM concurrently
	for i := 0; i < 10; i++ {
		go func(routineID int) {
			for j := 0; j < 50; j++ {
				line := routineID%3 + 1 // Use lines 1, 2, 3
				event := &CallEvent{Line: line, Type: CallTypeRing}
				lsm.ProcessCallEvent(event)
				lsm.GetLineState(line)
				lsm.GetAllLineStates()
				lsm.IsValidTransition(line, CallTypeConnect)
				if j%10 == 0 {
					lsm.ResetLine(line)
				}
			}
			done <- true
		}(i)
	}

	// Wait for all goroutines to complete
	for i := 0; i < 10; i++ {
		<-done
	}

	lsm.Cleanup()
}

func TestTimeoutIntegration(t *testing.T) {
	var stateChanges []struct {
		line     int
		newState CallStatus
	}
	var mu sync.Mutex

	lsm := NewLineStateMachine(func(line int, oldState, newState CallStatus) {
		mu.Lock()
		stateChanges = append(stateChanges, struct {
			line     int
			newState CallStatus
		}{line, newState})
		mu.Unlock()
	})

	// Create call that will result in notReached (calling -> disconnect)
	event := &CallEvent{Line: 1, Type: CallTypeCall}
	lsm.ProcessCallEvent(event)
	event.Type = CallTypeDisconnect
	lsm.ProcessCallEvent(event)

	// Should be in notReached state
	if lsm.GetLineState(1) != CallStatusNotReached {
		t.Errorf("Expected line 1 to be notReached, got %v", lsm.GetLineState(1))
	}

	// Wait for timeout
	time.Sleep(1200 * time.Millisecond)

	// Should be back to idle
	if lsm.GetLineState(1) != CallStatusIdle {
		t.Errorf("Expected line 1 to be idle after timeout, got %v", lsm.GetLineState(1))
	}

	// Check state changes include timeout transition
	mu.Lock()
	changes := stateChanges
	mu.Unlock()

	// Should have: idle -> calling -> notReached -> idle
	idleCount := 0
	for _, change := range changes {
		if change.newState == CallStatusIdle {
			idleCount++
		}
	}

	if idleCount == 0 {
		t.Errorf("Expected timeout transition to idle, but no idle transitions found: %v", changes)
	}

	lsm.Cleanup()
}

// Helper function to check if a string contains a substring
func contains(s, substr string) bool {
	return len(s) >= len(substr) &&
		(s == substr ||
			len(s) > len(substr) &&
				(s[:len(substr)] == substr ||
					s[len(s)-len(substr):] == substr ||
					findInString(s, substr)))
}

func findInString(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
