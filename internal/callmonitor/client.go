package callmonitor

import (
	"bufio"
	"fmt"
	"net"
	"strconv"
	"strings"
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
	msns              []string                    // Configured MSNs for detection
	phoneNumberLookup PhoneNumberLookup           // Optional phone number name lookup
	extensionLookup   ExtensionLookup             // Optional extension info lookup
	lineIdToTrunk     map[int]string              // Maps line ID to Line Name
	lineIdToDirection map[int]types.CallDirection // Maps line ID to Line Direction
	lineIdToCaller    map[int]string              // Maps line ID to Caller
	lineIdToCalled    map[int]string              // Maps line ID to Called
	lineIdToCallID    map[int]string              // Maps line ID to Call UUID for tracking across states
}

// NewClient creates a new callmonitor client
func NewClient(host string, port int, timezone *time.Location, countryCode string, localAreaCode string, msns []string) *Client {
	if timezone == nil {
		timezone = time.Local
	}
	return &Client{
		host:              host,
		port:              port,
		eventChan:         make(chan types.CallEvent, 100),
		errorChan:         make(chan error, 10),
		stopChan:          make(chan struct{}),
		timezone:          timezone,
		countryCode:       countryCode,
		localAreaCode:     localAreaCode,
		msns:              msns,
		lineIdToTrunk:     make(map[int]string),
		lineIdToDirection: make(map[int]types.CallDirection),
		lineIdToCaller:    make(map[int]string),
		lineIdToCalled:    make(map[int]string),
		lineIdToCallID:    make(map[int]string),
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

	return nil
}

// Disconnect closes the connection
func (c *Client) Disconnect() error {
	if !c.connected {
		return nil
	}

	c.connected = false

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

	// Store mapping for later DISCONNECT events
	if event.Trunk != "" {
		c.lineIdToTrunk[event.Line] = event.Trunk
	}
	c.lineIdToDirection[event.Line] = event.Direction
	c.lineIdToCaller[event.Line] = event.Caller
	c.lineIdToCalled[event.Line] = event.Called
	c.lineIdToCallID[event.Line] = event.ID

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

	// Store mapping for later DISCONNECT events
	if event.Trunk != "" {
		c.lineIdToTrunk[event.Line] = event.Trunk
	}
	c.lineIdToDirection[event.Line] = event.Direction
	c.lineIdToCaller[event.Line] = event.Caller
	c.lineIdToCalled[event.Line] = event.Called
	c.lineIdToCallID[event.Line] = event.ID

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

	// Look up stored call ID from RING/CALL event
	if callID, exists := c.lineIdToCallID[event.Line]; exists {
		event.ID = callID
	}

	// Look up stored line ID from RING/CALL event
	if trunk, exists := c.lineIdToTrunk[event.Line]; exists {
		event.Trunk = trunk
	}

	// Look up stored call direction from RING/CALL event
	if direction, exists := c.lineIdToDirection[event.Line]; exists {
		event.Direction = direction
	}

	// Look up stored caller and called numbers from RING/CALL event
	if caller, exists := c.lineIdToCaller[event.Line]; exists {
		event.Caller = caller
	}

	if called, exists := c.lineIdToCalled[event.Line]; exists {
		event.Called = called
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
	if callID, exists := c.lineIdToCallID[event.Line]; exists {
		event.ID = callID
	}

	// parse duration
	if duration, err := strconv.Atoi(parts[3]); err == nil {
		event.Duration = duration
	}

	// Look up and clean up the stored line ID mapping
	if trunk, exists := c.lineIdToTrunk[event.Line]; exists {
		event.Trunk = trunk
		delete(c.lineIdToTrunk, event.Line)
	}

	// Look up and clean up the stored call direction
	if direction, exists := c.lineIdToDirection[event.Line]; exists {
		event.Direction = direction
		delete(c.lineIdToDirection, event.Line)
	}

	// Look up and clean up the stored caller
	if caller, exists := c.lineIdToCaller[event.Line]; exists {
		event.Caller = caller
		delete(c.lineIdToCaller, event.Line)
	}

	// Look up and clean up the stored called
	if called, exists := c.lineIdToCalled[event.Line]; exists {
		event.Called = called
		delete(c.lineIdToCalled, event.Line)
	}

	// Clean up the stored call ID
	delete(c.lineIdToCallID, event.Line)

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
