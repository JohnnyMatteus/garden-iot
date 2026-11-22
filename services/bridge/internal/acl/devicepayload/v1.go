package devicepayload

import (
	"encoding/json"
	"fmt"
	"math"
	"time"
)

// luxToWm2 aproxima radiação solar a partir de lux (eficácia luminosa da luz do dia).
// É uma estimativa: a curva real depende do espectro e do sensor.
const luxToWm2 = 0.0079

const (
	soilSuspectMarginPct = 5.0
	minAirTempC          = -40.0
	maxAirTempC          = 85.0
)

// V1Translator traduz o payload do firmware 1.x (docs/payload-v1.md do firmware).
type V1Translator struct{}

type v1Payload struct {
	FWVersion    string   `json:"fw_version"`
	Seq          *uint32  `json:"seq"`
	TS           int64    `json:"ts"`
	Dropped      uint32   `json:"dropped"`
	SoilADC      *float64 `json:"soil_adc"`
	AirTempC     *float64 `json:"air_temp_c"`
	AirRHPct     *float64 `json:"air_rh_pct"`
	Lux          *float64 `json:"lux"`
	WaterLevelOK *bool    `json:"water_level_ok"`
}

func (V1Translator) Translate(raw []byte, cal Calibration) (Measurements, error) {
	var p v1Payload
	if err := json.Unmarshal(raw, &p); err != nil {
		return Measurements{}, fmt.Errorf("%w: %v", ErrInvalidPayload, err)
	}
	if p.Seq == nil {
		return Measurements{}, fmt.Errorf("%w: seq ausente", ErrInvalidPayload)
	}
	if p.TS <= 0 {
		return Measurements{}, fmt.Errorf("%w: timestamp ausente (nó sem SNTP)", ErrInvalidPayload)
	}

	m := Measurements{
		FirmwareVersion: p.FWVersion,
		Sequence:        *p.Seq,
		Timestamp:       time.Unix(p.TS, 0).UTC(),
		Dropped:         p.Dropped,
		WaterLevelOK:    p.WaterLevelOK,
	}

	if p.SoilADC != nil {
		if err := cal.Validate(); err != nil {
			return Measurements{}, err
		}
		m.SoilMoistureVWC = soilMoisture(*p.SoilADC, cal)
	}
	if p.AirTempC != nil && p.AirRHPct != nil {
		tq, hq := QualityMeasured, QualityMeasured
		if *p.AirTempC < minAirTempC || *p.AirTempC > maxAirTempC {
			tq = QualitySuspect
		}
		if *p.AirRHPct < 0 || *p.AirRHPct > 100 {
			hq = QualitySuspect
		}
		m.AirTempC = &Quantity{Value: *p.AirTempC, Quality: tq}
		m.AirHumidityPct = &Quantity{Value: *p.AirRHPct, Quality: hq}
	}
	if p.Lux != nil {
		lq := QualityMeasured
		if *p.Lux < 0 {
			lq = QualitySuspect
		}
		m.IlluminanceLux = &Quantity{Value: *p.Lux, Quality: lq}
		m.SolarRadiation = &Quantity{Value: round1(math.Max(*p.Lux, 0) * luxToWm2), Quality: QualityEstimated}
	}
	return m, nil
}

// soilMoisture faz o mapeamento linear seco→0% e saturado→100%, limitado a [0,100].
// Leitura bem fora da faixa calibrada (sensor desconectado, em curto) fica "suspeito".
func soilMoisture(adc float64, cal Calibration) *Quantity {
	span := cal.SoilDry - cal.SoilSaturated
	pct := (cal.SoilDry - adc) / span * 100
	q := QualityMeasured
	if pct < -soilSuspectMarginPct || pct > 100+soilSuspectMarginPct {
		q = QualitySuspect
	}
	return &Quantity{Value: math.Min(100, math.Max(0, pct)), Quality: q}
}

// round1 limita a estimativa a 0,1 W/m², que é a precisão que ela de fato tem.
func round1(v float64) float64 { return math.Round(v*10) / 10 }
