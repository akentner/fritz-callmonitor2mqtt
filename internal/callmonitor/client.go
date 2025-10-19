package callmonitor

import (
	"bufio"
	"fmt"
	"log/slog"
	"net"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"

	"fritz-callmonitor2mqtt/pkg/types"
)

// PhoneNumberLookup interface for resolving phone numbers to names
type PhoneNumberLookup interface {
	GetPhoneNumberName(phoneNumber string) (string, error)
}

// ExtensionLookup interface for resolving extension numbers to info
type ExtensionLookup interface {
	GetExtensionInfo(number string) *types.ExtensionInfo
}

// EventPersistence interface for storing raw events
type EventPersistence interface {
	InsertEvent(id uuid.UUID, timestamp time.Time, rawValue string) error
}

// lineState holds state information for a single line
type lineState struct {
	trunk     string
	direction types.CallDirection
	caller    string
	called    string
	callID    string
	lastSeen  time.Time
}

// Client represents a FRITZ!Box callmonitor client
type Client struct {
	host              string
	port              int
	conn              net.Conn
	eventChan         chan types.CallEvent
	errorChan         chan error
	stopChan          chan struct{}
	connected         bool
	timezone          *time.Location
	countryCode       string
	localAreaCode     string
	msns              []string          // Configured MSNs for detection
	phoneNumberLookup PhoneNumberLookup // Optional phone number name lookup
	extensionLookup   ExtensionLookup   // Optional extension info lookup
	eventPersistence  EventPersistence  // Optional raw event persistence
	lineStates        map[int]*lineState
	lineStatesMu      sync.RWMutex
	cleanupTicker     *time.Ticker
}

// NewClient creates a new callmonitor client
func NewClient(host string, port int, timezone *time.Location, countryCode string, localAreaCode string, msns []string) *Client {
	if timezone == nil {
		timezone = time.Local
	}
	return &Client{
		host:          host,
		port:          port,
		eventChan:     make(chan types.CallEvent, 100),
		errorChan:     make(chan error, 10),
		stopChan:      make(chan struct{}),
		timezone:      timezone,
		countryCode:   countryCode,
		localAreaCode: localAreaCode,
		msns:          msns,
		lineStates:    make(map[int]*lineState),
	}
}

// SetPhoneNumberLookup sets the phone number lookup interface
func (c *Client) SetPhoneNumberLookup(lookup PhoneNumberLookup) {
	c.phoneNumberLookup = lookup
}

// SetExtensionLookup sets the extension lookup interface
func (c *Client) SetExtensionLookup(lookup ExtensionLookup) {
	c.extensionLookup = lookup
}

// SetEventPersistence sets the event persistence interface
func (c *Client) SetEventPersistence(persistence EventPersistence) {
	c.eventPersistence = persistence
}

// Connect establishes connection to FRITZ!Box callmonitor
func (c *Client) Connect() error {
	// Create new stop channel for this connection
	c.stopChan = make(chan struct{})

	conn, err := net.Dial("tcp", net.JoinHostPort(c.host, fmt.Sprintf("%d", c.port)))
	if err != nil {
		return fmt.Errorf("failed to connect to FRITZ!Box callmonitor: %w", err)
	}

	c.conn = conn
	c.connected = true

	// Start reading in background
	go c.readLoop()

	// Start cleanup routine for stale line states
	c.startCleanupRoutine()

	return nil
}

// Disconnect closes the connection
func (c *Client) Disconnect() error {
	if !c.connected {
		return nil
	}

	c.connected = false

	// Stop cleanup ticker
	c.stopCleanupRoutine()

	// Close stop channel safely
	select {
	case <-c.stopChan:
		// Channel already closed
	default:
		close(c.stopChan)
	}

	if c.conn != nil {
		return c.conn.Close()
	}

	return nil
}

// Events returns the channel for call events
func (c *Client) Events() <-chan types.CallEvent {
	return c.eventChan
}

// Errors returns the channel for errors
func (c *Client) Errors() <-chan error {
	return c.errorChan
}

// IsConnected returns the connection status
func (c *Client) IsConnected() bool {
	return c.connected
}

