package config

import (
	"testing"
	"time"
)

func TestConfigTimezone(t *testing.T) {
	tests := []struct {
		name        string
		timezone    string
		expectError bool
		expectedTZ  string
	}{
		{
			name:       "Europe/Berlin timezone",
			timezone:   "Europe/Berlin",
			expectedTZ: "Europe/Berlin",
		},
		{
			name:       "UTC timezone",
			timezone:   "UTC",
			expectedTZ: "UTC",
		},
		{
			name:       "America/New_York timezone",
			timezone:   "America/New_York",
			expectedTZ: "America/New_York",
		},
		{
			name:       "empty timezone defaults to Local",
			timezone:   "",
			expectedTZ: time.Local.String(),
		},
		{
			name:        "invalid timezone",
			timezone:    "Invalid/Timezone",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := &Config{
				FritzBox: FritzBoxConfig{
					Host: "fritz.box",
					Port: 1012,
				},
				MQTT: MQTTConfig{
					Broker: "localhost",
					Port:   1883,
				},
				App: AppConfig{
					CallHistorySize: 50,
					Timezone:        tt.timezone,
				},
				Database: DatabaseConfig{
					DataDir: "./data",
				},
			}

			// Test validation
			err := config.Validate()
			if tt.expectError && err == nil {
				t.Error("Expected validation error, but got none")
				return
			}
			if !tt.expectError && err != nil {
				t.Errorf("Unexpected validation error: %v", err)
				return
			}

			// Test GetLocation if validation passed
			if !tt.expectError {
				location, err := config.GetLocation()
				if err != nil {
					t.Errorf("Unexpected error getting location: %v", err)
					return
				}

				if location.String() != tt.expectedTZ {
					t.Errorf("Expected timezone %s, got %s", tt.expectedTZ, location.String())
				}
			}
		})
	}
}

func TestLoadConfigTimezone(t *testing.T) {
	// Test default timezone
	config, err := LoadConfig()
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	if config.App.Timezone != "Europe/Berlin" {
		t.Errorf("Expected default timezone 'Europe/Berlin', got '%s'", config.App.Timezone)
	}

	// Verify it's a valid timezone
	_, err = config.GetLocation()
	if err != nil {
		t.Errorf("Default timezone should be valid: %v", err)
	}
}
