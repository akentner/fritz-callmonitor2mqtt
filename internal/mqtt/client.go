package mqtt

import (
	"encoding/json"
	"fmt"
	"log"
	"sync"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"

	"fritz-callmonitor2mqtt/pkg/types"
)

// Client represents an MQTT client using Eclipse Paho
type Client struct {
	broker         string
	port           int
	username       string
	password       string
	clientID       string
	topicPrefix    string
	qos            byte
	retain         bool
	keepAlive      time.Duration
	connectTimeout time.Duration
	logLevel       string

	// MQTT client
	client mqtt.Client

	// State management
	connected              bool
	mu                     sync.RWMutex
	lineStatuses           map[string]*types.LineStatus
	lineStatusExtensions   map[string]*types.LineStatusExtension
	lineStatusParticipants map[string]*types.LineStatusParticipant
	callHistory            *types.CallHistory
}

// NewClient creates a new MQTT client
func NewClient(broker string, port int, username, password, clientID, topicPrefix string, qos byte, retain bool, keepAlive, connectTimeout time.Duration, logLevel string) *Client {
	return &Client{
		broker:                 broker,
		port:                   port,
		username:               username,
		password:               password,
		clientID:               clientID,
		topicPrefix:            topicPrefix,
		qos:                    qos,
		retain:                 retain,
		keepAlive:              keepAlive,
		connectTimeout:         connectTimeout,
		logLevel:               logLevel,
		lineStatuses:           make(map[string]*types.LineStatus),
		lineStatusExtensions:   make(map[string]*types.LineStatusExtension),
		lineStatusParticipants: make(map[string]*types.LineStatusParticipant),
		callHistory: &types.CallHistory{
			Calls:   make([]types.CallEvent, 0),
			MaxSize: 50,
		},
	}
}

// Connect establishes connection to MQTT broker
func (c *Client) Connect() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.connected {
		return nil
	}

	// Setup MQTT client options
	opts := mqtt.NewClientOptions()
	brokerURL := fmt.Sprintf("tcp://%s:%d", c.broker, c.port)
	opts.AddBroker(brokerURL)
	opts.SetClientID(c.clientID)
	opts.SetKeepAlive(c.keepAlive)
	opts.SetConnectTimeout(c.connectTimeout)
	opts.SetAutoReconnect(true)
	opts.SetCleanSession(true)

	if c.username != "" {
		opts.SetUsername(c.username)
	}
	if c.password != "" {
		opts.SetPassword(c.password)
	}

	// Setup Last Will Testament (LWT)
	lastWillTopic := fmt.Sprintf("%s/status", c.topicPrefix)
	lastWillPayload, err := c.createStatusMessage("offline")
	if err != nil {
		return fmt.Errorf("failed to create last will message: %w", err)
	}
	opts.SetWill(lastWillTopic, string(lastWillPayload), c.qos, c.retain)

	// Setup callbacks
	opts.SetConnectionLostHandler(c.onConnectionLost)
	opts.SetOnConnectHandler(c.onConnect)

	log.Printf("Connecting to MQTT broker %s with client ID %s", brokerURL, c.clientID)

	// Create and connect client
	c.client = mqtt.NewClient(opts)
	if token := c.client.Connect(); token.Wait() && token.Error() != nil {
		return fmt.Errorf("failed to connect to MQTT broker: %w", token.Error())
	}

	c.connected = true
	log.Println("Successfully connected to MQTT broker")
	return nil
} // Disconnect closes the MQTT connection
func (c *Client) Disconnect() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if !c.connected || c.client == nil {
		return nil
	}

	log.Println("Disconnecting from MQTT broker...")

	// Send explicit offline message before disconnecting
	topic := fmt.Sprintf("%s/status", c.topicPrefix)
	payload, err := c.createStatusMessage("offline")
	if err != nil {
		log.Printf("Failed to create offline message: %v", err)
	} else {
		log.Printf("Publishing offline message to topic '%s'", topic)
		if token := c.client.Publish(topic, c.qos, c.retain, payload); token.Wait() && token.Error() != nil {
			log.Printf("Failed to publish offline message: %v", token.Error())
		}
	}

	c.client.Disconnect(250) // Wait up to 250ms for graceful disconnect
	c.connected = false
	log.Println("Disconnected from MQTT broker")
	return nil
}

