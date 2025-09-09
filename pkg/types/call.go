package types

import "time"

// CallType represents the type of call event
type CallType string

const (
	CallTypeIncoming CallType = "incoming"
	CallTypeOutgoing CallType = "outgoing"
	CallTypeConnect  CallType = "connect"
	CallTypeEnd      CallType = "end"
)

// CallStatus represents the current status of a phone line
type CallStatus string

const (
	CallStatusIdle   CallStatus = "idle"
	CallStatusRing   CallStatus = "ring"
	CallStatusActive CallStatus = "active"
)

// CallEvent represents a single call monitor event from Fritz!Box
type CallEvent struct {
	Timestamp  time.Time `json:"timestamp"`
	Type       CallType  `json:"type"`
	ID         string    `json:"id"`          // Call ID
	Extension  string    `json:"extension"`   // Internal extension (e.g., "1", "2")
	Caller     string    `json:"caller"`      // Calling number
	Called     string    `json:"called"`      // Called number
	LineID     string    `json:"line_id"`     // SIP line ID
	Duration   int       `json:"duration"`    // Duration in seconds (for end events)
	RawMessage string    `json:"raw_message"` // Original Fritz!Box message
}

// LineStatus represents the current status of a phone line
type LineStatus struct {
	LineID       string     `json:"line_id"`
	Extension    string     `json:"extension"`
	Status       CallStatus `json:"status"`
	CurrentCall  *CallEvent `json:"current_call,omitempty"`
	LastActivity time.Time  `json:"last_activity"`
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
