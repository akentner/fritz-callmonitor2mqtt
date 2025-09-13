package callmonitor

import (
	"testing"
)

func TestMSNDetectionInCallEvents(t *testing.T) {
	msns := []string{"990133", "990134", "3698237"}
	client := NewClient("test.host", 1012, nil, "49", "6181", msns)

	tests := []struct {
		name           string
		input          string
		expectedCaller string
		expectedCalled string
	}{
		{
			name:           "incoming call with MSN in caller number",
			input:          "09.09.25 15:30:45;RING;0;+4961813698237;+49123456789;SIP0",
			expectedCaller: "3698237",
			expectedCalled: "",
		},
		{
			name:           "incoming call with MSN in called number",
			input:          "09.09.25 15:30:45;RING;0;+49123456789;+4961813698237;SIP0",
			expectedCaller: "",
			expectedCalled: "3698237",
		},
		{
			name:           "outgoing call with MSN in caller number",
			input:          "09.09.25 15:30:45;CALL;1;2;+49618133698237;+49123456789;SIP1",
			expectedCaller: "3698237",
			expectedCalled: "",
		},
		{
			name:           "call between two MSNs",
			input:          "09.09.25 15:30:45;RING;0;+496181990133;+496181990134;SIP0",
			expectedCaller: "990133",
			expectedCalled: "990134",
		},
		{
			name:           "external call with no MSN",
			input:          "09.09.25 15:30:45;RING;0;+49123456789;+49987654321;SIP0",
			expectedCaller: "",
			expectedCalled: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			event, err := client.parseEvent(tt.input)
			if err != nil {
				t.Fatalf("Failed to parse event: %v", err)
			}

			if event.CallerMSN != tt.expectedCaller {
				t.Errorf("CallerMSN = %q, expected %q", event.CallerMSN, tt.expectedCaller)
			}

			if event.CalledMSN != tt.expectedCalled {
				t.Errorf("CalledMSN = %q, expected %q", event.CalledMSN, tt.expectedCalled)
			}
		})
	}
}

func TestMSNDetectionInConnectEvents(t *testing.T) {
	msns := []string{"990133", "990134", "3698237"}
	client := NewClient("test.host", 1012, nil, "49", "6181", msns)

	// First simulate a RING event to set up the line mapping
	ringEvent, err := client.parseEvent("09.09.25 15:30:45;RING;1;+49123456789;+4961813698237;SIP1")
	if err != nil {
		t.Fatalf("Failed to parse RING event: %v", err)
	}

	// Now test the CONNECT event
	connectEvent, err := client.parseEvent("09.09.25 15:30:50;CONNECT;1;23;+49123456789")
	if err != nil {
		t.Fatalf("Failed to parse CONNECT event: %v", err)
	}

	// Verify that MSN information is carried over
	if connectEvent.CalledMSN != "3698237" {
		t.Errorf("CONNECT CalledMSN = %q, expected %q", connectEvent.CalledMSN, "3698237")
	}

	// Verify that the IDs match
	if connectEvent.ID != ringEvent.ID {
		t.Errorf("CONNECT event ID doesn't match RING event ID")
	}
}

func TestMSNDetectionInDisconnectEvents(t *testing.T) {
	msns := []string{"990133", "990134", "3698237"}
	client := NewClient("test.host", 1012, nil, "49", "6181", msns)

	// First simulate a CALL event to set up the line mapping
	callEvent, err := client.parseEvent("09.09.25 15:30:45;CALL;2;1;+496181990133;+49123456789;SIP2")
	if err != nil {
		t.Fatalf("Failed to parse CALL event: %v", err)
	}

	// Now test the DISCONNECT event
	disconnectEvent, err := client.parseEvent("09.09.25 15:33:45;DISCONNECT;2;180")
	if err != nil {
		t.Fatalf("Failed to parse DISCONNECT event: %v", err)
	}

	// Verify that MSN information is carried over
	if disconnectEvent.CallerMSN != "990133" {
		t.Errorf("DISCONNECT CallerMSN = %q, expected %q", disconnectEvent.CallerMSN, "990133")
	}

	// Verify that the IDs match
	if disconnectEvent.ID != callEvent.ID {
		t.Errorf("DISCONNECT event ID doesn't match CALL event ID")
	}

	// Verify duration is set
	if disconnectEvent.Duration != 180 {
		t.Errorf("DISCONNECT duration = %d, expected %d", disconnectEvent.Duration, 180)
	}
}
