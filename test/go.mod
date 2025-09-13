module test

go 1.21

replace fritz-callmonitor2mqtt => ../

require fritz-callmonitor2mqtt v0.0.0-00010101000000-000000000000

require (
	github.com/eclipse/paho.mqtt.golang v1.4.3
	github.com/google/uuid v1.6.0
	github.com/gorilla/websocket v1.5.0
	github.com/mattn/go-sqlite3 v1.14.19
	golang.org/x/net v0.8.0
	golang.org/x/sync v0.1.0
)
