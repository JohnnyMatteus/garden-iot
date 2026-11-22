// Package config carrega a configuração da bridge do ambiente.
package config

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"os"
	"time"

	"github.com/caarlos0/env/v11"
)

type Config struct {
	MQTTURL        string        `env:"BRIDGE_MQTT_URL,required"`
	MQTTClientID   string        `env:"BRIDGE_MQTT_CLIENT_ID" envDefault:"bridge"`
	MQTTShareGroup string        `env:"BRIDGE_MQTT_SHARE_GROUP" envDefault:"bridge"`
	MQTTFilter     string        `env:"BRIDGE_MQTT_FILTER" envDefault:"horta/+/+/+/telemetria"`
	MQTTSession    time.Duration `env:"BRIDGE_MQTT_SESSION_EXPIRY" envDefault:"1h"`
	MaxInFlight    int           `env:"BRIDGE_MAX_IN_FLIGHT" envDefault:"32"`
	TLSCert        string        `env:"BRIDGE_TLS_CERT"`
	TLSKey         string        `env:"BRIDGE_TLS_KEY"`
	TLSCA          string        `env:"BRIDGE_TLS_CA"`
	KafkaBrokers   []string      `env:"BRIDGE_KAFKA_BROKERS,required" envSeparator:","`
	SchemaRegistry string        `env:"BRIDGE_SCHEMA_REGISTRY_URL,required"`
	Topic          string        `env:"BRIDGE_KAFKA_TOPIC" envDefault:"telemetria.bruta.v1"`
	DLQTopic       string        `env:"BRIDGE_KAFKA_DLQ_TOPIC" envDefault:"dlq.telemetria.bruta.v1"`
	RedisAddr      string        `env:"BRIDGE_REDIS_ADDR"`
	DedupTTL       time.Duration `env:"BRIDGE_DEDUP_TTL" envDefault:"24h"`
	HTTPAddr       string        `env:"BRIDGE_HTTP_ADDR" envDefault:":9102"`
	LogLevel       string        `env:"BRIDGE_LOG_LEVEL" envDefault:"info"`
}

func Load() (Config, error) {
	var c Config
	if err := env.Parse(&c); err != nil {
		return c, fmt.Errorf("configuração: %w", err)
	}
	return c, nil
}

// TLSConfig monta o mTLS da bridge com o broker; devolve nil quando nenhum certificado foi configurado.
func (c Config) TLSConfig() (*tls.Config, error) {
	if c.TLSCA == "" && c.TLSCert == "" {
		return nil, nil
	}
	cfg := &tls.Config{MinVersion: tls.VersionTLS12}
	if c.TLSCA != "" {
		pem, err := os.ReadFile(c.TLSCA)
		if err != nil {
			return nil, fmt.Errorf("ca: %w", err)
		}
		pool := x509.NewCertPool()
		if !pool.AppendCertsFromPEM(pem) {
			return nil, fmt.Errorf("ca sem certificados válidos")
		}
		cfg.RootCAs = pool
	}
	if c.TLSCert != "" {
		cert, err := tls.LoadX509KeyPair(c.TLSCert, c.TLSKey)
		if err != nil {
			return nil, fmt.Errorf("certificado do cliente: %w", err)
		}
		cfg.Certificates = []tls.Certificate{cert}
	}
	return cfg, nil
}
