package database

import (
	"os"
	"testing"
	"time"

	"fritz-callmonitor2mqtt/pkg/types"

	"github.com/google/uuid"
)

func TestCallPersistence(t *testing.T) {
	// Create temporary test database
	tmpDir, err := os.MkdirTemp("", "test_db")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Initialize test database
	client, err := NewClient(tmpDir)
	if err != nil {
		t.Fatalf("Failed to create database client: %v", err)
	}

	if err := client.Connect(); err != nil {
		t.Fatalf("Failed to connect to database: %v", err)
	}
	defer client.Close()

	if err := client.RunEmbeddedMigrations(); err != nil {
		t.Fatalf("Failed to run migrations: %v", err)
	}

	// Test data
	callID := uuid.New()
	line := 1
	status := types.CallStatusRinging

	event := &types.CallEvent{
		Timestamp: time.Now(),
		Type:      types.CallTypeRing,
		Line:      line,
		Caller:    "01234567890",
		Called:    "0987654321",
		CallerMSN: "123",
		CalledMSN: "456",
		Trunk:     "trunk1",
	}

	// Test InsertCall
	err = client.InsertCall(callID, line, status, event)
	if err != nil {
		t.Fatalf("Failed to insert call: %v", err)
	}

	// Test GetCall
	retrievedCall, err := client.GetCall(callID)
	if err != nil {
		t.Fatalf("Failed to retrieve call: %v", err)
	}

	// Verify call data
	if retrievedCall.CallID != callID {
		t.Errorf("Expected call ID %s, got %s", callID, retrievedCall.CallID)
	}
	if retrievedCall.Line != line {
		t.Errorf("Expected line %d, got %d", line, retrievedCall.Line)
	}
	if retrievedCall.Status != status {
		t.Errorf("Expected status %s, got %s", status, retrievedCall.Status)
	}
	if retrievedCall.Caller == nil || *retrievedCall.Caller != event.Caller {
		t.Errorf("Expected caller %s, got %v", event.Caller, retrievedCall.Caller)
	}

	// Test UpdateCall - transition to talking
	talkingStatus := types.CallStatusTalking
	connectEvent := &types.CallEvent{
		Timestamp: time.Now(),
		Type:      types.CallTypeConnect,
		Line:      line,
	}

	err = client.UpdateCall(callID, talkingStatus, nil, connectEvent)
	if err != nil {
		t.Fatalf("Failed to update call to talking: %v", err)
	}

	// Verify update
	updatedCall, err := client.GetCall(callID)
	if err != nil {
		t.Fatalf("Failed to retrieve updated call: %v", err)
	}
	if updatedCall.Status != talkingStatus {
		t.Errorf("Expected status %s, got %s", talkingStatus, updatedCall.Status)
	}
	if updatedCall.ConnectTimestamp == nil {
		t.Error("Expected connect timestamp to be set")
	}

	// Test UpdateCall - transition to finished with finish state
	finishedStatus := types.CallStatusFinished
	finishState := types.CallStatusFinished
	disconnectEvent := &types.CallEvent{
		Timestamp: time.Now(),
		Type:      types.CallTypeDisconnect,
		Line:      line,
		Duration:  120, // 2 minutes
	}

	err = client.UpdateCall(callID, finishedStatus, &finishState, disconnectEvent)
	if err != nil {
		t.Fatalf("Failed to update call to finished: %v", err)
	}

	// Verify final update
	finalCall, err := client.GetCall(callID)
	if err != nil {
		t.Fatalf("Failed to retrieve final call: %v", err)
	}
	if finalCall.Status != finishedStatus {
		t.Errorf("Expected status %s, got %s", finishedStatus, finalCall.Status)
	}
	if finalCall.FinishState == nil || *finalCall.FinishState != finishState {
		t.Errorf("Expected finish state %s, got %v", finishState, finalCall.FinishState)
	}
	if finalCall.EndTimestamp == nil {
		t.Error("Expected end timestamp to be set")
	}
	if finalCall.Duration == nil || *finalCall.Duration != 120 {
		t.Errorf("Expected duration 120, got %v", finalCall.Duration)
	}

	// Test GetCallsByLine
	calls, err := client.GetCallsByLine(line, 10)
	if err != nil {
		t.Fatalf("Failed to get calls by line: %v", err)
	}
	if len(calls) != 1 {
		t.Errorf("Expected 1 call, got %d", len(calls))
	}
	if calls[0].CallID != callID {
		t.Errorf("Expected call ID %s, got %s", callID, calls[0].CallID)
	}
}

func TestCallNotFound(t *testing.T) {
	// Create temporary test database
	tmpDir, err := os.MkdirTemp("", "test_db")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Initialize test database
	client, err := NewClient(tmpDir)
	if err != nil {
		t.Fatalf("Failed to create database client: %v", err)
	}

	if err := client.Connect(); err != nil {
		t.Fatalf("Failed to connect to database: %v", err)
	}
	defer client.Close()

	if err := client.RunEmbeddedMigrations(); err != nil {
		t.Fatalf("Failed to run migrations: %v", err)
	}

	// Test getting non-existent call
	nonExistentID := uuid.New()
	_, err = client.GetCall(nonExistentID)
	if err == nil {
		t.Error("Expected error for non-existent call")
	}

	// Test updating non-existent call
	err = client.UpdateCall(nonExistentID, types.CallStatusIdle, nil, nil)
	if err == nil {
		t.Error("Expected error for updating non-existent call")
	}
}
