#!/usr/bin/env python3
"""
Phone Number RPC Client Example

This example demonstrates how to interact with the fritz-callmonitor2mqtt
phone number RPC API using Python and the paho-mqtt library.

Prerequisites:
    pip install paho-mqtt

Usage:
    python3 phone-rpc-client.py --help
    python3 phone-rpc-client.py set +1234567890 "John Doe"
    python3 phone-rpc-client.py get +1234567890
    python3 phone-rpc-client.py list
    python3 phone-rpc-client.py search "John"
    python3 phone-rpc-client.py delete +1234567890
"""

import json
import time
import uuid
import argparse
import sys
from datetime import datetime
from typing import Optional, Dict, Any

try:
    import paho.mqtt.client as mqtt
except ImportError:
    print("Error: paho-mqtt not installed. Run: pip install paho-mqtt")
    sys.exit(1)

class PhoneNumberRPCClient:
    def __init__(self, broker_host: str = "localhost", broker_port: int = 1883, 
                 topic_prefix: str = "fritz/callmonitor", username: str = None, 
                 password: str = None):
        self.broker_host = broker_host
        self.broker_port = broker_port
        self.topic_prefix = topic_prefix
        self.username = username
        self.password = password
        
        self.client = mqtt.Client()
        self.responses = {}
        self.connected = False
        
        # Setup callbacks
        self.client.on_connect = self._on_connect
        self.client.on_message = self._on_message
        self.client.on_disconnect = self._on_disconnect
        
        # Setup authentication if provided
        if username and password:
            self.client.username_pw_set(username, password)
    
    def _on_connect(self, client, userdata, flags, rc):
        if rc == 0:
            self.connected = True
            print(f"✅ Connected to MQTT broker {self.broker_host}:{self.broker_port}")
            # Subscribe to response topic
            response_topic = f"{self.topic_prefix}/phone_number/response"
            client.subscribe(response_topic, qos=1)
            print(f"📡 Subscribed to {response_topic}")
        else:
            print(f"❌ Failed to connect to MQTT broker. Return code: {rc}")
    
    def _on_disconnect(self, client, userdata, rc):
        self.connected = False
        if rc != 0:
            print("⚠️ Unexpected MQTT disconnection")
    
    def _on_message(self, client, userdata, msg):
        try:
            response = json.loads(msg.payload.decode())
            request_id = response.get('id')
            if request_id:
                self.responses[request_id] = response
        except json.JSONDecodeError as e:
            print(f"❌ Failed to parse MQTT response: {e}")
    
    def connect(self) -> bool:
        """Connect to MQTT broker"""
        try:
            self.client.connect(self.broker_host, self.broker_port, 60)
            self.client.loop_start()
            
            # Wait for connection
            timeout = 10
            start_time = time.time()
            while not self.connected and (time.time() - start_time) < timeout:
                time.sleep(0.1)
            
            return self.connected
        except Exception as e:
            print(f"❌ Connection failed: {e}")
            return False
    
    def disconnect(self):
        """Disconnect from MQTT broker"""
        if self.connected:
            self.client.loop_stop()
            self.client.disconnect()
    
    def _send_rpc_request(self, method: str, **kwargs) -> Optional[Dict[str, Any]]:
        """Send RPC request and wait for response"""
        if not self.connected:
            print("❌ Not connected to MQTT broker")
            return None
        
        request_id = str(uuid.uuid4())
        request = {
            "id": request_id,
            "method": method,
            "timestamp": datetime.utcnow().isoformat() + "Z",
            **kwargs
        }
        
        # Publish request
        request_topic = f"{self.topic_prefix}/phone_number/request"
        payload = json.dumps(request)
        
        print(f"📤 Sending {method} request...")
        result = self.client.publish(request_topic, payload, qos=1)
        
        if result.rc != mqtt.MQTT_ERR_SUCCESS:
            print(f"❌ Failed to publish request: {result.rc}")
            return None
        
        # Wait for response
        timeout = 10
        start_time = time.time()
        while request_id not in self.responses and (time.time() - start_time) < timeout:
            time.sleep(0.1)
        
        if request_id in self.responses:
            return self.responses.pop(request_id)
        else:
            print("⏰ Timeout waiting for response")
            return None
    
    def set_phone_number(self, phone_number: str, name: str) -> bool:
        """Set/update a phone number with name"""
        response = self._send_rpc_request("set", phone_number=phone_number, name=name)
        
        if response and response.get('success'):
            pn = response.get('phone_number', {})
            print(f"✅ Set {pn.get('phone_number')} → {pn.get('name')}")
            return True
        else:
            error = response.get('error', 'Unknown error') if response else 'No response'
            print(f"❌ Failed to set phone number: {error}")
            return False
    
    def get_phone_number(self, phone_number: str) -> Optional[Dict[str, Any]]:
        """Get information for a phone number"""
        response = self._send_rpc_request("get", phone_number=phone_number)
        
        if response and response.get('success'):
            pn = response.get('phone_number', {})
            print(f"✅ Found: {pn.get('phone_number')} → {pn.get('name')}")
            if pn.get('created_at'):
                print(f"   Created: {pn.get('created_at')}")
            if pn.get('updated_at'):
                print(f"   Updated: {pn.get('updated_at')}")
            return pn
        else:
            error = response.get('error', 'Unknown error') if response else 'No response'
            print(f"❌ Phone number not found: {error}")
            return None
    
    def delete_phone_number(self, phone_number: str) -> bool:
        """Delete a phone number"""
        response = self._send_rpc_request("delete", phone_number=phone_number)
        
        if response and response.get('success'):
            print(f"✅ Deleted {phone_number}")
            return True
        else:
            error = response.get('error', 'Unknown error') if response else 'No response'
            print(f"❌ Failed to delete phone number: {error}")
            return False
    
    def list_phone_numbers(self, limit: int = 100) -> Optional[list]:
        """List all phone numbers"""
        response = self._send_rpc_request("list", limit=limit)
        
        if response and response.get('success'):
            phone_numbers = response.get('phone_numbers', [])
            count = response.get('count', 0)
            
            print(f"✅ Found {count} phone numbers:")
            for i, pn in enumerate(phone_numbers, 1):
                name = pn.get('name') or '(no name)'
                print(f"   {i:2d}. {pn.get('phone_number')} → {name}")
            
            return phone_numbers
        else:
            error = response.get('error', 'Unknown error') if response else 'No response'
            print(f"❌ Failed to list phone numbers: {error}")
            return None
    
    def search_phone_numbers(self, pattern: str, limit: int = 100) -> Optional[list]:
        """Search phone numbers by name pattern"""
        response = self._send_rpc_request("search", pattern=pattern, limit=limit)
        
        if response and response.get('success'):
            phone_numbers = response.get('phone_numbers', [])
            count = response.get('count', 0)
            
            print(f"✅ Found {count} phone numbers matching '{pattern}':")
            for i, pn in enumerate(phone_numbers, 1):
                name = pn.get('name') or '(no name)'
                print(f"   {i:2d}. {pn.get('phone_number')} → {name}")
            
            return phone_numbers
        else:
            error = response.get('error', 'Unknown error') if response else 'No response'
            print(f"❌ Failed to search phone numbers: {error}")
            return None

