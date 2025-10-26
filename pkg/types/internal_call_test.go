package types

import (
	"testing"
	"time"
)

// TestDetectInternalCall tests the DetectInternalCall method
func TestDetectInternalCall(t *testing.T) {
	tests := []struct {
		name       string
		callerMSN  string
		calledMSN  string
		wantResult bool
	}{
		{
			name:       "both MSNs present",
			callerMSN:  "12345",
			calledMSN:  "67890",
			wantResult: true,
		},
		{
			name:       "self-call same MSN",
			callerMSN:  "12345",
			calledMSN:  "12345",
			wantResult: true,
		},
		{
			name:       "only caller MSN",
			callerMSN:  "12345",
			calledMSN:  "",
			wantResult: false,
		},
		{
			name:       "only called MSN",
			callerMSN:  "",
			calledMSN:  "67890",
			wantResult: false,
		},
		{
			name:       "no MSNs",
			callerMSN:  "",
			calledMSN:  "",
			wantResult: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			event := &CallEvent{
				CallerMSN: tt.callerMSN,
				CalledMSN: tt.calledMSN,
			}

			got := event.DetectInternalCall()
			if got != tt.wantResult {
				t.Errorf("DetectInternalCall() = %v, want %v", got, tt.wantResult)
			}
		})
	}
}

// TestLinkInternalCallEvents tests the LinkInternalCallEvents function
func TestLinkInternalCallEvents(t *testing.T) {
	callerEvent := &CallEvent{
		ID:        "caller-uuid",
		Type:      CallTypeCall,
		CallerMSN: "12345",
		CalledMSN: "67890",
	}

	calleeEvent := &CallEvent{
		ID:        "callee-uuid",
		Type:      CallTypeRing,
		CallerMSN: "12345",
		CalledMSN: "67890",
	}

	LinkInternalCallEvents(callerEvent, calleeEvent)

	// Check caller event
	if !callerEvent.IsInternalCall {
		t.Error("Caller event IsInternalCall should be true")
	}
	if callerEvent.InternalCallRole != "caller" {
		t.Errorf("Caller event role = %s, want 'caller'", callerEvent.InternalCallRole)
	}
	if callerEvent.LinkedCallID != calleeEvent.ID {
		t.Errorf("Caller event LinkedCallID = %s, want %s", callerEvent.LinkedCallID, calleeEvent.ID)
	}

	// Check callee event
	if !calleeEvent.IsInternalCall {
		t.Error("Callee event IsInternalCall should be true")
	}
	if calleeEvent.InternalCallRole != "callee" {
		t.Errorf("Callee event role = %s, want 'callee'", calleeEvent.InternalCallRole)
	}
	if calleeEvent.LinkedCallID != callerEvent.ID {
		t.Errorf("Callee event LinkedCallID = %s, want %s", calleeEvent.LinkedCallID, callerEvent.ID)
	}
}

// TestLinkInternalCallEvents_NilEvents tests handling of nil events
func TestLinkInternalCallEvents_NilEvents(t *testing.T) {
	event := &CallEvent{ID: "test-uuid"}

	// Should not panic with nil events
	LinkInternalCallEvents(nil, nil)
	LinkInternalCallEvents(event, nil)
	LinkInternalCallEvents(nil, event)

	// Event should not be modified when partner is nil
	if event.IsInternalCall {
		t.Error("Event should not be marked as internal call when partner is nil")
	}
}

// TestCallManagerInternalCallLinking tests the call manager's internal call linking
func TestCallManagerInternalCallLinking(t *testing.T) {
	cm := NewCallManager(nil)
	defer cm.Cleanup()

	timestamp := time.Now()

	// Create a CALL event (caller perspective)
	callerEvent := &CallEvent{
		ID:        "caller-id",
		Timestamp: timestamp,
		Type:      CallTypeCall,
		Line:      1,
		CallerMSN: "12345",
		CalledMSN: "67890",
		Caller:    "0123412345",
		Called:    "0123467890",
	}

	// Create a RING event (callee perspective)
	calleeEvent := &CallEvent{
		ID:        "callee-id",
		Timestamp: timestamp.Add(100 * time.Millisecond), // Within 2-second window
		Type:      CallTypeRing,
		Line:      2,
		CallerMSN: "12345",
		CalledMSN: "67890",
		Caller:    "0123412345",
		Called:    "0123467890",
	}

	// Process both events
	callerEvent = cm.ProcessEvent(callerEvent)
	calleeEvent = cm.ProcessEvent(calleeEvent)

	// Both events should be linked
	if !callerEvent.IsInternalCall {
		t.Error("Caller event should be marked as internal call")
	}
	if !calleeEvent.IsInternalCall {
		t.Error("Callee event should be marked as internal call")
	}
	if callerEvent.LinkedCallID != calleeEvent.ID {
		t.Errorf("Caller event LinkedCallID = %s, want %s", callerEvent.LinkedCallID, calleeEvent.ID)
	}
	if calleeEvent.LinkedCallID != callerEvent.ID {
		t.Errorf("Callee event LinkedCallID = %s, want %s", calleeEvent.LinkedCallID, callerEvent.ID)
	}
	if callerEvent.InternalCallRole != "caller" {
		t.Errorf("Caller event role = %s, want 'caller'", callerEvent.InternalCallRole)
	}
	if calleeEvent.InternalCallRole != "callee" {
		t.Errorf("Callee event role = %s, want 'callee'", calleeEvent.InternalCallRole)
	}
}

