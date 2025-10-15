# FRITZ!Box Callmonitor Home Assistant Integration

This directory contains examples for integrating fritz-callmonitor2mqtt with Home Assistant.

## Features

- **Call Event Sensors**: Track incoming/outgoing calls
- **Phone Number Management**: Manage contact names via Home Assistant
- **Call Status Display**: Show current line status and last calls
- **Automation Examples**: Trigger actions based on call events

## Files  

### Core Configuration
- `configuration.yaml` - Complete Home Assistant configuration example
- `sensors.yaml` - MQTT sensors for call monitoring  
- `automations.yaml` - Example automations for call events

### Phone Number RPC Integration
- `scripts.yaml` - Phone Number RPC scripts for contact management
- `phone-rpc-automations.yaml` - Advanced automations for RPC responses
- `sensors.yaml` - RPC response sensors and contact statistics
- `inputs.yaml` - Input fields for manual contact management
- `lovelace-phone-rpc.yaml` - Dashboard card for phone number management
- `complete-example.yaml` - Complete setup examples and use cases

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

### Core Sensors
- `sensor.fritz_callmonitor_status` - Service availability
- `sensor.fritz_line_1_status` - Line 1 status
- `sensor.fritz_line_1_last_call` - Last call on line 1
- `sensor.fritz_call_history` - Recent call history

### Phone Number RPC Entities
- `sensor.fritz_phone_rpc_response` - Last RPC response status
- `sensor.fritz_phone_database_stats` - Contact database statistics
- `sensor.fritz_last_unknown_caller` - Detection of unknown callers
- `sensor.fritz_contact_resolution_rate` - Percentage of resolved contacts
- `binary_sensor.fritz_phone_rpc_available` - RPC service availability
- `binary_sensor.fritz_has_unknown_caller` - Unknown caller detection
- `binary_sensor.fritz_low_contact_resolution` - Low resolution rate alert

### Scripts for Contact Management
- `script.fritz_set_phone_name` - Add/update contact
- `script.fritz_get_phone_name` - Retrieve contact info
- `script.fritz_delete_phone_name` - Delete contact
- `script.fritz_list_phone_numbers` - List all contacts
- `script.fritz_search_phone_numbers` - Search contacts by name
- `script.fritz_add_last_unknown_caller` - Quick add unknown caller
- `script.fritz_bulk_import_contacts` - Bulk import from text

### Services
- `script.fritz_set_phone_name` - Set phone number name
- `script.fritz_get_phone_name` - Get phone number info
- `script.fritz_delete_phone_name` - Delete phone number

### Automations
- Call notification on incoming calls
- TTS announcements for caller names
- Log important calls to persistent notification
