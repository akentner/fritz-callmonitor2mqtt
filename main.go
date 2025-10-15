package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"fritz-callmonitor2mqtt/internal/callmonitor"
	"fritz-callmonitor2mqtt/internal/config"
	"fritz-callmonitor2mqtt/internal/database"
	"fritz-callmonitor2mqtt/internal/mqtt"
	"fritz-callmonitor2mqtt/pkg/types"
)

var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

func main() {
	var (
		showVersion = flag.Bool("version", false, "Show version information")
		help        = flag.Bool("help", false, "Show help")
		configTest  = flag.Bool("config-test", false, "Test configuration and exit")
	)
	flag.Parse()

	if *showVersion {
		fmt.Printf("fritz-callmonitor2mqtt %s (commit: %s, built: %s)\n", version, commit, date)
		os.Exit(0)
	}

	if *help {
		printUsage()
		os.Exit(0)
	}

	// Load configuration
	cfg, err := config.LoadConfig()
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	// Validate configuration
	if err := cfg.Validate(); err != nil {
		log.Fatalf("Invalid configuration: %v", err)
	}

	if *configTest {
		cfg.LogConfigurationSummary()
		fmt.Println("Configuration is valid")
		os.Exit(0)
	}

	log.Printf("Starting fritz-callmonitor2mqtt %s...", version)

	// Log configuration summary
	cfg.LogConfigurationSummary()

	// Create context for graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Setup signal handling
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// Initialize database client first
	dbClient, err := database.NewClient(cfg.Database.DataDir)
	if err != nil {
		log.Fatalf("Failed to create database client: %v", err)
	}

	// Initialize MQTT client with database client and PBX config
	mqttClient := mqtt.NewClient(
		cfg.MQTT.Broker,
		cfg.MQTT.Port,
		cfg.MQTT.Username,
		cfg.MQTT.Password,
		cfg.MQTT.ClientID,
		cfg.MQTT.TopicPrefix,
		cfg.MQTT.QoS,
		cfg.MQTT.Retain,
		cfg.MQTT.KeepAlive,
		cfg.MQTT.ConnectTimeout,
		cfg.App.LogLevel,
		dbClient,
		&cfg.PBX,
		cfg.PBX.MSN,
		cfg.App.MSNCallHistorySize,
	)

	// Connect to database
	if err := dbClient.Connect(); err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	log.Printf("Database: %s", dbClient.GetDatabasePath())

	// Run migrations
	if err := dbClient.RunEmbeddedMigrations(); err != nil {
		log.Fatalf("Failed to run database migrations: %v", err)
	}
	log.Println("Database migrations completed successfully")

	// Initialize callmonitor client
	timezone, err := cfg.GetLocation()
	if err != nil {
		log.Fatalf("Failed to load timezone: %v", err)
	}
	callmonitorClient := callmonitor.NewClient(cfg.FritzBox.Host, cfg.FritzBox.Port, timezone, cfg.PBX.CountryCode, cfg.PBX.LocalAreaCode, cfg.PBX.MSN)

	// Set phone number lookup in callmonitor client
	callmonitorClient.SetPhoneNumberLookup(dbClient)

	// Set extension lookup in callmonitor client
	callmonitorClient.SetExtensionLookup(&cfg.PBX)

	// Initialize call manager with MQTT and database integration
	callManager := types.NewCallManagerWithMQTTAndDB(mqttClient, dbClient, func(line int, oldStatus, newStatus types.CallStatus, event *types.CallEvent) {
		log.Printf("Line %d status changed: %s -> %s", line, oldStatus, newStatus)
	})

	// Start the application
	app := &Application{
		config:            cfg,
		mqttClient:        mqttClient,
		callmonitorClient: callmonitorClient,
		dbClient:          dbClient,
		callManager:       callManager,
		ctx:               ctx,
	}

	// Run application in background
	go func() {
		if err := app.Run(); err != nil {
			log.Printf("Application error: %v", err)
			cancel()
		}
	}()

	// Wait for shutdown signal
	select {
	case sig := <-sigChan:
		log.Printf("Received signal %v, shutting down gracefully...", sig)
	case <-ctx.Done():
		log.Println("Context cancelled, shutting down...")
	}

	// Shutdown
	app.Shutdown()
	log.Println("fritz-callmonitor2mqtt stopped")
}

