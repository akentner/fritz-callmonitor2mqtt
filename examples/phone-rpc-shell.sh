#!/bin/bash

# Phone Number RPC Shell Script Examples
# 
# This script demonstrates how to interact with the fritz-callmonitor2mqtt
# phone number RPC API using mosquitto_pub and mosquitto_sub command line tools.
#
# Prerequisites:
#   - mosquitto-clients package installed
#   - jq for JSON processing (optional, for pretty output)
#
# Configuration
BROKER_HOST="${MQTT_BROKER_HOST:-localhost}"
BROKER_PORT="${MQTT_BROKER_PORT:-1883}"
TOPIC_PREFIX="${MQTT_TOPIC_PREFIX:-fritz/callmonitor}"
REQUEST_TOPIC="${TOPIC_PREFIX}/phone_number/request"
RESPONSE_TOPIC="${TOPIC_PREFIX}/phone_number/response"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Check dependencies
check_dependencies() {
    if ! command -v mosquitto_pub &> /dev/null; then
        echo -e "${RED}❌ mosquitto_pub not found. Please install mosquitto-clients${NC}"
        echo "Ubuntu/Debian: apt install mosquitto-clients"
        echo "macOS: brew install mosquitto"
        exit 1
    fi
    
    if ! command -v mosquitto_sub &> /dev/null; then
        echo -e "${RED}❌ mosquitto_sub not found. Please install mosquitto-clients${NC}"
        exit 1
    fi
}

# Generate unique request ID
generate_request_id() {
    echo "req-$(date +%s)-$$"
}

# Get current timestamp in ISO format
get_timestamp() {
    date -u +"%Y-%m-%dT%H:%M:%SZ"
}

# Send RPC request and wait for response
send_rpc_request() {
    local method="$1"
    local phone_number="$2"
    local name="$3"
    local pattern="$4"
    local limit="${5:-100}"
    
    local request_id=$(generate_request_id)
    local timestamp=$(get_timestamp)
    
    # Build JSON request
    local request="{"
    request="\"id\":\"${request_id}\","
    request="${request}\"method\":\"${method}\","
    request="${request}\"timestamp\":\"${timestamp}\""
    
    if [[ -n "$phone_number" ]]; then
        request="${request},\"phone_number\":\"${phone_number}\""
    fi
    
    if [[ -n "$name" ]]; then
        request="${request},\"name\":\"${name}\""
    fi
    
    if [[ -n "$pattern" ]]; then
        request="${request},\"pattern\":\"${pattern}\""
    fi
    
    if [[ "$limit" != "100" ]]; then
        request="${request},\"limit\":${limit}"
    fi
    
    request="${request}}"
    
    echo -e "${BLUE}📤 Sending ${method} request...${NC}"
    echo -e "${YELLOW}Request: ${request}${NC}"
    
    # Start response listener in background
    local response_file=$(mktemp)
    mosquitto_sub -h "$BROKER_HOST" -p "$BROKER_PORT" -t "$RESPONSE_TOPIC" -C 1 > "$response_file" &
    local sub_pid=$!
    
    # Small delay to ensure subscription is active
    sleep 0.5
    
    # Send request
    echo "$request" | mosquitto_pub -h "$BROKER_HOST" -p "$BROKER_PORT" -t "$REQUEST_TOPIC" -s
    
    # Wait for response
    local timeout=10
    local count=0
    while [[ $count -lt $timeout ]] && kill -0 $sub_pid 2>/dev/null; do
        if [[ -s "$response_file" ]]; then
            break
        fi
        sleep 1
        ((count++))
    done
    
    # Kill subscriber if still running
    if kill -0 $sub_pid 2>/dev/null; then
        kill $sub_pid 2>/dev/null
    fi
    
    # Check response
    if [[ -s "$response_file" ]]; then
        local response=$(cat "$response_file")
        echo -e "${GREEN}📥 Response received:${NC}"
        
        # Pretty print JSON if jq is available
        if command -v jq &> /dev/null; then
            echo "$response" | jq .
        else
            echo "$response"
        fi
        
        # Extract success status
        local success
        if command -v jq &> /dev/null; then
            success=$(echo "$response" | jq -r '.success // false')
        else
            success=$(echo "$response" | grep -o '"success":[^,]*' | cut -d: -f2 | tr -d ' ')
        fi
        
        if [[ "$success" == "true" ]]; then
            echo -e "${GREEN}✅ Operation successful${NC}"
        else
            echo -e "${RED}❌ Operation failed${NC}"
        fi
    else
        echo -e "${RED}⏰ No response received within ${timeout} seconds${NC}"
    fi
    
    rm -f "$response_file"
}

