package types

import (
	"context"
	"sync"
	"time"
)

// CallStateMachine manages the state transitions for call events
type CallStateMachine struct {
	mu            sync.RWMutex
	currentState  CallStatus
	finishState   *CallStatus // Last meaningful state before idle
	timeoutTimer  *time.Timer
	timeoutCtx    context.Context
	timeoutCancel context.CancelFunc
	onStateChange func(oldState, newState CallStatus)
	mqttPublisher MQTTPublisher
	line          int
	lastEvent     *CallEvent
	lastEventType CallType
	lastEventTime time.Time
}

// NewCallStateMachine creates a new finite state machine for call status
func NewCallStateMachine(onStateChange func(oldState, newState CallStatus)) *CallStateMachine {
	return &CallStateMachine{
		currentState:  CallStatusIdle,
		onStateChange: onStateChange,
	}
}

// NewCallStateMachineWithMQTT creates a new FSM with MQTT publishing support
func NewCallStateMachineWithMQTT(line int, mqttPublisher MQTTPublisher, onStateChange func(oldState, newState CallStatus)) *CallStateMachine {
	return &CallStateMachine{
		currentState:  CallStatusIdle,
		onStateChange: onStateChange,
		mqttPublisher: mqttPublisher,
		line:          line,
	}
}

// GetState returns the current state of the FSM
func (fsm *CallStateMachine) GetState() CallStatus {
	fsm.mu.RLock()
	defer fsm.mu.RUnlock()
	return fsm.currentState
}

// ProcessEvent processes a call event and updates the state according to FSM rules
func (fsm *CallStateMachine) ProcessEvent(eventType CallType) CallStatus {
	return fsm.ProcessEventWithContext(eventType, nil)
}

// ProcessEventWithContext processes a call event with additional context for MQTT publishing
func (fsm *CallStateMachine) ProcessEventWithContext(eventType CallType, event *CallEvent) CallStatus {
	return fsm.processEventInternal(eventType, event, false)
}

// processEventInternal is the internal implementation that handles both regular events and timeouts
func (fsm *CallStateMachine) processEventInternal(eventType CallType, event *CallEvent, isTimeout bool) CallStatus {
	fsm.mu.Lock()
	defer fsm.mu.Unlock()

	oldState := fsm.currentState
	newState := fsm.getNextState(fsm.currentState, eventType)

	// Store event context
	if !isTimeout {
		fsm.lastEventType = eventType
		fsm.lastEventTime = time.Now()
		if event != nil {
			fsm.lastEvent = event
		}
	}

	if oldState != newState {
		fsm.setState(newState)
		if !isTimeout {
			fsm.handleTimeouts(newState)
		}

		// Publish MQTT status change
		if fsm.mqttPublisher != nil {
			// Use goroutine for non-blocking operation in production
			// For tests, MockMQTTPublisher will handle synchronously
			go func(line int, old, new CallStatus, evt *CallEvent, timeout bool) {
				var publishEvent *CallEvent
				if !timeout {
					publishEvent = evt
				}
				if err := fsm.mqttPublisher.PublishLineStatusChange(line, old, new, publishEvent); err != nil {
					// Log error but don't block FSM operation
					// TODO: Add proper logging interface
				}
			}(fsm.line, oldState, newState, event, isTimeout)
		}

		if fsm.onStateChange != nil {
			fsm.onStateChange(oldState, newState)
		}
	}

	return newState
}

// getNextState determines the next state based on current state and event type
func (fsm *CallStateMachine) getNextState(currentState CallStatus, eventType CallType) CallStatus {
	switch currentState {
	case CallStatusIdle:
		switch eventType {
		case CallTypeRing:
			return CallStatusRinging
		case CallTypeCall:
			return CallStatusCalling
		}

	case CallStatusRinging:
		switch eventType {
		case CallTypeConnect:
			return CallStatusTalking
		case CallTypeDisconnect:
			return CallStatusMissedCall
		}

	case CallStatusCalling:
		switch eventType {
		case CallTypeConnect:
			return CallStatusTalking
		case CallTypeDisconnect:
			return CallStatusNotReached
		}

	case CallStatusTalking:
		switch eventType {
		case CallTypeDisconnect:
			return CallStatusFinished
		}
	}

	// No valid transition found, stay in current state
	return currentState
}

// setState updates the current state and handles cleanup
func (fsm *CallStateMachine) setState(newState CallStatus) {
	// Cancel any existing timeout
	fsm.cancelTimeout()

	// Track finish states (final meaningful states before idle)
	if newState == CallStatusMissedCall || newState == CallStatusNotReached || newState == CallStatusFinished {
		fsm.finishState = &newState
	} else if newState == CallStatusIdle {
		// When returning to idle, keep the finish state for history
		// It will be reset on the next non-idle transition
	} else if newState != CallStatusIdle {
		// Reset finish state when starting a new call sequence
		fsm.finishState = nil
	}

	fsm.currentState = newState
}

// handleTimeouts sets up timeout transitions for states that need them
func (fsm *CallStateMachine) handleTimeouts(state CallStatus) {
	switch state {
	case CallStatusNotReached, CallStatusMissedCall, CallStatusFinished:
		fsm.startTimeout(1 * time.Second)
	}
}