// Application holds all application components
type Application struct {
	config            *config.Config
	mqttClient        *mqtt.Client
	callmonitorClient *callmonitor.Client
	dbClient          *database.Client
	callManager       *types.CallManager
	ctx               context.Context
}

// Run starts the main application loop
func (app *Application) Run() error {
	// Connect to MQTT broker
	log.Println("Connecting to MQTT broker...")
	if err := app.mqttClient.Connect(); err != nil {
		return fmt.Errorf("failed to connect to MQTT broker: %w", err)
	}
	log.Println("Connected to MQTT broker")

	// Bootstrap MQTT state from database
	if err := app.mqttClient.BootstrapFromDatabase(); err != nil {
		log.Printf("Bootstrap from database failed: %v", err)
		// Continue anyway - this is not critical
	}

	// Main connection loop with retry logic
	for {
		select {
		case <-app.ctx.Done():
			return nil
		default:
		}

		log.Println("Connecting to FRITZ!Box callmonitor...")
		if err := app.callmonitorClient.Connect(); err != nil {
			log.Printf("Failed to connect to FRITZ!Box: %v", err)
			log.Printf("Retrying in %v...", app.config.App.ReconnectDelay)

			select {
			case <-time.After(app.config.App.ReconnectDelay):
				continue
			case <-app.ctx.Done():
				return nil
			}
		}

		log.Println("Connected to FRITZ!Box callmonitor")

		// Process events until connection is lost
		if err := app.processEvents(); err != nil {
			log.Printf("Event processing error: %v", err)
		}

		// Clean up connection
		if err := app.callmonitorClient.Disconnect(); err != nil {
			log.Printf("Error disconnecting callmonitor: %v", err)
		}

		if app.ctx.Err() != nil {
			return nil
		}

		log.Printf("Connection lost, reconnecting in %v...", app.config.App.ReconnectDelay)
		select {
		case <-time.After(app.config.App.ReconnectDelay):
		case <-app.ctx.Done():
			return nil
		}
	}
}

// processEvents handles incoming call events
func (app *Application) processEvents() error {
	for {
		select {
		case <-app.ctx.Done():
			return nil

		case event := <-app.callmonitorClient.Events():
			log.Printf("Received call event: %s - %s -> %s (ID: %s,Type: %s, Line: %d, Trunk: %s)",
				event.Timestamp.Format("15:04:05"),
				event.Caller,
				event.Called,
				event.ID,
				event.Type,
				event.Line,
				event.Trunk)

			// Enrich event with extension information
			app.enrichEventWithExtensions(&event)

			// Process through FSM and publish event to MQTT
			processedEvent := app.callManager.ProcessEvent(&event)
			if err := app.mqttClient.PublishCallEvent(*processedEvent); err != nil {
				log.Printf("Failed to publish call event: %v", err)
			}

		case err := <-app.callmonitorClient.Errors():
			return fmt.Errorf("callmonitor error: %w", err)
		}
	}
}

// enrichEventWithExtensions adds extension information to call events
func (app *Application) enrichEventWithExtensions(event *types.CallEvent) {
	// Enrich caller extension if available
	if event.Extension != "" {
		// For incoming calls, the extension is the called party (our internal extension)
		// For outgoing calls, the extension is the calling party (our internal extension)
		if extInfo := app.config.PBX.GetExtensionInfo(event.Extension); extInfo != nil {
			if event.Direction == types.CallDirectionInbound {
				event.CalledExtension = &types.ExtensionInfo{
					Number: extInfo.Number,
					Name:   extInfo.Name,
					Type:   extInfo.Type,
				}
			} else if event.Direction == types.CallDirectionOutbound {
				event.CallerExtension = &types.ExtensionInfo{
					Number: extInfo.Number,
					Name:   extInfo.Name,
					Type:   extInfo.Type,
				}
			}
		}
	}
}

