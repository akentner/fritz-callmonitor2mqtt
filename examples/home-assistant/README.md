# FRITZ!Box Callmonitor Home Assistant Integration

This directory contains examples for integrating fritz-callmonitor2mqtt with Home Assistant.

## Features

- **Call Event Sensors**: Track incoming/outgoing calls
- **Phone Number Management**: Manage contact names via Home Assistant
- **Call Status Display**: Show current line status and last calls
- **Automation Examples**: Trigger actions based on call events

## Files

- `configuration.yaml` - Complete Home Assistant configuration example
- `sensors.yaml` - MQTT sensors for call monitoring  
- `automations.yaml` - Example automations for call events
- `scripts.yaml` - Scripts for phone number management
- `lovelace-card.yaml` - Dashboard card for call monitoring

## Setup Instructions

1. **Add MQTT Integration**: Ensure MQTT integration is configured in Home Assistant
2. **Copy Configurations**: Merge the relevant parts into your Home Assistant configuration
3. **Restart Home Assistant**: Restart to load new entities
4. **Add Dashboard Cards**: Use the Lovelace card examples for your dashboard

## MQTT Topics

The integration uses these topics (adjust prefix as needed):

### Call Events
- `fritz/callmonitor/status` - Service status
- `fritz/callmonitor/line/+/status` - Line status  
- `fritz/callmonitor/line/+/last_event` - Last call events
- `fritz/callmonitor/history` - Call history

### Phone Number Management
- `fritz/callmonitor/phone_number/request` - RPC requests
- `fritz/callmonitor/phone_number/response` - RPC responses

## Example Entities

After configuration, you'll have these entities:

### Sensors
- `sensor.fritz_callmonitor_status` - Service availability
- `sensor.fritz_line_1_status` - Line 1 status
- `sensor.fritz_line_1_last_call` - Last call on line 1
- `sensor.fritz_call_history` - Recent call history

### Services
- `script.fritz_set_phone_name` - Set phone number name
- `script.fritz_get_phone_name` - Get phone number info
- `script.fritz_delete_phone_name` - Delete phone number

### Automations
- Call notification on incoming calls
- TTS announcements for caller names
- Log important calls to persistent notification
