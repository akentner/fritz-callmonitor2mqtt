package types

import "time"

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
	CallStatusIdle   CallStatus = "idle"
	CallStatusRing   CallStatus = "ring"
	CallStatusActive CallStatus = "active"
)

// CallDirection represents the direction of a call
type CallDirection string

const (
	CallDirectionInbound  CallDirection = "inbound"
	CallDirectionOutbound CallDirection = "outbound"
)

// CallEvent represents a single call monitor event from Fritz!Box
type CallEvent struct {
	Timestamp  time.Time     `json:"timestamp"`
	Type       CallType      `json:"type"`
	Direction  CallDirection `json:"direction"`           // Call direction (inbound/outbound)
	Line       int           `json:"line"`                // Line ID
	Trunk      string        `json:"trunk"`               // SIP line ID
	Extension  string        `json:"extension,omitempty"` // Internal extension (e.g., "1", "2")
	Caller     string        `json:"caller"`              // Calling number
	Called     string        `json:"called"`              // Called number
	Duration   int           `json:"duration,omitempty"`  // Duration in seconds (for end events)
	RawMessage string        `json:"raw_message"`         // Original Fritz!Box message
}

// LineStatus represents the current status of a phone line
type LineStatus struct {
	Line         int           `json:"line"`
	Trunk        string        `json:"trunk"`
	Direction    CallDirection `json:"direction"`
	Extension    string        `json:"extension"`
	Status       CallStatus    `json:"status"`
	CurrentCall  *CallEvent    `json:"current_call,omitempty"`
	LastActivity time.Time     `json:"last_updated"`
}

// CallHistory represents a list of recent calls
type CallHistory struct {
	Calls     []CallEvent `json:"calls"`
	MaxSize   int         `json:"max_size"`
	UpdatedAt time.Time   `json:"updated_at"`
}

// AddCall adds a new call to the history, maintaining the maximum size
func (ch *CallHistory) AddCall(event CallEvent) {
	ch.Calls = append([]CallEvent{event}, ch.Calls...)
	if len(ch.Calls) > ch.MaxSize {
		ch.Calls = ch.Calls[:ch.MaxSize]
	}
	ch.UpdatedAt = time.Now()
}
