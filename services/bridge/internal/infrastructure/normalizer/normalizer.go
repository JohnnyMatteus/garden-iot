// Package normalizer liga a persistência à ACL do payload de dispositivo (ADR 0007).
package normalizer

import (
	"errors"

	"github.com/JohnnyMatteus/garden-iot/services/bridge/internal/acl/devicepayload"
	"github.com/JohnnyMatteus/garden-iot/services/bridge/internal/domain"
	"github.com/JohnnyMatteus/garden-iot/services/bridge/internal/readings"
)

// ACL converte lotes brutos usando o registro de tradutores por versão de firmware.
// A calibração vem de configuração única no M1; por canteiro virá do contexto Cultivo (M3).
type ACL struct {
	registry    *devicepayload.Registry
	calibration devicepayload.Calibration
}

func New(cal devicepayload.Calibration) *ACL {
	return &ACL{registry: devicepayload.NewRegistry(), calibration: cal}
}

func (a *ACL) Normalize(b readings.Batch) ([]readings.Reading, error) {
	m, err := a.registry.Translate(b.Payload, a.calibration)
	if err != nil {
		var unsupported *devicepayload.UnsupportedFirmwareError
		if errors.As(err, &unsupported) {
			return nil, domain.NewInvalid(domain.ReasonFirmware, err)
		}
		return nil, domain.NewInvalid(domain.ReasonSchema, err)
	}

	base := readings.Reading{Tenant: b.Tenant, Garden: b.Garden, Node: b.Node, ReadAt: b.ReadAt, Sequence: b.Sequence}
	var out []readings.Reading
	add := func(sensor, kind, unit string, q *devicepayload.Quantity) {
		if q == nil {
			return
		}
		r := base
		r.Sensor, r.Kind, r.Unit, r.Value, r.Quality = sensor, kind, unit, q.Value, quality(q.Quality)
		out = append(out, r)
	}
	add("soil", "soil_moisture_vwc", "pct", m.SoilMoistureVWC)
	add("air", "air_temp", "celsius", m.AirTempC)
	add("air", "air_humidity", "pct", m.AirHumidityPct)
	add("light", "illuminance", "lux", m.IlluminanceLux)
	add("light", "solar_radiation", "w_m2", m.SolarRadiation)
	if m.WaterLevelOK != nil {
		v := 0.0
		if *m.WaterLevelOK {
			v = 1
		}
		add("tank", "water_level_ok", "bool", &devicepayload.Quantity{Value: v, Quality: devicepayload.QualityMeasured})
	}
	return out, nil
}

// quality traduz a flag da ACL para o CHECK da tabela (ok, estimated, suspect).
func quality(q devicepayload.Quality) string {
	switch q {
	case devicepayload.QualityEstimated:
		return "estimated"
	case devicepayload.QualitySuspect:
		return "suspect"
	default:
		return "ok"
	}
}
