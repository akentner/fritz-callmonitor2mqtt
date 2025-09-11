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
	CallStatusCall   CallStatus = "call"
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
	ID         string        `json:"id"` // UUID v7 for tracking calls across states
	Timestamp  time.Time     `json:"timestamp"`
	Type       CallType      `json:"type"`
	Direction  CallDirection `json:"direction"`             // Call direction (inbound/outbound)
	Line       int           `json:"line"`                  // Line ID
	Trunk      string        `json:"trunk,omitempty"`       // SIP line ID
	Extension  string        `json:"extension,omitempty"`   // Internal extension (e.g., "1", "2")
	Caller     string        `json:"caller,omitempty"`      // Calling number
	Called     string        `json:"called,omitempty"`      // Called number
	Duration   int           `json:"duration,omitempty"`    // Duration in seconds (for end events)
	RawMessage string        `json:"raw_message,omitempty"` // Original Fritz!Box message
}

// LineStatus represents the current status of a phone line
type LineStatus struct {
	ID          string                `json:"id"`
	Line        int                   `json:"line"`
	Trunk       string                `json:"trunk"`
	Direction   CallDirection         `json:"direction"`
	Extension   LineStatusExtension   `json:"extension"`
	Status      CallStatus            `json:"status"`
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
