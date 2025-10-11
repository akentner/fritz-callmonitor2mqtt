package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"fritz-callmonitor2mqtt/pkg/types"
)

// Config holds all configuration for the application
type Config struct {
	// Fritz!Box settings
	FritzBox FritzBoxConfig `mapstructure:"fritzbox"`

	// PBX settings
	PBX PBXConfig `mapstructure:"pbx"`

	// MQTT settings
	MQTT MQTTConfig `mapstructure:"mqtt"`

	// Application settings
	App AppConfig `mapstructure:"app"`

	// Database settings
	Database DatabaseConfig `mapstructure:"database"`
}

// FritzBoxConfig contains Fritz!Box connection settings
type FritzBoxConfig struct {
	Host string `mapstructure:"host"`
	Port int    `mapstructure:"port"`
}

// Extension represents a Fritz!Box extension/nebenstelle
type Extension struct {
	Number string `json:"number"` // Extension number (e.g., "610", "620")
	Name   string `json:"name"`   // Human-readable name (e.g., "Büro", "Wohnzimmer")
	Type   string `json:"type"`   // Extension type: DECT, VOIP, VOICEBOX, ANALOG, UNKNOWN
}

type PBXConfig struct {
	MSN           []string    `mapstructure:"msn"`             // List of MSNs ["9876541","9876542",...]
	CountryCode   string      `mapstructure:"country_code"`    // Country code
	LocalAreaCode string      `mapstructure:"local_area_code"` // Local area code
	Extensions    []Extension `mapstructure:"extensions"`      // Extension configurations
}

// MQTTConfig contains MQTT broker settings
type MQTTConfig struct {
	Broker         string        `mapstructure:"broker"`
	Port           int           `mapstructure:"port"`
	Username       string        `mapstructure:"username"`
	Password       string        `mapstructure:"password"`
	ClientID       string        `mapstructure:"client_id"`
	TopicPrefix    string        `mapstructure:"topic_prefix"`
	QoS            byte          `mapstructure:"qos"`
	Retain         bool          `mapstructure:"retain"`
	KeepAlive      time.Duration `mapstructure:"keep_alive"`
	ConnectTimeout time.Duration `mapstructure:"connect_timeout"`
}

// AppConfig contains general application settings
type AppConfig struct {
	LogLevel        string        `mapstructure:"log_level"`
	CallHistorySize int           `mapstructure:"call_history_size"`
	ReconnectDelay  time.Duration `mapstructure:"reconnect_delay"`
	HealthCheckPort int           `mapstructure:"health_check_port"`
	Timezone        string        `mapstructure:"timezone"`
}

// DatabaseConfig contains database settings
type DatabaseConfig struct {
	DataDir string `mapstructure:"data_dir"` // Data directory path
}

// LoadConfig loads configuration from environment variables and defaults
func LoadConfig() (*Config, error) {
	config := &Config{
		FritzBox: FritzBoxConfig{
			Host: getEnvOrDefault("FRITZ_CALLMONITOR_FRITZBOX_HOST", "fritz.box"),
			Port: getEnvIntOrDefault("FRITZ_CALLMONITOR_FRITZBOX_PORT", 1012),
		},
		PBX: PBXConfig{
			MSN:           getEnvListOrDefault("FRITZ_CALLMONITOR_PBX_MSN", []string{}),
			CountryCode:   getEnvOrDefault("FRITZ_CALLMONITOR_PBX_COUNTRY_CODE", "49"),
			LocalAreaCode: getEnvOrDefault("FRITZ_CALLMONITOR_PBX_LOCAL_AREA_CODE", ""),
			Extensions:    loadExtensionsFromEnv(),
		},
		MQTT: MQTTConfig{
			Broker:         getEnvOrDefault("FRITZ_CALLMONITOR_MQTT_BROKER", "localhost"),
			Port:           getEnvIntOrDefault("FRITZ_CALLMONITOR_MQTT_PORT", 1883),
			Username:       getEnvOrDefault("FRITZ_CALLMONITOR_MQTT_USERNAME", ""),
			Password:       getEnvOrDefault("FRITZ_CALLMONITOR_MQTT_PASSWORD", ""),
			ClientID:       getEnvOrDefault("FRITZ_CALLMONITOR_MQTT_CLIENT_ID", "fritz-callmonitor2mqtt"),
			TopicPrefix:    getEnvOrDefault("FRITZ_CALLMONITOR_MQTT_TOPIC_PREFIX", "fritz/callmonitor"),
			QoS:            byte(getEnvIntOrDefault("FRITZ_CALLMONITOR_MQTT_QOS", 1)),
			Retain:         getEnvBoolOrDefault("FRITZ_CALLMONITOR_MQTT_RETAIN", true),
			KeepAlive:      getEnvDurationOrDefault("FRITZ_CALLMONITOR_MQTT_KEEP_ALIVE", 60*time.Second),
			ConnectTimeout: getEnvDurationOrDefault("FRITZ_CALLMONITOR_MQTT_CONNECT_TIMEOUT", 30*time.Second),
		},
		App: AppConfig{
			LogLevel:        getEnvOrDefault("FRITZ_CALLMONITOR_APP_LOG_LEVEL", "info"),
			CallHistorySize: getEnvIntOrDefault("FRITZ_CALLMONITOR_APP_CALL_HISTORY_SIZE", 50),
			ReconnectDelay:  getEnvDurationOrDefault("FRITZ_CALLMONITOR_APP_RECONNECT_DELAY", 10*time.Second),
			HealthCheckPort: getEnvIntOrDefault("FRITZ_CALLMONITOR_APP_HEALTH_CHECK_PORT", 8080),
			Timezone:        getEnvOrDefault("FRITZ_CALLMONITOR_APP_TIMEZONE", "Europe/Berlin"),
		},
		Database: DatabaseConfig{
			DataDir: getEnvOrDefault("FRITZ_CALLMONITOR_DATABASE_DATA_DIR", "./data"),
		},
	}

	return config, nil
}

