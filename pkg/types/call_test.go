package types

import (
	"testing"
	"time"
)

func TestCallHistoryAddCall(t *testing.T) {
	history := &CallHistory{
		Calls:   make([]CallEvent, 0),
		MaxSize: 3,
	}

	// Add first call
	event1 := CallEvent{
		Timestamp: time.Now(),
		Type:      CallTypeRing,
		Line:      1,
		Caller:    "123456789",
		Called:    "987654321",
	}
	history.AddCall(event1)

	if len(history.Calls) != 1 {
		t.Errorf("Expected 1 call, got %d", len(history.Calls))
	}

	// Add more calls
	event2 := CallEvent{
		Timestamp: time.Now(),
		Type:      CallTypeCall,
		Line:      2,
	}
	event3 := CallEvent{
		Timestamp: time.Now(),
		Type:      CallTypeConnect,
		Line:      3,
	}
	event4 := CallEvent{
		Timestamp: time.Now(),
		Type:      CallTypeDisconnect,
		Line:      4,
	}

	history.AddCall(event2)
	history.AddCall(event3)
	history.AddCall(event4)

	// Should only keep last 3 calls
	if len(history.Calls) != 3 {
		t.Errorf("Expected 3 calls, got %d", len(history.Calls))
	}

	// Should be in reverse chronological order (newest first)
	if history.Calls[0].Line != 4 {
		t.Errorf("Expected newest call first, got ID %d", history.Calls[0].Line)
	}

	if history.Calls[2].Line != 2 {
		t.Errorf("Expected oldest call last, got ID %d", history.Calls[2].Line)
	}
}

func TestCallTypeConstants(t *testing.T) {
	tests := []struct {
		callType CallType
		expected string
	}{
		{CallTypeRing, "ring"},
		{CallTypeCall, "call"},
		{CallTypeConnect, "connect"},
		{CallTypeDisconnect, "disconnect"},
	}

	for _, tt := range tests {
		if string(tt.callType) != tt.expected {
			t.Errorf("Expected %s, got %s", tt.expected, string(tt.callType))
		}
	}
}

func TestCallStatusConstants(t *testing.T) {
	tests := []struct {
		status   CallStatus
		expected string
	}{
		{CallStatusIdle, "idle"},
		{CallStatusRinging, "ringing"},
		{CallStatusCalling, "calling"},
		{CallStatusTalking, "talking"},
		{CallStatusNotReached, "notReached"},
		{CallStatusMissedCall, "missedCall"},
		{CallStatusFinished, "finished"},
		{CallStatusMessageBox, "messageBox"},
	}

	for _, tt := range tests {
		if string(tt.status) != tt.expected {
			t.Errorf("Expected %s, got %s", tt.expected, string(tt.status))
		}
	}
}

func TestServiceStatus(t *testing.T) {
	// Test online status
	onlineStatus := ServiceStatus{
		State:       "online",
		LastChanged: time.Now(),
	}

	if onlineStatus.State != "online" {
		t.Errorf("Expected state 'online', got '%s'", onlineStatus.State)
	}

	// Test offline status
	offlineStatus := ServiceStatus{
		State:       "offline",
		LastChanged: time.Now(),
	}

	if offlineStatus.State != "offline" {
		t.Errorf("Expected state 'offline', got '%s'", offlineStatus.State)
	}

	// Test time field
	timeDiff := time.Since(offlineStatus.LastChanged)
	if timeDiff > 1*time.Second {
		t.Errorf("LastChanged timestamp seems incorrect: %v", timeDiff)
	}
}

func TestDetectMSN(t *testing.T) {
	msns := []string{"990133", "990134", "990135", "3698237"}

	tests := []struct {
		name        string
		phoneNumber string
		msns        []string
		expected    string
	}{
		{
			name:        "exact match with MSN",
			phoneNumber: "990133",
			msns:        msns,
			expected:    "990133",
		},
		{
			name:        "phone number ends with MSN",
			phoneNumber: "+4961813698237",
			msns:        msns,
			expected:    "3698237",
		},
		{
			name:        "full number ending with MSN",
			phoneNumber: "0618133698237",
			msns:        msns,
			expected:    "3698237",
		},
		{
			name:        "no match",
			phoneNumber: "123456789",
			msns:        msns,
			expected:    "",
		},
		{
			name:        "empty phone number",
			phoneNumber: "",
			msns:        msns,
			expected:    "",
		},
		{
			name:        "empty MSN list",
			phoneNumber: "990133",
			msns:        []string{},
			expected:    "",
		},
		{
			name:        "MSN with country code",
			phoneNumber: "+49618133698237",
			msns:        msns,
			expected:    "3698237",
		},
		{
			name:        "shorter MSN match",
			phoneNumber: "61819990134",
			msns:        msns,
			expected:    "990134",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := DetectMSN(tt.phoneNumber, tt.msns)
			if result != tt.expected {
				t.Errorf("DetectMSN(%s, %v) = %s, expected %s",
					tt.phoneNumber, tt.msns, result, tt.expected)
			}
		})
	}
}

