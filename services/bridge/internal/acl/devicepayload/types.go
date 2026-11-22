package devicepayload

import (
	"errors"
	"fmt"
	"time"

	brutav1 "github.com/JohnnyMatteus/garden-iot/contracts/gen/go/telemetria/bruta/v1"
	"github.com/JohnnyMatteus/garden-iot/contracts/quantity"
)

// Quality diz o quanto a grandeza convertida merece confiança (design doc, seção 6.3).
type Quality string

const (
	QualityMeasured  Quality = "medido"
	QualityEstimated Quality = "estimado"
	QualitySuspect   Quality = "suspeito"
)

// Grandezas de domínio produzidas pelo tradutor.
const (
	SoilMoistureVWC = quantity.SoilMoisture
	AirTempC        = quantity.AirTemp
	AirHumidityPct  = quantity.AirHumidity
	IlluminanceLux  = quantity.Illuminance
	SolarRadiation  = quantity.SolarRadiation
	WaterLevelOK    = quantity.WaterLevelOK
)

// Calibration é a calibração do sensor de solo: leitura ADC no solo seco e saturado.
type Calibration struct {
	SoilDry       float64
	SoilSaturated float64
}

// Validate rejeita calibração degenerada, que causaria divisão por zero ou curva invertida.
func (c Calibration) Validate() error {
	if c.SoilDry <= c.SoilSaturated {
		return fmt.Errorf("%w: seco (%v) deve ser maior que saturado (%v)", ErrInvalidCalibration, c.SoilDry, c.SoilSaturated)
	}
	return nil
}

// CalibrationSource devolve a calibração de um sensor do nó (hoje de configuração; do contexto
// Cultivo a partir do M3).
type CalibrationSource interface {
	Calibration(node, sensorID string) (Calibration, bool)
}

// StaticCalibrations é uma CalibrationSource em memória, indexada por "nó/sensor".
type StaticCalibrations map[string]Calibration

func (s StaticCalibrations) Calibration(node, sensorID string) (Calibration, bool) {
	c, ok := s[node+"/"+sensorID]
	return c, ok
}

// Reading é uma grandeza de domínio de um sensor, com a qualidade da conversão.
type Reading struct {
	SensorID string  `json:"sensor_id"`
	Quantity string  `json:"quantity"`
	Value    float64 `json:"value"`
	Quality  Quality `json:"quality"`
}

// Measurements é o resultado da tradução de um lote.
type Measurements struct {
	TenantID        string    `json:"tenant_id"`
	GardenID        string    `json:"garden_id"`
	NodeID          string    `json:"node_id"`
	FirmwareVersion string    `json:"fw_version"`
	Sequence        uint64    `json:"sequence"`
	ReadAt          time.Time `json:"read_at"`
	// Dropped é o número de lotes que o nó descartou por buffer cheio antes deste.
	Dropped  uint32    `json:"dropped"`
	Readings []Reading `json:"readings"`
}

var (
	ErrInvalidPayload     = errors.New("payload de dispositivo inválido")
	ErrInvalidCalibration = errors.New("calibração inválida")
	ErrCalibrationMissing = errors.New("calibração do sensor ausente")
)

// UnsupportedFirmwareError indica que não há tradutor para a versão de firmware do payload.
type UnsupportedFirmwareError struct {
	Version string
}

func (e *UnsupportedFirmwareError) Error() string {
	return fmt.Sprintf("versão de firmware sem tradutor registrado: %q", e.Version)
}

// Translator converte o corpo de lotes de uma versão de firmware em Measurements.
type Translator interface {
	Translate(msg *brutav1.TelemetriaBruta, cals CalibrationSource) (Measurements, error)
}
