package types_test

import (
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"fritz-callmonitor2mqtt/internal/database"
	"fritz-callmonitor2mqtt/pkg/types"
)

// MockMQTTPublisher for testing
type MockMQTTPublisher struct {
	publishedEvents []types.CallEvent
}

func (m *MockMQTTPublisher) PublishLineStatusChange(line int, status types.CallStatus, event *types.CallEvent) error {
	if event != nil {
		m.publishedEvents = append(m.publishedEvents, *event)
	}
	return nil
}

func TestFSMWithDatabasePersistence(t *testing.T) {
	// Setup test database
	tempDir, err := os.MkdirTemp("", "fsm_test")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	dbClient, err := database.NewClient(tempDir)
	require.NoError(t, err)

	err = dbClient.Connect()
	require.NoError(t, err)
	defer dbClient.Close()

	err = dbClient.RunEmbeddedMigrations()
	require.NoError(t, err)

	// Setup mock MQTT publisher
	mockMQTT := &MockMQTTPublisher{}

	// Create FSM with database persistence
	fsm := types.NewCallStateMachineWithMQTTAndDB(1, mockMQTT, dbClient, nil)

	// Test call lifecycle with database persistence
	t.Run("Complete call lifecycle persistence", func(t *testing.T) {
		// Create test events
		ringingEvent := &types.CallEvent{
			Timestamp:    time.Now(),
			Date:         "01.01.25",
			Time:         "10:00:00",
			Type:         types.EventTypeCall,
			CallID:       "123456",
			Line:         1,
			LocalNumber:  "+49123456789",
			RemoteNumber: "+49987654321",
			RemoteName:   "Test Caller",
		}

		talkingEvent := &types.CallEvent{
			Timestamp:    time.Now(),
			Date:         "01.01.25",
			Time:         "10:01:00",
			Type:         types.EventTypeConnect,
			CallID:       "123456",
			Line:         1,
			LocalNumber:  "+49123456789",
			RemoteNumber: "+49987654321",
		}

		hangupEvent := &types.CallEvent{
			Timestamp: time.Now(),
			Date:      "01.01.25",
			Time:      "10:05:00",
			Type:      types.EventTypeDisconnect,
			CallID:    "123456",
			Line:      1,
			Duration:  "00:04:00",
		}

		// Transition: idle -> ringing (should insert new call)
		err := fsm.ProcessCallEvent(ringingEvent)
		require.NoError(t, err)
		assert.Equal(t, types.CallStatusRinging, fsm.GetStatus())

		// Verify call was inserted into database
		callID := fsm.GetCallID()
		assert.NotEqual(t, uuid.Nil, callID)

		call, err := dbClient.GetCall(callID)
		require.NoError(t, err)
		assert.Equal(t, callID, call.CallID)
		assert.Equal(t, 1, call.Line)
		assert.Equal(t, types.CallStatusRinging, call.Status)
		assert.Equal(t, ringingEvent.RemoteNumber, call.RemoteNumber)
		assert.Equal(t, ringingEvent.RemoteName, call.RemoteName)

		// Transition: ringing -> talking (should update call)
		err = fsm.ProcessCallEvent(talkingEvent)
		require.NoError(t, err)
		assert.Equal(t, types.CallStatusTalking, fsm.GetStatus())

		// Verify call was updated
		call, err = dbClient.GetCall(callID)
		require.NoError(t, err)
		assert.Equal(t, types.CallStatusTalking, call.Status)

		// Transition: talking -> finished (should update call with finish state)
		err = fsm.ProcessCallEvent(hangupEvent)
		require.NoError(t, err)
		assert.Equal(t, types.CallStatusFinished, fsm.GetStatus())

		// Verify call was updated with finish state
		call, err = dbClient.GetCall(callID)
		require.NoError(t, err)
		assert.Equal(t, types.CallStatusFinished, call.Status)
		assert.NotNil(t, call.FinishState)
		assert.Equal(t, types.CallStatusTalking, *call.FinishState)
		assert.Equal(t, hangupEvent.Duration, call.Duration)

		// Transition: finished -> idle (should update call)
		err = fsm.ProcessCallEvent(&types.CallEvent{
			Timestamp: time.Now(),
			Type:      types.EventTypeCall, // New call triggers reset to idle first
			CallID:    "789012",            // Different call ID
			Line:      1,
		})
		require.NoError(t, err)
		assert.Equal(t, types.CallStatusRinging, fsm.GetStatus()) // New call starts ringing

		// Verify previous call still exists in database
		call, err = dbClient.GetCall(callID)
		require.NoError(t, err)
		assert.Equal(t, types.CallStatusFinished, call.Status)
	})

	t.Run("MQTT publishing works with FSM", func(t *testing.T) {
		// Reset mock
		mockMQTT.publishedEvents = nil

		// Create new FSM for clean test
		fsm := types.NewCallStateMachineWithMQTTAndDB(2, mockMQTT, dbClient, nil)

		event := &types.CallEvent{
			Timestamp:    time.Now(),
			Date:         "01.01.25",
			Time:         "11:00:00",
			Type:         types.EventTypeCall,
			CallID:       "555666",
			Line:         2,
			LocalNumber:  "+49111222333",
			RemoteNumber: "+49444555666",
		}

		err := fsm.ProcessCallEvent(event)
		require.NoError(t, err)

		// Verify MQTT event was published
		assert.Len(t, mockMQTT.publishedEvents, 1)
		assert.Equal(t, event.CallID, mockMQTT.publishedEvents[0].CallID)
		assert.Equal(t, event.RemoteNumber, mockMQTT.publishedEvents[0].RemoteNumber)
	})
}

