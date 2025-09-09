package mqtt

import (
	"encoding/json"
	"fmt"
	"log"
	"sync"
	"time"

	"fritz-callmonitor2mqtt/pkg/types"
)

// Client represents a simple MQTT client interface
// Note: This is a placeholder implementation for demonstration
// In production, you would use a proper MQTT library like paho.mqtt.golang
type Client struct {
	broker      string
	port        int
	username    string
	password    string
	clientID    string
	topicPrefix string
	qos         byte
	retain      bool
	connected   bool
	mu          sync.RWMutex

	// Simulated connection for demo purposes
	lineStatuses map[string]*types.LineStatus
	callHistory  *types.CallHistory
}

// NewClient creates a new MQTT client
func NewClient(broker string, port int, username, password, clientID, topicPrefix string, qos byte, retain bool) *Client {
	return &Client{
		broker:       broker,
		port:         port,
		username:     username,
		password:     password,
		clientID:     clientID,
		topicPrefix:  topicPrefix,
		qos:          qos,
		retain:       retain,
		lineStatuses: make(map[string]*types.LineStatus),
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

	// TODO: Implement actual MQTT connection
	// This is a placeholder for demonstration
	log.Printf("Connecting to MQTT broker %s:%d with client ID %s", c.broker, c.port, c.clientID)

	c.connected = true
	return nil
}

// Disconnect closes the MQTT connection
func (c *Client) Disconnect() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.connected = false
	log.Println("Disconnected from MQTT broker")
	return nil
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
	lineKey := fmt.Sprintf("%s_%s", event.LineID, event.Extension)
	lineStatus := c.getOrCreateLineStatus(lineKey, event.LineID, event.Extension)

	// Update status based on call type
	switch event.Type {
	case types.CallTypeIncoming, types.CallTypeOutgoing:
		lineStatus.Status = types.CallStatusRing
		lineStatus.CurrentCall = &event
	case types.CallTypeConnect:
		lineStatus.Status = types.CallStatusActive
		lineStatus.CurrentCall = &event
	case types.CallTypeEnd:
		lineStatus.Status = types.CallStatusIdle
		lineStatus.CurrentCall = nil
	}
	lineStatus.LastActivity = event.Timestamp

	// Publish line status
	if err := c.publishLineStatus(lineStatus); err != nil {
		return fmt.Errorf("failed to publish line status: %w", err)
	}

	// Publish call history
	if err := c.publishCallHistory(); err != nil {
		return fmt.Errorf("failed to publish call history: %w", err)
	}

	// Publish individual call event
	if err := c.publishEvent(event); err != nil {
		return fmt.Errorf("failed to publish call event: %w", err)
	}

	return nil
}

// publishLineStatus publishes the status of a phone line
func (c *Client) publishLineStatus(status *types.LineStatus) error {
	topic := fmt.Sprintf("%s/line/%s/status", c.topicPrefix, status.LineID)

	payload, err := json.Marshal(status)
	if err != nil {
		return fmt.Errorf("failed to marshal line status: %w", err)
	}

	return c.publish(topic, payload)
}

// publishCallHistory publishes the call history
func (c *Client) publishCallHistory() error {
	topic := fmt.Sprintf("%s/history", c.topicPrefix)

	payload, err := json.Marshal(c.callHistory)
	if err != nil {
		return fmt.Errorf("failed to marshal call history: %w", err)
	}

	return c.publish(topic, payload)
}

// publishEvent publishes a single call event
func (c *Client) publishEvent(event types.CallEvent) error {
	topic := fmt.Sprintf("%s/events/%s", c.topicPrefix, event.Type)

	payload, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("failed to marshal call event: %w", err)
	}

	return c.publish(topic, payload)
}

// publish sends a message to the MQTT broker
func (c *Client) publish(topic string, payload []byte) error {
	// TODO: Implement actual MQTT publishing
	// This is a placeholder for demonstration
	log.Printf("Publishing to topic '%s': %s", topic, string(payload))
	return nil
}

// getOrCreateLineStatus gets or creates a line status entry
func (c *Client) getOrCreateLineStatus(key, lineID, extension string) *types.LineStatus {
	if status, exists := c.lineStatuses[key]; exists {
		return status
	}

	status := &types.LineStatus{
		LineID:       lineID,
		Extension:    extension,
		Status:       types.CallStatusIdle,
		LastActivity: time.Now(),
	}
	c.lineStatuses[key] = status
	return status
}

// GetLineStatuses returns all current line statuses
func (c *Client) GetLineStatuses() map[string]*types.LineStatus {
	c.mu.RLock()
	defer c.mu.RUnlock()

	// Return a copy to avoid race conditions
	result := make(map[string]*types.LineStatus)
	for k, v := range c.lineStatuses {
		statusCopy := *v
		if v.CurrentCall != nil {
			callCopy := *v.CurrentCall
			statusCopy.CurrentCall = &callCopy
		}
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
