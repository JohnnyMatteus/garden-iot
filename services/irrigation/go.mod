module github.com/JohnnyMatteus/garden-iot/services/irrigation

go 1.26.0

require (
	github.com/JohnnyMatteus/garden-iot/contracts v0.0.0-00010101000000-000000000000
	github.com/caarlos0/env/v11 v11.4.2
	github.com/eclipse/paho.golang v0.23.0
	github.com/jackc/pgx/v5 v5.13.0
	github.com/mochi-mqtt/server/v2 v2.7.9
	github.com/stretchr/testify v1.12.1
)

require (
	github.com/gorilla/websocket v1.5.3 // indirect
	github.com/jackc/pgpassfile v1.0.0 // indirect
	github.com/jackc/pgservicefile v0.0.0-20240606120523-5a60cdf6a761 // indirect
	github.com/jackc/puddle/v2 v2.2.3 // indirect
	github.com/rs/xid v1.4.0 // indirect
	go.yaml.in/yaml/v3 v3.0.5 // indirect
	golang.org/x/net v0.43.0 // indirect
	golang.org/x/sync v0.22.0 // indirect
	golang.org/x/text v0.41.0 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)

replace github.com/JohnnyMatteus/garden-iot/contracts => ../../contracts
