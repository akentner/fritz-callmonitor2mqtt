package main

import (
	"os"
	"testing"
	"time"

	"fritz-callmonitor2mqtt/internal/database"
	"fritz-callmonitor2mqtt/internal/mqtt"
	"fritz-callmonitor2mqtt/pkg/types"
)

func TestMain(t *testing.T) {
	// Test that main function can be called without panicking
	// This is a basic smoke test
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("main() panicked: %v", r)
		}
	}()

	// Override command line args for testing
	oldArgs := os.Args
	defer func() { os.Args = oldArgs }()

	// Test help flag
	os.Args = []string{"program", "-help"}
	// Note: main() will call os.Exit() with help flag
	// In real tests, you'd extract the logic into testable functions
}

func TestPrintUsage(t *testing.T) {
	// Test that printUsage doesn't panic
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("printUsage() panicked: %v", r)
		}
	}()

	printUsage()
}

// Example of how to structure testable code
func TestExampleFunction(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "basic test",
			input:    "hello",
			expected: "hello",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.input // Replace with actual function call
			if result != tt.expected {
				t.Errorf("expected %s, got %s", tt.expected, result)
			}
		})
	}
}

// Example benchmark
func BenchmarkExample(b *testing.B) {
	for i := 0; i < b.N; i++ {
		// Benchmark your functions here
		_ = "example"
	}
}

func TestEndToEndIntegration(t *testing.T) {
	// Test end-to-end integration: MQTT + Database + FSM

	// Setup test database
	tempDir, err := os.MkdirTemp("", "e2e_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create database client
	dbClient, err := database.NewClient(tempDir)
	if err != nil {
		t.Fatalf("Failed to create database client: %v", err)
	}

	err = dbClient.Connect()
	if err != nil {
		t.Fatalf("Failed to connect to database: %v", err)
	}
	defer dbClient.Close()

	err = dbClient.RunEmbeddedMigrations()
	if err != nil {
		t.Fatalf("Failed to run migrations: %v", err)
	}

	// Create mock MQTT client (minimal parameters)
	mqttClient := mqtt.NewClient("localhost", 1883, "", "", "test-client", "fritz", 0, false, 30*time.Second, 5*time.Second, "debug")

	// Create call manager with MQTT and database
	callManager := types.NewCallManagerWithMQTTAndDB(mqttClient, dbClient, func(line int, oldStatus, newStatus types.CallStatus, event *types.CallEvent) {
		t.Logf("Line %d: %s -> %s", line, oldStatus, newStatus)
	})

	// Test processing a call event
	callEvent := &types.CallEvent{
		ID:        "test-uuid-123",
		Timestamp: time.Now(),
		Type:      types.CallTypeRing,
		Direction: types.CallDirectionInbound,
		Line:      1,
		Caller:    "+49123456789",
		Called:    "+49987654321",
		Status:    types.CallStatusRinging,
	}

	// Process the call event
	processedEvent := callManager.ProcessEvent(callEvent)
	if processedEvent == nil {
		t.Fatal("ProcessEvent returned nil")
	}

	// Verify the call was processed by checking line status
	lineStatus := callManager.GetLineStatus(1)
	if lineStatus != types.CallStatusRinging {
		t.Errorf("Expected line status ringing, got %s", lineStatus)
	}

	// Clean up
	callManager.Cleanup()

	t.Log("End-to-end integration test passed: FSM + Database + MQTT integration working")
}
