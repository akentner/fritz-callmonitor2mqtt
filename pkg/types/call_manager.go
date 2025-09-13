package types

import (
	"fmt"
	"log"
	"time"
)

// CallManager demonstrates how to use the LineStateMachine for call management
type CallManager struct {
	lineStateMachine *LineStateMachine
	onStatusChange   func(line int, oldStatus, newStatus CallStatus, event *CallEvent)
	mqttPublisher    MQTTPublisher
}

// NewCallManager creates a new call manager with FSM
func NewCallManager(onStatusChange func(line int, oldStatus, newStatus CallStatus, event *CallEvent)) *CallManager {
	cm := &CallManager{
		onStatusChange: onStatusChange,
	}

	cm.lineStateMachine = NewLineStateMachine(func(line int, oldState, newState CallStatus) {
		log.Printf("Line %d: %s -> %s", line, oldState, newState)
		if cm.onStatusChange != nil {
			cm.onStatusChange(line, oldState, newState, nil)
		}
	})

	return cm
}

// NewCallManagerWithMQTT creates a new call manager with MQTT publishing support
func NewCallManagerWithMQTT(mqttPublisher MQTTPublisher, onStatusChange func(line int, oldStatus, newStatus CallStatus, event *CallEvent)) *CallManager {
	cm := &CallManager{
		onStatusChange: onStatusChange,
		mqttPublisher:  mqttPublisher,
	}

	cm.lineStateMachine = NewLineStateMachineWithMQTT(mqttPublisher, func(line int, oldState, newState CallStatus) {
		log.Printf("Line %d: %s -> %s", line, oldState, newState)
		if cm.onStatusChange != nil {
			cm.onStatusChange(line, oldState, newState, nil)
		}
		// For timeout transitions, also publish line status update to MQTT
		if oldState != newState && cm.mqttPublisher != nil {
			if err := cm.mqttPublisher.PublishTimeoutStatusUpdate(line, newState); err != nil {
				log.Printf("Failed to publish timeout status update: %v", err)
			}
		}
	})

	return cm
}

// ProcessEvent processes a call event and returns the updated event with status
func (cm *CallManager) ProcessEvent(event *CallEvent) *CallEvent {
	// Validate event
	if err := cm.validateEvent(event); err != nil {
		log.Printf("Invalid event: %v", err)
		return event
	}

	// Process through FSM
	oldStatus := cm.lineStateMachine.GetLineState(event.Line)
	newStatus := cm.lineStateMachine.ProcessCallEvent(event)

	// Update event with current FSM status and finish state
	event.Status = newStatus
	event.FinishState = cm.lineStateMachine.GetLineFinishState(event.Line)

	// Log transition if status changed
	if oldStatus != newStatus {
		log.Printf("Event processed - Line %d: %s -> %s (Event: %s)",
			event.Line, oldStatus, newStatus, event.Type)

		if cm.onStatusChange != nil {
			cm.onStatusChange(event.Line, oldStatus, newStatus, event)
		}
	}

	return event
}

// validateEvent performs basic validation on call events
func (cm *CallManager) validateEvent(event *CallEvent) error {
	if event == nil {
		return fmt.Errorf("event cannot be nil")
	}

	if event.Line < 0 {
		return fmt.Errorf("invalid line number: %d", event.Line)
	}

	if event.Type == "" {
		return fmt.Errorf("event type cannot be empty")
	}

	// Check if transition is valid
	if !cm.lineStateMachine.IsValidTransition(event.Line, event.Type) {
		currentState := cm.lineStateMachine.GetLineState(event.Line)
		return fmt.Errorf("invalid transition: %s event not allowed in %s state for line %d",
			event.Type, currentState, event.Line)
	}

	return nil
}

// GetLineStatus returns the current status of a line
func (cm *CallManager) GetLineStatus(line int) CallStatus {
	return cm.lineStateMachine.GetLineState(line)
}

// GetAllLineStatuses returns the current status of all lines
func (cm *CallManager) GetAllLineStatuses() map[int]CallStatus {
	return cm.lineStateMachine.GetAllLineStates()
}

// ResetLine resets a specific line to idle
func (cm *CallManager) ResetLine(line int) {
	cm.lineStateMachine.ResetLine(line)
}

// SetMQTTPublisher sets the MQTT publisher for status changes
func (cm *CallManager) SetMQTTPublisher(publisher MQTTPublisher) {
	cm.mqttPublisher = publisher
	cm.lineStateMachine.SetMQTTPublisher(publisher)
}

// GetActiveLines returns all lines that have active state machines
func (cm *CallManager) GetActiveLines() []int {
	return cm.lineStateMachine.GetActiveLines()
}

// GetStatusSummary returns a formatted summary of all line statuses
func (cm *CallManager) GetStatusSummary() string {
	return cm.lineStateMachine.GetLineStateSummary()
}

