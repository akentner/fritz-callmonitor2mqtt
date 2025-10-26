package types

import (
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/google/uuid"
)

// CallManager demonstrates how to use the LineStateMachine for call management
type CallManager struct {
	lineStateMachine     *LineStateMachine
	onStatusChange       func(line int, oldStatus, newStatus CallStatus, event *CallEvent)
	mqttPublisher        MQTTPublisher
	dbPersister          DatabasePersister
	pendingInternalCalls map[string]*CallEvent // Key: CallerMSN-CalledMSN-Timestamp for matching internal calls
	mu                   sync.RWMutex          // Protects pendingInternalCalls
}

// NewCallManager creates a new call manager with FSM
func NewCallManager(onStatusChange func(line int, oldStatus, newStatus CallStatus, event *CallEvent)) *CallManager {
	cm := &CallManager{
		onStatusChange:       onStatusChange,
		pendingInternalCalls: make(map[string]*CallEvent),
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
		onStatusChange:       onStatusChange,
		mqttPublisher:        mqttPublisher,
		pendingInternalCalls: make(map[string]*CallEvent),
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

// NewCallManagerWithMQTTAndDB creates a new call manager with MQTT and database persistence support
func NewCallManagerWithMQTTAndDB(mqttPublisher MQTTPublisher, dbPersister DatabasePersister, onStatusChange func(line int, oldStatus, newStatus CallStatus, event *CallEvent)) *CallManager {
	cm := &CallManager{
		onStatusChange:       onStatusChange,
		mqttPublisher:        mqttPublisher,
		dbPersister:          dbPersister,
		pendingInternalCalls: make(map[string]*CallEvent),
	}

	cm.lineStateMachine = NewLineStateMachineWithMQTTAndDB(mqttPublisher, dbPersister, func(line int, oldState, newState CallStatus) {
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

	// Detect and link internal calls
	if event.DetectInternalCall() {
		cm.linkInternalCallIfPossible(event)
	}

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

// GetLineCallID returns the current call UUID for a specific line
func (cm *CallManager) GetLineCallID(line int) *uuid.UUID {
	return cm.lineStateMachine.GetLineCallID(line)
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

// linkInternalCallIfPossible attempts to link an internal call event with its partner event
// Internal calls generate two events: CALL (from caller) and RING (at callee)
// This method matches them within a 2-second window based on caller/called MSNs and timestamp
func (cm *CallManager) linkInternalCallIfPossible(event *CallEvent) {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	// Try to find a matching partner event
	// Check timestamps within ±2 seconds window
	var matchedKey string
	var partner *CallEvent

	// Search through pending events for a match
	for key, pendingEvent := range cm.pendingInternalCalls {
		// Must have same MSNs
		if pendingEvent.CallerMSN != event.CallerMSN || pendingEvent.CalledMSN != event.CalledMSN {
			continue
		}

		// Must be within 2 second window
		timeDiff := event.Timestamp.Sub(pendingEvent.Timestamp)
		if timeDiff < 0 {
			timeDiff = -timeDiff
		}
		if timeDiff > 2*time.Second {
			continue
		}

		// Found a match!
		matchedKey = key
		partner = pendingEvent
		break
	}

	if partner != nil {
		// Link the events
		LinkInternalCallEvents(partner, event)

		// Remove from pending
		delete(cm.pendingInternalCalls, matchedKey)

		log.Printf("Linked internal call: %s (%s) <-> %s (%s)",
			partner.ID, partner.Type, event.ID, event.Type)

		// Update partner event in database if persister is available
		if cm.dbPersister != nil {
			partnerCallID, err := uuid.Parse(partner.ID)
			if err == nil {
				if err := cm.dbPersister.UpdateCall(partnerCallID, partner.Status, partner.FinishState, partner); err != nil {
					log.Printf("Failed to update linked partner event in database: %v", err)
				}
			}
		}
	} else {
		// No match yet, store this event as pending
		// Use event ID as key since it's unique
		key := event.ID
		cm.pendingInternalCalls[key] = event

		// Schedule cleanup of old pending events after 5 seconds
		go cm.cleanupOldPendingCall(key, 5*time.Second)
	}
}

// cleanupOldPendingCall removes a pending internal call after a timeout
// This prevents the map from growing indefinitely with unmatched events
func (cm *CallManager) cleanupOldPendingCall(key string, delay time.Duration) {
	time.Sleep(delay)

	cm.mu.Lock()
	defer cm.mu.Unlock()

	if event, exists := cm.pendingInternalCalls[key]; exists {
		log.Printf("Cleaning up unmatched internal call event: %s (type: %s, caller_msn: %s, called_msn: %s)",
			event.ID, event.Type, event.CallerMSN, event.CalledMSN)
		delete(cm.pendingInternalCalls, key)
	}
}
