# MQTT Integration Guide

## Overview

The fritz-callmonitor2mqtt service uses the Eclipse Paho MQTT Go client to publish Fritz!Box call events to an MQTT broker. This enables integration with home automation systems, monitoring dashboards, and other MQTT-aware applications.

## MQTT Library

- **Library**: `github.com/eclipse/paho.mqtt.golang`
- **Version**: v1.5.0
- **Features**: Auto-reconnection, TLS support, WebSocket support

## Connection Features

### Automatic Reconnection
- Built-in reconnection logic
- Configurable connection timeout
- Connection state monitoring

### Security
- Username/Password authentication
- TLS support (configure via broker URL: `ssl://broker:8883`)
- Clean session handling

### Quality of Service
- Configurable QoS levels (0, 1, 2)
- Retained messages for status persistence
- Message delivery confirmation

## Topic Structure

All topics use the configurable prefix (default: `fritz/callmonitor`):

### Service Status Topic (Birth/Last Will)
```
{prefix}/status
```
- **Retained**: Yes
- **QoS**: Configurable (default: 1)
- **Payload**: JSON ServiceStatus object
- **Updates**: On connect (birth) and disconnect (last will)

**Payload Structure:**
```json
{
  "state": "online|offline",
  "last_changed": "2025-09-09T10:30:45Z"
}
```

This topic implements MQTT Birth and Last Will Testament (LWT):
- **Birth Message**: Published when the service connects with `"state": "online"`
- **Last Will**: Automatically published by broker when connection is lost with `"state": "offline"`
- **Graceful Shutdown**: Explicit offline message sent before disconnect

### Line Status Topics
```
{prefix}/line/{line_id}/status
```
- **Retained**: Yes
- **QoS**: Configurable (default: 1)
- **Payload**: JSON LineStatus object
- **Example**: `fritz/callmonitor/line/SIP0/status`

**Payload Structure:**
```json
{
  "line_id": "SIP0",
  "extension": "1", 
  "status": "idle|ring|active",
  "current_call": {
    "timestamp": "2025-09-09T10:30:45Z",
    "type": "incoming",
    "id": "12345",
    "caller": "123456789",
    "called": "987654321"
  },
  "last_updated": "2025-09-09T10:30:45Z"
}
```

### Call History Topic
```
{prefix}/history
```
- **Retained**: Yes
- **QoS**: Configurable (default: 1)
- **Payload**: JSON CallHistory object
- **Updates**: On every call event

**Payload Structure:**
```json
{
  "calls": [
    {
      "timestamp": "2025-09-09T10:30:45Z",
      "type": "incoming",
      "id": "12345",
      "extension": "1",
      "caller": "123456789",
      "called": "987654321",
      "line_id": "SIP0",
      "duration": 235,
      "raw_message": "21.09.25 15:35:00;DISCONNECT;1;2;987654321;123456789;SIP1;235"
    }
  ],
  "max_size": 50,
  "updated_at": "2025-09-09T10:30:45Z"
}
```

### Event Topics
```
{prefix}/events/{call_type}
```
- **Retained**: No
- **QoS**: Configurable (default: 1)
- **Payload**: JSON CallEvent object

**Call Types:**
- `ring` - Incoming call started
- `call` - Outgoing call started  
- `connect` - Call answered/connected
- `disconnect` - Call ended

**Call Tracking:**
Each call receives a unique UUID v7 identifier that persists across all call states (ring/call → connect → disconnect). This enables tracking of complete call lifecycles and correlating events for the same call.

**Payload Structure:**
```json
{
  "id": "01933e88-a140-7d2c-b0a8-123456789abc",
  "timestamp": "2025-09-09T10:30:45Z", 
  "type": "ring",
  "direction": "inbound",
  "line": 0,
  "trunk": "SIP0",
  "extension": "1",
  "caller": "+493023456789", 
  "called": "+493087654321",
  "duration": 0,
  "raw_message": "21.09.25 15:30:45;RING;0;123456789;987654321;SIP0"
}
```

## Configuration

