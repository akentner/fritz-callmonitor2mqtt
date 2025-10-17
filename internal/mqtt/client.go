package mqtt

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"

	"fritz-callmonitor2mqtt/internal/config"
	"fritz-callmonitor2mqtt/internal/database"
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

	// Home Assistant device configuration
	deviceName       string
	deviceIdentifier string
	version          string

	// Database client
	db *database.Client

	// PBX configuration for extension info
	pbxConfig *config.PBXConfig

	// MQTT client
	client mqtt.Client

	// State management
	connected              bool
	mu                     sync.RWMutex
	lineStatuses           map[string]*types.LineStatus
	lineStatusExtensions   map[string]*types.LineStatusExtension
	lineStatusParticipants map[string]*types.LineStatusParticipant
	callHistory            *types.CallHistory
	msnCallHistories       map[string]*types.MSNCallHistory
}

// NewClient creates a new MQTT client
func NewClient(broker string, port int, username, password, clientID, topicPrefix string, qos byte, retain bool, keepAlive, connectTimeout time.Duration, logLevel string, db *database.Client, pbxConfig *config.PBXConfig, msns []string, msnCallHistorySize int, deviceName, deviceIdentifier, version string) *Client {
	client := &Client{
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
		deviceName:             deviceName,
		deviceIdentifier:       deviceIdentifier,
		version:                version,
		db:                     db,
		pbxConfig:              pbxConfig,
		lineStatuses:           make(map[string]*types.LineStatus),
		lineStatusExtensions:   make(map[string]*types.LineStatusExtension),
		lineStatusParticipants: make(map[string]*types.LineStatusParticipant),
		callHistory: &types.CallHistory{
			Calls:   make([]types.CallEvent, 0),
			MaxSize: 50,
		},
		msnCallHistories: make(map[string]*types.MSNCallHistory),
	}

	// Initialize MSN call histories for each configured MSN
	for _, msn := range msns {
		if msn != "" {
			client.msnCallHistories[msn] = &types.MSNCallHistory{
				MSN:       msn,
				Calls:     make([]types.MSNCallEvent, 0),
				MaxSize:   msnCallHistorySize,
				UpdatedAt: time.Now(),
			}
		}
	}

	return client
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

	slog.Info("Connecting to MQTT broker",
		"broker", brokerURL,
		"client_id", c.clientID)

	// Create and connect client
	c.client = mqtt.NewClient(opts)
	if token := c.client.Connect(); token.Wait() && token.Error() != nil {
		return fmt.Errorf("failed to connect to MQTT broker: %w", token.Error())
	}

	c.connected = true
	slog.Info("Successfully connected to MQTT broker")
	return nil
} // Disconnect closes the MQTT connection
func (c *Client) Disconnect() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if !c.connected || c.client == nil {
		return nil
	}

	slog.Info("Disconnecting from MQTT broker...")

	// // Remove Home Assistant Discovery configurations
	// if err := c.removeHomeAssistantDiscovery(); err != nil {
	// 	log.Printf("Failed to remove Home Assistant discovery: %v", err)
	// }

	// Send explicit offline message before disconnecting
	topic := fmt.Sprintf("%s/status", c.topicPrefix)
	payload, err := c.createStatusMessage("offline")
	if err != nil {
		slog.Error("Failed to create offline message", "error", err)
	} else {
		slog.Debug("Publishing offline message", "topic", topic)
		if token := c.client.Publish(topic, c.qos, c.retain, payload); token.Wait() && token.Error() != nil {
			slog.Error("Failed to publish offline message", "error", token.Error())
		}
	}

	c.client.Disconnect(250) // Wait up to 250ms for graceful disconnect
	c.connected = false
	slog.Info("Disconnected from MQTT broker")
	return nil
}

// onConnect is called when the MQTT connection is established
func (c *Client) onConnect(client mqtt.Client) {
	slog.Info("MQTT client connected")

	// Publish birth message
	if err := c.publishBirthMessage(); err != nil {
		slog.Error("Failed to publish birth message", "error", err)
	}

	// Setup Home Assistant MQTT Discovery
	if err := c.setupHomeAssistantDiscovery(); err != nil {
		slog.Error("Failed to setup Home Assistant discovery", "error", err)
	}

	// Subscribe to phone number RPC topics
	if err := c.subscribeToPhoneNumberRPC(); err != nil {
		slog.Error("Failed to subscribe to phone number RPC topics", "error", err)
	}

	// Load MSN call histories from database and publish them
	if err := c.LoadMSNCallHistoriesFromDB(); err != nil {
		slog.Error("Failed to load MSN call histories from database", "error", err)
	} else {
		// Publish all MSN call histories
		if err := c.PublishAllMSNCallHistories(); err != nil {
			slog.Error("Failed to publish MSN call histories", "error", err)
		}
	}
}