// startTimeout starts a timeout that will transition to idle state
func (fsm *CallStateMachine) startTimeout(duration time.Duration) {
	fsm.timeoutCtx, fsm.timeoutCancel = context.WithCancel(context.Background())

	fsm.timeoutTimer = time.AfterFunc(duration, func() {
		select {
		case <-fsm.timeoutCtx.Done():
			// Timeout was cancelled
			return
		default:
			// Execute timeout transition
			fsm.executeTimeoutTransition()
		}
	})
}

// executeTimeoutTransition handles timeout-based transitions to idle
func (fsm *CallStateMachine) executeTimeoutTransition() {
	fsm.mu.Lock()
	oldState := fsm.currentState
	if oldState == CallStatusNotReached || oldState == CallStatusMissedCall || oldState == CallStatusFinished {
		// Set finishState before transitioning to idle
		fsm.finishState = &oldState
		// Use setState to properly handle the idle transition
		fsm.setState(CallStatusIdle)

		// Publish MQTT timeout transition (nil event indicates timeout)
		if fsm.mqttPublisher != nil {
			go func(line int, old CallStatus) {
				if err := fsm.mqttPublisher.PublishLineStatusChange(line, old, CallStatusIdle, nil); err != nil {
					// Ignore error for timeout transitions
				}
			}(fsm.line, oldState)
		}

		if fsm.onStateChange != nil {
			// Call state change callback outside of lock to avoid deadlock
			go fsm.onStateChange(oldState, CallStatusIdle)
		}
	}
	fsm.mu.Unlock()
}

// cancelTimeout cancels any active timeout
func (fsm *CallStateMachine) cancelTimeout() {
	if fsm.timeoutTimer != nil {
		fsm.timeoutTimer.Stop()
		fsm.timeoutTimer = nil
	}
	if fsm.timeoutCancel != nil {
		fsm.timeoutCancel()
		fsm.timeoutCancel = nil
	}
}

// Reset resets the FSM to idle state and cancels any timeouts
func (fsm *CallStateMachine) Reset() {
	fsm.mu.Lock()
	defer fsm.mu.Unlock()

	oldState := fsm.currentState
	fsm.cancelTimeout()
	fsm.currentState = CallStatusIdle
	fsm.finishState = nil

	if oldState != CallStatusIdle && fsm.onStateChange != nil {
		fsm.onStateChange(oldState, CallStatusIdle)
	}
}

// GetFinishState returns the last meaningful state before idle
func (fsm *CallStateMachine) GetFinishState() *CallStatus {
	fsm.mu.RLock()
	defer fsm.mu.RUnlock()
	return fsm.finishState
}

// IsValidTransition checks if a transition from current state with given event is valid
func (fsm *CallStateMachine) IsValidTransition(eventType CallType) bool {
	fsm.mu.RLock()
	defer fsm.mu.RUnlock()

	newState := fsm.getNextState(fsm.currentState, eventType)
	return newState != fsm.currentState
}

// GetValidTransitions returns all valid event types for the current state
func (fsm *CallStateMachine) GetValidTransitions() []CallType {
	fsm.mu.RLock()
	defer fsm.mu.RUnlock()

	var validEvents []CallType
	allEvents := []CallType{CallTypeRing, CallTypeCall, CallTypeConnect, CallTypeDisconnect}

	for _, event := range allEvents {
		if fsm.getNextState(fsm.currentState, event) != fsm.currentState {
			validEvents = append(validEvents, event)
		}
	}

	return validEvents
}

// SetMQTTPublisher sets the MQTT publisher for status changes
func (fsm *CallStateMachine) SetMQTTPublisher(publisher MQTTPublisher, line int) {
	fsm.mu.Lock()
	defer fsm.mu.Unlock()
	fsm.mqttPublisher = publisher
	fsm.line = line
}

// GetFSMStatus returns the current FSM status for MQTT publishing
func (fsm *CallStateMachine) GetFSMStatus() FSMStatusMessage {
	fsm.mu.RLock()
	defer fsm.mu.RUnlock()

	msg := FSMStatusMessage{
		Line:             fsm.line,
		Status:           fsm.currentState,
		Timestamp:        time.Now().Format(time.RFC3339),
		ValidTransitions: fsm.getValidTransitionsUnsafe(),
		IsTimeoutActive:  fsm.timeoutTimer != nil,
		LastEventType:    fsm.lastEventType,
	}

	if !fsm.lastEventTime.IsZero() {
		msg.LastEventTimestamp = fsm.lastEventTime.Format(time.RFC3339)
	}

	return msg
}

// getValidTransitionsUnsafe returns valid transitions without locking (assumes caller has lock)
func (fsm *CallStateMachine) getValidTransitionsUnsafe() []CallType {
	var validEvents []CallType
	allEvents := []CallType{CallTypeRing, CallTypeCall, CallTypeConnect, CallTypeDisconnect}

	for _, event := range allEvents {
		if fsm.getNextState(fsm.currentState, event) != fsm.currentState {
			validEvents = append(validEvents, event)
		}
	}

	return validEvents
}

// Cleanup should be called when the FSM is no longer needed
func (fsm *CallStateMachine) Cleanup() {
	fsm.mu.Lock()
	defer fsm.mu.Unlock()
	fsm.cancelTimeout()
}