func TestCallEvent_EnrichWithMSNs(t *testing.T) {
	msns := []string{"990133", "990134", "3698237"}

	tests := []struct {
		name           string
		event          CallEvent
		msns           []string
		expectedCaller string
		expectedCalled string
	}{
		{
			name: "caller has MSN",
			event: CallEvent{
				Caller: "+4961813698237",
				Called: "0123456789",
			},
			msns:           msns,
			expectedCaller: "3698237",
			expectedCalled: "",
		},
		{
			name: "called has MSN",
			event: CallEvent{
				Caller: "0123456789",
				Called: "990134",
			},
			msns:           msns,
			expectedCaller: "",
			expectedCalled: "990134",
		},
		{
			name: "both have MSN",
			event: CallEvent{
				Caller: "990133",
				Called: "990134",
			},
			msns:           msns,
			expectedCaller: "990133",
			expectedCalled: "990134",
		},
		{
			name: "neither has MSN",
			event: CallEvent{
				Caller: "0123456789",
				Called: "0987654321",
			},
			msns:           msns,
			expectedCaller: "",
			expectedCalled: "",
		},
		{
			name: "empty numbers",
			event: CallEvent{
				Caller: "",
				Called: "",
			},
			msns:           msns,
			expectedCaller: "",
			expectedCalled: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Make a copy to avoid modifying the original
			event := tt.event
			event.EnrichWithMSNs(tt.msns)

			if event.CallerMSN != tt.expectedCaller {
				t.Errorf("CallerMSN = %s, expected %s", event.CallerMSN, tt.expectedCaller)
			}
			if event.CalledMSN != tt.expectedCalled {
				t.Errorf("CalledMSN = %s, expected %s", event.CalledMSN, tt.expectedCalled)
			}
		})
	}
}

func TestFinishStateTracking(t *testing.T) {
	fsm := NewCallStateMachine(nil)

	// Test sequence: Ring -> Disconnect (missed call) -> timeout to Idle
	fsm.ProcessEvent(CallTypeRing)
	fsm.ProcessEvent(CallTypeDisconnect) // This triggers MissedCall

	// Wait for timeout transition to idle
	time.Sleep(1100 * time.Millisecond)

	finishState := fsm.GetFinishState()
	if finishState == nil || *finishState != "missedCall" {
		t.Errorf("Expected finish state 'missedCall', got %v", finishState)
	}

	// Test sequence: Call -> Disconnect (not reached) -> timeout to Idle
	fsm2 := NewCallStateMachine(nil)
	fsm2.ProcessEvent(CallTypeCall)
	fsm2.ProcessEvent(CallTypeDisconnect) // This triggers NotReached

	// Wait for timeout transition to idle
	time.Sleep(1100 * time.Millisecond)

	finishState2 := fsm2.GetFinishState()
	if finishState2 == nil || *finishState2 != "notReached" {
		t.Errorf("Expected finish state 'notReached', got %v", finishState2)
	}

	// Test sequence: Ring -> Connect -> Disconnect (finished) -> timeout to Idle
	fsm3 := NewCallStateMachine(nil)
	fsm3.ProcessEvent(CallTypeRing)
	fsm3.ProcessEvent(CallTypeConnect)
	fsm3.ProcessEvent(CallTypeDisconnect) // This triggers Finished

	// Wait for timeout transition to idle
	time.Sleep(1100 * time.Millisecond)

	finishState3 := fsm3.GetFinishState()
	if finishState3 == nil || *finishState3 != "finished" {
		t.Errorf("Expected finish state 'finished', got %v", finishState3)
	}
}