### Environment Variables
```bash
# MQTT Broker
FRITZ_CALLMONITOR_MQTT_BROKER=localhost
FRITZ_CALLMONITOR_MQTT_PORT=1883

# Authentication (optional)
FRITZ_CALLMONITOR_MQTT_USERNAME=your_username
FRITZ_CALLMONITOR_MQTT_PASSWORD=your_password

# Client Settings
FRITZ_CALLMONITOR_MQTT_CLIENT_ID=fritz-callmonitor2mqtt
FRITZ_CALLMONITOR_MQTT_TOPIC_PREFIX=fritz/callmonitor
FRITZ_CALLMONITOR_MQTT_QOS=1
FRITZ_CALLMONITOR_MQTT_RETAIN=true

# Connection Timeouts
FRITZ_CALLMONITOR_MQTT_KEEP_ALIVE=60s
FRITZ_CALLMONITOR_MQTT_CONNECT_TIMEOUT=30s
```

### TLS/SSL Connection
For secure connections, use SSL URL:
```bash
FRITZ_CALLMONITOR_MQTT_BROKER=ssl://secure-broker.example.com
FRITZ_CALLMONITOR_MQTT_PORT=8883
```

### WebSocket Connection
For WebSocket connections:
```bash
FRITZ_CALLMONITOR_MQTT_BROKER=ws://broker.example.com/mqtt
FRITZ_CALLMONITOR_MQTT_PORT=9001
```

## Home Assistant Integration

### Example MQTT Sensor Configuration

```yaml
# configuration.yaml
mqtt:
  sensor:
    - name: "Fritz Line Status SIP0"
      state_topic: "fritz/callmonitor/line/SIP0/status"
      value_template: "{{ value_json.status }}"
      json_attributes_topic: "fritz/callmonitor/line/SIP0/status"
      
    - name: "Fritz Last Call"
      state_topic: "fritz/callmonitor/history" 
      value_template: "{{ value_json.calls[0].caller if value_json.calls else 'none' }}"
      json_attributes_topic: "fritz/callmonitor/history"
      json_attributes_template: "{{ value_json.calls[0] | default({}) | tojson }}"

  binary_sensor:
    - name: "Phone Ringing"
      state_topic: "fritz/callmonitor/line/SIP0/status"
      value_template: "{{ 'ON' if value_json.status == 'ring' else 'OFF' }}"
      device_class: sound

# Automation Example
automation:
  - alias: "Phone Call Notification"
    trigger:
      - platform: mqtt
        topic: "fritz/callmonitor/events/incoming"
    action:
      - service: notify.mobile_app
        data:
          message: "Incoming call from {{ trigger.payload_json.caller }}"
```

## Node-RED Integration

### Flow Example
```json
[
  {
    "id": "mqtt-in",
    "type": "mqtt in",
    "topic": "fritz/callmonitor/events/+",
    "qos": "1",
    "broker": "your-broker"
  },
  {
    "id": "call-processor", 
    "type": "function",
    "func": "const event = JSON.parse(msg.payload);\nif (event.type === 'incoming') {\n    msg.payload = `Call from ${event.caller}`;\n    return msg;\n}\nreturn null;"
  }
]
```

## Troubleshooting

### Common Issues

1. **Connection Refused**
   - Check MQTT broker is running
   - Verify broker host/port
   - Check firewall settings

2. **Authentication Failed**
   - Verify username/password
   - Check broker ACL settings

3. **Messages Not Retained**
   - Ensure `FRITZ_CALLMONITOR_MQTT_RETAIN=true`
   - Check broker configuration supports retained messages

### Debug Logging
Enable debug logging to see MQTT operations:
```bash
FRITZ_CALLMONITOR_APP_LOG_LEVEL=debug fritz-callmonitor2mqtt
```

### Testing Connection
Test MQTT connectivity with mosquitto tools:
```bash
# Subscribe to all topics
mosquitto_sub -h localhost -t "fritz/callmonitor/#" -v

# Test connection
mosquitto_pub -h localhost -t "fritz/callmonitor/test" -m "hello"
```
