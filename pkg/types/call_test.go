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
		{CallStatusCall, "call"},
		{CallStatusRing, "ring"},
		{CallStatusActive, "active"},
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
