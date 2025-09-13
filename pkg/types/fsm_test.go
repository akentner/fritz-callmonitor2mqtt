package types

import (
	"sync"
	"testing"
	"time"
)

func TestNewCallStateMachine(t *testing.T) {
	fsm := NewCallStateMachine(nil)
	if fsm.GetState() != CallStatusIdle {
		t.Errorf("Expected initial state to be idle, got %v", fsm.GetState())
	}
}

func TestBasicStateTransitions(t *testing.T) {
	tests := []struct {
		name          string
		initialState  CallStatus
		eventType     CallType
		expectedState CallStatus
	}{
		// From idle
		{"idle -> ringing on RING", CallStatusIdle, CallTypeRing, CallStatusRinging},
		{"idle -> calling on CALL", CallStatusIdle, CallTypeCall, CallStatusCalling},
		{"idle stays idle on CONNECT", CallStatusIdle, CallTypeConnect, CallStatusIdle},
		{"idle stays idle on DISCONNECT", CallStatusIdle, CallTypeDisconnect, CallStatusIdle},

		// From ringing
		{"ringing -> talking on CONNECT", CallStatusRinging, CallTypeConnect, CallStatusTalking},
		{"ringing -> missedCall on DISCONNECT", CallStatusRinging, CallTypeDisconnect, CallStatusMissedCall},
		{"ringing stays ringing on RING", CallStatusRinging, CallTypeRing, CallStatusRinging},
		{"ringing stays ringing on CALL", CallStatusRinging, CallTypeCall, CallStatusRinging},

		// From calling
		{"calling -> talking on CONNECT", CallStatusCalling, CallTypeConnect, CallStatusTalking},
		{"calling -> notReached on DISCONNECT", CallStatusCalling, CallTypeDisconnect, CallStatusNotReached},
		{"calling stays calling on RING", CallStatusCalling, CallTypeRing, CallStatusCalling},
		{"calling stays calling on CALL", CallStatusCalling, CallTypeCall, CallStatusCalling},

		// From talking
		{"talking -> finished on DISCONNECT", CallStatusTalking, CallTypeDisconnect, CallStatusFinished},
		{"talking stays talking on RING", CallStatusTalking, CallTypeRing, CallStatusTalking},
		{"talking stays talking on CALL", CallStatusTalking, CallTypeCall, CallStatusTalking},
		{"talking stays talking on CONNECT", CallStatusTalking, CallTypeConnect, CallStatusTalking},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fsm := NewCallStateMachine(nil)
			// Set initial state manually for testing
			fsm.mu.Lock()
			fsm.currentState = tt.initialState
			fsm.mu.Unlock()

			newState := fsm.ProcessEvent(tt.eventType)
			if newState != tt.expectedState {
				t.Errorf("Expected state %v, got %v", tt.expectedState, newState)
			}

			if fsm.GetState() != tt.expectedState {
				t.Errorf("FSM state should be %v, got %v", tt.expectedState, fsm.GetState())
			}
		})
	}
}

func TestTimeoutTransitions(t *testing.T) {
	tests := []struct {
		name         string
		initialState CallStatus
		hasTimeout   bool
	}{
		{"notReached has timeout", CallStatusNotReached, true},
		{"missedCall has timeout", CallStatusMissedCall, true},
		{"finished has timeout", CallStatusFinished, true},
		{"idle has no timeout", CallStatusIdle, false},
		{"ringing has no timeout", CallStatusRinging, false},
		{"calling has no timeout", CallStatusCalling, false},
		{"talking has no timeout", CallStatusTalking, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var stateChanges []CallStatus
			var mu sync.Mutex

			fsm := NewCallStateMachine(func(oldState, newState CallStatus) {
				mu.Lock()
				stateChanges = append(stateChanges, newState)
				mu.Unlock()
			})

			// Set initial state
			fsm.mu.Lock()
			fsm.currentState = tt.initialState
			fsm.mu.Unlock()
			fsm.handleTimeouts(tt.initialState)

			if tt.hasTimeout {
				// Wait for timeout + some buffer
				time.Sleep(1200 * time.Millisecond)

				mu.Lock()
				changes := stateChanges
				mu.Unlock()

				if len(changes) == 0 {
					t.Errorf("Expected timeout transition, but no state changes occurred")
				} else if changes[len(changes)-1] != CallStatusIdle {
					t.Errorf("Expected timeout to transition to idle, got %v", changes[len(changes)-1])
				}

				if fsm.GetState() != CallStatusIdle {
					t.Errorf("FSM should be in idle state after timeout, got %v", fsm.GetState())
				}
			} else {
				// Wait a bit to ensure no timeout occurs
				time.Sleep(100 * time.Millisecond)

				mu.Lock()
				changes := stateChanges
				mu.Unlock()

				if len(changes) > 0 {
					t.Errorf("Expected no timeout transition, but got state changes: %v", changes)
				}

				if fsm.GetState() != tt.initialState {
					t.Errorf("FSM should remain in %v state, got %v", tt.initialState, fsm.GetState())
				}
			}

			fsm.Cleanup()
		})
	}
}

func TestStateChangeCallback(t *testing.T) {
	var lastOldState, lastNewState CallStatus
	var callbackCount int

	fsm := NewCallStateMachine(func(oldState, newState CallStatus) {
		lastOldState = oldState
		lastNewState = newState
		callbackCount++
	})

	// Test valid transition
	fsm.ProcessEvent(CallTypeRing)

	if callbackCount != 1 {
		t.Errorf("Expected 1 callback call, got %d", callbackCount)
	}
	if lastOldState != CallStatusIdle {
		t.Errorf("Expected old state to be idle, got %v", lastOldState)
	}
	if lastNewState != CallStatusRinging {
		t.Errorf("Expected new state to be ringing, got %v", lastNewState)
	}

	// Test invalid transition (should not trigger callback)
	callbackCount = 0
	fsm.ProcessEvent(CallTypeRing) // Already ringing

	if callbackCount != 0 {
		t.Errorf("Expected 0 callback calls for invalid transition, got %d", callbackCount)
	}

	fsm.Cleanup()
}

