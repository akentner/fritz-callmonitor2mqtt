# Republish RPC API

Die Republish RPC-Funktion ermöglicht es, alle aktuellen MQTT Line Status Topics auf Anfrage neu zu veröffentlichen.

## Übersicht

Nach einem Neustart des MQTT-Subscribers oder bei Synchronisationsproblemen kann die Republish-Funktion verwendet werden, um sicherzustellen, dass alle Topics aktualisiert sind.

## MQTT Topics

- **Request Topic**: `{prefix}/republish/request`
- **Response Topic**: `{prefix}/republish/response`

## Request Format

```json
{
  "id": "unique-request-id",
  "method": "republish",
  "timestamp": "2025-10-19T10:30:00Z"
}
```

### Request Felder

| Feld | Typ | Pflicht | Beschreibung |
|------|-----|---------|--------------|
| `id` | string | Ja | Eindeutige Request-ID für Zuordnung der Response |
| `method` | string | Ja | Muss `"republish"` sein |
| `timestamp` | string | Nein | Zeitstempel des Requests (ISO 8601) |

## Response Format

### Erfolgreiche Response

```json
{
  "id": "unique-request-id",
  "success": true,
  "republished_count": 3,
  "timestamp": "2025-10-19T10:30:01Z"
}
```

### Fehler Response

```json
{
  "id": "unique-request-id",
  "success": false,
  "error": "MQTT client not connected",
  "timestamp": "2025-10-19T10:30:01Z"
}
```

### Response Felder

| Feld | Typ | Beschreibung |
|------|-----|--------------|
| `id` | string | Request-ID aus dem Request |
| `success` | boolean | `true` bei Erfolg, `false` bei Fehler |
| `error` | string | Fehlermeldung (nur bei `success: false`) |
| `republished_count` | integer | Anzahl der neu publizierten Line Status Topics (nur bei Erfolg) |
| `timestamp` | string | Zeitstempel der Response (ISO 8601) |

## Beispiel mit mosquitto_pub/mosquitto_sub

### 1. Response-Topic abonnieren

```bash
mosquitto_sub -h localhost -t 'fritz/callmonitor/republish/response' -v
```

### 2. Republish-Request senden

```bash
mosquitto_pub -h localhost \
  -t 'fritz/callmonitor/republish/request' \
  -m '{"id":"req-123","method":"republish","timestamp":"2025-10-19T10:30:00Z"}'
```

### 3. Response empfangen

```
fritz/callmonitor/republish/response {"id":"req-123","success":true,"republished_count":3,"timestamp":"2025-10-19T10:30:01.234Z"}
```

## Beispiel mit Node-RED

### Request-Node konfigurieren

```javascript
// Inject Node -> Function Node
msg.payload = {
    id: Date.now().toString(),
    method: "republish",
    timestamp: new Date().toISOString()
};
msg.topic = "fritz/callmonitor/republish/request";
return msg;
```

### Response-Node konfigurieren

```javascript
// MQTT In Node (Topic: fritz/callmonitor/republish/response)
// -> Function Node
const response = JSON.parse(msg.payload);
if (response.success) {
    node.status({fill: "green", shape: "dot", text: `${response.republished_count} topics republished`});
} else {
    node.status({fill: "red", shape: "dot", text: `Error: ${response.error}`});
}
return msg;
```

## Beispiel mit Python (paho-mqtt)

```python
import paho.mqtt.client as mqtt
import json
import time
from datetime import datetime

def on_connect(client, userdata, flags, rc):
    print("Connected with result code " + str(rc))
    # Subscribe to response topic
    client.subscribe("fritz/callmonitor/republish/response")

    # Send republish request
    request = {
        "id": str(int(time.time() * 1000)),
        "method": "republish",
        "timestamp": datetime.utcnow().isoformat() + "Z"
    }
    client.publish("fritz/callmonitor/republish/request", json.dumps(request))
    print(f"Sent republish request: {request}")

def on_message(client, userdata, msg):
    print(f"Received response on {msg.topic}:")
    response = json.loads(msg.payload.decode())

    if response["success"]:
        print(f"✓ Success: {response['republished_count']} topics republished")
    else:
        print(f"✗ Error: {response['error']}")

    client.disconnect()

client = mqtt.Client()
client.on_connect = on_connect
client.on_message = on_message

client.connect("localhost", 1883, 60)
client.loop_forever()
```

## Anwendungsfälle

1. **Nach MQTT-Broker Neustart**: Alle Topics neu publishen, um sicherzustellen, dass retained Messages aktuell sind
2. **Nach Home Assistant Neustart**: Synchronisation der Sensoren mit den aktuellen Werten
3. **Manuelle Synchronisation**: Bei Verdacht auf veraltete oder fehlende Topics
4. **Automatische Überwachung**: Periodisches Republishing über Automation/Cron für maximale Zuverlässigkeit

## Hinweise

- Die Funktion ist **idempotent**: Mehrfaches Aufrufen hat keine negativen Auswirkungen
- Alle Topics werden mit **retain=true** publiziert
- Die Operation ist **thread-safe** und kann parallel zu laufenden Callmonitor-Events ausgeführt werden
- Bei MQTT-Verbindungsproblemen wird ein entsprechender Fehler zurückgegeben

## Siehe auch

- [Phone Number RPC](./PHONE_NUMBER_RPC.md) - Verwaltung von Telefonnummern via MQTT
- [MQTT Topics](./MQTT_TOPICS.md) - Übersicht aller MQTT Topics
