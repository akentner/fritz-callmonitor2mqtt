package integration

import (
	"os"
	"testing"
	"time"

	"fritz-callmonitor2mqtt/internal/database"
	"fritz-callmonitor2mqtt/pkg/types"
)

func TestFSMWithPersistence(t *testing.T) {
	// Create temporary test database
	tmpDir, err := os.MkdirTemp("", "test_fsm_db")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	// Initialize test database
	dbClient, err := database.NewClient(tmpDir)
	if err != nil {
		t.Fatalf("Failed to create database client: %v", err)
	}

	if err := dbClient.Connect(); err != nil {
		t.Fatalf("Failed to connect to database: %v", err)
	}
	defer func() { _ = dbClient.Close() }()

	if err := dbClient.RunEmbeddedMigrations(); err != nil {
		t.Fatalf("Failed to run migrations: %v", err)
	}

	// Create FSM with database persistence
	line := 1
	fsm := NewCallStateMachineWithMQTTAndDB(line, nil, dbClient, nil)

	// Test: Ring -> Connect -> Disconnect (Finished)

	// 1. Ring event (should create new call)
	fsm.ProcessEvent(CallTypeRing)

	// Verify call was inserted
	callID := fsm.GetCallID()
	if callID == nil {
		t.Fatal("Expected call ID to be generated after ringing")
	}

	call, err := dbClient.GetCall(*callID)
	if err != nil {
		t.Fatalf("Failed to retrieve call: %v", err)
	}
	if call.Status != types.CallStatusRinging {
		t.Errorf("Expected status %s, got %s", types.CallStatusRinging, call.Status)
	}
	if call.Line != line {
		t.Errorf("Expected line %d, got %d", line, call.Line)
	}

	// 2. Connect event (should update call)
	fsm.ProcessEvent(CallTypeConnect)

	updatedCall, err := dbClient.GetCall(*callID)
	if err != nil {
		t.Fatalf("Failed to retrieve updated call: %v", err)
	}
	if updatedCall.Status != types.CallStatusTalking {
		t.Errorf("Expected status %s, got %s", types.CallStatusTalking, updatedCall.Status)
	}
	if updatedCall.ConnectTimestamp == nil {
		t.Error("Expected connect timestamp to be set")
	}

	// 3. Disconnect event (should update call to finished)
	fsm.ProcessEvent(CallTypeDisconnect)

	finalCall, err := dbClient.GetCall(*callID)
	if err != nil {
		t.Fatalf("Failed to retrieve final call: %v", err)
	}
	if finalCall.Status != types.CallStatusFinished {
		t.Errorf("Expected status %s, got %s", types.CallStatusFinished, finalCall.Status)
	}
	if finalCall.EndTimestamp == nil {
		t.Error("Expected end timestamp to be set")
	}

	// 4. Wait for timeout to idle (should update finish state but keep call record)
	time.Sleep(1100 * time.Millisecond)

	if fsm.GetState() != types.CallStatusIdle {
		t.Errorf("Expected FSM to be idle after timeout, got %s", fsm.GetState())
	}

	// Call ID should be cleared after returning to idle
	if fsm.GetCallID() != nil {
		t.Error("Expected call ID to be cleared after returning to idle")
	}

	// But the call record should still exist with finish state
	persistedCall, err := dbClient.GetCall(*callID)
	if err != nil {
		t.Fatalf("Failed to retrieve persisted call: %v", err)
	}
	if persistedCall.FinishState == nil || *persistedCall.FinishState != types.CallStatusFinished {
		t.Errorf("Expected finish state %s, got %v", types.CallStatusFinished, persistedCall.FinishState)
	}
}