func TestIsValidTransition(t *testing.T) {
	tests := []struct {
		name         string
		currentState CallStatus
		eventType    CallType
		expected     bool
	}{
		{"idle -> RING valid", CallStatusIdle, CallTypeRing, true},
		{"idle -> CALL valid", CallStatusIdle, CallTypeCall, true},
		{"idle -> CONNECT invalid", CallStatusIdle, CallTypeConnect, false},
		{"ringing -> CONNECT valid", CallStatusRinging, CallTypeConnect, true},
		{"ringing -> DISCONNECT valid", CallStatusRinging, CallTypeDisconnect, true},
		{"ringing -> RING invalid", CallStatusRinging, CallTypeRing, false},
		{"talking -> DISCONNECT valid", CallStatusTalking, CallTypeDisconnect, true},
		{"talking -> RING invalid", CallStatusTalking, CallTypeRing, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fsm := NewCallStateMachine(nil)
			fsm.mu.Lock()
			fsm.currentState = tt.currentState
			fsm.mu.Unlock()

			result := fsm.IsValidTransition(tt.eventType)
			if result != tt.expected {
				t.Errorf("Expected IsValidTransition to return %v, got %v", tt.expected, result)
			}
		})
	}
}

func TestGetValidTransitions(t *testing.T) {
	tests := []struct {
		name         string
		currentState CallStatus
		expected     []CallType
	}{
		{"idle valid transitions", CallStatusIdle, []CallType{CallTypeRing, CallTypeCall}},
		{"ringing valid transitions", CallStatusRinging, []CallType{CallTypeConnect, CallTypeDisconnect}},
		{"calling valid transitions", CallStatusCalling, []CallType{CallTypeConnect, CallTypeDisconnect}},
		{"talking valid transitions", CallStatusTalking, []CallType{CallTypeDisconnect}},
		{"notReached valid transitions", CallStatusNotReached, []CallType{}},
		{"missedCall valid transitions", CallStatusMissedCall, []CallType{}},
		{"finished valid transitions", CallStatusFinished, []CallType{}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fsm := NewCallStateMachine(nil)
			fsm.mu.Lock()
			fsm.currentState = tt.currentState
			fsm.mu.Unlock()

			validTransitions := fsm.GetValidTransitions()

			if len(validTransitions) != len(tt.expected) {
				t.Errorf("Expected %d valid transitions, got %d", len(tt.expected), len(validTransitions))
				return
			}

			// Check if all expected transitions are present
			for _, expected := range tt.expected {
				found := false
				for _, valid := range validTransitions {
					if valid == expected {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("Expected transition %v not found in valid transitions %v", expected, validTransitions)
				}
			}
		})
	}
}

func TestReset(t *testing.T) {
	var stateChanges []CallStatus
	var mu sync.Mutex

	fsm := NewCallStateMachine(func(oldState, newState CallStatus) {
		mu.Lock()
		stateChanges = append(stateChanges, newState)
		mu.Unlock()
	})

	// Move to talking state
	fsm.ProcessEvent(CallTypeRing)
	fsm.ProcessEvent(CallTypeConnect)

	// Reset
	fsm.Reset()

	if fsm.GetState() != CallStatusIdle {
		t.Errorf("Expected state to be idle after reset, got %v", fsm.GetState())
	}

	mu.Lock()
	lastChange := stateChanges[len(stateChanges)-1]
	mu.Unlock()

	if lastChange != CallStatusIdle {
		t.Errorf("Expected last state change to be idle, got %v", lastChange)
	}

	fsm.Cleanup()
}

func TestConcurrentAccess(t *testing.T) {
	fsm := NewCallStateMachine(nil)
	done := make(chan bool)

	// Start multiple goroutines that access the FSM concurrently
	for i := 0; i < 10; i++ {
		go func() {
			for j := 0; j < 100; j++ {
				fsm.ProcessEvent(CallTypeRing)
				fsm.GetState()
				fsm.IsValidTransition(CallTypeConnect)
				fsm.GetValidTransitions()
				fsm.Reset()
			}
			done <- true
		}()
	}

	// Wait for all goroutines to complete
	for i := 0; i < 10; i++ {
		<-done
	}

	fsm.Cleanup()
}

func TestTimeoutCancellation(t *testing.T) {
	var stateChanges []CallStatus
	var mu sync.Mutex

	fsm := NewCallStateMachine(func(oldState, newState CallStatus) {
		mu.Lock()
		stateChanges = append(stateChanges, newState)
		mu.Unlock()
	})

	// Transition to notReached (which has timeout)
	fsm.mu.Lock()
	fsm.currentState = CallStatusCalling
	fsm.mu.Unlock()
	fsm.ProcessEvent(CallTypeDisconnect) // Should go to notReached

	// Immediately reset before timeout
	time.Sleep(100 * time.Millisecond)
	fsm.Reset()

	// Wait past the original timeout period
	time.Sleep(1200 * time.Millisecond)

	mu.Lock()
	changes := stateChanges
	mu.Unlock()

	// Should have: calling -> notReached -> idle (from reset)
	// Should NOT have additional idle from timeout
	idleCount := 0
	for _, state := range changes {
		if state == CallStatusIdle {
			idleCount++
		}
	}

	if idleCount > 1 {
		t.Errorf("Expected only one transition to idle (from reset), got %d transitions: %v", idleCount, changes)
	}

	fsm.Cleanup()
}
