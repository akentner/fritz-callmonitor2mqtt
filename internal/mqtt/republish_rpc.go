package mqtt

import (
	"fmt"
	"time"
)

// RepublishRPCRequest represents an RPC request for republishing MQTT topics
type RepublishRPCRequest struct {
	ID        string    `json:"id"`        // Unique request ID
	Method    string    `json:"method"`    // Method: "republish"
	Timestamp time.Time `json:"timestamp"` // Request timestamp
}

// RepublishRPCResponse represents an RPC response for republish operations
type RepublishRPCResponse struct {
	ID               string    `json:"id"`                          // Request ID from request
	Success          bool      `json:"success"`                     // Operation success
	Error            string    `json:"error,omitempty"`             // Error message if success is false
	RepublishedCount int       `json:"republished_count,omitempty"` // Number of topics republished
	Timestamp        time.Time `json:"timestamp"`                   // Response timestamp
}

// Validate validates the RPC request
func (req *RepublishRPCRequest) Validate() error {
	if req.ID == "" {
		return fmt.Errorf("request ID is required")
	}

	if req.Method != "republish" {
		return fmt.Errorf("invalid method: %s (only 'republish' is supported)", req.Method)
	}

	return nil
}

// NewRepublishErrorResponse creates an error response
func NewRepublishErrorResponse(requestID, errorMsg string) *RepublishRPCResponse {
	return &RepublishRPCResponse{
		ID:        requestID,
		Success:   false,
		Error:     errorMsg,
		Timestamp: time.Now(),
	}
}

// NewRepublishSuccessResponse creates a success response
func NewRepublishSuccessResponse(requestID string, republishedCount int) *RepublishRPCResponse {
	return &RepublishRPCResponse{
		ID:               requestID,
		Success:          true,
		RepublishedCount: republishedCount,
		Timestamp:        time.Now(),
	}
}
