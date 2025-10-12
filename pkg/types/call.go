package types

import (
	"strings"
	"time"
)

// CallType represents the type of call event
type CallType string

const (
	CallTypeRing       CallType = "ring"
	CallTypeCall       CallType = "call"
	CallTypeConnect    CallType = "connect"
	CallTypeDisconnect CallType = "disconnect"
)

// CallStatus represents the current status of a phone line
type CallStatus string

const (
	CallStatusIdle       CallStatus = "idle"
	CallStatusRinging    CallStatus = "ringing"
	CallStatusCalling    CallStatus = "calling"
	CallStatusTalking    CallStatus = "talking"
	CallStatusNotReached CallStatus = "notReached"
	CallStatusMissedCall CallStatus = "missedCall"
	CallStatusFinished   CallStatus = "finished"
	CallStatusMessageBox CallStatus = "messageBox"
)

// CallDirection represents the direction of a call
type CallDirection string

const (
	CallDirectionInbound  CallDirection = "inbound"
	CallDirectionOutbound CallDirection = "outbound"
)

// ExtensionInfo represents extension information for call participants
type ExtensionInfo struct {
	Number string `json:"number"` // Extension number (e.g., "610", "620")
	Name   string `json:"name"`   // Human-readable name (e.g., "Büro", "Wohnzimmer")
	Type   string `json:"type"`   // Extension type: DECT, VOIP, VOICEBOX, ANALOG, UNKNOWN
}

// CallEvent represents a single call monitor event from Fritz!Box
type CallEvent struct {
	ID              string         `json:"id"` // UUID v7 for tracking calls across states
	Timestamp       time.Time      `json:"timestamp"`
	Type            CallType       `json:"type"`
	Direction       CallDirection  `json:"direction"`                  // Call direction (inbound/outbound)
	Line            int            `json:"line"`                       // Line ID
	Trunk           string         `json:"trunk,omitempty"`            // SIP line ID
	Extension       string         `json:"extension,omitempty"`        // Internal extension (e.g., "1", "2")
	Caller          string         `json:"caller,omitempty"`           // Calling number
	Called          string         `json:"called,omitempty"`           // Called number
	CallerName      string         `json:"caller_name,omitempty"`      // Name associated with calling number
	CalledName      string         `json:"called_name,omitempty"`      // Name associated with called number
	CallerMSN       string         `json:"caller_msn,omitempty"`       // MSN if caller matches configured MSNs
	CalledMSN       string         `json:"called_msn,omitempty"`       // MSN if called matches configured MSNs
	CallerExtension *ExtensionInfo `json:"caller_extension,omitempty"` // Extension info for caller (if internal)
	CalledExtension *ExtensionInfo `json:"called_extension,omitempty"` // Extension info for called (if internal)
	Duration        int            `json:"duration,omitempty"`         // Duration in seconds (for end events)
	Status          CallStatus     `json:"status"`                     // Current FSM status
	FinishState     *CallStatus    `json:"finish_state,omitempty"`     // Final status before idle (missedCall, notReached, finished)
	RawMessage      string         `json:"raw_message,omitempty"`      // Original Fritz!Box message
}

// LineStatus represents the current status of a phone line
type LineStatus struct {
	ID          string                `json:"id"`
	Line        int                   `json:"line"`
	Trunk       string                `json:"trunk"`
	Direction   CallDirection         `json:"direction"`
	Extension   LineStatusExtension   `json:"extension"`
	Status      CallStatus            `json:"status"`
	FinishState *CallStatus           `json:"finish_state,omitempty"` // Final status before idle (missedCall, notReached, finished)
	Caller      LineStatusParticipant `json:"caller"`
	Called      LineStatusParticipant `json:"called"`
	Duration    *int                  `json:"duration,omitempty"`
	LastEvent   string                `json:"last_event"`
	LastUpdated time.Time             `json:"last_updated"`
}

type LineStatusParticipant struct {
	PhoneNumber string `json:"phone_number"`
	Name        string `json:"name"`
}

