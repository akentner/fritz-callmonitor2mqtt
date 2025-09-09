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

	// MQTT client
	client mqtt.Client

	// State management
	connected    bool
	mu           sync.RWMutex
	lineStatuses map[string]*types.LineStatus
	callHistory  *types.CallHistory
}

// NewClient creates a new MQTT client
func NewClient(broker string, port int, username, password, clientID, topicPrefix string, qos byte, retain bool, keepAlive, connectTimeout time.Duration) *Client {
	return &Client{
		broker:         broker,
		port:           port,
		username:       username,
		password:       password,
		clientID:       clientID,
		topicPrefix:    topicPrefix,
		qos:            qos,
		retain:         retain,
		keepAlive:      keepAlive,
		connectTimeout: connectTimeout,
		lineStatuses:   make(map[string]*types.LineStatus),
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
	c.client.Disconnect(250) // Wait up to 250ms for graceful disconnect
	c.connected = false
	log.Println("Disconnected from MQTT broker")
	return nil
}

// onConnect is called when the MQTT connection is established
func (c *Client) onConnect(client mqtt.Client) {
	log.Println("MQTT client connected")
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

	// Update status based on call type
	switch event.Type {
	case types.CallTypeRing, types.CallTypeCall:
		lineStatus.Status = types.CallStatusRing
		lineStatus.CurrentCall = &event
	case types.CallTypeConnect:
		lineStatus.Status = types.CallStatusActive
		lineStatus.CurrentCall = &event
	case types.CallTypeDisconnect:
		lineStatus.Status = types.CallStatusIdle
		lineStatus.CurrentCall = nil
	}
	lineStatus.LastActivity = event.Timestamp

	// Publish line status
	if err := c.publishLineStatus(lineStatus); err != nil {
		return fmt.Errorf("failed to publish line status: %w", err)
	}

	if err := c.publishLineLastEvent(event); err != nil {
		return fmt.Errorf("failed to publish line last event: %w", err)
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

func (c *Client) publishLineLastEvent(event types.CallEvent) error {
	topic := fmt.Sprintf("%s/line/%d/last_event", c.topicPrefix, event.Line)

	payload, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("failed to marshal call event: %w", err)
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
		Line:         event.Line,
		Extension:    event.Extension,
		Trunk:        event.Trunk,
		Direction:    event.Direction,
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
