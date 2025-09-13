package main

import (
	"encoding/json"
	"fmt"
	"log"
	"time"

	"fritz-callmonitor2mqtt/internal/database"
)

func main() {
	// Connect to database
	dbClient, err := database.NewClient("/workspaces/fritz-callmonitor2mqtt/data/database")
	if err != nil {
		log.Fatalf("Failed to create database client: %v", err)
	}

	if err := dbClient.Connect(); err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer dbClient.Close()

	fmt.Println("=== Testing Database Phone Number Operations ===\n")

	// Test 1: Insert a phone number
	fmt.Println("1. Setting phone number +496181990134 to 'Max Mustermann'")
	err = dbClient.InsertPhoneNumber("+496181990134", "Max Mustermann")
	if err != nil {
		fmt.Printf("Error: %v\n", err)
	} else {
		fmt.Println("✅ Success")
	}

	// Test 2: Get the phone number
	fmt.Println("\n2. Getting phone number +496181990134")
	phoneNumber, err := dbClient.GetPhoneNumber("+496181990134")
	if err != nil {
		fmt.Printf("Error: %v\n", err)
	} else {
		fmt.Printf("✅ Found: %s -> %s\n", phoneNumber.PhoneNumber, *phoneNumber.Name)
	}

	// Test 3: Get phone number name (optimized lookup)
	fmt.Println("\n3. Getting name for phone number +496181990134")
	name, err := dbClient.GetPhoneNumberName("+496181990134")
	if err != nil {
		fmt.Printf("Error: %v\n", err)
	} else if name != nil {
		fmt.Printf("✅ Name: %s\n", *name)
	} else {
		fmt.Println("No name found")
	}

	// Test 4: Insert another phone number
	fmt.Println("\n4. Setting phone number +491783278576 to 'Anna Schmidt'")
	err = dbClient.InsertPhoneNumber("+491783278576", "Anna Schmidt")
	if err != nil {
		fmt.Printf("Error: %v\n", err)
	} else {
		fmt.Println("✅ Success")
	}

	// Test 5: List all phone numbers
	fmt.Println("\n5. Listing all phone numbers")
	phoneNumbers, err := dbClient.GetAllPhoneNumbers(10, 0)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
	} else {
		fmt.Printf("✅ Found %d phone numbers:\n", len(phoneNumbers))
		for i, pn := range phoneNumbers {
			name := "null"
			if pn.Name != nil {
				name = *pn.Name
			}
			fmt.Printf("  %d. %s -> %s\n", i+1, pn.PhoneNumber, name)
		}
	}

	// Test 6: Search for phone numbers
	fmt.Println("\n6. Searching for phone numbers with pattern 'Max'")
	phoneNumbers, err = dbClient.SearchPhoneNumbersByName("Max", 10, 0)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
	} else {
		fmt.Printf("✅ Found %d matching phone numbers:\n", len(phoneNumbers))
		for i, pn := range phoneNumbers {
			name := "null"
			if pn.Name != nil {
				name = *pn.Name
			}
			fmt.Printf("  %d. %s -> %s\n", i+1, pn.PhoneNumber, name)
		}
	}

	// Test 7: RPC Processing
	fmt.Println("\n7. Testing RPC Processing")

	// Test RPC list operation
	listRequest := &database.PhoneNumberRPCRequest{
		ID:        "test-list-001",
		Method:    "list",
		Timestamp: time.Now(),
	}

	response := dbClient.ProcessPhoneNumberRPC(listRequest)
	fmt.Printf("✅ RPC List Response:\n")
	responseJSON, _ := json.MarshalIndent(response, "", "  ")
	fmt.Println(string(responseJSON))

	// Test RPC get operation
	fmt.Println("\n8. Testing RPC Get Operation")
	getRequest := &database.PhoneNumberRPCRequest{
		ID:          "test-get-001",
		Method:      "get",
		PhoneNumber: "+496181990134",
		Timestamp:   time.Now(),
	}

	response = dbClient.ProcessPhoneNumberRPC(getRequest)
	fmt.Printf("✅ RPC Get Response:\n")
	responseJSON, _ = json.MarshalIndent(response, "", "  ")
	fmt.Println(string(responseJSON))

	fmt.Println("\n=== Database Tests Completed ===")
}
