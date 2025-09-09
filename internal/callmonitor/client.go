package callmonitor

import (
	"bufio"
	"fmt"
	"net"
	"strconv"
	"strings"
	"time"

	"fritz-callmonitor2mqtt/pkg/types"
)

// Client represents a Fritz!Box callmonitor client
type Client struct {
	host      string
	port      int
	conn      net.Conn
	eventChan chan types.CallEvent
	errorChan chan error
	stopChan  chan struct{}
	connected bool
}

// NewClient creates a new callmonitor client
func NewClient(host string, port int) *Client {
	return &Client{
		host:      host,
		port:      port,
		eventChan: make(chan types.CallEvent, 100),
		errorChan: make(chan error, 10),
		stopChan:  make(chan struct{}),
	}
}

// Connect establishes connection to Fritz!Box callmonitor
func (c *Client) Connect() error {
	conn, err := net.Dial("tcp", fmt.Sprintf("%s:%d", c.host, c.port))
	if err != nil {
		return fmt.Errorf("failed to connect to Fritz!Box callmonitor: %w", err)
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

	close(c.stopChan)
	c.connected = false

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

// readLoop continuously reads from the Fritz!Box connection
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

			event, err := c.parseCallEvent(line)
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

// parseCallEvent parses a Fritz!Box callmonitor line into a CallEvent
func (c *Client) parseCallEvent(line string) (*types.CallEvent, error) {
	// Fritz!Box callmonitor format:
	// timestamp;type;id;extension;caller;called;sip
	// Example: 21.09.25 15:30:45;RING;0;1;123456789;987654321;SIP0

	parts := strings.Split(line, ";")
	if len(parts) < 6 {
		return nil, fmt.Errorf("invalid callmonitor format: %s", line)
	}

	// Parse timestamp
	timestamp, err := parseTimestamp(parts[0])
	if err != nil {
		timestamp = time.Now() // Fallback to current time
	}

	// Parse call type
	var callType types.CallType
	switch strings.ToUpper(parts[1]) {
	case "RING":
		callType = types.CallTypeIncoming
	case "CALL":
		callType = types.CallTypeOutgoing
	case "CONNECT":
		callType = types.CallTypeConnect
	case "DISCONNECT":
		callType = types.CallTypeEnd
	default:
		return nil, fmt.Errorf("unknown call type: %s", parts[1])
	}

	event := &types.CallEvent{
		Timestamp:  timestamp,
		Type:       callType,
		ID:         parts[2],
		Extension:  parts[3],
		Caller:     parts[4],
		Called:     parts[5],
		RawMessage: line,
	}

	// Add SIP line if available
	if len(parts) > 6 {
		event.LineID = parts[6]
	}

	// Parse duration for end events (if available)
	if callType == types.CallTypeEnd && len(parts) > 7 {
		if duration, err := strconv.Atoi(parts[7]); err == nil {
			event.Duration = duration
		}
	}

	return event, nil
}

// parseTimestamp parses Fritz!Box timestamp format
func parseTimestamp(timestampStr string) (time.Time, error) {
	// Fritz!Box format: "21.09.25 15:30:45"
	layout := "02.01.06 15:04:05"

	// Try parsing as-is first (assuming it's from current century)
	if t, err := time.ParseInLocation(layout, timestampStr, time.Local); err == nil {
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
	return time.ParseInLocation(layout, fullTimestamp, time.Local)
}
