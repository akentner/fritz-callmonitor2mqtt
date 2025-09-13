package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"time"

	"fritz-callmonitor2mqtt/internal/database"

	mqtt "github.com/eclipse/paho.mqtt.golang"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Usage: go run test_rpc.go <method> [args...]")
		fmt.Println("Examples:")
		fmt.Println("  go run test_rpc.go set +496181990134 \"Max Mustermann\"")
		fmt.Println("  go run test_rpc.go get +496181990134")
		fmt.Println("  go run test_rpc.go list")
		fmt.Println("  go run test_rpc.go delete +496181990134")
		return
	}

	method := os.Args[1]

	// Create RPC request
	request := database.PhoneNumberRPCRequest{
		ID:        "test-" + time.Now().Format("20060102-150405"),
		Method:    method,
		Timestamp: time.Now(),
	}

	switch method {
	case "set":
		if len(os.Args) < 4 {
			fmt.Println("Usage: go run test_rpc.go set <phone_number> <name>")
			return
		}
		request.PhoneNumber = os.Args[2]
		request.Name = os.Args[3]
	case "get", "delete":
		if len(os.Args) < 3 {
			fmt.Printf("Usage: go run test_rpc.go %s <phone_number>\n", method)
			return
		}
		request.PhoneNumber = os.Args[2]
	case "search":
		if len(os.Args) < 3 {
			fmt.Println("Usage: go run test_rpc.go search <pattern>")
			return
		}
		request.Pattern = os.Args[2]
	case "list":
		// No additional arguments needed
	default:
		fmt.Println("Unknown method:", method)
		return
	}

	// MQTT client options
	opts := mqtt.NewClientOptions()
	opts.AddBroker("tcp://192.168.178.3:1883")
	opts.SetClientID("test-rpc-client-" + time.Now().Format("150405"))

	// Response channel
	responseChan := make(chan database.PhoneNumberRPCResponse, 1)

	// Message handler for responses
	opts.SetDefaultPublishHandler(func(client mqtt.Client, msg mqtt.Message) {
		fmt.Printf("Received response on topic %s: %s\n", msg.Topic(), string(msg.Payload()))

		var response database.PhoneNumberRPCResponse
		if err := json.Unmarshal(msg.Payload(), &response); err != nil {
			log.Printf("Failed to unmarshal response: %v", err)
			return
		}

		if response.ID == request.ID {
			responseChan <- response
		}
	})

	// Create client and connect
	client := mqtt.NewClient(opts)
	if token := client.Connect(); token.Wait() && token.Error() != nil {
		log.Fatalf("Failed to connect to MQTT broker: %v", token.Error())
	}
	defer client.Disconnect(250)

	fmt.Println("Connected to MQTT broker")

	// Subscribe to response topic
	responseTopic := "fritz/callmonitor/dev/phone_number/response"
	if token := client.Subscribe(responseTopic, 0, nil); token.Wait() && token.Error() != nil {
		log.Fatalf("Failed to subscribe to response topic: %v", token.Error())
	}
	fmt.Printf("Subscribed to response topic: %s\n", responseTopic)

	// Wait a moment for subscription to be established
	time.Sleep(500 * time.Millisecond)

	// Marshal request
	requestData, err := json.Marshal(request)
	if err != nil {
		log.Fatalf("Failed to marshal request: %v", err)
	}

	// Publish request
	requestTopic := "fritz/callmonitor/dev/phone_number/request"
	fmt.Printf("Publishing request to topic %s: %s\n", requestTopic, string(requestData))

	if token := client.Publish(requestTopic, 0, false, requestData); token.Wait() && token.Error() != nil {
		log.Fatalf("Failed to publish request: %v", token.Error())
	}

	// Wait for response
	select {
	case response := <-responseChan:
		fmt.Printf("\n=== RPC Response ===\n")
		fmt.Printf("Request ID: %s\n", response.ID)
		fmt.Printf("Success: %t\n", response.Success)
		if response.Error != "" {
			fmt.Printf("Error: %s\n", response.Error)
		}
		if response.PhoneNumber != nil {
			fmt.Printf("Phone Number: %s\n", response.PhoneNumber.PhoneNumber)
			if response.PhoneNumber.Name != nil {
				fmt.Printf("Name: %s\n", *response.PhoneNumber.Name)
			}
		}
		if len(response.PhoneNumbers) > 0 {
			fmt.Printf("Phone Numbers (%d):\n", len(response.PhoneNumbers))
			for i, pn := range response.PhoneNumbers {
				name := "null"
				if pn.Name != nil {
					name = *pn.Name
				}
				fmt.Printf("  %d. %s -> %s\n", i+1, pn.PhoneNumber, name)
			}
		}
		if response.Count > 0 {
			fmt.Printf("Count: %d\n", response.Count)
		}
	case <-time.After(5 * time.Second):
		fmt.Println("\nTimeout waiting for response")
	}
}