// TestCallManagerInternalCallSelfCall tests self-call linking (MSN calls same MSN)
func TestCallManagerInternalCallSelfCall(t *testing.T) {
	cm := NewCallManager(nil)
	defer cm.Cleanup()

	timestamp := time.Now()

	// Extension 10 calls Extension 11, both have same MSN "12345"
	callerEvent := &CallEvent{
		ID:        "self-caller-id",
		Timestamp: timestamp,
		Type:      CallTypeCall,
		Line:      1,
		CallerMSN: "12345",
		CalledMSN: "12345", // Same MSN!
		Caller:    "0123412345",
		Called:    "0123412345",
	}

	calleeEvent := &CallEvent{
		ID:        "self-callee-id",
		Timestamp: timestamp.Add(50 * time.Millisecond),
		Type:      CallTypeRing,
		Line:      2,
		CallerMSN: "12345",
		CalledMSN: "12345", // Same MSN!
		Caller:    "0123412345",
		Called:    "0123412345",
	}

	cm.ProcessEvent(callerEvent)
	cm.ProcessEvent(calleeEvent)

	// Should still be linked even with same MSN
	if !callerEvent.IsInternalCall {
		t.Error("Self-call caller event should be marked as internal call")
	}
	if !calleeEvent.IsInternalCall {
		t.Error("Self-call callee event should be marked as internal call")
	}
	if callerEvent.LinkedCallID != calleeEvent.ID {
		t.Error("Self-call events should be linked")
	}
}

// TestCallManagerInternalCallTimeout tests that unmatched events are cleaned up
func TestCallManagerInternalCallTimeout(t *testing.T) {
	cm := NewCallManager(nil)
	defer cm.Cleanup()

	event := &CallEvent{
		ID:        "unmatched-id",
		Timestamp: time.Now(),
		Type:      CallTypeCall,
		Line:      1,
		CallerMSN: "12345",
		CalledMSN: "67890",
	}

	cm.ProcessEvent(event)

	// Check that event is in pending map
	cm.mu.RLock()
	pendingCount := len(cm.pendingInternalCalls)
	cm.mu.RUnlock()

	if pendingCount != 1 {
		t.Errorf("Expected 1 pending internal call, got %d", pendingCount)
	}

	// Wait for cleanup (5 seconds + buffer)
	time.Sleep(6 * time.Second)

	// Check that pending map is cleaned up
	cm.mu.RLock()
	pendingCount = len(cm.pendingInternalCalls)
	cm.mu.RUnlock()

	if pendingCount != 0 {
		t.Errorf("Expected 0 pending internal calls after timeout, got %d", pendingCount)
	}
}

// TestCallManagerExternalCallNotLinked tests that external calls are not linked
func TestCallManagerExternalCallNotLinked(t *testing.T) {
	cm := NewCallManager(nil)
	defer cm.Cleanup()

	timestamp := time.Now()

	// External incoming call (no caller MSN)
	event1 := &CallEvent{
		ID:        "external-1",
		Timestamp: timestamp,
		Type:      CallTypeRing,
		Line:      1,
		CallerMSN: "",
		CalledMSN: "12345",
		Caller:    "+491234567890",
		Called:    "0123412345",
	}

	// External outgoing call (no called MSN)
	event2 := &CallEvent{
		ID:        "external-2",
		Timestamp: timestamp,
		Type:      CallTypeCall,
		Line:      2,
		CallerMSN: "12345",
		CalledMSN: "",
		Caller:    "0123412345",
		Called:    "+491234567890",
	}

	cm.ProcessEvent(event1)
	cm.ProcessEvent(event2)

	// Neither event should be marked as internal
	if event1.IsInternalCall {
		t.Error("External call should not be marked as internal call")
	}
	if event2.IsInternalCall {
		t.Error("External call should not be marked as internal call")
	}
	if event1.LinkedCallID != "" {
		t.Error("External call should not have LinkedCallID")
	}
	if event2.LinkedCallID != "" {
		t.Error("External call should not have LinkedCallID")
	}
}