// readLoop continuously reads from the FRITZ!Box connection
func (c *Client) readLoop() {
	defer func() {
		c.connected = false
		if c.conn != nil {
			_ = c.conn.Close() // Ignore error in cleanup
		}
	}()

	scanner := bufio.NewScanner(c.conn)

	for {
		select {
		case <-c.stopChan:
			return
		default:
			if !scanner.Scan() {
				if err := scanner.Err(); err != nil {
					c.errorChan <- fmt.Errorf("error reading from connection: %w", err)
				} else {
					c.errorChan <- fmt.Errorf("connection closed by remote")
				}
				return
			}

			line := strings.TrimSpace(scanner.Text())
			if line == "" {
				continue
			}

			event, err := c.parseEvent(line)
			if err != nil {
				c.errorChan <- fmt.Errorf("error parsing call event: %w", err)
				continue
			}

			select {
			case c.eventChan <- *event:
			case <-c.stopChan:
				return
			default:
				// Channel is full, skip this event
			}
		}
	}
}

// parseEvent parses a FRITZ!Box callmonitor line into a CallEvent
func (c *Client) parseEvent(rawMessage string) (*types.CallEvent, error) {
	// Split the message into parts
	parts := strings.Split(rawMessage, ";")
	if len(parts) < 4 {
		return nil, fmt.Errorf("invalid callmonitor format (too few parts): %s", rawMessage)
	}

	// Parse timestamp
	timestamp, err := c.parseTimestamp(parts[0])
	if err != nil {
		timestamp = time.Now() // Fallback to current time
	}

	// Generate UUID v7 for this raw event
	eventUUID, err := uuid.NewV7()
	if err != nil {
		return nil, fmt.Errorf("failed to generate UUID v7 for event: %w", err)
	}

	// Persist raw event to database if persistence is configured
	if c.eventPersistence != nil {
		if err := c.eventPersistence.InsertEvent(eventUUID, timestamp, rawMessage); err != nil {
			// Log error but don't fail event processing
			// The error will be visible in the application logs
			_ = err // We could add proper logging here
		}
	}

	// Parse call type and delegate to specific parser
	callTypeStr := strings.ToUpper(parts[1])

	lineID, err := strconv.Atoi(parts[2])
	if err != nil {
		return nil, fmt.Errorf("invalid LineID (not an int): %v", err)
	}

	switch callTypeStr {
	case "RING":
		return c.parseEventRing(parts, timestamp, lineID, rawMessage)
	case "CALL":
		return c.parseEventCall(parts, timestamp, lineID, rawMessage)
	case "CONNECT":
		return c.parseEventConnect(parts, timestamp, lineID, rawMessage)
	case "DISCONNECT":
		return c.parseEventDisconnect(parts, timestamp, lineID, rawMessage)
	default:
		return nil, fmt.Errorf("unknown call type: %s", callTypeStr)
	}
}

// parseEventRing parses RING events
// Format: timestamp;RING;line;caller;called;trunk;
// Example: 09.09.25 17:33:01;RING;0;0178123456789;0119876543;SIP4;
func (c *Client) parseEventRing(parts []string, timestamp time.Time, lineID int, rawMessage string) (*types.CallEvent, error) {
	if len(parts) < 5 {
		return nil, fmt.Errorf("invalid RING format: need at least 5 parts, got %d", len(parts))
	}

	// Generate UUID v7 for this call
	callUUID, err := uuid.NewV7()
	if err != nil {
		return nil, fmt.Errorf("failed to generate UUID v7: %w", err)
	}
	callID := callUUID.String()

	event := &types.CallEvent{
		ID:         callID,
		Timestamp:  timestamp,
		Type:       types.CallTypeRing,
		Direction:  types.CallDirectionInbound,
		Line:       lineID,
		Trunk:      parts[5],
		Caller:     c.normalizePhoneNumber(parts[3]),
		Called:     c.normalizePhoneNumber(parts[4]),
		RawMessage: rawMessage,
	}

	// Enrich with MSN information
	event.EnrichWithMSNs(c.msns)

	// Enrich with phone number names
	c.enrichEventWithNames(event)

	// Store state for later events
	state := c.getOrCreateLineState(event.Line)
	if event.Trunk != "" {
		state.trunk = event.Trunk
	}
	state.direction = event.Direction
	state.caller = event.Caller
	state.called = event.Called
	state.callID = event.ID

	return event, nil
}

