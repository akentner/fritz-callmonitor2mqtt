package types

import (
	"encoding/json"
	"fmt"
	"time"
)

// PhoneNumberRPCRequest represents an RPC request for phone number operations
type PhoneNumberRPCRequest struct {
	ID          string    `json:"id"`                     // Unique request ID
	Method      string    `json:"method"`                 // get, set, delete, list, search
	PhoneNumber string    `json:"phone_number,omitempty"` // Target phone number
	Name        string    `json:"name,omitempty"`         // Name for set operations
	Limit       int       `json:"limit,omitempty"`        // Limit for list operations (default: 100)
	Pattern     string    `json:"pattern,omitempty"`      // Search pattern for search operations
	Timestamp   time.Time `json:"timestamp"`              // Request timestamp
}

// PhoneNumberRPCResponse represents an RPC response for phone number operations
type PhoneNumberRPCResponse struct {
	ID           string            `json:"id"`                      // Request ID from request
	Success      bool              `json:"success"`                 // Operation success
	Error        string            `json:"error,omitempty"`         // Error message if success is false
	PhoneNumber  *PhoneNumberInfo  `json:"phone_number,omitempty"`  // Single phone number result
	PhoneNumbers []PhoneNumberInfo `json:"phone_numbers,omitempty"` // List of phone numbers
	Count        int               `json:"count,omitempty"`         // Total count of results
	Timestamp    time.Time         `json:"timestamp"`               // Response timestamp
}

// PhoneNumberInfo represents phone number information in RPC responses
type PhoneNumberInfo struct {
	PhoneNumber string     `json:"phone_number"`
	Name        *string    `json:"name"`
	CreatedAt   *time.Time `json:"created_at,omitempty"`
	UpdatedAt   *time.Time `json:"updated_at,omitempty"`
}

// PhoneNumberRPC interface for RPC operations
type PhoneNumberRPC interface {
	ProcessRequest(request *PhoneNumberRPCRequest) *PhoneNumberRPCResponse
}

// ToJSON converts PhoneNumberRPCRequest to JSON string
func (req *PhoneNumberRPCRequest) ToJSON() (string, error) {
	data, err := json.Marshal(req)
	if err != nil {
		return "", fmt.Errorf("failed to marshal phone number RPC request: %w", err)
	}
	return string(data), nil
}

// ToJSON converts PhoneNumberRPCResponse to JSON string
func (resp *PhoneNumberRPCResponse) ToJSON() (string, error) {
	data, err := json.Marshal(resp)
	if err != nil {
		return "", fmt.Errorf("failed to marshal phone number RPC response: %w", err)
	}
	return string(data), nil
}

// ParsePhoneNumberRPCRequest parses JSON string to PhoneNumberRPCRequest
func ParsePhoneNumberRPCRequest(jsonStr string) (*PhoneNumberRPCRequest, error) {
	var req PhoneNumberRPCRequest
	err := json.Unmarshal([]byte(jsonStr), &req)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal phone number RPC request: %w", err)
	}
	return &req, nil
}

// Validate validates the RPC request
func (req *PhoneNumberRPCRequest) Validate() error {
	if req.ID == "" {
		return fmt.Errorf("request ID is required")
	}

	switch req.Method {
	case "get":
		if req.PhoneNumber == "" {
			return fmt.Errorf("phone_number is required for get method")
		}
	case "set":
		if req.PhoneNumber == "" {
			return fmt.Errorf("phone_number is required for set method")
		}
		if req.Name == "" {
			return fmt.Errorf("name is required for set method")
		}
	case "delete":
		if req.PhoneNumber == "" {
			return fmt.Errorf("phone_number is required for delete method")
		}
	case "list":
		if req.Limit <= 0 {
			req.Limit = 100 // Default limit
		}
		if req.Limit > 1000 {
			return fmt.Errorf("limit cannot exceed 1000")
		}
	case "search":
		if req.Pattern == "" {
			return fmt.Errorf("pattern is required for search method")
		}
		if req.Limit <= 0 {
			req.Limit = 100 // Default limit
		}
		if req.Limit > 1000 {
			return fmt.Errorf("limit cannot exceed 1000")
		}
	default:
		return fmt.Errorf("invalid method: %s (supported: get, set, delete, list, search)", req.Method)
	}

	return nil
}

// NewErrorResponse creates an error response
func NewErrorResponse(requestID, errorMsg string) *PhoneNumberRPCResponse {
	return &PhoneNumberRPCResponse{
		ID:        requestID,
		Success:   false,
		Error:     errorMsg,
		Timestamp: time.Now(),
	}
}

// NewSuccessResponse creates a success response
func NewSuccessResponse(requestID string) *PhoneNumberRPCResponse {
	return &PhoneNumberRPCResponse{
		ID:        requestID,
		Success:   true,
		Timestamp: time.Now(),
	}
}
