package callmonitor

import (
	"testing"
	"time"

	"fritz-callmonitor2mqtt/pkg/types"
)

func TestParseCallEvent(t *testing.T) {
	client := NewClient("test.host", 1012)

	tests := []struct {
		name        string
		input       string
		expectError bool
		expected    *types.CallEvent
	}{
		{
			name:  "incoming call",
			input: "21.09.25 15:30:45;RING;0;1;123456789;987654321;SIP0",
			expected: &types.CallEvent{
				Type:      types.CallTypeIncoming,
				ID:        "0",
				Extension: "1",
				Caller:    "123456789",
				Called:    "987654321",
				LineID:    "SIP0",
			},
		},
		{
			name:  "outgoing call",
			input: "21.09.25 15:31:00;CALL;1;2;987654321;123456789;SIP1",
			expected: &types.CallEvent{
				Type:      types.CallTypeOutgoing,
				ID:        "1",
				Extension: "2",
				Caller:    "987654321",
				Called:    "123456789",
				LineID:    "SIP1",
			},
		},
		{
			name:  "connect",
			input: "21.09.25 15:31:05;CONNECT;1;2;987654321;123456789",
			expected: &types.CallEvent{
				Type:      types.CallTypeConnect,
				ID:        "1",
				Extension: "2",
				Caller:    "987654321",
				Called:    "123456789",
			},
		},
		{
			name:  "disconnect with duration",
			input: "21.09.25 15:35:00;DISCONNECT;1;2;987654321;123456789;SIP1;235",
			expected: &types.CallEvent{
				Type:      types.CallTypeEnd,
				ID:        "1",
				Extension: "2",
				Caller:    "987654321",
				Called:    "123456789",
				LineID:    "SIP1",
				Duration:  235,
			},
		},
		{
			name:        "invalid format - too few parts",
			input:       "21.09.25 15:30:45;RING;0",
			expectError: true,
		},
		{
			name:        "invalid call type",
			input:       "21.09.25 15:30:45;UNKNOWN;0;1;123456789;987654321;SIP0",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := client.parseCallEvent(tt.input)

			if tt.expectError {
				if err == nil {
					t.Error("Expected error, but got none")
				}
				return
			}

			if err != nil {
				t.Errorf("Unexpected error: %v", err)
				return
			}

			if result == nil {
				t.Error("Expected result, but got nil")
				return
			}

			// Check main fields
			if result.Type != tt.expected.Type {
				t.Errorf("Type: expected %s, got %s", tt.expected.Type, result.Type)
			}
			if result.ID != tt.expected.ID {
				t.Errorf("ID: expected %s, got %s", tt.expected.ID, result.ID)
			}
			if result.Extension != tt.expected.Extension {
				t.Errorf("Extension: expected %s, got %s", tt.expected.Extension, result.Extension)
			}
			if result.Caller != tt.expected.Caller {
				t.Errorf("Caller: expected %s, got %s", tt.expected.Caller, result.Caller)
			}
			if result.Called != tt.expected.Called {
				t.Errorf("Called: expected %s, got %s", tt.expected.Called, result.Called)
			}
			if result.LineID != tt.expected.LineID {
				t.Errorf("LineID: expected %s, got %s", tt.expected.LineID, result.LineID)
			}
			if result.Duration != tt.expected.Duration {
				t.Errorf("Duration: expected %d, got %d", tt.expected.Duration, result.Duration)
			}
			if result.RawMessage != tt.input {
				t.Errorf("RawMessage: expected %s, got %s", tt.input, result.RawMessage)
			}
		})
	}
}

func TestParseTimestamp(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		expectError bool
	}{
		{
			name:  "valid timestamp",
			input: "21.09.25 15:30:45",
		},
		{
			name:  "valid timestamp with different time",
			input: "21.12.31 23:59:59",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := parseTimestamp(tt.input)

			if tt.expectError && err == nil {
				t.Error("Expected error, but got none")
				return
			}

			if !tt.expectError && err != nil {
				t.Errorf("Unexpected error: %v", err)
				return
			}

			if !tt.expectError {
				// Check that we got a reasonable timestamp
				if result.IsZero() {
					t.Error("Expected non-zero timestamp")
				}

				// Should be within reasonable bounds (not too far in future/past)
				now := time.Now()
				// Allow for a wider range since we're dealing with Fritz!Box timestamp format
				if result.After(now.Add(10*365*24*time.Hour)) || result.Before(now.Add(-50*365*24*time.Hour)) {
					t.Errorf("Timestamp seems unreasonable: %v", result)
				}
			}
		})
	}
}
