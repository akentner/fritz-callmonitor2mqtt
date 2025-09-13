package types

import (
	"fmt"
	"sync"
)

// LineStateMachine manages FSMs for multiple phone lines
type LineStateMachine struct {
	mu            sync.RWMutex
	machines      map[int]*CallStateMachine
	onStateChange func(line int, oldState, newState CallStatus)
	mqttPublisher MQTTPublisher
}

// NewLineStateMachine creates a new line state machine manager
func NewLineStateMachine(onStateChange func(line int, oldState, newState CallStatus)) *LineStateMachine {
	return &LineStateMachine{
		machines:      make(map[int]*CallStateMachine),
		onStateChange: onStateChange,
	}
}

// NewLineStateMachineWithMQTT creates a new line state machine with MQTT publishing
func NewLineStateMachineWithMQTT(mqttPublisher MQTTPublisher, onStateChange func(line int, oldState, newState CallStatus)) *LineStateMachine {
	return &LineStateMachine{
		machines:      make(map[int]*CallStateMachine),
		onStateChange: onStateChange,
		mqttPublisher: mqttPublisher,
	}
}

// ProcessCallEvent processes a call event and updates the appropriate line FSM
func (lsm *LineStateMachine) ProcessCallEvent(event *CallEvent) CallStatus {
	lsm.mu.Lock()
	defer lsm.mu.Unlock()

	// Get or create FSM for this line
	fsm, exists := lsm.machines[event.Line]
	if !exists {
		if lsm.mqttPublisher != nil {
			fsm = NewCallStateMachineWithMQTT(event.Line, lsm.mqttPublisher, func(oldState, newState CallStatus) {
				if lsm.onStateChange != nil {
					lsm.onStateChange(event.Line, oldState, newState)
				}
			})
		} else {
			fsm = NewCallStateMachine(func(oldState, newState CallStatus) {
				if lsm.onStateChange != nil {
					lsm.onStateChange(event.Line, oldState, newState)
				}
			})
		}
		lsm.machines[event.Line] = fsm
	}

	// Process event and update call event with new status
	var newStatus CallStatus
	if lsm.mqttPublisher != nil {
		newStatus = fsm.ProcessEventWithContext(event.Type, event)
	} else {
		newStatus = fsm.ProcessEvent(event.Type)
	}
	event.Status = newStatus

	return newStatus
}

// GetLineState returns the current state of a specific line
func (lsm *LineStateMachine) GetLineState(line int) CallStatus {
	lsm.mu.RLock()
	defer lsm.mu.RUnlock()

	if fsm, exists := lsm.machines[line]; exists {
		return fsm.GetState()
	}
	return CallStatusIdle
}

// GetAllLineStates returns the current states of all lines
func (lsm *LineStateMachine) GetAllLineStates() map[int]CallStatus {
	lsm.mu.RLock()
	defer lsm.mu.RUnlock()

	states := make(map[int]CallStatus)
	for line, fsm := range lsm.machines {
		states[line] = fsm.GetState()
	}
	return states
}

// GetLineFinishState returns the finish state of a specific line
func (lsm *LineStateMachine) GetLineFinishState(line int) *CallStatus {
	lsm.mu.RLock()
	defer lsm.mu.RUnlock()

	if fsm, exists := lsm.machines[line]; exists {
		return fsm.GetFinishState()
	}
	return nil
}

// ResetLine resets a specific line to idle state
func (lsm *LineStateMachine) ResetLine(line int) {
	lsm.mu.Lock()
	defer lsm.mu.Unlock()

	if fsm, exists := lsm.machines[line]; exists {
		fsm.Reset()
	}
}

// ResetAllLines resets all lines to idle state
func (lsm *LineStateMachine) ResetAllLines() {
	lsm.mu.Lock()
	defer lsm.mu.Unlock()

	for _, fsm := range lsm.machines {
		fsm.Reset()
	}
}

// IsValidTransition checks if a transition is valid for a specific line
func (lsm *LineStateMachine) IsValidTransition(line int, eventType CallType) bool {
	lsm.mu.RLock()
	defer lsm.mu.RUnlock()

	if fsm, exists := lsm.machines[line]; exists {
		return fsm.IsValidTransition(eventType)
	}

	// For new lines, only RING and CALL are valid from idle state
	return eventType == CallTypeRing || eventType == CallTypeCall
}

// GetValidTransitions returns valid transitions for a specific line
func (lsm *LineStateMachine) GetValidTransitions(line int) []CallType {
	lsm.mu.RLock()
	defer lsm.mu.RUnlock()

	if fsm, exists := lsm.machines[line]; exists {
		return fsm.GetValidTransitions()
	}

	// For new lines, return idle state transitions
	return []CallType{CallTypeRing, CallTypeCall}
}

// RemoveLine removes a line's FSM (useful for cleanup)
func (lsm *LineStateMachine) RemoveLine(line int) {
	lsm.mu.Lock()
	defer lsm.mu.Unlock()

	if fsm, exists := lsm.machines[line]; exists {
		fsm.Cleanup()
		delete(lsm.machines, line)
	}
}

// GetLineCount returns the number of active lines
func (lsm *LineStateMachine) GetLineCount() int {
	lsm.mu.RLock()
	defer lsm.mu.RUnlock()
	return len(lsm.machines)
}

// GetActiveLines returns a list of all line numbers that have active FSMs
func (lsm *LineStateMachine) GetActiveLines() []int {
	lsm.mu.RLock()
	defer lsm.mu.RUnlock()

	lines := make([]int, 0, len(lsm.machines))
	for line := range lsm.machines {
		lines = append(lines, line)
	}
	return lines
}

// Cleanup cleans up all FSMs
func (lsm *LineStateMachine) Cleanup() {
	lsm.mu.Lock()
	defer lsm.mu.Unlock()

	for _, fsm := range lsm.machines {
		fsm.Cleanup()
	}
	lsm.machines = make(map[int]*CallStateMachine)
}

// SetMQTTPublisher sets the MQTT publisher for all existing and future FSMs
func (lsm *LineStateMachine) SetMQTTPublisher(publisher MQTTPublisher) {
	lsm.mu.Lock()
	defer lsm.mu.Unlock()

	lsm.mqttPublisher = publisher

	// Update existing FSMs
	for line, fsm := range lsm.machines {
		fsm.SetMQTTPublisher(publisher, line)
	}
}

// GetAllFSMStatuses returns FSM status messages for all active lines
func (lsm *LineStateMachine) GetAllFSMStatuses() []FSMStatusMessage {
	lsm.mu.RLock()
	defer lsm.mu.RUnlock()

	statuses := make([]FSMStatusMessage, 0, len(lsm.machines))
	for lineNum, fsm := range lsm.machines {
		status := fsm.GetFSMStatus()
		// Ensure line number is set correctly
		status.Line = lineNum
		statuses = append(statuses, status)
	}
	return statuses
}

// GetLineStateSummary returns a formatted summary of all line states
func (lsm *LineStateMachine) GetLineStateSummary() string {
	states := lsm.GetAllLineStates()
	if len(states) == 0 {
		return "No active lines"
	}

	summary := "Line States:\n"
	for line, state := range states {
		summary += fmt.Sprintf("  Line %d: %s\n", line, state)
	}
	return summary
}
