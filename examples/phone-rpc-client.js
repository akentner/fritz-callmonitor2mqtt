#!/usr/bin/env node
/**
 * Phone Number RPC Client Example (Node.js)
 * 
 * This example demonstrates how to interact with the fritz-callmonitor2mqtt
 * phone number RPC API using Node.js and the mqtt library.
 * 
 * Prerequisites:
 *     npm install mqtt
 * 
 * Usage:
 *     node phone-rpc-client.js --help
 *     node phone-rpc-client.js set +1234567890 "John Doe"
 *     node phone-rpc-client.js get +1234567890
 *     node phone-rpc-client.js list
 *     node phone-rpc-client.js search "John"
 *     node phone-rpc-client.js delete +1234567890
 */

const mqtt = require('mqtt');
const { v4: uuidv4 } = require('crypto');

class PhoneNumberRPCClient {
    constructor(options = {}) {
        this.brokerUrl = options.brokerUrl || 'mqtt://localhost:1883';
        this.topicPrefix = options.topicPrefix || 'fritz/callmonitor';
        this.username = options.username;
        this.password = options.password;
        
        this.client = null;
        this.responses = new Map();
        this.connected = false;
    }

    async connect() {
        return new Promise((resolve, reject) => {
            const connectOptions = {
                clientId: `phone-rpc-client-${Date.now()}`,
                clean: true
            };

            if (this.username && this.password) {
                connectOptions.username = this.username;
                connectOptions.password = this.password;
            }

            this.client = mqtt.connect(this.brokerUrl, connectOptions);

            this.client.on('connect', () => {
                this.connected = true;
                console.log(`✅ Connected to MQTT broker ${this.brokerUrl}`);
                
                // Subscribe to response topic
                const responseTopic = `${this.topicPrefix}/phone_number/response`;
                this.client.subscribe(responseTopic, { qos: 1 }, (err) => {
                    if (err) {
                        console.error(`❌ Failed to subscribe to ${responseTopic}:`, err);
                        reject(err);
                    } else {
                        console.log(`📡 Subscribed to ${responseTopic}`);
                        resolve();
                    }
                });
            });

            this.client.on('error', (err) => {
                console.error('❌ MQTT connection error:', err);
                reject(err);
            });

            this.client.on('message', (topic, message) => {
                try {
                    const response = JSON.parse(message.toString());
                    const requestId = response.id;
                    if (requestId) {
                        this.responses.set(requestId, response);
                    }
                } catch (err) {
                    console.error('❌ Failed to parse MQTT response:', err);
                }
            });

            this.client.on('close', () => {
                this.connected = false;
                console.log('📡 MQTT connection closed');
            });

            // Set timeout for connection
            setTimeout(() => {
                if (!this.connected) {
                    reject(new Error('Connection timeout'));
                }
            }, 10000);
        });
    }

    disconnect() {
        if (this.client && this.connected) {
            this.client.end(true);
        }
    }

    async sendRPCRequest(method, options = {}) {
        if (!this.connected) {
            throw new Error('Not connected to MQTT broker');
        }

        const requestId = uuidv4();
        const request = {
            id: requestId,
            method: method,
            timestamp: new Date().toISOString(),
            ...options
        };

        // Publish request
        const requestTopic = `${this.topicPrefix}/phone_number/request`;
        const payload = JSON.stringify(request);

        console.log(`📤 Sending ${method} request...`);
        
        return new Promise((resolve, reject) => {
            this.client.publish(requestTopic, payload, { qos: 1 }, (err) => {
                if (err) {
                    reject(new Error(`Failed to publish request: ${err.message}`));
                    return;
                }

                // Wait for response
                const timeout = setTimeout(() => {
                    this.responses.delete(requestId);
                    reject(new Error('Timeout waiting for response'));
                }, 10000);

                const checkResponse = () => {
                    if (this.responses.has(requestId)) {
                        const response = this.responses.get(requestId);
                        this.responses.delete(requestId);
                        clearTimeout(timeout);
                        resolve(response);
                    } else {
                        setTimeout(checkResponse, 100);
                    }
                };

                checkResponse();
            });
        });
    }

    async setPhoneNumber(phoneNumber, name) {
        try {
            const response = await this.sendRPCRequest('set', {
                phone_number: phoneNumber,
                name: name
            });

            if (response.success) {
                const pn = response.phone_number;
                console.log(`✅ Set ${pn.phone_number} → ${pn.name}`);
                return true;
            } else {
                console.error(`❌ Failed to set phone number: ${response.error}`);
                return false;
            }
        } catch (err) {
            console.error(`❌ Error: ${err.message}`);
            return false;
        }
    }

    async getPhoneNumber(phoneNumber) {
        try {
            const response = await this.sendRPCRequest('get', {
                phone_number: phoneNumber
            });

            if (response.success) {
                const pn = response.phone_number;
                console.log(`✅ Found: ${pn.phone_number} → ${pn.name}`);
                if (pn.created_at) console.log(`   Created: ${pn.created_at}`);
                if (pn.updated_at) console.log(`   Updated: ${pn.updated_at}`);
                return pn;
            } else {
                console.error(`❌ Phone number not found: ${response.error}`);
                return null;
            }
        } catch (err) {
            console.error(`❌ Error: ${err.message}`);
            return null;
        }
    }

