#!/usr/bin/env python3
"""
Test script for Phone Number RPC functionality
"""

import json
import time
import uuid
import paho.mqtt.client as mqtt
from datetime import datetime

# MQTT configuration
MQTT_BROKER = "192.168.178.3"
MQTT_PORT = 1883
TOPIC_PREFIX = "fritz/callmonitor/dev"

class PhoneRPCTester:
    def __init__(self):
        self.client = mqtt.Client()
        self.client.on_connect = self.on_connect
        self.client.on_message = self.on_message
        self.responses = {}
        
    def on_connect(self, client, userdata, flags, rc):
        print(f"Connected to MQTT broker with result code {rc}")
        # Subscribe to response topic
        response_topic = f"{TOPIC_PREFIX}/phone_number/response"
        client.subscribe(response_topic)
        print(f"Subscribed to {response_topic}")
    
    def on_message(self, client, userdata, msg):
        print(f"Received response: {msg.payload.decode()}")
        try:
            response = json.loads(msg.payload.decode())
            request_id = response.get('id')
            if request_id:
                self.responses[request_id] = response
        except json.JSONDecodeError as e:
            print(f"Failed to parse response: {e}")
    
    def send_rpc_request(self, method, **kwargs):
        request_id = str(uuid.uuid4())
        request = {
            "id": request_id,
            "method": method,
            "timestamp": datetime.utcnow().isoformat() + "Z",
            **kwargs
        }
        
        request_topic = f"{TOPIC_PREFIX}/phone_number/request"
        payload = json.dumps(request)
        
        print(f"Sending RPC request: {payload}")
        self.client.publish(request_topic, payload)
        
        # Wait for response
        timeout = 5  # 5 seconds timeout
        start_time = time.time()
        while request_id not in self.responses and (time.time() - start_time) < timeout:
            self.client.loop(0.1)
        
        if request_id in self.responses:
            response = self.responses.pop(request_id)
            print(f"Response: {json.dumps(response, indent=2)}")
            return response
        else:
            print("Timeout waiting for response")
            return None
    
    def test_phone_number_operations(self):
        # Connect to MQTT broker
        self.client.connect(MQTT_BROKER, MQTT_PORT, 60)
        self.client.loop_start()
        
        # Wait a bit for connection
        time.sleep(1)
        
        print("\n=== Testing Phone Number RPC Operations ===\n")
        
        # Test 1: Set a phone number
        print("1. Setting phone number +496181990134 to 'Max Mustermann'")
        response = self.send_rpc_request(
            method="set",
            phone_number="+496181990134", 
            name="Max Mustermann"
        )
        
        # Test 2: Get the phone number
        print("\n2. Getting phone number +496181990134")
        response = self.send_rpc_request(
            method="get",
            phone_number="+496181990134"
        )
        
        # Test 3: Set another phone number 
        print("\n3. Setting phone number +491783278576 to 'Anna Schmidt'")
        response = self.send_rpc_request(
            method="set",
            phone_number="+491783278576",
            name="Anna Schmidt"
        )
        
        # Test 4: List all phone numbers
        print("\n4. Listing all phone numbers")
        response = self.send_rpc_request(method="list")
        
        # Test 5: Search for phone numbers
        print("\n5. Searching for phone numbers containing 'Max'")
        response = self.send_rpc_request(
            method="search",
            pattern="Max"
        )
        
        # Test 6: Delete a phone number
        print("\n6. Deleting phone number +491783278576")
        response = self.send_rpc_request(
            method="delete",
            phone_number="+491783278576"
        )
        
        # Test 7: List all phone numbers again
        print("\n7. Listing all phone numbers after deletion")
        response = self.send_rpc_request(method="list")
        
        self.client.loop_stop()
        self.client.disconnect()

if __name__ == "__main__":
    tester = PhoneRPCTester()
    tester.test_phone_number_operations()