// GetAllFSMStatuses returns FSM status messages for all active lines
func (cm *CallManager) GetAllFSMStatuses() []FSMStatusMessage {
	return cm.lineStateMachine.GetAllFSMStatuses()
}

// Cleanup cleans up resources
func (cm *CallManager) Cleanup() {
	cm.lineStateMachine.Cleanup()
}

// SimulateCall demonstrates a complete call flow
func (cm *CallManager) SimulateCall(line int, direction CallDirection, caller, called string) {
	log.Printf("=== Simulating %s call on line %d ===", direction, line)

	var events []*CallEvent

	if direction == CallDirectionInbound {
		// Incoming call: RING -> CONNECT -> DISCONNECT
		events = []*CallEvent{
			{Type: CallTypeRing, Line: line, Direction: direction, Caller: caller, Called: called, Timestamp: time.Now()},
			{Type: CallTypeConnect, Line: line, Direction: direction, Caller: caller, Called: called, Timestamp: time.Now().Add(5 * time.Second)},
			{Type: CallTypeDisconnect, Line: line, Direction: direction, Caller: caller, Called: called, Timestamp: time.Now().Add(65 * time.Second), Duration: 60},
		}
	} else {
		// Outgoing call: CALL -> CONNECT -> DISCONNECT
		events = []*CallEvent{
			{Type: CallTypeCall, Line: line, Direction: direction, Caller: caller, Called: called, Timestamp: time.Now()},
			{Type: CallTypeConnect, Line: line, Direction: direction, Caller: caller, Called: called, Timestamp: time.Now().Add(3 * time.Second)},
			{Type: CallTypeDisconnect, Line: line, Direction: direction, Caller: caller, Called: called, Timestamp: time.Now().Add(45 * time.Second), Duration: 42},
		}
	}

	// Process events with delays
	for i, event := range events {
		if i > 0 {
			time.Sleep(100 * time.Millisecond) // Small delay between events
		}
		cm.ProcessEvent(event)
	}

	log.Printf("Call simulation completed")
}

// SimulateMissedCall demonstrates a missed call flow
func (cm *CallManager) SimulateMissedCall(line int) {
	log.Printf("=== Simulating missed call on line %d ===", line)

	events := []*CallEvent{
		{Type: CallTypeRing, Line: line, Direction: CallDirectionInbound, Caller: "01234567890", Called: "987654321", Timestamp: time.Now()},
		{Type: CallTypeDisconnect, Line: line, Direction: CallDirectionInbound, Caller: "01234567890", Called: "987654321", Timestamp: time.Now().Add(15 * time.Second)},
	}

	for i, event := range events {
		if i > 0 {
			time.Sleep(100 * time.Millisecond)
		}
		cm.ProcessEvent(event)
	}

	// Wait for timeout
	log.Printf("Waiting for timeout transition...")
	time.Sleep(1200 * time.Millisecond)
	log.Printf("Final status: %s", cm.GetLineStatus(line))
}

// SimulateNotReachedCall demonstrates a call that is not reached
func (cm *CallManager) SimulateNotReachedCall(line int) {
	log.Printf("=== Simulating not reached call on line %d ===", line)

	events := []*CallEvent{
		{Type: CallTypeCall, Line: line, Direction: CallDirectionOutbound, Caller: "987654321", Called: "01234567890", Timestamp: time.Now()},
		{Type: CallTypeDisconnect, Line: line, Direction: CallDirectionOutbound, Caller: "987654321", Called: "01234567890", Timestamp: time.Now().Add(10 * time.Second)},
	}

	for i, event := range events {
		if i > 0 {
			time.Sleep(100 * time.Millisecond)
		}
		cm.ProcessEvent(event)
	}

	// Wait for timeout
	log.Printf("Waiting for timeout transition...")
	time.Sleep(1200 * time.Millisecond)
	log.Printf("Final status: %s", cm.GetLineStatus(line))
}

// Example usage function
func ExampleCallManager() {
	// Create call manager with status change handler
	cm := NewCallManager(func(line int, oldStatus, newStatus CallStatus, event *CallEvent) {
		log.Printf("Status change notification: Line %d changed from %s to %s", line, oldStatus, newStatus)
		if event != nil {
			log.Printf("  Triggered by: %s event", event.Type)
		}
	})
	defer cm.Cleanup()

	// Simulate different call scenarios
	cm.SimulateCall(1, CallDirectionInbound, "01234567890", "987654321")
	cm.SimulateCall(2, CallDirectionOutbound, "987654321", "01234567890")
	cm.SimulateMissedCall(3)
	cm.SimulateNotReachedCall(4)

	// Print final summary
	log.Printf("Final state summary:")
	log.Printf("%s", cm.GetStatusSummary())
}
