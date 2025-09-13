# Examples

This directory contains examples for integrating with the fritz-callmonitor2mqtt phone number RPC API.

## Available Examples

### Python Client (`phone-rpc-client.py`)
Complete Python client using paho-mqtt for phone number management.

**Prerequisites:**
```bash
pip install paho-mqtt
```

**Usage:**
```bash
python3 phone-rpc-client.py set +1234567890 "John Doe"
python3 phone-rpc-client.py get +1234567890
python3 phone-rpc-client.py list --limit 50
python3 phone-rpc-client.py search "John"
python3 phone-rpc-client.py delete +1234567890
```

**Features:**
- Full RPC API support (set, get, delete, list, search)
- Configurable MQTT connection parameters
- Response timeout handling
- Pretty-printed output with status indicators

### Node.js Client (`phone-rpc-client.js`)
Node.js client for phone number management via MQTT.

**Prerequisites:**
```bash
npm install mqtt
```

**Usage:**
```bash
node phone-rpc-client.js set +1234567890 "John Doe"
node phone-rpc-client.js get +1234567890
node phone-rpc-client.js list --limit 50
node phone-rpc-client.js search "John"
node phone-rpc-client.js delete +1234567890
```

**Features:**
- Full RPC API support
- Promise-based async/await pattern
- Configurable broker connection
- Error handling and timeouts

### Shell Script Client (`phone-rpc-shell.sh`)
Bash script using mosquitto command line tools for RPC operations.

**Prerequisites:**
```bash
# Ubuntu/Debian
apt install mosquitto-clients jq

# macOS
brew install mosquitto jq
```

**Usage:**
```bash
./phone-rpc-shell.sh set +1234567890 "John Doe"
./phone-rpc-shell.sh get +1234567890
./phone-rpc-shell.sh list 50
./phone-rpc-shell.sh search "John" 10
./phone-rpc-shell.sh delete +1234567890
./phone-rpc-shell.sh monitor  # Monitor all responses
```

**Features:**
- Uses standard mosquitto-clients tools
- JSON pretty-printing with jq
- Response monitoring mode
- Configurable via environment variables

### Home Assistant Integration (`home-assistant/`)
Complete Home Assistant integration with sensors, automations, and dashboard cards.

**Files:**
- `configuration.yaml` - Sensors, scripts, and basic setup
- `automations.yaml` - Call notifications and automated responses
- `README.md` - Setup instructions and entity descriptions

**Features:**
- Call event sensors and binary sensors
- Automatic caller name resolution
- Phone number management scripts
- Call notifications and TTS announcements
- Persistent logging of important calls

## Configuration

All examples use these default MQTT settings:

- **Broker**: `localhost:1883`
- **Topic Prefix**: `fritz/callmonitor`
- **Request Topic**: `fritz/callmonitor/phone_number/request`  
- **Response Topic**: `fritz/callmonitor/phone_number/response`

### Environment Variables

You can configure the examples using these environment variables:

```bash
export MQTT_BROKER_HOST="mqtt.example.com"
export MQTT_BROKER_PORT="1883"
export MQTT_BROKER_USERNAME="user"
export MQTT_BROKER_PASSWORD="pass"  
export MQTT_TOPIC_PREFIX="fritz/callmonitor"
```

## RPC API Quick Reference

### Set Phone Number
```json
{
  "id": "unique-id",
  "method": "set",
  "phone_number": "+1234567890",
  "name": "John Doe",
  "timestamp": "2025-09-13T12:30:00Z"
}
```

### Get Phone Number
```json
{
  "id": "unique-id",
  "method": "get", 
  "phone_number": "+1234567890",
  "timestamp": "2025-09-13T12:30:00Z"
}
```

### List All Numbers
```json
{
  "id": "unique-id",
  "method": "list",
  "limit": 100,
  "timestamp": "2025-09-13T12:30:00Z"
}
```

### Search Numbers
```json
{
  "id": "unique-id",
  "method": "search",
  "pattern": "John",
  "limit": 50,
  "timestamp": "2025-09-13T12:30:00Z"  
}
```

### Delete Number
```json
{
  "id": "unique-id",
  "method": "delete",
  "phone_number": "+1234567890",
  "timestamp": "2025-09-13T12:30:00Z"
}
```

## Testing

To test the examples, first ensure fritz-callmonitor2mqtt is running:

```bash
# Start the service
make dev-run

# In another terminal, test the examples
python3 examples/phone-rpc-client.py set "+1234567890" "Test Contact"
python3 examples/phone-rpc-client.py get "+1234567890" 
python3 examples/phone-rpc-client.py list
```

## Integration Tips

### Error Handling
Always check the `success` field in responses:

```python
response = client.send_request("get", phone_number="+123456789")
if response and response.get('success'):
    # Handle success
    contact = response.get('phone_number')
else:
    # Handle error  
    error = response.get('error', 'Unknown error')
    print(f"Error: {error}")
```

### Request ID Correlation
Use unique request IDs to correlate requests with responses:

```javascript
const requestId = `req-${Date.now()}-${Math.random()}`;
const request = {
    id: requestId,
    method: 'get',
    phone_number: '+123456789'
};
```

### Timeout Handling
Always implement timeouts for MQTT operations:

```bash
# Shell timeout example
timeout 10 mosquitto_sub -h localhost -t response_topic -C 1
```

## Troubleshooting

### Common Issues

1. **No Response**: Check MQTT broker connectivity and topic subscriptions
2. **Permission Denied**: Verify MQTT authentication credentials  
3. **Invalid JSON**: Ensure request JSON is properly formatted
4. **Timeout**: Check if fritz-callmonitor2mqtt service is running

### Debug Mode

Enable debug output in the examples:

```bash
# Python
python3 phone-rpc-client.py --broker localhost --debug set "+123" "Test"

# Node.js  
DEBUG=* node phone-rpc-client.js set "+123" "Test"

# Shell
MQTT_DEBUG=1 ./phone-rpc-shell.sh set "+123" "Test"
```

### MQTT Monitoring

Monitor MQTT traffic to debug issues:

```bash
# Monitor all topics
mosquitto_sub -h localhost -t 'fritz/callmonitor/#' -v

# Monitor only phone number RPC  
mosquitto_sub -h localhost -t 'fritz/callmonitor/phone_number/+' -v
```
