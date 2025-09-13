#!/bin/bash

# Test script to demonstrate FSM debug topics are only published at debug log level

echo "=== FSM Debug Topics Log Level Test ==="
echo

echo "Testing with LOG_LEVEL=info (FSM topics should NOT be published):"
echo "FRITZ_CALLMONITOR_APP_LOG_LEVEL=info"
echo

echo "Testing with LOG_LEVEL=debug (FSM topics SHOULD be published):"
echo "FRITZ_CALLMONITOR_APP_LOG_LEVEL=debug"
echo

echo "To test manually:"
echo "1. Set FRITZ_CALLMONITOR_APP_LOG_LEVEL=info and monitor MQTT topics - no /fsm/ topics"
echo "2. Set FRITZ_CALLMONITOR_APP_LOG_LEVEL=debug and monitor MQTT topics - /fsm/ topics will appear"
echo
echo "Example MQTT topics published in debug mode:"
echo "  - fritz/callmonitor/fsm/line/1/status_change"
echo "  - fritz/callmonitor/fsm/line/1/status"
echo
echo "These topics are NOT published in info/warn/error log levels."