    async deletePhoneNumber(phoneNumber) {
        try {
            const response = await this.sendRPCRequest('delete', {
                phone_number: phoneNumber
            });

            if (response.success) {
                console.log(`✅ Deleted ${phoneNumber}`);
                return true;
            } else {
                console.error(`❌ Failed to delete phone number: ${response.error}`);
                return false;
            }
        } catch (err) {
            console.error(`❌ Error: ${err.message}`);
            return false;
        }
    }

    async listPhoneNumbers(limit = 100) {
        try {
            const response = await this.sendRPCRequest('list', { limit });

            if (response.success) {
                const phoneNumbers = response.phone_numbers || [];
                const count = response.count || 0;

                console.log(`✅ Found ${count} phone numbers:`);
                phoneNumbers.forEach((pn, index) => {
                    const name = pn.name || '(no name)';
                    console.log(`   ${(index + 1).toString().padStart(2)}. ${pn.phone_number} → ${name}`);
                });

                return phoneNumbers;
            } else {
                console.error(`❌ Failed to list phone numbers: ${response.error}`);
                return null;
            }
        } catch (err) {
            console.error(`❌ Error: ${err.message}`);
            return null;
        }
    }

    async searchPhoneNumbers(pattern, limit = 100) {
        try {
            const response = await this.sendRPCRequest('search', { pattern, limit });

            if (response.success) {
                const phoneNumbers = response.phone_numbers || [];
                const count = response.count || 0;

                console.log(`✅ Found ${count} phone numbers matching '${pattern}':`);
                phoneNumbers.forEach((pn, index) => {
                    const name = pn.name || '(no name)';
                    console.log(`   ${(index + 1).toString().padStart(2)}. ${pn.phone_number} → ${name}`);
                });

                return phoneNumbers;
            } else {
                console.error(`❌ Failed to search phone numbers: ${response.error}`);
                return null;
            }
        } catch (err) {
            console.error(`❌ Error: ${err.message}`);
            return null;
        }
    }
}

// CLI handling
async function main() {
    const args = process.argv.slice(2);
    
    if (args.length === 0 || args[0] === '--help') {
        console.log(`
Phone Number RPC Client (Node.js)

Usage:
    node phone-rpc-client.js <command> [args...]

Commands:
    set <phone_number> <name>     Set phone number with name
    get <phone_number>            Get phone number information  
    delete <phone_number>         Delete phone number
    list [--limit N]              List all phone numbers
    search <pattern> [--limit N]  Search phone numbers by name

Options:
    --broker <url>                MQTT broker URL (default: mqtt://localhost:1883)
    --prefix <prefix>             MQTT topic prefix (default: fritz/callmonitor)
    --username <username>         MQTT username
    --password <password>         MQTT password
    --limit <N>                   Limit results (default: 100)

Examples:
    node phone-rpc-client.js set +1234567890 "John Doe"
    node phone-rpc-client.js get +1234567890
    node phone-rpc-client.js list
    node phone-rpc-client.js search "John"
    node phone-rpc-client.js delete +1234567890
        `);
        return;
    }

    // Parse command line options
    const options = {
        brokerUrl: 'mqtt://localhost:1883',
        topicPrefix: 'fritz/callmonitor',
        limit: 100
    };

    let command = null;
    let commandArgs = [];

    for (let i = 0; i < args.length; i++) {
        const arg = args[i];
        if (arg.startsWith('--')) {
            const option = arg.substring(2);
            const value = args[i + 1];
            if (value && !value.startsWith('--')) {
                options[option] = value;
                i++; // Skip value
            }
        } else if (!command) {
            command = arg;
        } else {
            commandArgs.push(arg);
        }
    }

    if (!command) {
        console.error('❌ No command specified');
        return;
    }

    const client = new PhoneNumberRPCClient(options);

    try {
        await client.connect();

        switch (command) {
            case 'set':
                if (commandArgs.length < 2) {
                    console.error('❌ Usage: set <phone_number> <name>');
                    return;
                }
                await client.setPhoneNumber(commandArgs[0], commandArgs[1]);
                break;

            case 'get':
                if (commandArgs.length < 1) {
                    console.error('❌ Usage: get <phone_number>');
                    return;
                }
                await client.getPhoneNumber(commandArgs[0]);
                break;

            case 'delete':
                if (commandArgs.length < 1) {
                    console.error('❌ Usage: delete <phone_number>');
                    return;
                }
                await client.deletePhoneNumber(commandArgs[0]);
                break;

            case 'list':
                await client.listPhoneNumbers(options.limit);
                break;

            case 'search':
                if (commandArgs.length < 1) {
                    console.error('❌ Usage: search <pattern>');
                    return;
                }
                await client.searchPhoneNumbers(commandArgs[0], options.limit);
                break;

            default:
                console.error(`❌ Unknown command: ${command}`);
        }

    } catch (err) {
        console.error('❌ Error:', err.message);
    } finally {
        client.disconnect();
        // Give some time for graceful disconnect
        setTimeout(() => process.exit(0), 500);
    }
}

// Handle uncaught errors
process.on('uncaughtException', (err) => {
    console.error('❌ Uncaught Exception:', err);
    process.exit(1);
});

process.on('unhandledRejection', (reason, promise) => {
    console.error('❌ Unhandled Rejection at:', promise, 'reason:', reason);
    process.exit(1);
});

if (require.main === module) {
    main();
}

module.exports = PhoneNumberRPCClient;
