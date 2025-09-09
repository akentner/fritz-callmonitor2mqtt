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
		Type:      CallTypeIncoming,
		ID:        "1",
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
		Type:      CallTypeOutgoing,
		ID:        "2",
	}
	event3 := CallEvent{
		Timestamp: time.Now(),
		Type:      CallTypeConnect,
		ID:        "3",
	}
	event4 := CallEvent{
		Timestamp: time.Now(),
		Type:      CallTypeEnd,
		ID:        "4",
	}

	history.AddCall(event2)
	history.AddCall(event3)
	history.AddCall(event4)

	// Should only keep last 3 calls
	if len(history.Calls) != 3 {
		t.Errorf("Expected 3 calls, got %d", len(history.Calls))
	}

	// Should be in reverse chronological order (newest first)
	if history.Calls[0].ID != "4" {
		t.Errorf("Expected newest call first, got ID %s", history.Calls[0].ID)
	}

	if history.Calls[2].ID != "2" {
		t.Errorf("Expected oldest call last, got ID %s", history.Calls[2].ID)
	}
}

func TestCallTypeConstants(t *testing.T) {
	tests := []struct {
		callType CallType
		expected string
	}{
		{CallTypeIncoming, "incoming"},
		{CallTypeOutgoing, "outgoing"},
		{CallTypeConnect, "connect"},
		{CallTypeEnd, "end"},
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
		{CallStatusRing, "ring"},
		{CallStatusActive, "active"},
	}

	for _, tt := range tests {
		if string(tt.status) != tt.expected {
			t.Errorf("Expected %s, got %s", tt.expected, string(tt.status))
		}
	}
}
