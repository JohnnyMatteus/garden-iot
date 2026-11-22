module github.com/JohnnyMatteus/garden-iot/services/irrigation

go 1.26.0

require (
	github.com/eclipse/paho.golang v0.23.0
	github.com/mochi-mqtt/server/v2 v2.7.9
	github.com/stretchr/testify v1.12.1
)

require (
	github.com/gorilla/websocket v1.5.3 // indirect
	github.com/rs/xid v1.4.0 // indirect
	go.yaml.in/yaml/v3 v3.0.5 // indirect
	golang.org/x/net v0.43.0 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)

replace github.com/JohnnyMatteus/garden-iot/contracts => ../../contracts