def main():
    parser = argparse.ArgumentParser(description='Phone Number RPC Client')
    parser.add_argument('--broker', default='localhost', help='MQTT broker host')
    parser.add_argument('--port', type=int, default=1883, help='MQTT broker port')
    parser.add_argument('--prefix', default='fritz/callmonitor', help='MQTT topic prefix')
    parser.add_argument('--username', help='MQTT username')
    parser.add_argument('--password', help='MQTT password')
    
    subparsers = parser.add_subparsers(dest='command', help='Available commands')
    
    # Set command
    set_parser = subparsers.add_parser('set', help='Set phone number with name')
    set_parser.add_argument('phone_number', help='Phone number (e.g., +1234567890)')
    set_parser.add_argument('name', help='Contact name (e.g., "John Doe")')
    
    # Get command
    get_parser = subparsers.add_parser('get', help='Get phone number information')
    get_parser.add_argument('phone_number', help='Phone number to lookup')
    
    # Delete command
    delete_parser = subparsers.add_parser('delete', help='Delete phone number')
    delete_parser.add_argument('phone_number', help='Phone number to delete')
    
    # List command
    list_parser = subparsers.add_parser('list', help='List all phone numbers')
    list_parser.add_argument('--limit', type=int, default=100, help='Maximum results')
    
    # Search command
    search_parser = subparsers.add_parser('search', help='Search phone numbers by name')
    search_parser.add_argument('pattern', help='Search pattern')
    search_parser.add_argument('--limit', type=int, default=100, help='Maximum results')
    
    args = parser.parse_args()
    
    if not args.command:
        parser.print_help()
        return
    
    # Create and connect client
    client = PhoneNumberRPCClient(
        broker_host=args.broker,
        broker_port=args.port,
        topic_prefix=args.prefix,
        username=args.username,
        password=args.password
    )
    
    if not client.connect():
        print("❌ Failed to connect to MQTT broker")
        return
    
    try:
        # Execute command
        if args.command == 'set':
            client.set_phone_number(args.phone_number, args.name)
        elif args.command == 'get':
            client.get_phone_number(args.phone_number)
        elif args.command == 'delete':
            client.delete_phone_number(args.phone_number)
        elif args.command == 'list':
            client.list_phone_numbers(args.limit)
        elif args.command == 'search':
            client.search_phone_numbers(args.pattern, args.limit)
    
    finally:
        client.disconnect()

if __name__ == '__main__':
    main()
