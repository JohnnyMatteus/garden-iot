// Package config carrega a configuração do serviço de irrigação do ambiente e o cadastro de canteiros.
package config

import (
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/caarlos0/env/v11"

	"github.com/JohnnyMatteus/garden-iot/services/irrigation/application"
	"github.com/JohnnyMatteus/garden-iot/services/irrigation/domain/agronomy"
	"github.com/JohnnyMatteus/garden-iot/services/irrigation/domain/safety"
)

type Config struct {
	MQTTURL      string        `env:"IRRIGATION_MQTT_URL,required"`
	MQTTClientID string        `env:"IRRIGATION_MQTT_CLIENT_ID" envDefault:"irrigation"`
	TLSCert      string        `env:"IRRIGATION_TLS_CERT"`
	TLSKey       string        `env:"IRRIGATION_TLS_KEY"`
	TLSCA        string        `env:"IRRIGATION_TLS_CA"`
	DatabaseURL  string        `env:"IRRIGATION_DATABASE_URL,required"`
	BedsFile     string        `env:"IRRIGATION_BEDS_FILE,required"`
	Interval     time.Duration `env:"IRRIGATION_TICK_INTERVAL" envDefault:"5m"`
	MaxDuration  time.Duration `env:"IRRIGATION_MAX_DURATION" envDefault:"30m"`
	DefaultWind  float64       `env:"IRRIGATION_DEFAULT_WIND_MS" envDefault:"2"`
	WeatherOff   bool          `env:"IRRIGATION_WEATHER_DISABLED"`
	HTTPAddr     string        `env:"IRRIGATION_HTTP_ADDR" envDefault:":9104"`
	LogLevel     string        `env:"IRRIGATION_LOG_LEVEL" envDefault:"info"`
}

func Load() (Config, error) {
	var c Config
	if err := env.Parse(&c); err != nil {
		return c, fmt.Errorf("configuração: %w", err)
	}
	return c, nil
}

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

type bedFile struct {
	ID             string  `json:"id"`
	Tenant         string  `json:"tenant"`
	Garden         string  `json:"garden"`
	Node           string  `json:"node"`
	Crop           string  `json:"crop"`
	Stage          string  `json:"stage"`
	StageProgress  float64 `json:"stage_progress"`
	AreaM2         float64 `json:"area_m2"`
	Method         string  `json:"method"`
	FlowLpm        float64 `json:"flow_lpm"`
	Lat            float64 `json:"lat"`
	Lon            float64 `json:"lon"`
	ElevationM     float64 `json:"elevation_m"`
	MoistureMinPct float64 `json:"moisture_threshold_pct"`
}

// LoadBeds lê o cadastro de canteiros (JSON) e valida cada um. O cadastro vem do contexto Cultivo (M3).
func LoadBeds(path string) ([]application.Bed, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var files []bedFile
	if err := json.Unmarshal(raw, &files); err != nil {
		return nil, fmt.Errorf("canteiros: %w", err)
	}
	beds := make([]application.Bed, 0, len(files))
	for _, f := range files {
		crop, ok := agronomy.Crops[f.Crop]
		if !ok {
			return nil, fmt.Errorf("canteiro %q: cultura %q desconhecida", f.ID, f.Crop)
		}
		m := safety.Method(f.Method)
		if m != safety.Drip && m != safety.Sprinkler {
			return nil, fmt.Errorf("canteiro %q: método %q inválido", f.ID, f.Method)
		}
		if f.AreaM2 <= 0 || f.FlowLpm <= 0 || f.ID == "" || f.Tenant == "" || f.Garden == "" || f.Node == "" {
			return nil, fmt.Errorf("canteiro %q: id, tenant, horta, nó, área e vazão são obrigatórios", f.ID)
		}
		beds = append(beds, application.Bed{
			ID: f.ID, Ref: application.NodeRef{Tenant: f.Tenant, Garden: f.Garden, Node: f.Node}, Crop: crop,
			Soil: agronomy.LoamSoil, Stage: agronomy.Stage(f.Stage), StageProgress: f.StageProgress,
			AreaM2: f.AreaM2, Method: m, FlowLpm: f.FlowLpm, LatDeg: f.Lat, LonDeg: f.Lon,
			ElevationM: f.ElevationM, MoistureThresholdPct: f.MoistureMinPct,
		})
	}
	return beds, nil
}
