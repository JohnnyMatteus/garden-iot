package config

import (
	"fmt"

	"github.com/caarlos0/env/v11"
)

// Persist é a configuração do consumidor Kafka → TimescaleDB.
type Persist struct {
	KafkaBrokers  []string `env:"PERSIST_KAFKA_BROKERS,required" envSeparator:","`
	Topic         string   `env:"PERSIST_KAFKA_TOPIC" envDefault:"telemetria.bruta.v1"`
	DLQTopic      string   `env:"PERSIST_KAFKA_DLQ_TOPIC" envDefault:"dlq.telemetria.bruta.v1"`
	Group         string   `env:"PERSIST_KAFKA_GROUP" envDefault:"persist-timescale"`
	BatchSize     int      `env:"PERSIST_BATCH_SIZE" envDefault:"500"`
	DatabaseURL   string   `env:"PERSIST_DATABASE_URL,required"`
	SoilDry       float64  `env:"PERSIST_SOIL_ADC_DRY" envDefault:"3000"`
	SoilSaturated float64  `env:"PERSIST_SOIL_ADC_SATURATED" envDefault:"1200"`
	HTTPAddr      string   `env:"PERSIST_HTTP_ADDR" envDefault:":9103"`
	LogLevel      string   `env:"PERSIST_LOG_LEVEL" envDefault:"info"`
}

func LoadPersist() (Persist, error) {
	var c Persist
	if err := env.Parse(&c); err != nil {
		return c, fmt.Errorf("configuração: %w", err)
	}
	return c, nil
}