func TestLineStateMachineWithDatabase(t *testing.T) {
	// Setup test database
	tempDir, err := os.MkdirTemp("", "line_fsm_test")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	dbClient, err := database.NewClient(tempDir)
	require.NoError(t, err)

	err = dbClient.Connect()
	require.NoError(t, err)
	defer dbClient.Close()

	err = dbClient.RunEmbeddedMigrations()
	require.NoError(t, err)

	// Setup mock MQTT publisher
	mockMQTT := &MockMQTTPublisher{}

	// Create line state machine with database
	lsm := types.NewLineStateMachineWithMQTTAndDB(mockMQTT, dbClient, func(line int, oldState, newState types.CallStatus) {
		t.Logf("Line %d: %s -> %s", line, oldState, newState)
	})

	t.Run("Multiple lines with persistence", func(t *testing.T) {
		// Events for line 1
		line1Event := &types.CallEvent{
			Timestamp:    time.Now(),
			Date:         "01.01.25",
			Time:         "12:00:00",
			Type:         types.EventTypeCall,
			CallID:       "line1call",
			Line:         1,
			LocalNumber:  "+49111111111",
			RemoteNumber: "+49222222222",
		}

		// Events for line 2
		line2Event := &types.CallEvent{
			Timestamp:    time.Now(),
			Date:         "01.01.25",
			Time:         "12:01:00",
			Type:         types.EventTypeCall,
			CallID:       "line2call",
			Line:         2,
			LocalNumber:  "+49333333333",
			RemoteNumber: "+49444444444",
		}

		// Process events on both lines
		err := lsm.ProcessCallEvent(line1Event)
		require.NoError(t, err)

		err = lsm.ProcessCallEvent(line2Event)
		require.NoError(t, err)

		// Verify both FSMs are created and in ringing state
		fsm1 := lsm.GetFSM(1)
		require.NotNil(t, fsm1)
		assert.Equal(t, types.CallStatusRinging, fsm1.GetStatus())

		fsm2 := lsm.GetFSM(2)
		require.NotNil(t, fsm2)
		assert.Equal(t, types.CallStatusRinging, fsm2.GetStatus())

		// Verify calls were persisted
		call1, err := dbClient.GetCall(fsm1.GetCallID())
		require.NoError(t, err)
		assert.Equal(t, line1Event.CallID, call1.OriginalCallID)
		assert.Equal(t, 1, call1.Line)

		call2, err := dbClient.GetCall(fsm2.GetCallID())
		require.NoError(t, err)
		assert.Equal(t, line2Event.CallID, call2.OriginalCallID)
		assert.Equal(t, 2, call2.Line)
	})
}
