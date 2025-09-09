package callmonitor

import (
	"testing"
	"time"

	"fritz-callmonitor2mqtt/pkg/types"
)

func TestParseCallEvent(t *testing.T) {

	tests := []struct {
		name        string
		input       string
		expectError bool
		expected    *types.CallEvent
	}{
		{
			name:  "incoming call",
			input: "21.09.25 15:30:45;RING;0;123456789;987654321;SIP0",
			expected: &types.CallEvent{
				Type:   types.CallTypeRing,
				Line:   0,
				Caller: "+493023456789",
				Called: "+493087654321",
				Trunk:  "SIP0",
			},
		},
		{
			name:  "outgoing call",
			input: "21.09.25 15:31:00;CALL;1;2;987654321;123456789;SIP1",
			expected: &types.CallEvent{
				Type:      types.CallTypeCall,
				Line:      1,
				Extension: "2",
				Caller:    "+493087654321",
				Called:    "+493023456789",
				Trunk:     "SIP1",
			},
		},
		{
			name:  "connect",
			input: "21.09.25 15:31:05;CONNECT;1;2;987654321;123456789",
			expected: &types.CallEvent{
				Type:      types.CallTypeConnect,
				Line:      1,
				Extension: "2",
				Caller:    "", // Will be set from stored state in real lifecycle
				Called:    "", // Will be set from stored state in real lifecycle
			},
		},
		{
			name:  "disconnect with id and duration",
			input: "21.09.25 15:35:00;DISCONNECT;1;235;",
			expected: &types.CallEvent{
				Type:      types.CallTypeDisconnect,
				Line:      1,
				Extension: "",
				Caller:    "",
				Called:    "",
				Trunk:     "", // Empty without prior call setup
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
		{
			name:  "disconnect with minimal fields (real world case)",
			input: "09.09.25 12:50:15;DISCONNECT;1;0;",
			expected: &types.CallEvent{
				Type:      types.CallTypeDisconnect,
				Line:      1,
				Extension: "",
				Caller:    "",
				Called:    "",
				Trunk:     "", // Will be empty without prior call setup
				Duration:  0,
			},
		},
		{
			name:  "disconnect with id and duration (user reported case)",
			input: "09.09.25 13:51:39;DISCONNECT;0;7;",
			expected: &types.CallEvent{
				Type:      types.CallTypeDisconnect,
				Line:      0,
				Extension: "",
				Caller:    "",
				Called:    "",
				Trunk:     "", // Will be empty without prior call setup
				Duration:  7,
			},
		},
		{
			name:  "incoming call with SIP field (real Fritz!Box format)",
			input: "09.09.25 16:27:15;RING;0;01784567890;990134;SIP1;",
			expected: &types.CallEvent{
				Type:   types.CallTypeRing,
				Line:   0,
				Caller: "+491784567890", // 01784567890 normalized
				Called: "+493090134",    // 990134 normalized with area code
				Trunk:  "SIP1",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := NewClient("test.host", 1012, nil, "49", "30") // Fresh client for each test
			result, err := client.parseEvent(tt.input)

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
			if result.Line != tt.expected.Line {
				t.Errorf("Line: expected %d, got %d", tt.expected.Line, result.Line)
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
			if result.Trunk != tt.expected.Trunk {
				t.Errorf("Trunk: expected %s, got %s", tt.expected.Trunk, result.Trunk)
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

func TestCallLifecycleIDMapping(t *testing.T) {
	client := NewClient("test.host", 1012, nil, "49", "30")

	// Test full call lifecycle: RING -> CONNECT -> DISCONNECT
	// This tests that the ID to LineID mapping works correctly

	// 1. Incoming call - should store ID->LineID mapping
	ringEvent, err := client.parseEvent("21.09.25 15:30:45;RING;0;123456789;987654321;SIP0")
	if err != nil {
		t.Fatalf("Failed to parse RING event: %v", err)
	}

	if ringEvent.Line != 0 {
		t.Errorf("RING: Expected ID '0', got '%d'", ringEvent.Line)
	}
	if ringEvent.Trunk != "SIP0" {
		t.Errorf("RING: Expected Trunk 'SIP0', got '%s'", ringEvent.Trunk)
	}

	// 2. Call connected - should also maintain the mapping
	connectEvent, err := client.parseEvent("21.09.25 15:30:50;CONNECT;0;1;123456789;987654321")
	if err != nil {
		t.Fatalf("Failed to parse CONNECT event: %v", err)
	}

	if connectEvent.Line != 0 {
		t.Errorf("CONNECT: Expected ID '0', got '%d'", connectEvent.Line)
	}

	// 3. Call disconnected - should use stored LineID based on ID
	disconnectEvent, err := client.parseEvent("21.09.25 15:35:00;DISCONNECT;0;235;")
	if err != nil {
		t.Fatalf("Failed to parse DISCONNECT event: %v", err)
	}

	if disconnectEvent.Line != 0 {
		t.Errorf("DISCONNECT: Expected ID '0', got '%d'", disconnectEvent.Line)
	}
	if disconnectEvent.Trunk != "SIP0" {
		t.Errorf("DISCONNECT: Expected Trunk 'SIP0' (from stored mapping), got '%s'", disconnectEvent.Trunk)
	}
	if disconnectEvent.Duration != 235 {
		t.Errorf("DISCONNECT: Expected Duration 235, got %d", disconnectEvent.Duration)
	}

	// 4. Verify mapping was cleaned up
	if len(client.lineIdToTrunk) != 0 {
		t.Errorf("Expected callIDToLine map to be empty after disconnect, but has %d entries", len(client.lineIdToTrunk))
	}
}

func TestParseTimestamp(t *testing.T) {
	client := NewClient("test.host", 1012, nil, "49", "30")

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
			result, err := client.parseTimestamp(tt.input)

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

func TestCallIDToLineIDMapping(t *testing.T) {
	client := NewClient("test.host", 1012, nil, "49", "30")

	// First, simulate an incoming call (RING) that establishes the ID-to-LineID mapping
	ringEvent, err := client.parseEvent("09.09.25 13:50:00;RING;0;123456789;987654321;SIP0")
	if err != nil {
		t.Fatalf("Failed to parse RING event: %v", err)
	}

	// Verify the RING event has the correct data
	if ringEvent.Line != 0 {
		t.Errorf("RING: Expected ID '0', got '%d'", ringEvent.Line)
	}
	if ringEvent.Trunk != "SIP0" {
		t.Errorf("RING: Expected Trunk 'SIP0', got '%s'", ringEvent.Trunk)
	}

	// Now simulate the corresponding DISCONNECT with the same ID
	disconnectEvent, err := client.parseEvent("09.09.25 13:51:39;DISCONNECT;0;7;")
	if err != nil {
		t.Fatalf("Failed to parse DISCONNECT event: %v", err)
	}

	// Verify the DISCONNECT event now has the LineID from the RING event
	if disconnectEvent.Line != 0 {
		t.Errorf("DISCONNECT: Expected ID '0', got '%d'", disconnectEvent.Line)
	}
	if disconnectEvent.Trunk != "SIP0" {
		t.Errorf("DISCONNECT: Expected Trunk 'SIP0' (from RING), got '%s'", disconnectEvent.Trunk)
	}
	if disconnectEvent.Duration != 7 {
		t.Errorf("DISCONNECT: Expected Duration 7, got %d", disconnectEvent.Duration)
	}

	// Verify the mapping has been cleaned up after DISCONNECT
	if len(client.lineIdToTrunk) != 0 {
		t.Errorf("Expected callIDToLine map to be empty after DISCONNECT, but has %d entries", len(client.lineIdToTrunk))
	}
}

func TestMultipleCallIDMappings(t *testing.T) {
	client := NewClient("test.host", 1012, nil, "49", "30")

	// Simulate multiple concurrent calls
	// Call 1: RING
	_, err := client.parseEvent("09.09.25 13:50:00;RING;0;111111111;222222222;SIP0")
	if err != nil {
		t.Fatalf("Failed to parse RING event for call 0: %v", err)
	}

	// Call 2: CALL (outgoing)
	_, err = client.parseEvent("09.09.25 13:50:30;CALL;1;2;333333333;444444444;SIP1")
	if err != nil {
		t.Fatalf("Failed to parse CALL event for call 1: %v", err)
	}

	// Verify both mappings are stored
	if len(client.lineIdToTrunk) != 2 {
		t.Errorf("Expected 2 mappings, got %d", len(client.lineIdToTrunk))
	}

	// End call 0
	disconnect0, err := client.parseEvent("09.09.25 13:51:00;DISCONNECT;0;60;")
	if err != nil {
		t.Fatalf("Failed to parse DISCONNECT event for call 0: %v", err)
	}

	if disconnect0.Trunk != "SIP0" {
		t.Errorf("DISCONNECT call 0: Expected Trunk 'SIP0', got '%s'", disconnect0.Trunk)
	}

	// Verify only one mapping remains
	if len(client.lineIdToTrunk) != 1 {
		t.Errorf("Expected 1 mapping after first DISCONNECT, got %d", len(client.lineIdToTrunk))
	}

	// End call 1
	disconnect1, err := client.parseEvent("09.09.25 13:52:00;DISCONNECT;1;90;")
	if err != nil {
		t.Fatalf("Failed to parse DISCONNECT event for call 1: %v", err)
	}

	if disconnect1.Trunk != "SIP1" {
		t.Errorf("DISCONNECT call 1: Expected Trunk 'SIP1', got '%s'", disconnect1.Trunk)
	}

	// Verify all mappings are cleaned up
	if len(client.lineIdToTrunk) != 0 {
		t.Errorf("Expected 0 mappings after all DISCONNECTs, got %d", len(client.lineIdToTrunk))
	}
}

func TestTimezoneHandling(t *testing.T) {
	// Create client with Berlin timezone
	berlinTZ, err := time.LoadLocation("Europe/Berlin")
	if err != nil {
		t.Fatalf("Failed to load Berlin timezone: %v", err)
	}

	client := NewClient("test.host", 1012, berlinTZ, "49", "30")

	// Test parsing timestamp with Berlin timezone
	result, err := client.parseTimestamp("21.09.25 15:30:45")
	if err != nil {
		t.Fatalf("Failed to parse timestamp: %v", err)
	}

	// Check that the timestamp was parsed with the correct timezone
	expectedLocation := berlinTZ
	if result.Location() != expectedLocation {
		t.Errorf("Expected timezone %v, got %v", expectedLocation, result.Location())
	}

	// Test with UTC timezone
	utcClient := NewClient("test.host", 1012, time.UTC, "49", "30")
	utcResult, err := utcClient.parseTimestamp("21.09.25 15:30:45")
	if err != nil {
		t.Fatalf("Failed to parse timestamp with UTC: %v", err)
	}

	if utcResult.Location() != time.UTC {
		t.Errorf("Expected UTC timezone, got %v", utcResult.Location())
	}
}