// onConnect is called when the MQTT connection is established
func (c *Client) onConnect(client mqtt.Client) {
	log.Println("MQTT client connected")

	// Publish birth message
	if err := c.publishBirthMessage(); err != nil {
		log.Printf("Failed to publish birth message: %v", err)
	}
}

// onConnectionLost is called when the MQTT connection is lost
func (c *Client) onConnectionLost(client mqtt.Client, err error) {
	c.mu.Lock()
	c.connected = false
	c.mu.Unlock()
	log.Printf("MQTT connection lost: %v", err)
}

// IsConnected returns the connection status
func (c *Client) IsConnected() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.connected
}

// PublishCallEvent publishes a call event and updates line status
func (c *Client) PublishCallEvent(event types.CallEvent) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if !c.connected {
		return fmt.Errorf("MQTT client not connected")
	}

	// Update call history
	c.callHistory.AddCall(event)

	// Update line status
	lineKey := fmt.Sprintf("%s_%d", event.Trunk, event.Line)
	lineStatus := c.getOrCreateLineStatus(lineKey, event)

	// Use FSM status if available, otherwise fall back to call type mapping
	if event.Status != "" {
		lineStatus.Status = event.Status
	} else {
		// Fallback for events without FSM processing
		switch event.Type {
		case types.CallTypeRing:
			lineStatus.Status = types.CallStatusRinging
		case types.CallTypeCall:
			lineStatus.Status = types.CallStatusCalling
		case types.CallTypeConnect:
			lineStatus.Status = types.CallStatusTalking
		case types.CallTypeDisconnect:
			lineStatus.Status = types.CallStatusIdle
		}
	}

	// Update finish state from FSM
	lineStatus.FinishState = event.FinishState

	if event.Type == types.CallTypeDisconnect {
		lineStatus.Duration = &event.Duration
	}

	lineStatus.LastEvent = event.RawMessage
	lineStatus.LastUpdated = event.Timestamp

	// Publish line status
	if err := c.publishLineStatus(lineStatus); err != nil {
		return fmt.Errorf("failed to publish line status: %w", err)
	}

	if err := c.publishLineLastEvent(event); err != nil {
		return fmt.Errorf("failed to publish line last event: %w", err)
	}

	if err := c.publishCallStatus(lineStatus); err != nil {
		return fmt.Errorf("failed to publish call status: %w", err)
	}

	// Publish call history
	// if err := c.publishCallHistory(); err != nil {
	// 	return fmt.Errorf("failed to publish call history: %w", err)
	// }

	// Publish individual call event
	// if err := c.publishEvent(event); err != nil {
	// 	return fmt.Errorf("failed to publish call event: %w", err)
	// }

	return nil
}

// publishLineStatus publishes the status of a phone line
func (c *Client) publishLineStatus(status *types.LineStatus) error {
	topic := fmt.Sprintf("%s/line/%d/status", c.topicPrefix, status.Line)

	payload, err := json.Marshal(status)
	if err != nil {
		return fmt.Errorf("failed to marshal line status: %w", err)
	}

	return c.publish(topic, payload)
}

func (c *Client) publishCallStatus(status *types.LineStatus) error {
	topic := fmt.Sprintf("%s/call/%s", c.topicPrefix, status.ID)

	payload, err := json.Marshal(status)
	if err != nil {
		return fmt.Errorf("failed to marshal call status: %w", err)
	}

	return c.publish(topic, payload)
}

func (c *Client) publishLineLastEvent(event types.CallEvent) error {
	topic := fmt.Sprintf("%s/line/%d/last_event", c.topicPrefix, event.Line)

	payload, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("failed to marshal call event: %w", err)
	}

	return c.publish(topic, payload)
}

// publishCallHistory publishes the call history
// func (c *Client) publishCallHistory() error {
// 	topic := fmt.Sprintf("%s/history", c.topicPrefix)