# Command functions
cmd_set() {
    if [[ $# -lt 2 ]]; then
        echo -e "${RED}Usage: $0 set <phone_number> <name>${NC}"
        echo "Example: $0 set +1234567890 \"John Doe\""
        exit 1
    fi
    
    local phone_number="$1"
    local name="$2"
    
    echo -e "${BLUE}Setting phone number ${phone_number} to '${name}'${NC}"
    send_rpc_request "set" "$phone_number" "$name"
}

cmd_get() {
    if [[ $# -lt 1 ]]; then
        echo -e "${RED}Usage: $0 get <phone_number>${NC}"
        echo "Example: $0 get +1234567890"
        exit 1
    fi
    
    local phone_number="$1"
    
    echo -e "${BLUE}Getting information for phone number ${phone_number}${NC}"
    send_rpc_request "get" "$phone_number"
}

cmd_delete() {
    if [[ $# -lt 1 ]]; then
        echo -e "${RED}Usage: $0 delete <phone_number>${NC}"
        echo "Example: $0 delete +1234567890"
        exit 1
    fi
    
    local phone_number="$1"
    
    echo -e "${BLUE}Deleting phone number ${phone_number}${NC}"
    send_rpc_request "delete" "$phone_number"
}

cmd_list() {
    local limit="${1:-100}"
    
    echo -e "${BLUE}Listing all phone numbers (limit: ${limit})${NC}"
    send_rpc_request "list" "" "" "" "$limit"
}

cmd_search() {
    if [[ $# -lt 1 ]]; then
        echo -e "${RED}Usage: $0 search <pattern> [limit]${NC}"
        echo "Example: $0 search \"Max\" 50"
        exit 1
    fi
    
    local pattern="$1"
    local limit="${2:-100}"
    
    echo -e "${BLUE}Searching phone numbers for pattern '${pattern}' (limit: ${limit})${NC}"
    send_rpc_request "search" "" "" "$pattern" "$limit"
}

cmd_monitor() {
    echo -e "${BLUE}Monitoring RPC responses (Press Ctrl+C to stop)${NC}"
    echo -e "${YELLOW}Topic: ${RESPONSE_TOPIC}${NC}"
    
    mosquitto_sub -h "$BROKER_HOST" -p "$BROKER_PORT" -t "$RESPONSE_TOPIC" | while read -r line; do
        echo -e "${GREEN}📥 $(date '+%H:%M:%S'):${NC}"
        if command -v jq &> /dev/null; then
            echo "$line" | jq .
        else
            echo "$line"
        fi
        echo
    done
}

# Usage function
usage() {
    cat << EOF
Phone Number RPC Shell Client

Usage: $0 [OPTIONS] <command> [args...]

Commands:
    set <phone_number> <name>     Set phone number with name
    get <phone_number>            Get phone number information
    delete <phone_number>         Delete phone number  
    list [limit]                  List all phone numbers
    search <pattern> [limit]      Search phone numbers by name
    monitor                       Monitor all RPC responses

Environment Variables:
    MQTT_BROKER_HOST             MQTT broker host (default: localhost)
    MQTT_BROKER_PORT             MQTT broker port (default: 1883)  
    MQTT_TOPIC_PREFIX            MQTT topic prefix (default: fritz/callmonitor)

Examples:
    $0 set +1234567890 "John Doe"
    $0 get +1234567890
    $0 list 50
    $0 search "John" 10
    $0 delete +1234567890
    $0 monitor

Dependencies:
    - mosquitto-clients (mosquitto_pub, mosquitto_sub)
    - jq (optional, for pretty JSON output)

EOF
}

# Main execution
main() {
    check_dependencies
    
    if [[ $# -eq 0 ]] || [[ "$1" == "--help" ]] || [[ "$1" == "-h" ]]; then
        usage
        exit 0
    fi
    
    local command="$1"
    shift
    
    case "$command" in
        "set")
            cmd_set "$@"
            ;;
        "get")
            cmd_get "$@"
            ;;
        "delete")
            cmd_delete "$@"
            ;;
        "list")
            cmd_list "$@"
            ;;
        "search")
            cmd_search "$@"
            ;;
        "monitor")
            cmd_monitor
            ;;
        *)
            echo -e "${RED}❌ Unknown command: $command${NC}"
            echo
            usage
            exit 1
            ;;
    esac
}

# Run main function if script is executed directly
if [[ "${BASH_SOURCE[0]}" == "${0}" ]]; then
    main "$@"
fi