func getEnvListOrDefault(key string, defaultValue []string) []string {
	if value := os.Getenv(key); value != "" {
		return strings.Split(value, ",")
	}
	return defaultValue
}

// Helper functions for environment variable handling
func getEnvOrDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getEnvIntOrDefault(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		if intValue, err := strconv.Atoi(value); err == nil {
			return intValue
		}
	}
	return defaultValue
}

func getEnvBoolOrDefault(key string, defaultValue bool) bool {
	if value := os.Getenv(key); value != "" {
		if boolValue, err := strconv.ParseBool(value); err == nil {
			return boolValue
		}
	}
	return defaultValue
}

func getEnvDurationOrDefault(key string, defaultValue time.Duration) time.Duration {
	if value := os.Getenv(key); value != "" {
		if duration, err := time.ParseDuration(value); err == nil {
			return duration
		}
	}
	return defaultValue
}

// Validate checks if the configuration is valid
func (c *Config) Validate() error {
	if c.FritzBox.Host == "" {
		return fmt.Errorf("fritz.box host cannot be empty")
	}

	if c.FritzBox.Port <= 0 || c.FritzBox.Port > 65535 {
		return fmt.Errorf("fritz.box port must be between 1 and 65535")
	}

	if c.MQTT.Broker == "" {
		return fmt.Errorf("MQTT broker cannot be empty")
	}

	if c.MQTT.Port <= 0 || c.MQTT.Port > 65535 {
		return fmt.Errorf("MQTT port must be between 1 and 65535")
	}

	if c.App.CallHistorySize <= 0 {
		return fmt.Errorf("call history size must be greater than 0")
	}

	if c.App.Timezone != "" {
		if _, err := time.LoadLocation(c.App.Timezone); err != nil {
			return fmt.Errorf("invalid timezone '%s': %w", c.App.Timezone, err)
		}
	}

	if c.Database.DataDir == "" {
		return fmt.Errorf("database data directory cannot be empty")
	}

	return nil
}

// loadExtensionsFromEnv loads extension configurations from environment variables
// Format: FRITZ_CALLMONITOR_PBX_EXTENSION_<INDEX>_<PROPERTY>
// Example: FRITZ_CALLMONITOR_PBX_EXTENSION_0_NUMBER="610"
//
//	FRITZ_CALLMONITOR_PBX_EXTENSION_0_NAME="Büro"
//	FRITZ_CALLMONITOR_PBX_EXTENSION_0_TYPE="DECT"
func loadExtensionsFromEnv() []Extension {
	extensions := []Extension{}
	extensionMap := make(map[string]map[string]string)

	// Parse all environment variables with the extension prefix
	for _, env := range os.Environ() {
		parts := strings.SplitN(env, "=", 2)
		if len(parts) != 2 {
			continue
		}

		key, value := parts[0], parts[1]

		// Check if this is an extension environment variable
		if !strings.HasPrefix(key, "FRITZ_CALLMONITOR_PBX_EXTENSION_") {
			continue
		}

		// Extract index and property from key
		// FRITZ_CALLMONITOR_PBX_EXTENSION_0_NUMBER -> ["0", "NUMBER"]
		suffix := strings.TrimPrefix(key, "FRITZ_CALLMONITOR_PBX_EXTENSION_")
		parts = strings.SplitN(suffix, "_", 2)
		if len(parts) != 2 {
			continue
		}

		index, property := parts[0], parts[1]

		// Initialize extension map for this index
		if extensionMap[index] == nil {
			extensionMap[index] = make(map[string]string)
		}

		extensionMap[index][property] = value
	}

	// Convert map to Extension structs
	for _, extMap := range extensionMap {
		number := extMap["NUMBER"]
		name := extMap["NAME"]

		// Skip if required fields are missing
		if number == "" || name == "" {
			continue
		}

		// Get type from env or derive automatically
		extType := extMap["TYPE"]
		if extType == "" {
			// Use a temporary PBXConfig to access the method
			tempConfig := &PBXConfig{}
			extType = tempConfig.getExtensionTypeByNumber(number)
		}

		extensions = append(extensions, Extension{
			Number: number,
			Name:   name,
			Type:   extType,
		})
	}

	return extensions
}