func TestFSMWithPersistenceMissedCall(t *testing.T) {
	// Create temporary test database
	tmpDir, err := os.MkdirTemp("", "test_fsm_missed_db")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	// Initialize test database
	dbClient, err := database.NewClient(tmpDir)
	if err != nil {
		t.Fatalf("Failed to create database client: %v", err)
	}

	if err := dbClient.Connect(); err != nil {
		t.Fatalf("Failed to connect to database: %v", err)
	}
	defer func() { _ = dbClient.Close() }()

	if err := dbClient.RunEmbeddedMigrations(); err != nil {
		t.Fatalf("Failed to run migrations: %v", err)
	}

	// Create FSM with database persistence
	line := 2
	fsm := NewCallStateMachineWithMQTTAndDB(line, nil, dbClient, nil)

	// Test: Ring -> Disconnect (Missed Call)

	// 1. Ring event
	fsm.ProcessEvent(CallTypeRing)
	callID := fsm.GetCallID()
	if callID == nil {
		t.Fatal("Expected call ID to be generated")
	}

	// 2. Disconnect event (missed call)
	fsm.ProcessEvent(CallTypeDisconnect)

	missedCall, err := dbClient.GetCall(*callID)
	if err != nil {
		t.Fatalf("Failed to retrieve missed call: %v", err)
	}
	if missedCall.Status != types.CallStatusMissedCall {
		t.Errorf("Expected status %s, got %s", types.CallStatusMissedCall, missedCall.Status)
	}

	// 3. Wait for timeout to idle
	time.Sleep(1100 * time.Millisecond)

	// Verify finish state is preserved
	finalCall, err := dbClient.GetCall(*callID)
	if err != nil {
		t.Fatalf("Failed to retrieve final call: %v", err)
	}
	if finalCall.FinishState == nil || *finalCall.FinishState != types.CallStatusMissedCall {
		t.Errorf("Expected finish state %s, got %v", types.CallStatusMissedCall, finalCall.FinishState)
	}
}

func TestLineStateMachineWithPersistence(t *testing.T) {
	// Create temporary test database
	tmpDir, err := os.MkdirTemp("", "test_lsm_db")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	// Initialize test database
	dbClient, err := database.NewClient(tmpDir)
	if err != nil {
		t.Fatalf("Failed to create database client: %v", err)
	}

	if err := dbClient.Connect(); err != nil {
		t.Fatalf("Failed to connect to database: %v", err)
	}
	defer func() { _ = dbClient.Close() }()

	if err := dbClient.RunEmbeddedMigrations(); err != nil {
		t.Fatalf("Failed to run migrations: %v", err)
	}

	// Create LineStateMachine with database persistence
	lsm := NewLineStateMachineWithMQTTAndDB(nil, dbClient, nil)

	// Test multiple lines with calls
	event1 := &types.CallEvent{
		Timestamp: time.Now(),
		Type:      CallTypeRing,
		Line:      1,
		Caller:    "123456789",
		Called:    "987654321",
	}

	event2 := &types.CallEvent{
		Timestamp: time.Now(),
		Type:      CallTypeCall,
		Line:      2,
		Caller:    "987654321",
		Called:    "123456789",
	}

	// Process events
	lsm.Processtypes.CallEvent(event1)
	lsm.Processtypes.CallEvent(event2)

	// Verify call IDs are generated
	callID1 := lsm.GetLineCallID(1)
	callID2 := lsm.GetLineCallID(2)

	if callID1 == nil {
		t.Error("Expected call ID for line 1")
	}
	if callID2 == nil {
		t.Error("Expected call ID for line 2")
	}

	// Verify calls are in database
	if callID1 != nil {
		call1, err := dbClient.GetCall(*callID1)
		if err != nil {
			t.Fatalf("Failed to retrieve call for line 1: %v", err)
		}
		if call1.Line != 1 || call1.Status != types.CallStatusRinging {
			t.Errorf("Unexpected call1: line=%d, status=%s", call1.Line, call1.Status)
		}
	}

	if callID2 != nil {
		call2, err := dbClient.GetCall(*callID2)
		if err != nil {
			t.Fatalf("Failed to retrieve call for line 2: %v", err)
		}
		if call2.Line != 2 || call2.Status != types.CallStatusCalling {
			t.Errorf("Unexpected call2: line=%d, status=%s", call2.Line, call2.Status)
		}
	}

	// Test GetCallsByLine
	line1Calls, err := dbClient.GetCallsByLine(1, 10)
	if err != nil {
		t.Fatalf("Failed to get calls for line 1: %v", err)
	}
	if len(line1Calls) != 1 {
		t.Errorf("Expected 1 call for line 1, got %d", len(line1Calls))
	}

	line2Calls, err := dbClient.GetCallsByLine(2, 10)
	if err != nil {
		t.Fatalf("Failed to get calls for line 2: %v", err)
	}
	if len(line2Calls) != 1 {
		t.Errorf("Expected 1 call for line 2, got %d", len(line2Calls))
	}
}