// 	payload, err := json.Marshal(c.callHistory)
// 	if err != nil {
// 		return fmt.Errorf("failed to marshal call history: %w", err)
// 	}

// 	return c.publish(topic, payload)
// }

// publishEvent publishes a single call event
// func (c *Client) publishEvent(event types.CallEvent) error {
// 	topic := fmt.Sprintf("%s/events/%s", c.topicPrefix, event.Type)

// 	payload, err := json.Marshal(event)
// 	if err != nil {
// 		return fmt.Errorf("failed to marshal call event: %w", err)
// 	}

// 	return c.publish(topic, payload)
// }

// publish sends a message to the MQTT broker
func (c *Client) publish(topic string, payload []byte) error {
	if c.client == nil || !c.client.IsConnected() {
		return fmt.Errorf("MQTT client not connected")
	}

	log.Printf("Publishing to topic '%s': %s", topic, string(payload))

	token := c.client.Publish(topic, c.qos, c.retain, payload)
	if token.Wait() && token.Error() != nil {
		return fmt.Errorf("failed to publish message: %w", token.Error())
	}

	return nil
}

// getOrCreateLineStatus gets or creates a line status entry
func (c *Client) getOrCreateLineStatus(key string, event types.CallEvent) *types.LineStatus {
	if status, exists := c.lineStatuses[key]; exists {
		return status
	}

	status := &types.LineStatus{
		ID:          event.ID,
		Line:        event.Line,
		Trunk:       event.Trunk,
		Direction:   event.Direction,
		Status:      types.CallStatusIdle,
		Extension:   *c.getOrCreateLineStatusExtension(event.Extension, ""),
		Caller:      *c.getOrCreateLineStatusParticipant(event.Caller, ""),
		Called:      *c.getOrCreateLineStatusParticipant(event.Called, ""),
		LastEvent:   event.RawMessage,
		LastUpdated: time.Now(),
	}
	c.lineStatuses[key] = status
	return status
}

func (c *Client) getOrCreateLineStatusParticipant(phoneNumber string, name string) *types.LineStatusParticipant {
	if participant, exists := c.lineStatusParticipants[phoneNumber]; exists {
		return participant
	}

	participant := &types.LineStatusParticipant{
		PhoneNumber: phoneNumber,
		Name:        name,
	}
	c.lineStatusParticipants[phoneNumber] = participant
	return participant
}

// getOrCreateExtension gets or creates a line status extension
func (c *Client) getOrCreateLineStatusExtension(key string, name string) *types.LineStatusExtension {
	if extension, exists := c.lineStatusExtensions[key]; exists {
		return extension
	}

	extension := &types.LineStatusExtension{
		ID:   key,
		Name: name,
	}
	c.lineStatusExtensions[key] = extension
	return extension
}

// GetLineStatuses returns all current line statuses
func (c *Client) GetLineStatuses() map[string]*types.LineStatus {
	c.mu.RLock()
	defer c.mu.RUnlock()

	// Return a copy to avoid race conditions
	result := make(map[string]*types.LineStatus)
	for k, v := range c.lineStatuses {
		statusCopy := *v
		result[k] = &statusCopy
	}
	return result
}

// GetCallHistory returns the current call history
func (c *Client) GetCallHistory() *types.CallHistory {
	c.mu.RLock()
	defer c.mu.RUnlock()

	// Return a copy to avoid race conditions
	historyCopy := &types.CallHistory{
		Calls:     make([]types.CallEvent, len(c.callHistory.Calls)),
		MaxSize:   c.callHistory.MaxSize,
		UpdatedAt: c.callHistory.UpdatedAt,
	}
	copy(historyCopy.Calls, c.callHistory.Calls)
	return historyCopy
}

// createStatusMessage creates a JSON payload for service status (online/offline)
func (c *Client) createStatusMessage(state string) ([]byte, error) {
	status := types.ServiceStatus{
		State:       state,
		LastChanged: time.Now(),
	}
	return json.Marshal(status)
}