// GetExtensionByNumber returns the extension configuration for a given number
func (p *PBXConfig) GetExtensionByNumber(number string) *Extension {
	for i := range p.Extensions {
		if p.Extensions[i].Number == number {
			return &p.Extensions[i]
		}
	}
	return nil
}

// GetExtensionInfo returns extension info in types.ExtensionInfo format
// It supports both GUI numbers (e.g., "621") and callmonitor event numbers (e.g., "21")
func (p *PBXConfig) GetExtensionInfo(number string) *types.ExtensionInfo {
	// First try direct lookup with the provided number
	ext := p.GetExtensionByNumber(number)
	if ext != nil {
		return &types.ExtensionInfo{
			Number: ext.Number,
			Name:   ext.Name,
			Type:   ext.Type,
		}
	}

	// If not found, try to find by mapped extension number
	// Fritz!Box transforms GUI extension numbers to callmonitor event numbers:
	// GUI 621 -> Event 21, GUI 615 -> Event 15, etc.
	ext = p.findExtensionByCallmonitorNumber(number)
	if ext != nil {
		return &types.ExtensionInfo{
			Number: ext.Number,
			Name:   ext.Name,
			Type:   ext.Type,
		}
	}

	return nil
}

// findExtensionByCallmonitorNumber finds an extension by its callmonitor event number
// Fritz!Box transforms GUI extension numbers to callmonitor event numbers:
// - GUI 6xx -> Event xx (e.g., 621 -> 21, 615 -> 15)
// - Analogue Box numbers map differently (e.g., AB 22 -> 40)
func (p *PBXConfig) findExtensionByCallmonitorNumber(eventNumber string) *Extension {
	for _, ext := range p.Extensions {
		if p.getCallmonitorNumber(ext.Number) == eventNumber {
			return &ext
		}
	}
	return nil
}

// getCallmonitorNumber converts extension numbers to their callmonitor event equivalent
// Based on Fritz!Box internal number mapping:
// - **600 to **609 are VOICEBOX and appear as Extensions 40-49 in events
// - **610 to **619 are DECT and appear as Extensions 10-19 in events
// - **620 to **629 are VOIP and appear as Extensions 20-29 in events
// - GUI 6xx extensions: remove leading "6" (621 -> 21, 615 -> 15)
// - Other numbers return as-is
func (p *PBXConfig) getCallmonitorNumber(guiNumber string) string {
	// Handle internal **6xx numbers directly
	if strings.HasPrefix(guiNumber, "**6") && len(guiNumber) == 5 {
		internalNum := guiNumber[3:] // Extract last two digits from "**6xx"
		if num, err := strconv.Atoi(internalNum); err == nil {
			if num >= 0 && num <= 9 {
				// **600-**609 -> VOICEBOX -> Events 40-49
				return strconv.Itoa(40 + num)
			} else if num >= 10 && num <= 19 {
				// **610-**619 -> DECT -> Events 10-19
				return strconv.Itoa(num) // **615 -> Event 15, **612 -> Event 12
			} else if num >= 20 && num <= 29 {
				// **620-**629 -> VOIP -> Events 20-29
				return strconv.Itoa(num) // **623 -> Event 23, **621 -> Event 21
			}
		}
	}

	// Handle 6xx GUI extensions: remove leading "6"
	if len(guiNumber) == 3 && strings.HasPrefix(guiNumber, "6") {
		return guiNumber[1:] // "621" -> "21", "615" -> "15"
	}

	// For all other extensions, return as-is
	return guiNumber
}

