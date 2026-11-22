package devicepayload

import (
	"errors"
	"fmt"
	"time"
)

// Quality diz o quanto a grandeza convertida merece confiança (design doc, seção 6.3).
type Quality string

const (
	QualityMeasured  Quality = "medido"
	QualityEstimated Quality = "estimado"
	QualitySuspect   Quality = "suspeito"
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

// Quantity é uma grandeza de domínio com a qualidade da conversão.
type Quantity struct {
	Value   float64 `json:"value"`
	Quality Quality `json:"quality"`
}

// Measurements é o resultado da tradução de um lote, já em grandezas de domínio.
// Campos opcionais ficam nulos quando o sensor não reportou.
type Measurements struct {
	FirmwareVersion string    `json:"fw_version"`
	Sequence        uint32    `json:"sequence"`
	Timestamp       time.Time `json:"timestamp"`
	// Dropped é o número de lotes que o nó descartou por buffer cheio antes deste.
	Dropped uint32 `json:"dropped"`

	SoilMoistureVWC *Quantity `json:"soil_moisture_vwc_pct,omitempty"`
	AirTempC        *Quantity `json:"air_temp_c,omitempty"`
	AirHumidityPct  *Quantity `json:"air_humidity_pct,omitempty"`
	IlluminanceLux  *Quantity `json:"illuminance_lux,omitempty"`
	SolarRadiation  *Quantity `json:"solar_radiation_w_m2,omitempty"`
	WaterLevelOK    *bool     `json:"water_level_ok,omitempty"`
}

var (
	ErrInvalidPayload     = errors.New("payload de dispositivo inválido")
	ErrInvalidCalibration = errors.New("calibração inválida")
)

// UnsupportedFirmwareError indica que não há tradutor para a versão de firmware do payload.
type UnsupportedFirmwareError struct {
	Version string
}

func (e *UnsupportedFirmwareError) Error() string {
	return fmt.Sprintf("versão de firmware sem tradutor registrado: %q", e.Version)
}

// Translator converte o payload de uma versão de firmware em Measurements.
type Translator interface {
	Translate(raw []byte, cal Calibration) (Measurements, error)
}
