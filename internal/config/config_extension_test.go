package config

import (
	"os"
	"testing"
)

func TestLoadExtensionsFromEnv(t *testing.T) {
	// Clean environment before test
	cleanExtensionEnv(t)
	defer cleanExtensionEnv(t)

	// Test case 1: No extensions configured
	extensions := loadExtensionsFromEnv()
	if len(extensions) != 0 {
		t.Errorf("Expected 0 extensions, got %d", len(extensions))
	}

	// Test case 2: Configure extensions with env variables
	testEnvs := map[string]string{
		"FRITZ_CALLMONITOR_PBX_EXTENSION_0_NUMBER": "610",
		"FRITZ_CALLMONITOR_PBX_EXTENSION_0_NAME":   "Büro Telefon",
		"FRITZ_CALLMONITOR_PBX_EXTENSION_0_TYPE":   "DECT",

		"FRITZ_CALLMONITOR_PBX_EXTENSION_1_NUMBER": "620",
		"FRITZ_CALLMONITOR_PBX_EXTENSION_1_NAME":   "DG 24",
		// No type - should be auto-derived as VOIP

		"FRITZ_CALLMONITOR_PBX_EXTENSION_2_NUMBER": "**600",
		"FRITZ_CALLMONITOR_PBX_EXTENSION_2_NAME":   "AB 22",
		// No type - should be auto-derived as VOICEBOX

		// Test missing name - should be ignored
		"FRITZ_CALLMONITOR_PBX_EXTENSION_3_NUMBER": "611",

		// Test missing number - should be ignored
		"FRITZ_CALLMONITOR_PBX_EXTENSION_4_NAME": "Invalid Extension",
	}

	// Set test environment variables
	for key, value := range testEnvs {
		if err := os.Setenv(key, value); err != nil {
			t.Fatalf("Failed to set env var %s: %v", key, err)
		}
	}

	extensions = loadExtensionsFromEnv()

	// Should have 3 valid extensions (ignoring incomplete ones)
	if len(extensions) != 3 {
		t.Errorf("Expected 3 extensions, got %d", len(extensions))
	}

	// Verify extensions
	expectedExtensions := map[string]Extension{
		"610":   {Number: "610", Name: "Büro Telefon", Type: "DECT"},
		"620":   {Number: "620", Name: "DG 24", Type: "VOIP"},
		"**600": {Number: "**600", Name: "AB 22", Type: "VOICEBOX"},
	}

	for _, ext := range extensions {
		expected, exists := expectedExtensions[ext.Number]
		if !exists {
			t.Errorf("Unexpected extension number: %s", ext.Number)
			continue
		}

		if ext.Name != expected.Name {
			t.Errorf("Extension %s: expected name %s, got %s", ext.Number, expected.Name, ext.Name)
		}

		if ext.Type != expected.Type {
			t.Errorf("Extension %s: expected type %s, got %s", ext.Number, expected.Type, ext.Type)
		}
	}
}

func TestDeriveExtensionType(t *testing.T) {
	testCases := []struct {
		number   string
		expected string
	}{
		// Test **6xx internal numbers
		{"**600", "VOICEBOX"},
		{"**605", "VOICEBOX"},
		{"**609", "VOICEBOX"},
		{"**610", "DECT"},
		{"**615", "DECT"},
		{"**619", "DECT"},
		{"**620", "VOIP"},
		{"**625", "VOIP"},
		{"**629", "VOIP"},
		// Test GUI 6xx numbers
		{"610", "DECT"},
		{"615", "DECT"},
		{"619", "DECT"},
		{"620", "VOIP"},
		{"625", "VOIP"},
		{"629", "VOIP"},
		// Test other numbers
		{"600", "UNKNOWN"}, // Not in valid range
		{"1", "UNKNOWN"},
		{"22", "UNKNOWN"},
		{"99", "UNKNOWN"},
		{"123", "UNKNOWN"},
		{"6000", "UNKNOWN"}, // Too long
		{"", "UNKNOWN"},     // Empty
	}

	for _, tc := range testCases {
		tempConfig := &PBXConfig{}
		result := tempConfig.getExtensionTypeByNumber(tc.number)
		if result != tc.expected {
			t.Errorf("getExtensionTypeByNumber(%s): expected %s, got %s", tc.number, tc.expected, result)
		}
	}
}

func TestPBXConfig_GetExtensionInfo(t *testing.T) {
	pbx := &PBXConfig{
		Extensions: []Extension{
			{Number: "610", Name: "Büro", Type: "DECT"},
			{Number: "620", Name: "VoIP Phone", Type: "VOIP"},
		},
	}

	// Test existing extension
	info := pbx.GetExtensionInfo("610")
	if info == nil {
		t.Fatal("Expected extension info, got nil")
	}

	if info.Number != "610" || info.Name != "Büro" || info.Type != "DECT" {
		t.Errorf("Expected {Number: 610, Name: Büro, Type: DECT}, got %+v", info)
	}

	// Test non-existing extension
	info = pbx.GetExtensionInfo("999")
	if info != nil {
		t.Errorf("Expected nil for non-existing extension, got %+v", info)
	}
}

func TestLoadConfig_WithExtensions(t *testing.T) {
	// Clean environment before test
	cleanExtensionEnv(t)
	defer cleanExtensionEnv(t)

	// Set test extensions
	testEnvs := map[string]string{
		"FRITZ_CALLMONITOR_PBX_EXTENSION_0_NUMBER": "610",
		"FRITZ_CALLMONITOR_PBX_EXTENSION_0_NAME":   "Test Extension",
		"FRITZ_CALLMONITOR_PBX_EXTENSION_0_TYPE":   "DECT",
	}

	for key, value := range testEnvs {
		if err := os.Setenv(key, value); err != nil {
			t.Fatalf("Failed to set env var %s: %v", key, err)
		}
	}

	cfg, err := LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig failed: %v", err)
	}

	if len(cfg.PBX.Extensions) != 1 {
		t.Errorf("Expected 1 extension, got %d", len(cfg.PBX.Extensions))
	}

	ext := cfg.PBX.Extensions[0]
	if ext.Number != "610" || ext.Name != "Test Extension" || ext.Type != "DECT" {
		t.Errorf("Extension not loaded correctly: %+v", ext)
	}
}

// Helper function to clean extension environment variables
func cleanExtensionEnv(t *testing.T) {
	// Get all environment variables
	for _, env := range os.Environ() {
		if len(env) == 0 {
			continue
		}

		// Find the key part
		eqIdx := -1
		for i, c := range env {
			if c == '=' {
				eqIdx = i
				break
			}
		}

		if eqIdx == -1 {
			continue
		}

		key := env[:eqIdx]

		// Unset extension environment variables
		if len(key) > 32 && key[:32] == "FRITZ_CALLMONITOR_PBX_EXTENSION_" {
			if err := os.Unsetenv(key); err != nil {
				t.Logf("Warning: Failed to unset env var %s: %v", key, err)
			}
		}
	}
}