// Shutdown gracefully shuts down the application
func (app *Application) Shutdown() {
	log.Println("Shutting down application...")

	if app.callManager != nil {
		app.callManager.Cleanup()
	}

	if app.callmonitorClient != nil {
		if err := app.callmonitorClient.Disconnect(); err != nil {
			log.Printf("Error disconnecting callmonitor: %v", err)
		}
	}

	if app.mqttClient != nil {
		if err := app.mqttClient.Disconnect(); err != nil {
			log.Printf("Error disconnecting MQTT: %v", err)
		}
	}

	if app.dbClient != nil {
		if err := app.dbClient.Close(); err != nil {
			log.Printf("Error closing database: %v", err)
		}
	}
}

func printUsage() {
	fmt.Printf(`Usage: fritz-callmonitor2mqtt [OPTIONS]

FRITZ!Box Callmonitor to MQTT Bridge - Monitors FRITZ!Box call events and publishes them to MQTT.

Options:
  -version       Show version information
  -help          Show this help message
  -config-test   Test configuration and exit

Configuration via Environment Variables:
  FRITZ_CALLMONITOR_FRITZBOX_HOST            FRITZ!Box hostname (default: fritz.box)
  FRITZ_CALLMONITOR_FRITZBOX_PORT            FRITZ!Box callmonitor port (default: 1012)
  FRITZ_CALLMONITOR_MQTT_BROKER              MQTT broker hostname (default: localhost)
  FRITZ_CALLMONITOR_MQTT_PORT                MQTT broker port (default: 1883)
  FRITZ_CALLMONITOR_MQTT_USERNAME            MQTT username (optional)
  FRITZ_CALLMONITOR_MQTT_PASSWORD            MQTT password (optional)
  FRITZ_CALLMONITOR_MQTT_CLIENT_ID           MQTT client ID (default: fritz-callmonitor2mqtt)
  FRITZ_CALLMONITOR_MQTT_TOPIC_PREFIX        MQTT topic prefix (default: fritz/callmonitor)
  FRITZ_CALLMONITOR_MQTT_QOS                 MQTT QoS level (default: 1)
  FRITZ_CALLMONITOR_MQTT_RETAIN              MQTT retain messages (default: true)
  FRITZ_CALLMONITOR_APP_LOG_LEVEL            Log level (default: info)
  FRITZ_CALLMONITOR_APP_CALL_HISTORY_SIZE    Call history size (default: 50)
  FRITZ_CALLMONITOR_DATABASE_DATA_DIR        Database data directory (default: ./data)

MQTT Topics:
  {prefix}/line/{line_id}/status   - Current status of each phone line (retained)
  {prefix}/history                 - Last 50 calls as JSON array (retained)  
  {prefix}/events/{call_type}      - Individual call events (incoming/outgoing/connect/end)

Examples:
  fritz-callmonitor2mqtt                                    # Run with defaults
  fritz-callmonitor2mqtt -version                           # Show version
  fritz-callmonitor2mqtt -config-test                       # Test configuration
  
  # With custom FRITZ!Box
  FRITZ_CALLMONITOR_FRITZBOX_HOST=192.168.1.1 fritz-callmonitor2mqtt
  
  # With custom MQTT broker
  FRITZ_CALLMONITOR_MQTT_BROKER=mqtt.example.com \
  FRITZ_CALLMONITOR_MQTT_USERNAME=user \
  FRITZ_CALLMONITOR_MQTT_PASSWORD=pass \
  fritz-callmonitor2mqtt

`)
}
