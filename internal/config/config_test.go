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
					CallHistorySize:    50,
					MSNCallHistorySize: 30,
					Timezone:           tt.timezone,
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

func TestExtensionMapping(t *testing.T) {
	config := &PBXConfig{
		Extensions: []Extension{
			{Number: "21", Name: "Direct Extension 21", Type: "DECT"},
			{Number: "621", Name: "GUI Extension 621", Type: "DECT"},
			{Number: "615", Name: "GUI Extension 615", Type: "VOIP"},
			{Number: "**600", Name: "Anrufbeantworter System", Type: "VOICEBOX"},
		},
	}

	// Test direct extension lookup
	info := config.GetExtensionInfo("21")
	if info == nil {
		t.Fatal("Expected extension info for '21', got nil")
		return
	}
	if info.Number != "21" {
		t.Errorf("Expected Number '21', got '%s'", info.Number)
	}
	if info.Name != "Direct Extension 21" {
		t.Errorf("Expected Name 'Direct Extension 21', got '%s'", info.Name)
	}

	// Test GUI extension mapping: Fritz GUI 615 -> Callmonitor Event 15
	info = config.GetExtensionInfo("15")
	if info == nil {
		t.Fatal("Expected extension info for callmonitor number '15' (GUI 615), got nil")
		return
	}
	if info.Number != "615" {
		t.Errorf("Expected Number '615' (GUI number), got '%s'", info.Number)
	}
	if info.Name != "GUI Extension 615" {
		t.Errorf("Expected Name 'GUI Extension 615', got '%s'", info.Name)
	}
	if info.Type != "VOIP" {
		t.Errorf("Expected Type 'VOIP', got '%s'", info.Type)
	}

	// Test VOICEBOX mapping: Internal **600 -> Callmonitor Event 40
	info = config.GetExtensionInfo("40")
	if info == nil {
		t.Fatal("Expected extension info for callmonitor number '40' (Internal **600), got nil")
		return
	}
	if info.Number != "**600" {
		t.Errorf("Expected Number '**600' (Internal number), got '%s'", info.Number)
	}
	if info.Name != "Anrufbeantworter System" {
		t.Errorf("Expected Name 'Anrufbeantworter System', got '%s'", info.Name)
	}
	if info.Type != "VOICEBOX" {
		t.Errorf("Expected Type 'VOICEBOX', got '%s'", info.Type)
	}

	// Test non-existing extension
	info = config.GetExtensionInfo("99")
	if info != nil {
		t.Errorf("Expected nil for non-existing extension '99', got %v", info)
	}
}

func TestGetCallmonitorNumber(t *testing.T) {
	config := &PBXConfig{}

	tests := []struct {
		guiNumber     string
		expectedEvent string
		description   string
	}{
		// Test internal **6xx VOICEBOX numbers (**600-**609 -> Events 40-49)
		{"**600", "40", "Internal **600 VOICEBOX -> Event 40"},
		{"**601", "41", "Internal **601 VOICEBOX -> Event 41"},
		{"**609", "49", "Internal **609 VOICEBOX -> Event 49"},

		// Test internal **6xx DECT numbers (**610-**619 -> Events 10-19)
		{"**610", "10", "Internal **610 DECT -> Event 10"},
		{"**615", "15", "Internal **615 DECT -> Event 15"},
		{"**619", "19", "Internal **619 DECT -> Event 19"},

		// Test internal **6xx VOIP numbers (**620-**629 -> Events 20-29)
		{"**620", "20", "Internal **620 VOIP -> Event 20"},
		{"**621", "21", "Internal **621 VOIP -> Event 21"},
		{"**629", "29", "Internal **629 VOIP -> Event 29"},

		// Test 6xx GUI extensions (remove leading "6")
		{"621", "21", "GUI 621 -> Event 21"},
		{"615", "15", "GUI 615 -> Event 15"},
		{"610", "10", "GUI 610 -> Event 10"},
		{"622", "22", "GUI 622 -> Event 22"},

		// Test extensions that don't need mapping (direct)
		{"21", "21", "Direct extension 21 -> Event 21"},
		{"1", "1", "Analog extension 1 -> Event 1"},
		{"42", "42", "Direct extension 42 -> Event 42"},
	}

	for _, tt := range tests {
		t.Run(tt.description, func(t *testing.T) {
			result := config.getCallmonitorNumber(tt.guiNumber)
			if result != tt.expectedEvent {
				t.Errorf("getCallmonitorNumber(%s) = %s, want %s", tt.guiNumber, result, tt.expectedEvent)
			}
		})
	}
}

func TestGetExtensionTypeByNumber(t *testing.T) {
	config := &PBXConfig{}

	tests := []struct {
		number       string
		expectedType string
		description  string
	}{
		// Test internal **600-**609 VOICEBOX range
		{"**600", "VOICEBOX", "Internal **600 -> VOICEBOX"},
		{"**605", "VOICEBOX", "Internal **605 -> VOICEBOX"},
		{"**609", "VOICEBOX", "Internal **609 -> VOICEBOX"},

		// Test internal **610-**619 DECT range
		{"**610", "DECT", "Internal **610 -> DECT"},
		{"**615", "DECT", "Internal **615 -> DECT"},
		{"**619", "DECT", "Internal **619 -> DECT"},

		// Test internal **620-**629 VOIP range
		{"**620", "VOIP", "Internal **620 -> VOIP"},
		{"**625", "VOIP", "Internal **625 -> VOIP"},
		{"**629", "VOIP", "Internal **629 -> VOIP"},

		// Test GUI 610-619 DECT range
		{"610", "DECT", "GUI 610 -> DECT"},
		{"615", "DECT", "GUI 615 -> DECT"},
		{"619", "DECT", "GUI 619 -> DECT"},

		// Test GUI 620-629 VOIP range
		{"620", "VOIP", "GUI 620 -> VOIP"},
		{"625", "VOIP", "GUI 625 -> VOIP"},
		{"629", "VOIP", "GUI 629 -> VOIP"},

		// Test UNKNOWN for other ranges
		{"1", "UNKNOWN", "Extension 1 -> UNKNOWN"},
		{"22", "UNKNOWN", "Extension 22 (AB name, not systematic) -> UNKNOWN"},
		{"24", "UNKNOWN", "Extension 24 (AB name, not systematic) -> UNKNOWN"},
		{"42", "UNKNOWN", "Extension 42 -> UNKNOWN"},
		{"600", "UNKNOWN", "GUI 600 (out of range) -> UNKNOWN"},
		{"630", "UNKNOWN", "GUI 630 (out of range) -> UNKNOWN"},
		{"**599", "UNKNOWN", "Internal **599 (out of range) -> UNKNOWN"},
		{"**630", "UNKNOWN", "Internal **630 (out of range) -> UNKNOWN"},
	}

	for _, tt := range tests {
		t.Run(tt.description, func(t *testing.T) {
			result := config.getExtensionTypeByNumber(tt.number)
			if result != tt.expectedType {
				t.Errorf("getExtensionTypeByNumber(%s) = %s, want %s", tt.number, result, tt.expectedType)
			}
		})
	}
}
