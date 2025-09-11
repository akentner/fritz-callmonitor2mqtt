# Architecture Overview

## Components

```
┌─────────────────┐    TCP 1012     ┌─────────────────┐
│   Fritz!Box     │◄───────────────►│  Callmonitor    │
│   Router        │                 │     Client      │
└─────────────────┘                 └─────────┬───────┘
                                              │
                                              │ Events
                                              ▼
                                    ┌─────────────────┐
                                    │   Application   │
                                    │   Main Loop     │
                                    └─────────┬───────┘
                                              │
                                              │ Publish
                                              ▼
                                    ┌─────────────────┐
                                    │   MQTT Client   │
┌─────────────────┐    MQTT 1883    │                 │
│   MQTT Broker   │◄───────────────►│   • Status      │
│                 │                 │   • History     │
│   • Mosquitto   │                 │   • Events      │
│   • HiveMQ      │                 │                 │
└─────────────────┘                 └─────────────────┘
```

## Data Flow

1. **Fritz!Box Connection**: Application connects to Fritz!Box callmonitor on TCP port 1012
2. **Event Parsing**: Raw callmonitor messages are parsed into structured events
3. **Status Tracking**: Line statuses are maintained and updated based on events  
4. **History Management**: Call history is maintained (last 50 calls)
5. **MQTT Publishing**: Events and statuses are published to MQTT topics

## Message Types

### Fritz!Box Callmonitor Format
```
timestamp;type;id;extension;caller;called;sip[;duration]
```

Examples:
- `21.09.25 15:30:45;RING;0;1;123456789;987654321;SIP0` - Incoming call
- `21.09.25 15:31:00;CALL;1;2;987654321;123456789;SIP1` - Outgoing call  
- `21.09.25 15:31:05;CONNECT;1;2;987654321;123456789` - Call connected
- `21.09.25 15:35:00;DISCONNECT;1;2;987654321;123456789;SIP1;235` - Call ended (235 seconds)

### MQTT Topics Structure

```
fritz/callmonitor/
├── line/
│   ├── SIP0/
│   │   └── status          # Line status (retained)
│   └── SIP1/
│       └── status          # Line status (retained)  
├── history                 # Call history (retained)
└── events/
    ├── incoming           # Incoming call events
    ├── outgoing           # Outgoing call events
    ├── connect            # Call connection events
    └── end                # Call end events
```

## Error Handling

- **Connection Loss**: Automatic reconnection with configurable delay
- **Parse Errors**: Invalid messages are logged but don't crash the application
- **MQTT Errors**: Failed publishes are logged, application continues
- **Graceful Shutdown**: SIGINT/SIGTERM handling for clean shutdown

## Performance Considerations

- **Buffered Channels**: Event processing uses buffered channels to prevent blocking
- **Concurrent Safety**: Thread-safe access to shared state (line status, history)
- **Memory Management**: Call history is limited to configured size (default: 50)
- **Retained Messages**: MQTT status and history messages are retained for persistence