// parseEventCall parses CALL events
// Format: timestamp;CALL;line;extension;caller;called;trunk;
// Example: 09.09.25 17:33:34;CALL;1;21;9876543;0178123456789;SIP1;
func (c *Client) parseEventCall(parts []string, timestamp time.Time, line int, rawMessage string) (*types.CallEvent, error) {
	if len(parts) < 6 {
		return nil, fmt.Errorf("invalid CALL format: need at least 6 parts, got %d", len(parts))
	}

	// Generate UUID v7 for this call
	callUUID, err := uuid.NewV7()
	if err != nil {
		return nil, fmt.Errorf("failed to generate UUID v7: %w", err)
	}
	callID := callUUID.String()

	event := &types.CallEvent{
		ID:         callID,
		Timestamp:  timestamp,
		Type:       types.CallTypeCall,
		Direction:  types.CallDirectionOutbound,
		Line:       line,
		Trunk:      parts[6],
		Extension:  parts[3],
		Caller:     c.normalizePhoneNumber(parts[4]),
		Called:     c.normalizePhoneNumber(parts[5]),
		RawMessage: rawMessage,
	}

	// Enrich with MSN information
	event.EnrichWithMSNs(c.msns)

	// Enrich with phone number names
	c.enrichEventWithNames(event)

	// Store state for later events
	state := c.getOrCreateLineState(event.Line)
	if event.Trunk != "" {
		state.trunk = event.Trunk
	}
	state.direction = event.Direction
	state.caller = event.Caller
	state.called = event.Called
	state.callID = event.ID

	return event, nil
}

// parseConnectEvent parses CONNECT events
// caller_or_called depends on direction, it's always the number on the external
// Format: timestamp;CONNECT;line;extension;caller_or_called;
// Example 09.09.25 17:33:07;CONNECT;0;23;0178123456789;
func (c *Client) parseEventConnect(parts []string, timestamp time.Time, line int, rawMessage string) (*types.CallEvent, error) {
	if len(parts) < 4 {
		return nil, fmt.Errorf("invalid CONNECT format: need at least 4 parts, got %d", len(parts))
	}

	event := &types.CallEvent{
		Timestamp:  timestamp,
		Type:       types.CallTypeConnect,
		Line:       line,
		Extension:  parts[3],
		RawMessage: rawMessage,
	}

	// Look up stored state from RING/CALL event
	if state := c.getLineState(event.Line); state != nil {
		event.ID = state.callID
		event.Trunk = state.trunk
		event.Direction = state.direction
		event.Caller = state.caller
		event.Called = state.called
	}

	// Enrich with MSN information
	event.EnrichWithMSNs(c.msns)

	// Enrich with phone number names
	c.enrichEventWithNames(event)

	return event, nil
}

// parseEventDisconnect parses DISCONNECT events
// Format: timestamp;DISCONNECT;id;duration
// Example: 09.09.25 17:33:34;DISCONNECT;1;30;
func (c *Client) parseEventDisconnect(parts []string, timestamp time.Time, line int, rawMessage string) (*types.CallEvent, error) {
	if len(parts) < 3 {
		return nil, fmt.Errorf("invalid DISCONNECT format: need at least 3 parts, got %d", len(parts))
	}

	event := &types.CallEvent{
		Timestamp:  timestamp,
		Type:       types.CallTypeDisconnect,
		Line:       line,
		RawMessage: rawMessage,
	}

	// Look up stored call ID from RING/CALL event
	if state := c.getLineState(event.Line); state != nil {
		event.ID = state.callID
	}

	// parse duration
	if duration, err := strconv.Atoi(parts[3]); err == nil {
		event.Duration = duration
	}

	// Check for extension number in 5th part and look up if it's a VOICEBOX
	if len(parts) >= 5 && parts[4] != "" {
		// First check if it's the literal "VOICEBOX" string
		if parts[4] == "VOICEBOX" {
			// Try to find a configured VOICEBOX extension
			if voiceboxExt := c.findFirstVoiceboxExtension(); voiceboxExt != nil {
				event.CalledExtension = voiceboxExt
			} else {
				// Fallback if no VOICEBOX extension is configured
				event.CalledExtension = &types.ExtensionInfo{
					Number: "VOICEBOX",
					Name:   "Voicebox",
					Type:   "VOICEBOX",
				}
			}
		} else {
			// Check if the extension number corresponds to a configured VOICEBOX extension
			if c.extensionLookup != nil {
				if ext := c.extensionLookup.GetExtensionInfo(parts[4]); ext != nil && ext.Type == "VOICEBOX" {
					event.CalledExtension = ext
				}
			}
		}
	}

	// Look up and clean up the stored state
	if state := c.getLineState(event.Line); state != nil {
		event.ID = state.callID
		event.Trunk = state.trunk
		event.Direction = state.direction
		event.Caller = state.caller
		event.Called = state.called
	}

	// Clean up line state after DISCONNECT
	c.deleteLineState(event.Line)

	// Enrich with MSN information
	event.EnrichWithMSNs(c.msns)

	// Enrich with phone number names
	c.enrichEventWithNames(event)

	return event, nil
}