// getExtensionTypeByNumber determines the extension type based on Fritz!Box internal number ranges
func (p *PBXConfig) getExtensionTypeByNumber(number string) string {
	// Handle internal **6xx numbers
	if strings.HasPrefix(number, "**6") && len(number) == 5 {
		internalNum := number[3:] // Extract last two digits from "**6xx"
		if num, err := strconv.Atoi(internalNum); err == nil {
			if num >= 0 && num <= 9 {
				return "VOICEBOX" // **600-**609 -> VOICEBOX
			} else if num >= 10 && num <= 19 {
				return "DECT" // **610-**619 -> DECT
			} else if num >= 20 && num <= 29 {
				return "VOIP" // **620-**629 -> VOIP
			}
		}
	}

	// Handle 6xx GUI extensions
	if len(number) == 3 && strings.HasPrefix(number, "6") {
		if num, err := strconv.Atoi(number[1:]); err == nil {
			if num >= 10 && num <= 19 {
				return "DECT" // 610-619 -> DECT
			} else if num >= 20 && num <= 29 {
				return "VOIP" // 620-629 -> VOIP
			}
		}
	}

	// All other numbers are UNKNOWN
	return "UNKNOWN"
}

// GetLocation returns the configured timezone location
func (c *Config) GetLocation() (*time.Location, error) {
	if c.App.Timezone == "" {
		return time.Local, nil
	}
	return time.LoadLocation(c.App.Timezone)
}

// LogConfigurationSummary logs a summary of the loaded configuration
func (c *Config) LogConfigurationSummary() {
	fmt.Printf("✅ Configuration loaded:\n")
	fmt.Printf("   Fritz!Box: %s:%d\n", c.FritzBox.Host, c.FritzBox.Port)
	fmt.Printf("   MQTT:      %s:%d\n", c.MQTT.Broker, c.MQTT.Port)
	fmt.Printf("   Topics:    %s/*\n", c.MQTT.TopicPrefix)
	fmt.Printf("   Log Level: %s\n", c.App.LogLevel)
	fmt.Printf("   Database:  %s\n", c.Database.DataDir)

	// MSN information
	if len(c.PBX.MSN) > 0 {
		fmt.Printf("   MSNs:      %s\n", strings.Join(c.PBX.MSN, ", "))
	} else {
		fmt.Printf("   MSNs:      none configured\n")
	}

	// Extension information
	if len(c.PBX.Extensions) > 0 {
		fmt.Printf("   Extensions: %d configured\n", len(c.PBX.Extensions))

		// Group extensions by type for better overview
		voiceboxExts := []Extension{}
		dectExts := []Extension{}
		voipExts := []Extension{}
		otherExts := []Extension{}

		for _, ext := range c.PBX.Extensions {
			switch ext.Type {
			case "VOICEBOX":
				voiceboxExts = append(voiceboxExts, ext)
			case "DECT":
				dectExts = append(dectExts, ext)
			case "VOIP":
				voipExts = append(voipExts, ext)
			default:
				otherExts = append(otherExts, ext)
			}
		}

		fmt.Printf("\n")
		fmt.Printf("Extensions Mapping:\n")

		if len(voiceboxExts) > 0 {
			fmt.Printf("   VOICEBOX: ")
			for i, ext := range voiceboxExts {
				if i > 0 {
					fmt.Printf(", ")
				}
				eventNum := c.PBX.getCallmonitorNumber(ext.Number)
				fmt.Printf("%s→%s (%s)", ext.Number, eventNum, ext.Name)
			}
			fmt.Printf("\n")
		}

		if len(dectExts) > 0 {
			fmt.Printf("   DECT:     ")
			for i, ext := range dectExts {
				eventNum := c.PBX.getCallmonitorNumber(ext.Number)
				fmt.Printf("%s→%s (%s)", ext.Number, eventNum, ext.Name)
				// Line break after every 3 DECT extensions for better readability
				if (i+1)%3 == 0 && i < len(dectExts)-1 {
					fmt.Printf("\n             ")
				} else if i < len(dectExts)-1 {
					fmt.Printf(", ")
				}
			}
			fmt.Printf("\n")
		}

		if len(voipExts) > 0 {
			fmt.Printf("   VOIP:     ")
			for i, ext := range voipExts {
				if i > 0 {
					fmt.Printf(", ")
				}
				eventNum := c.PBX.getCallmonitorNumber(ext.Number)
				fmt.Printf("%s→%s (%s)", ext.Number, eventNum, ext.Name)
			}
			fmt.Printf("\n")
		}

		if len(otherExts) > 0 {
			fmt.Printf("   OTHER:    ")
			for i, ext := range otherExts {
				if i > 0 {
					fmt.Printf(", ")
				}
				eventNum := c.PBX.getCallmonitorNumber(ext.Number)
				fmt.Printf("%s→%s (%s)", ext.Number, eventNum, ext.Name)
			}
			fmt.Printf("\n")
		}
	} else {
		fmt.Printf("   Extensions: none configured\n")
	}

	fmt.Printf("\n")
}