// publishBirthMessage publishes the birth message indicating the service is online
func (c *Client) publishBirthMessage() error {
	topic := fmt.Sprintf("%s/status", c.topicPrefix)
	payload, err := c.createStatusMessage("online")
	if err != nil {
		return fmt.Errorf("failed to create birth message: %w", err)
	}

	log.Printf("Publishing birth message to topic '%s'", topic)
	return c.publish(topic, payload)
}

// PublishLineStatusChange publishes FSM status changes via MQTT
func (c *Client) PublishLineStatusChange(line int, oldStatus, newStatus types.CallStatus, event *types.CallEvent) error {
	c.mu.RLock()
	defer c.mu.RUnlock()

	// Only publish FSM debug topics when log level is debug
	if c.logLevel == "debug" {
		if !c.connected {
			return fmt.Errorf("MQTT client not connected")
		}

		// Create status change message
		msg := types.LineStatusChangeMessage{
			Line:      line,
			OldStatus: oldStatus,
			NewStatus: newStatus,
			Timestamp: time.Now().Format(time.RFC3339),
			Event:     event,
		}

		// Determine reason for status change
		if event != nil {
			msg.Reason = "event"
		} else {
			msg.Reason = "timeout"
		}

		// Publish to line-specific FSM status topic
		topic := fmt.Sprintf("%s/fsm/line/%d/status_change", c.topicPrefix, line)
		payload, err := json.Marshal(msg)
		if err != nil {
			return fmt.Errorf("failed to marshal FSM status change: %w", err)
		}

		if err := c.publish(topic, payload); err != nil {
			return fmt.Errorf("failed to publish FSM status change: %w", err)
		}

		// Also publish current FSM status
		return c.publishFSMStatus(line, newStatus, event)
	}

	// When not in debug mode, FSM status changes are not published to debug topics
	return nil
}

// publishFSMStatus publishes the current FSM status
func (c *Client) publishFSMStatus(line int, status types.CallStatus, lastEvent *types.CallEvent) error {
	msg := types.FSMStatusMessage{
		Line:      line,
		Status:    status,
		Timestamp: time.Now().Format(time.RFC3339),
	}

	// Add last event info if available
	if lastEvent != nil {
		msg.LastEventType = lastEvent.Type
		msg.LastEventTimestamp = lastEvent.Timestamp.Format(time.RFC3339)
	}

	// Determine valid transitions based on current status
	msg.ValidTransitions = c.getValidTransitionsForStatus(status)

	// Check if timeout is active
	msg.IsTimeoutActive = status == types.CallStatusNotReached ||
		status == types.CallStatusMissedCall ||
		status == types.CallStatusFinished

	topic := fmt.Sprintf("%s/fsm/line/%d/status", c.topicPrefix, line)
	payload, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("failed to marshal FSM status: %w", err)
	}

	return c.publish(topic, payload)
}

// PublishTimeoutStatusUpdate publishes a line status update for timeout transitions
func (c *Client) PublishTimeoutStatusUpdate(line int, newStatus types.CallStatus) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if !c.connected {
		return fmt.Errorf("MQTT client not connected")
	}

	// Find existing line status and update it
	var lineStatus *types.LineStatus
	for _, status := range c.lineStatuses {
		if status.Line == line {
			lineStatus = status
			break
		}
	}

	if lineStatus == nil {
		// No existing line status found, skip update
		return nil
	}

	// Update status from FSM timeout transition
	lineStatus.Status = newStatus
	lineStatus.LastUpdated = time.Now()

	// Publish updated line status
	return c.publishLineStatus(lineStatus)
}

// getValidTransitionsForStatus returns valid transitions for a given status
func (c *Client) getValidTransitionsForStatus(status types.CallStatus) []types.CallType {
	switch status {
	case types.CallStatusIdle:
		return []types.CallType{types.CallTypeRing, types.CallTypeCall}
	case types.CallStatusRinging:
		return []types.CallType{types.CallTypeConnect, types.CallTypeDisconnect}
	case types.CallStatusCalling:
		return []types.CallType{types.CallTypeConnect, types.CallTypeDisconnect}
	case types.CallStatusTalking:
		return []types.CallType{types.CallTypeDisconnect}
	default:
		return []types.CallType{} // Final states have no valid transitions
	}
}