func (c *Client) normalizePhoneNumber(phoneNumber string) string {
	// First, clean up FRITZ!Box specific control characters
	// Remove all # characters which FRITZ!Box uses for call control (e.g., DTMF, transfer codes)
	phoneNumber = strings.ReplaceAll(phoneNumber, "#", "")

	// Remove trailing * if present (sometimes used for special calls)
	phoneNumber = strings.TrimSuffix(phoneNumber, "*")

	// If already international format (starts with +), return as is
	if strings.HasPrefix(phoneNumber, "+") {
		return phoneNumber
	}

	// Replace leading "00" with "+"
	if strings.HasPrefix(phoneNumber, "00") {
		phoneNumber = "+" + phoneNumber[2:]
		return phoneNumber
	}

	// Replace leading "0" with countryCode if configured
	if strings.HasPrefix(phoneNumber, "0") && c.countryCode != "" {
		phoneNumber = "+" + c.countryCode + phoneNumber[1:]
		return phoneNumber
	}

	// If phoneNumber does not start with "0" and is not international, prepend localAreaCode
	if c.localAreaCode != "" && c.countryCode != "" {
		phoneNumber = "+" + c.countryCode + c.localAreaCode + phoneNumber
	}

	return phoneNumber
}

// lookupPhoneNumberName retrieves the name for a phone number if available
func (c *Client) lookupPhoneNumberName(phoneNumber string) string {
	if c.phoneNumberLookup == nil || phoneNumber == "" {
		return ""
	}

	name, err := c.phoneNumberLookup.GetPhoneNumberName(phoneNumber)
	if err != nil {
		// Log error but don't fail event processing
		// TODO: Add proper logging interface
		return ""
	}

	return name
}

// enrichEventWithNames adds caller and called names and extension info to the event
func (c *Client) enrichEventWithNames(event *types.CallEvent) {
	// Resolve phone number names
	if event.Caller != "" {
		event.CallerName = c.lookupPhoneNumberName(event.Caller)
	}
	if event.Called != "" {
		event.CalledName = c.lookupPhoneNumberName(event.Called)
	}

	// Resolve extension information
	// For outbound calls, the extension is usually the caller (internal number)
	// For inbound calls, the extension is usually the called party (internal number)
	if event.Direction == types.CallDirectionOutbound && event.Extension != "" {
		// Outbound call: extension is the caller
		event.CallerExtension = c.resolveExtensionInfo(event.Extension)
	} else if event.Direction == types.CallDirectionInbound && event.Extension != "" {
		// Inbound call: extension is the called party
		event.CalledExtension = c.resolveExtensionInfo(event.Extension)
	}

	// Also try to resolve extension info for internal-to-internal calls
	// Check if caller or called number matches extension pattern
	if c.isExtensionNumber(event.Caller) {
		event.CallerExtension = c.resolveExtensionInfo(event.Caller)
	}
	if c.isExtensionNumber(event.Called) {
		event.CalledExtension = c.resolveExtensionInfo(event.Called)
	}
}

// parseTimestamp parses FRITZ!Box timestamp format
func (c *Client) parseTimestamp(timestampStr string) (time.Time, error) {
	// FRITZ!Box format: "21.09.25 15:30:45"
	layout := "02.01.06 15:04:05"

	// Try parsing as-is first (assuming it's from current century)
	if t, err := time.ParseInLocation(layout, timestampStr, c.timezone); err == nil {
		// Check if this results in a reasonable year (within last 50 years or next 10 years)
		currentYear := time.Now().Year()
		if t.Year() >= currentYear-50 && t.Year() <= currentYear+10 {
			return t, nil
		}

		// If year seems wrong, adjust it to be within reasonable bounds
		// Usually this means the 2-digit year was interpreted incorrectly
		if t.Year() < currentYear-50 {
			// Add 100 years (e.g., 1925 -> 2025)
			return t.AddDate(100, 0, 0), nil
		} else if t.Year() > currentYear+10 {
			// Subtract 100 years (e.g., 2125 -> 2025)
			return t.AddDate(-100, 0, 0), nil
		}

		return t, nil
	}

	// Fallback: try parsing with current year prefix
	currentYear := time.Now().Year()
	fullTimestamp := fmt.Sprintf("%02d.%s", currentYear%100, timestampStr[3:])
	return time.ParseInLocation(layout, fullTimestamp, c.timezone)
}