// onConnectionLost is called when the MQTT connection is lost
func (c *Client) onConnectionLost(client mqtt.Client, err error) {
	c.mu.Lock()
	c.connected = false
	c.mu.Unlock()
	slog.Error("MQTT connection lost", "error", err)
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

	// Extension enrichment and MessageBox detection now happen in main.go before FSM processing

	// Update call history
	c.callHistory.AddCall(event)

	// Update MSN-specific call histories
	c.updateMSNCallHistories(event)

	// Update line status
	lineKey := fmt.Sprintf("%s_%d", event.Trunk, event.Line)
	lineStatus := c.getOrCreateLineStatus(lineKey, event)

	// Update extension information from enriched event data
	// For inbound calls, use called extension (our internal extension)
	// For outbound calls, use caller extension (our internal extension)
	if event.Direction == types.CallDirectionInbound && event.CalledExtension != nil {
		lineStatus.Extension = &types.LineStatusExtension{
			ID:   event.CalledExtension.Number,
			Name: event.CalledExtension.Name,
			Type: event.CalledExtension.Type,
		}
	} else if event.Direction == types.CallDirectionOutbound && event.CallerExtension != nil {
		lineStatus.Extension = &types.LineStatusExtension{
			ID:   event.CallerExtension.Number,
			Name: event.CallerExtension.Name,
			Type: event.CallerExtension.Type,
		}
	} else if event.Extension != "" {
		// Fallback to simple extension ID if no enriched data available
		lineStatus.Extension = c.getOrCreateLineStatusExtension(event.Extension, "")
	} else if event.Type == types.CallTypeRing || event.Type == types.CallTypeCall {
		// Only clear extension for new calls (ring/call), not for connect/disconnect
		lineStatus.Extension = nil
	}
	// For connect/disconnect events without extension info, keep existing extension

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

// getIconForStatus returns the appropriate MDI icon for a call status
func getIconForStatus(status types.CallStatus) string {
	iconMapping := map[types.CallStatus]string{
		"idle":       "mdi:phone-hangup",
		"calling":    "mdi:phone-outgoing",
		"notReached": "mdi:phone-remove",
		"ringing":    "mdi:phone-ring",
		"missedCall": "mdi:phone-remove",
		"talking":    "mdi:phone",
		"finished":   "mdi:phone-hangup",
		"voiceBox":   "mdi:phone-message",
	}

	if icon, exists := iconMapping[status]; exists {
		return icon
	}

	return "mdi:phone" // default fallback
}

// publishLineStatus publishes the status of a phone line
func (c *Client) publishLineStatus(status *types.LineStatus) error {
	// Set the appropriate icon for the status
	status.Icon = getIconForStatus(status.Status)

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

	slog.Debug("Publishing message",
		"topic", topic,
		"payload", string(payload))

	token := c.client.Publish(topic, c.qos, c.retain, payload)
	if token.Wait() && token.Error() != nil {
		return fmt.Errorf("failed to publish message: %w", token.Error())
	}

	return nil
}

// publishWithoutRetain publishes a message without retain flag (for call history topics)
func (c *Client) publishWithoutRetain(topic string, payload []byte) error {
	if c.client == nil || !c.client.IsConnected() {
		return fmt.Errorf("MQTT client not connected")
	}

	slog.Debug("Publishing message without retain",
		"topic", topic,
		"payload", string(payload))

	token := c.client.Publish(topic, c.qos, false, payload)
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

	var extension *types.LineStatusExtension
	if event.Extension != "" {
		extension = c.getOrCreateLineStatusExtension(event.Extension, "")
	}

	status := &types.LineStatus{
		ID:          event.ID,
		Line:        event.Line,
		Trunk:       event.Trunk,
		Direction:   event.Direction,
		Status:      types.CallStatusIdle,
		Extension:   extension,
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

	// Try to get extension info from PBX configuration
	extensionName := name
	extensionType := "UNKNOWN"

	if c.pbxConfig != nil {
		if extInfo := c.pbxConfig.GetExtensionInfo(key); extInfo != nil {
			extensionName = extInfo.Name
			extensionType = extInfo.Type
		}
	}

	extension := &types.LineStatusExtension{
		ID:   key,
		Name: extensionName,
		Type: extensionType,
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

// updateMSNCallHistories updates MSN-specific call histories and publishes to MQTT
func (c *Client) updateMSNCallHistories(event types.CallEvent) {
	// Only process disconnect events
	if event.Type != types.CallTypeDisconnect {
		return
	}

	// Update all MSN histories that this call involves
	msnsToUpdate := make([]string, 0)

	if event.CallerMSN != "" {
		msnsToUpdate = append(msnsToUpdate, event.CallerMSN)
	}
	if event.CalledMSN != "" && event.CalledMSN != event.CallerMSN {
		msnsToUpdate = append(msnsToUpdate, event.CalledMSN)
	}

	for _, msn := range msnsToUpdate {
		if history, exists := c.msnCallHistories[msn]; exists {
			// Resolve caller and called names from database
			var callerName, calledName string
			if event.Caller != "" && c.db != nil {
				if name, err := c.db.GetPhoneNumberName(event.Caller); err == nil && name != "" {
					callerName = name
				}
			}
			if event.Called != "" && c.db != nil {
				if name, err := c.db.GetPhoneNumberName(event.Called); err == nil && name != "" {
					calledName = name
				}
			}

			// Determine direction for this MSN
			var direction types.CallDirection
			if event.CallerMSN == msn {
				direction = types.CallDirectionOutbound
			} else if event.CalledMSN == msn {
				direction = types.CallDirectionInbound
			}

			// Convert CallEvent to MSNCallEvent
			msnCallEvent := types.MSNCallEvent{
				ID:        event.ID,
				Timestamp: event.Timestamp,
				Direction: direction,
				Line:      event.Line,
				Trunk:     event.Trunk,
				Caller: types.LineStatusParticipant{
					PhoneNumber: event.Caller,
					Name:        callerName,
				},
				Called: types.LineStatusParticipant{
					PhoneNumber: event.Called,
					Name:        calledName,
				},
				CallerMSN:       event.CallerMSN,
				CalledMSN:       event.CalledMSN,
				CallerExtension: event.CallerExtension,
				CalledExtension: event.CalledExtension,
				Duration:        event.Duration,
				FinishState:     string(event.Status), // Use the current status as finish state
			}

			// Add to front of slice (newest first)
			history.Calls = append([]types.MSNCallEvent{msnCallEvent}, history.Calls...)
			if len(history.Calls) > history.MaxSize {
				history.Calls = history.Calls[:history.MaxSize]
			}
			history.UpdatedAt = time.Now()

			// Publish updated history
			if c.connected {
				if err := c.publishMSNCallHistory(msn, history); err != nil {
					slog.Error("Failed to publish MSN call history",
						"msn", msn,
						"error", err)
				}
			}
		}
	}
}

// publishMSNCallHistory publishes the call history for a specific MSN (without retain to avoid republishing loops)
func (c *Client) publishMSNCallHistory(msn string, history *types.MSNCallHistory) error {
	if !c.connected {
		return fmt.Errorf("MQTT client not connected")
	}

	topic := fmt.Sprintf("%s/msn/%s/call_history", c.topicPrefix, msn)
	payload, err := json.Marshal(history)
	if err != nil {
		return fmt.Errorf("failed to marshal MSN call history: %w", err)
	}

	slog.Debug("Publishing MSN call history",
		"topic", topic,
		"msn", msn,
		"call_count", len(history.Calls))

	return c.publishWithoutRetain(topic, payload)
}

// LoadMSNCallHistoriesFromDB loads MSN call histories from database and populates in-memory histories
func (c *Client) LoadMSNCallHistoriesFromDB() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.db == nil {
		return fmt.Errorf("database client not available")
	}

	for msn, history := range c.msnCallHistories {
		// Load calls from database for this MSN
		dbCalls, err := c.db.GetCallsByMSN(msn, history.MaxSize)
		if err != nil {
			slog.Error("Failed to load calls from database",
				"msn", msn,
				"error", err)
			continue
		}

		// Convert database calls to MSNCallEvents
		msnCallEvents := make([]types.MSNCallEvent, 0)
		for _, dbCall := range dbCalls {
			// Skip calls without end timestamp
			if dbCall.EndTimestamp == nil {
				continue
			}

			// Helper function to safely dereference string pointers
			safeString := func(s *string) string {
				if s == nil {
					return ""
				}
				return *s
			}

			// Helper function to safely dereference int pointers
			safeInt := func(i *int) int {
				if i == nil {
					return 0
				}
				return *i
			}

			caller := safeString(dbCall.Caller)
			called := safeString(dbCall.Called)
			callerMSN := safeString(dbCall.CallerMSN)
			calledMSN := safeString(dbCall.CalledMSN)

			// Resolve caller and called names from database
			var callerName, calledName string
			if caller != "" {
				if name, err := c.db.GetPhoneNumberName(caller); err == nil && name != "" {
					callerName = name
				}
			}
			if called != "" {
				if name, err := c.db.GetPhoneNumberName(called); err == nil && name != "" {
					calledName = name
				}
			}

			// Determine direction based on MSN
			var direction types.CallDirection
			if callerMSN == msn {
				direction = types.CallDirectionOutbound
			} else if calledMSN == msn {
				direction = types.CallDirectionInbound
			}

			// Get finish state
			finishState := ""
			if dbCall.FinishState != nil {
				finishState = string(*dbCall.FinishState)
			}

			// Use extension information from database if available, otherwise try MSN associations
			var callerExtension, calledExtension *types.ExtensionInfo

			// First priority: Use extension info stored in database
			if dbCall.CallerExtension != nil {
				callerExtension = dbCall.CallerExtension
			} else if callerMSN != "" && c.pbxConfig != nil {
				// Fallback: If caller has MSN, find the extension associated with that MSN
				for _, ext := range c.pbxConfig.Extensions {
					// Check if extension name contains the MSN (like "T 22 (990134)")
					if strings.Contains(ext.Name, callerMSN) {
						callerExtension = &types.ExtensionInfo{
							Number: ext.Number,
							Name:   ext.Name,
							Type:   ext.Type,
						}
						break
					}
				}
			}

			// First priority: Use extension info stored in database
			if dbCall.CalledExtension != nil {
				calledExtension = dbCall.CalledExtension
			} else if calledMSN != "" && c.pbxConfig != nil {
				// Fallback: If called has MSN, find the extension associated with that MSN
				for _, ext := range c.pbxConfig.Extensions {
					// Check if extension name contains the MSN (like "T 22 (3698237)")
					if strings.Contains(ext.Name, calledMSN) {
						calledExtension = &types.ExtensionInfo{
							Number: ext.Number,
							Name:   ext.Name,
							Type:   ext.Type,
						}
						break
					}
				}
			}

			// Convert database Call to MSNCallEvent
			msnCallEvent := types.MSNCallEvent{
				ID:        dbCall.CallID.String(),
				Timestamp: *dbCall.EndTimestamp, // Use end timestamp for completed calls
				Direction: direction,
				Line:      dbCall.Line,
				Trunk:     safeString(dbCall.Trunk),
				Caller: types.LineStatusParticipant{
					PhoneNumber: caller,
					Name:        callerName,
				},
				Called: types.LineStatusParticipant{
					PhoneNumber: called,
					Name:        calledName,
				},
				CallerMSN:       callerMSN,
				CalledMSN:       calledMSN,
				CallerExtension: callerExtension,
				CalledExtension: calledExtension,
				Duration:        safeInt(dbCall.Duration),
				FinishState:     finishState,
			}

			msnCallEvents = append(msnCallEvents, msnCallEvent)
		}

		// Update the in-memory history
		history.Calls = msnCallEvents
		history.UpdatedAt = time.Now()

		slog.Debug("Loaded calls from database",
			"msn", msn,
			"call_count", len(msnCallEvents))
	}

	return nil
}

// PublishAllMSNCallHistories publishes all MSN call histories (typically called at startup)
func (c *Client) PublishAllMSNCallHistories() error {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if !c.connected {
		return fmt.Errorf("MQTT client not connected")
	}

	for msn, history := range c.msnCallHistories {
		if err := c.publishMSNCallHistory(msn, history); err != nil {
			slog.Error("Failed to publish MSN call history",
				"msn", msn,
				"error", err)
			// Continue with other MSNs even if one fails
		}
	}

	return nil
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

	slog.Debug("Publishing birth message", "topic", topic)
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

// subscribeToPhoneNumberRPC subscribes to phone number RPC topics
func (c *Client) subscribeToPhoneNumberRPC() error {
	if c.db == nil {
		slog.Warn("Database client not available, skipping phone number RPC subscription")
		return nil
	}

	// Subscribe to phone_number RPC request topic
	topic := fmt.Sprintf("%s/phone_number/request", c.topicPrefix)
	slog.Info("Subscribing to phone number RPC topic", "topic", topic)

	if token := c.client.Subscribe(topic, c.qos, c.handlePhoneNumberRPC); token.Wait() && token.Error() != nil {
		return fmt.Errorf("failed to subscribe to phone number RPC topic %s: %w", topic, token.Error())
	}

	slog.Info("Successfully subscribed to phone number RPC topic", "topic", topic)
	return nil
}

// handlePhoneNumberRPC handles phone number RPC requests
func (c *Client) handlePhoneNumberRPC(client mqtt.Client, msg mqtt.Message) {
	slog.Info("Received phone number RPC request",
		"topic", msg.Topic(),
		"payload", string(msg.Payload()))

	// Parse RPC request
	var request database.PhoneNumberRPCRequest
	if err := json.Unmarshal(msg.Payload(), &request); err != nil {
		slog.Error("Failed to parse phone number RPC request", "error", err)
		c.publishRPCError("", fmt.Sprintf("Invalid JSON: %v", err))
		return
	}

	// Validate request
	if err := request.Validate(); err != nil {
		slog.Error("Invalid phone number RPC request", "error", err)
		c.publishRPCError(request.ID, err.Error())
		return
	}

	// Process request using database client
	response := c.db.ProcessPhoneNumberRPC(&request)

	// Publish response
	if err := c.publishRPCResponse(*response); err != nil {
		slog.Error("Failed to publish RPC response", "error", err)
	}
}

// publishRPCResponse publishes an RPC response
func (c *Client) publishRPCResponse(response database.PhoneNumberRPCResponse) error {
	topic := fmt.Sprintf("%s/phone_number/response", c.topicPrefix)

	payload, err := json.Marshal(response)
	if err != nil {
		return fmt.Errorf("failed to marshal RPC response: %w", err)
	}

	slog.Info("Publishing RPC response",
		"topic", topic,
		"payload", string(payload))

	if token := c.client.Publish(topic, c.qos, false, payload); token.Wait() && token.Error() != nil {
		return fmt.Errorf("failed to publish RPC response: %w", token.Error())
	}

	return nil
}

// publishRPCError publishes an RPC error response
func (c *Client) publishRPCError(requestID, errorMsg string) {
	response := database.PhoneNumberRPCResponse{
		ID:        requestID,
		Success:   false,
		Error:     errorMsg,
		Timestamp: time.Now(),
	}

	if err := c.publishRPCResponse(response); err != nil {
		slog.Error("Failed to publish RPC error response", "error", err)
	}
}

// BootstrapFromDatabase loads current state from database and publishes initial MQTT messages
func (c *Client) BootstrapFromDatabase() error {
	slog.Info("Bootstrapping MQTT state from database...")

	// 1. Bootstrap line statuses with real database data
	if err := c.bootstrapLineStatuses(); err != nil {
		slog.Error("Failed to bootstrap line statuses", "error", err)
		return err
	}

	// 2. Bootstrap MSN call histories
	if err := c.LoadMSNCallHistoriesFromDB(); err != nil {
		slog.Error("Failed to load MSN call histories", "error", err)
		return err
	}

	// 3. Publish all MSN call histories
	if err := c.PublishAllMSNCallHistories(); err != nil {
		slog.Error("Failed to publish MSN call histories", "error", err)
		return err
	}

	slog.Info("Database bootstrap completed")
	return nil
}

// bootstrapLineStatuses creates line status based on real database events or publishes null for unused lines
func (c *Client) bootstrapLineStatuses() error {
	slog.Info("Bootstrapping line statuses...")

	if c.db == nil {
		slog.Warn("No database available, skipping line status bootstrap")
		return nil
	}

	// Check typical FRITZ!Box lines (0-7)
	for line := 0; line <= 7; line++ {
		// Try to get the most recent call for this line from database
		calls, err := c.db.GetCallsByLine(line, 1)
		if err != nil {
			slog.Error("Failed to query calls for line",
				"line", line,
				"error", err)
			continue
		}

		if len(calls) == 0 {
			// No calls found for this line - send retained null message
			topic := fmt.Sprintf("%s/line/%d/status", c.topicPrefix, line)
			if err := c.publishNull(topic); err != nil {
				slog.Error("Failed to publish null message for line",
					"line", line,
					"error", err)
			} else {
				slog.Debug("Published null message for unused line", "line", line)
			}
			continue
		}

		// Found real call data - create line status from most recent call
		lastCall := calls[0]

		// Get names for phone numbers
		var callerName, calledName string
		if lastCall.Caller != nil {
			if name, err := c.db.GetPhoneNumberName(*lastCall.Caller); err == nil && name != "" {
				callerName = name
			}
		}
		if lastCall.Called != nil {
			if name, err := c.db.GetPhoneNumberName(*lastCall.Called); err == nil && name != "" {
				calledName = name
			}
		}

		// Determine current status (finished calls should show as idle)
		currentStatus := types.CallStatusIdle
		var callID string
		if lastCall.FinishState == nil {
			// Call is still ongoing, use the stored status and call ID
			currentStatus = lastCall.Status
			callID = lastCall.CallID.String()
		}
		// If call is finished, don't set call ID - let FSM start fresh

		lineStatus := &types.LineStatus{
			ID:   callID, // Empty for finished calls, allows new events to start fresh session
			Line: line,
			Trunk: func() string {
				if lastCall.Trunk != nil {
					return *lastCall.Trunk
				}
				return fmt.Sprintf("SIP%d", line)
			}(),
			Direction: func() types.CallDirection {
				if lastCall.CallerMSN != nil {
					return types.CallDirectionOutbound
				}
				return types.CallDirectionInbound
			}(),
			Status:    currentStatus,
			Extension: nil, // No extension info available during bootstrap
			Caller: types.LineStatusParticipant{
				PhoneNumber: func() string {
					if lastCall.Caller != nil {
						return *lastCall.Caller
					}
					return ""
				}(),
				Name: callerName,
			},
			Called: types.LineStatusParticipant{
				PhoneNumber: func() string {
					if lastCall.Called != nil {
						return *lastCall.Called
					}
					return ""
				}(),
				Name: calledName,
			},
			LastEvent:   fmt.Sprintf("Database bootstrap - %s", string(currentStatus)),
			LastUpdated: time.Now(),
		}

		// Publish line status
		if err := c.publishLineStatus(lineStatus); err != nil {
			slog.Error("Failed to publish bootstrap line status",
				"line", line,
				"error", err)
		} else {
			slog.Info("Published bootstrap status for line from database",
				"line", line,
				"status", currentStatus)
		}
	}

	slog.Info("Line status bootstrap completed")
	return nil
}

// publishNull sends a retained null message to clear a topic
func (c *Client) publishNull(topic string) error {
	if c.client == nil || !c.client.IsConnected() {
		return fmt.Errorf("MQTT client not connected")
	}

	slog.Debug("Publishing null message", "topic", topic)

	// Send empty byte slice with retain=true to clear the topic
	token := c.client.Publish(topic, c.qos, true, []byte{})
	if token.Wait() && token.Error() != nil {
		return fmt.Errorf("failed to publish null message: %w", token.Error())
	}

	return nil
}

// Home Assistant MQTT Discovery structures and methods

// HADevice represents a Home Assistant device in MQTT Discovery
type HADevice struct {
	Identifiers  []string `json:"identifiers"`
	Name         string   `json:"name"`
	Model        string   `json:"model"`
	Manufacturer string   `json:"manufacturer"`
	SwVersion    string   `json:"sw_version,omitempty"`
}

// HAEntityConfig represents base configuration for Home Assistant entities
type HAEntityConfig struct {
	Name                 string    `json:"name"`
	UniqueID             string    `json:"unique_id"`
	StateTopic           string    `json:"state_topic"`
	Device               *HADevice `json:"device"`
	AvailabilityTopic    string    `json:"availability_topic"`
	AvailabilityMode     string    `json:"availability_mode,omitempty"`
	AvailabilityTemplate string    `json:"availability_template,omitempty"`
	PayloadAvailable     string    `json:"payload_available,omitempty"`
	PayloadNotAvailable  string    `json:"payload_not_available,omitempty"`
}

// HASensorConfig represents a Home Assistant sensor configuration
type HASensorConfig struct {
	HAEntityConfig
	ValueTemplate          string   `json:"value_template,omitempty"`
	JSONAttributesTemplate string   `json:"json_attributes_template,omitempty"`
	JSONAttributesTopic    string   `json:"json_attributes_topic,omitempty"`
	Icon                   string   `json:"icon,omitempty"`
	UnitOfMeasurement      string   `json:"unit_of_measurement,omitempty"`
	DeviceClass            string   `json:"device_class,omitempty"`
	Options                []string `json:"options,omitempty"`
}

// MarshalJSON implements custom JSON marshaling for HASensorConfig
// Automatically sets DeviceClass to "enum" when Options is set
func (h *HASensorConfig) MarshalJSON() ([]byte, error) {
	// Create a type alias to avoid infinite recursion
	type Alias HASensorConfig

	// Create a copy of the struct
	temp := struct {
		*Alias
	}{
		Alias: (*Alias)(h),
	}

	// If Options is set and not empty, force DeviceClass to "enum"
	if len(h.Options) > 0 {
		temp.Alias.DeviceClass = "enum"
	}

	return json.Marshal(temp)
}

// setupHomeAssistantDiscovery publishes Home Assistant MQTT Discovery configurations
func (c *Client) setupHomeAssistantDiscovery() error {
	slog.Debug("Setting up Home Assistant MQTT Discovery...")

	device := &HADevice{
		Identifiers:  []string{c.deviceIdentifier},
		Name:         c.deviceName,
		Model:        "FRITZ!Box Callmonitor to MQTT Bridge",
		Manufacturer: "fritz-callmonitor2mqtt",
		SwVersion:    c.version,
	}

	// Setup discovery for line status sensors
	for line := 0; line < 8; line++ {
		if err := c.setupLineStatusDiscovery(line, device); err != nil {
			slog.Error("Failed to setup line status discovery",
				"line", line,
				"error", err)
		}
		if err := c.setupLineLastEventDiscovery(line, device); err != nil {
			slog.Error("Failed to setup line last_event discovery",
				"line", line,
				"error", err)
		}
	}

	// Setup discovery for MSN call history sensors
	for msn := range c.msnCallHistories {
		if err := c.setupMSNCallHistoryDiscovery(msn, device); err != nil {
			slog.Error("Failed to setup MSN call history discovery",
				"msn", msn,
				"error", err)
		}
	}

	slog.Debug("Home Assistant MQTT Discovery setup completed")
	return nil
}

// setupLineStatusDiscovery creates discovery config for line status sensors
func (c *Client) setupLineStatusDiscovery(line int, device *HADevice) error {
	config := &HASensorConfig{
		HAEntityConfig: HAEntityConfig{
			Name:                 fmt.Sprintf("FRITZ!Box Line %d Status", line),
			UniqueID:             fmt.Sprintf("%s_line_%d_status", c.deviceIdentifier, line),
			StateTopic:           fmt.Sprintf("%s/line/%d/status", c.topicPrefix, line),
			Device:               device,
			AvailabilityTopic:    fmt.Sprintf("%s/status", c.topicPrefix),
			AvailabilityTemplate: `{{ value_json.state }}`,
			PayloadAvailable:     "online",
			PayloadNotAvailable:  "offline",
		},
		ValueTemplate:       `{{ value_json.status }}`,
		JSONAttributesTopic: fmt.Sprintf("%s/line/%d/status", c.topicPrefix, line),
		Icon:                "mdi:phone",
		Options: []string{
			"idle",
			"ringing",
			"calling",
			"talking",
			"missedCall",
			"notReached",
			"finished",
			"voiceBox",
		},
	}

	discoveryTopic := fmt.Sprintf("homeassistant/sensor/%s/line_%d_status/config", c.deviceIdentifier, line)
	return c.publishDiscoveryConfig(discoveryTopic, config)
}

// setupLineLastEventDiscovery creates discovery config for line last_event sensors
func (c *Client) setupLineLastEventDiscovery(line int, device *HADevice) error {
	config := &HASensorConfig{
		HAEntityConfig: HAEntityConfig{
			Name:                 fmt.Sprintf("FRITZ!Box Line %d Last Event", line),
			UniqueID:             fmt.Sprintf("%s_line_%d_last_event", c.deviceIdentifier, line),
			StateTopic:           fmt.Sprintf("%s/line/%d/last_event", c.topicPrefix, line),
			Device:               device,
			AvailabilityTopic:    fmt.Sprintf("%s/status", c.topicPrefix),
			AvailabilityTemplate: `{{ value_json.state }}`,
			PayloadAvailable:     "online",
			PayloadNotAvailable:  "offline",
		},
		ValueTemplate:       `{{ value_json.type }}`,
		JSONAttributesTopic: fmt.Sprintf("%s/line/%d/last_event", c.topicPrefix, line),
		Icon:                "mdi:phone-log",
		Options: []string{
			"ring",
			"call",
			"connect",
			"disconnect",
		},
	}

	discoveryTopic := fmt.Sprintf("homeassistant/sensor/%s/line_%d_last_event/config", c.deviceIdentifier, line)
	return c.publishDiscoveryConfig(discoveryTopic, config)
}

// setupMSNCallHistoryDiscovery creates discovery config for MSN call history sensors
func (c *Client) setupMSNCallHistoryDiscovery(msn string, device *HADevice) error {
	config := &HASensorConfig{
		HAEntityConfig: HAEntityConfig{
			Name:                 fmt.Sprintf("FRITZ!Box MSN %s Call History", msn),
			UniqueID:             fmt.Sprintf("%s_msn_%s_call_history", c.deviceIdentifier, msn),
			StateTopic:           fmt.Sprintf("%s/msn/%s/call_history", c.topicPrefix, msn),
			Device:               device,
			AvailabilityTopic:    fmt.Sprintf("%s/status", c.topicPrefix),
			AvailabilityTemplate: `{{ value_json.state }}`,
			PayloadAvailable:     "online",
			PayloadNotAvailable:  "offline",
		},
		ValueTemplate:       `{{ value_json.calls | length }}`,
		JSONAttributesTopic: fmt.Sprintf("%s/msn/%s/call_history", c.topicPrefix, msn),
		Icon:                "mdi:phone-log",
		UnitOfMeasurement:   "calls",
	}

	discoveryTopic := fmt.Sprintf("homeassistant/sensor/%s/msn_%s_call_history/config", c.deviceIdentifier, msn)
	return c.publishDiscoveryConfig(discoveryTopic, config)
}

// publishDiscoveryConfig publishes a discovery configuration to Home Assistant
func (c *Client) publishDiscoveryConfig(topic string, config interface{}) error {
	payload, err := json.Marshal(config)
	if err != nil {
		return fmt.Errorf("failed to marshal discovery config: %w", err)
	}

	slog.Debug("Publishing HA discovery config", "topic", topic)

	// Discovery configs should be retained
	token := c.client.Publish(topic, c.qos, true, payload)
	if token.Wait() && token.Error() != nil {
		return fmt.Errorf("failed to publish discovery config: %w", token.Error())
	}

	return nil
}

// removeHomeAssistantDiscovery removes Home Assistant MQTT Discovery configurations
//
//nolint:unused // Keep for future use or manual cleanup
func (c *Client) removeHomeAssistantDiscovery() error {
	slog.Debug("Removing Home Assistant MQTT Discovery configurations...")

	// Remove discovery for line status sensors
	for line := 0; line < 8; line++ {
		statusTopic := fmt.Sprintf("homeassistant/sensor/%s/line_%d_status/config", c.deviceIdentifier, line)
		if err := c.publishNull(statusTopic); err != nil {
			slog.Error("Failed to remove line status discovery",
				"line", line,
				"error", err)
		}

		eventTopic := fmt.Sprintf("homeassistant/sensor/%s/line_%d_last_event/config", c.deviceIdentifier, line)
		if err := c.publishNull(eventTopic); err != nil {
			slog.Error("Failed to remove line last_event discovery",
				"line", line,
				"error", err)
		}
	}

	// Remove discovery for MSN call history sensors
	for msn := range c.msnCallHistories {
		discoveryTopic := fmt.Sprintf("homeassistant/sensor/%s/msn_%s_call_history/config", c.deviceIdentifier, msn)
		if err := c.publishNull(discoveryTopic); err != nil {
			slog.Error("Failed to remove MSN call history discovery",
				"msn", msn,
				"error", err)
		}
	}

	return nil
}