type LineStatusExtension struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// CallHistory represents a list of recent calls
type CallHistory struct {
	Calls     []CallEvent `json:"calls"`
	MaxSize   int         `json:"max_size"`
	UpdatedAt time.Time   `json:"updated_at"`
}

// MSNCallEvent represents a call event optimized for MSN call history
type MSNCallEvent struct {
	ID          string                `json:"id"`
	Timestamp   time.Time             `json:"timestamp"`
	Direction   CallDirection         `json:"direction"`
	Line        int                   `json:"line"`
	Trunk       string                `json:"trunk,omitempty"`
	Caller      LineStatusParticipant `json:"caller"`
	Called      LineStatusParticipant `json:"called"`
	CallerMSN   string                `json:"caller_msn,omitempty"`
	CalledMSN   string                `json:"called_msn,omitempty"`
	Duration    int                   `json:"duration,omitempty"`
	FinishState string                `json:"finish_state"`
}

// MSNCallHistory represents call history for a specific MSN
type MSNCallHistory struct {
	MSN       string         `json:"msn"`
	Calls     []MSNCallEvent `json:"calls"`
	MaxSize   int            `json:"max_size"`
	UpdatedAt time.Time      `json:"updated_at"`
}

// ServiceStatus represents the online/offline status of the service
type ServiceStatus struct {
	State       string    `json:"state"`        // "online" or "offline"
	LastChanged time.Time `json:"last_changed"` // When the state changed
}

// AddCall adds a new call to the history, maintaining the maximum size
func (ch *CallHistory) AddCall(event CallEvent) {
	ch.Calls = append([]CallEvent{event}, ch.Calls...)
	if len(ch.Calls) > ch.MaxSize {
		ch.Calls = ch.Calls[:ch.MaxSize]
	}
	ch.UpdatedAt = time.Now()
}

// AddCall adds a new call to the MSN-specific history, but only if the call involves this MSN
// and is a completed call (disconnect event). Maintains maximum size with newest calls first.
func (mh *MSNCallHistory) AddCall(event CallEvent) {
	// Only add completed calls (disconnect events) that involve this MSN
	if event.Type != CallTypeDisconnect {
		return
	}

	// Check if this call involves the MSN (either caller or called)
	if event.CallerMSN != mh.MSN && event.CalledMSN != mh.MSN {
		return
	}

	// Convert CallEvent to MSNCallEvent
	msnEvent := MSNCallEvent{
		ID:        event.ID,
		Timestamp: event.Timestamp,
		Direction: event.Direction,
		Line:      event.Line,
		Trunk:     event.Trunk,
		Caller: LineStatusParticipant{
			PhoneNumber: event.Caller,
			Name:        event.CallerName,
		},
		Called: LineStatusParticipant{
			PhoneNumber: event.Called,
			Name:        event.CalledName,
		},
		CallerMSN: event.CallerMSN,
		CalledMSN: event.CalledMSN,
		Duration:  event.Duration,
		// FinishState will be set from event.Status or derived
	}

	// Add to front of slice (newest first)
	mh.Calls = append([]MSNCallEvent{msnEvent}, mh.Calls...)
	if len(mh.Calls) > mh.MaxSize {
		mh.Calls = mh.Calls[:mh.MaxSize]
	}
	mh.UpdatedAt = time.Now()
}

// DetectMSN checks if a phone number ends with one of the configured MSNs
// Returns the matching MSN or empty string if no match found
func DetectMSN(phoneNumber string, msns []string) string {
	if phoneNumber == "" {
		return ""
	}

	for _, msn := range msns {
		if msn != "" && strings.HasSuffix(phoneNumber, msn) {
			return msn
		}
	}
	return ""
}

// EnrichWithMSNs adds MSN information to a CallEvent based on configured MSNs
func (ce *CallEvent) EnrichWithMSNs(msns []string) {
	ce.CallerMSN = DetectMSN(ce.Caller, msns)
	ce.CalledMSN = DetectMSN(ce.Called, msns)
}