// resolveExtensionInfo resolves extension information for a given number
func (c *Client) resolveExtensionInfo(extension string) *types.ExtensionInfo {
	if c.extensionLookup == nil || extension == "" {
		return nil
	}

	// Try to get extension info from lookup
	if ext := c.extensionLookup.GetExtensionInfo(extension); ext != nil {
		return ext
	}

	return nil
}

// findFirstVoiceboxExtension finds the first configured VOICEBOX extension
func (c *Client) findFirstVoiceboxExtension() *types.ExtensionInfo {
	if c.extensionLookup == nil {
		return nil
	}

	// Check the common VOICEBOX range (40-49) which maps to internal **600-**609
	for i := 40; i <= 49; i++ {
		extNum := fmt.Sprintf("%d", i)
		if ext := c.extensionLookup.GetExtensionInfo(extNum); ext != nil && ext.Type == "VOICEBOX" {
			return ext
		}
	}
	return nil
}

// isExtensionNumber checks if a number looks like an extension (internal number)
func (c *Client) isExtensionNumber(number string) bool {
	if number == "" {
		return false
	}

	// Extension numbers are typically 1-4 digits and start with specific ranges
	// Common ranges: 1-99 (analog), 600-699 (voicebox), 610-619 (DECT), 620-629 (VoIP)
	if len(number) <= 4 && len(number) >= 1 {
		// Check for common extension patterns
		if strings.HasPrefix(number, "6") || // 6xx extensions (DECT/VoIP/Voicebox)
			(len(number) <= 2 && number >= "1" && number <= "99") { // 1-99 analog
			return true
		}
	}

	return false
}

// startCleanupRoutine starts a background goroutine to clean up stale line states
func (c *Client) startCleanupRoutine() {
	// Stop existing ticker if running
	c.stopCleanupRoutine()

	// Create new ticker that runs every 5 minutes
	c.cleanupTicker = time.NewTicker(5 * time.Minute)

	go func() {
		for {
			select {
			case <-c.cleanupTicker.C:
				c.cleanupStaleLineStates()
			case <-c.stopChan:
				return
			}
		}
	}()

	slog.Debug("Callmonitor cleanup routine started")
}

// stopCleanupRoutine stops the cleanup ticker
func (c *Client) stopCleanupRoutine() {
	if c.cleanupTicker != nil {
		c.cleanupTicker.Stop()
		c.cleanupTicker = nil
		slog.Debug("Callmonitor cleanup routine stopped")
	}
}

// cleanupStaleLineStates removes line states that haven't been updated in a while
func (c *Client) cleanupStaleLineStates() {
	c.lineStatesMu.Lock()
	defer c.lineStatesMu.Unlock()

	// Consider states stale after 1 hour of inactivity
	staleTimeout := 1 * time.Hour
	now := time.Now()

	cleanedCount := 0
	for line, state := range c.lineStates {
		if now.Sub(state.lastSeen) > staleTimeout {
			delete(c.lineStates, line)
			cleanedCount++
			slog.Debug("Cleaned up stale line state",
				"line", line,
				"last_seen", state.lastSeen.Format(time.RFC3339))
		}
	}

	if cleanedCount > 0 {
		slog.Info("Cleaned up stale line states",
			"cleaned_count", cleanedCount,
			"remaining", len(c.lineStates))
	}
}

// getOrCreateLineState retrieves or creates a line state for the given line
func (c *Client) getOrCreateLineState(line int) *lineState {
	c.lineStatesMu.Lock()
	defer c.lineStatesMu.Unlock()

	state, exists := c.lineStates[line]
	if !exists {
		state = &lineState{
			lastSeen: time.Now(),
		}
		c.lineStates[line] = state
	}
	state.lastSeen = time.Now()
	return state
}

// getLineState retrieves a line state (read-only)
func (c *Client) getLineState(line int) *lineState {
	c.lineStatesMu.RLock()
	defer c.lineStatesMu.RUnlock()
	return c.lineStates[line]
}

// deleteLineState removes a line state
func (c *Client) deleteLineState(line int) {
	c.lineStatesMu.Lock()
	defer c.lineStatesMu.Unlock()
	delete(c.lineStates, line)
}
