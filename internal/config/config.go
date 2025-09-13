package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
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

type PBXConfig struct {
	MSN           []string `mapstructure:"msn"`             // List of MSNs ["9876541","9876542",...]
	CountryCode   string   `mapstructure:"country_code"`    // Country code
	LocalAreaCode string   `mapstructure:"local_area_code"` // Local area code
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

// GetLocation returns the configured timezone location
func (c *Config) GetLocation() (*time.Location, error) {
	if c.App.Timezone == "" {
		return time.Local, nil
	}
	return time.LoadLocation(c.App.Timezone)
}
