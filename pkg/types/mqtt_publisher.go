package types

import (
	"encoding/json"
	"fmt"
)

// MQTTPublisher interface for publishing FSM status changes via MQTT
type MQTTPublisher interface {
	PublishLineStatusChange(line int, oldStatus, newStatus CallStatus, event *CallEvent) error
	PublishTimeoutStatusUpdate(line int, newStatus CallStatus) error
}

// LineStatusChangeMessage represents an FSM status change message
type LineStatusChangeMessage struct {
	Line      int        `json:"line"`
	OldStatus CallStatus `json:"old_status"`
	NewStatus CallStatus `json:"new_status"`
	Timestamp string     `json:"timestamp"`
	Event     *CallEvent `json:"event,omitempty"`
	Reason    string     `json:"reason,omitempty"` // "event" or "timeout"
}

// FSMStatusMessage represents the current FSM status for MQTT publishing
type FSMStatusMessage struct {
	Line               int        `json:"line"`
	Status             CallStatus `json:"status"`
	Timestamp          string     `json:"timestamp"`
	ValidTransitions   []CallType `json:"valid_transitions,omitempty"`
	IsTimeoutActive    bool       `json:"is_timeout_active"`
	LastEventType      CallType   `json:"last_event_type,omitempty"`
	LastEventTimestamp string     `json:"last_event_timestamp,omitempty"`
}

// ToJSON converts LineStatusChangeMessage to JSON string
func (msg *LineStatusChangeMessage) ToJSON() (string, error) {
	data, err := json.Marshal(msg)
	if err != nil {
		return "", fmt.Errorf("failed to marshal status change message: %w", err)
	}
	return string(data), nil
}

// ToJSON converts FSMStatusMessage to JSON string
func (msg *FSMStatusMessage) ToJSON() (string, error) {
	data, err := json.Marshal(msg)
	if err != nil {
		return "", fmt.Errorf("failed to marshal FSM status message: %w", err)
	}
	return string(data), nil
}
