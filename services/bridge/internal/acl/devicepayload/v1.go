package devicepayload

import (
	"encoding/json"
	"fmt"
	"math"
	"time"

	brutav1 "github.com/JohnnyMatteus/garden-iot/contracts/gen/go/telemetria/bruta/v1"
)

// luxToWm2 aproxima radiação solar a partir de lux (eficácia luminosa da luz do dia).
// É uma estimativa: a curva real depende do espectro e do sensor.
const luxToWm2 = 0.0079

const (
	soilSuspectMarginPct = 5.0
	minAirTempC          = -40.0
	maxAirTempC          = 85.0
)

// V1Translator traduz o corpo do firmware 1.x (contracts/mqtt/README.md).
type V1Translator struct{}

type v1Payload struct {
	TS      int64      `json:"ts"`
	Dropped uint32     `json:"dropped"` // extensão do firmware, ver firmware/docs/payload-v1.md
	R       []v1Sample `json:"r"`
}

type v1Sample struct {
	ID string   `json:"id"`
	K  string   `json:"k"`
	V  *float64 `json:"v"`
}

func (V1Translator) Translate(msg *brutav1.TelemetriaBruta, cals CalibrationSource) (Measurements, error) {
	var p v1Payload
	if err := json.Unmarshal(msg.GetPayload(), &p); err != nil {
		return Measurements{}, fmt.Errorf("%w: %v", ErrInvalidPayload, err)
	}
	readAt := p.TS
	if msg.GetReadAt() != nil {
		readAt = msg.GetReadAt().GetSeconds()
	}
	if readAt <= 0 {
		return Measurements{}, fmt.Errorf("%w: timestamp ausente (nó sem SNTP)", ErrInvalidPayload)
	}

	m := Measurements{
		TenantID:        msg.GetTenantId(),
		GardenID:        msg.GetGardenId(),
		NodeID:          msg.GetNodeId(),
		FirmwareVersion: msg.GetFwVersion(),
		Sequence:        msg.GetSequence(),
		ReadAt:          time.Unix(readAt, 0).UTC(),
		Dropped:         p.Dropped,
	}
	for _, s := range p.R {
		if s.ID == "" || s.V == nil {
			return Measurements{}, fmt.Errorf("%w: leitura sem id ou v (k=%q)", ErrInvalidPayload, s.K)
		}
		rs, err := translateSample(msg.GetNodeId(), s, *s.V, cals)
		if err != nil {
			return Measurements{}, err
		}
		m.Readings = append(m.Readings, rs...)
	}
	return m, nil
}

// translateSample converte uma leitura. Grandeza desconhecida é ignorada: evolução compatível.
func translateSample(node string, s v1Sample, v float64, cals CalibrationSource) ([]Reading, error) {
	one := func(qty string, val float64, q Quality) []Reading {
		return []Reading{{SensorID: s.ID, Quantity: qty, Value: val, Quality: q}}
	}
	switch s.K {
	case "soil_adc":
		var cal Calibration
		var ok bool
		if cals != nil {
			cal, ok = cals.Calibration(node, s.ID)
		}
		if !ok {
			return nil, fmt.Errorf("%w: nó %q sensor %q", ErrCalibrationMissing, node, s.ID)
		}
		if err := cal.Validate(); err != nil {
			return nil, err
		}
		pct, q := soilMoisture(v, cal)
		return one(SoilMoistureVWC, pct, q), nil
	case "air_temp_c":
		q := QualityMeasured
		if v < minAirTempC || v > maxAirTempC {
			q = QualitySuspect
		}
		return one(AirTempC, v, q), nil
	case "air_rh_pct":
		q := QualityMeasured
		if v < 0 || v > 100 {
			q = QualitySuspect
		}
		return one(AirHumidityPct, v, q), nil
	case "light_lux":
		q := QualityMeasured
		if v < 0 {
			q = QualitySuspect
		}
		return []Reading{
			{SensorID: s.ID, Quantity: IlluminanceLux, Value: v, Quality: q},
			{SensorID: s.ID, Quantity: SolarRadiation, Value: round1(math.Max(v, 0) * luxToWm2), Quality: QualityEstimated},
		}, nil
	case "water_level_ok":
		return one(WaterLevelOK, v, QualityMeasured), nil
	}
	return nil, nil
}

// soilMoisture faz o mapeamento linear seco→0% e saturado→100%, limitado a [0,100].
// Leitura bem fora da faixa calibrada (sensor desconectado, em curto) fica "suspeito".
func soilMoisture(adc float64, cal Calibration) (float64, Quality) {
	pct := (cal.SoilDry - adc) / (cal.SoilDry - cal.SoilSaturated) * 100
	q := QualityMeasured
	if pct < -soilSuspectMarginPct || pct > 100+soilSuspectMarginPct {
		q = QualitySuspect
	}
	return math.Min(100, math.Max(0, pct)), q
}

// round1 limita a estimativa a 0,1 W/m², que é a precisão que ela de fato tem.
func round1(v float64) float64 { return math.Round(v*10) / 10 }
