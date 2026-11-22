// Package normalizer liga a persistência à ACL do payload de dispositivo (ADR 0007).
package normalizer

import (
	"errors"

	"google.golang.org/protobuf/types/known/timestamppb"

	brutav1 "github.com/JohnnyMatteus/garden-iot/contracts/gen/go/telemetria/bruta/v1"
	"github.com/JohnnyMatteus/garden-iot/services/bridge/internal/acl/devicepayload"
	"github.com/JohnnyMatteus/garden-iot/services/bridge/internal/domain"
	"github.com/JohnnyMatteus/garden-iot/services/bridge/internal/readings"
)

// uniformCalibration usa a mesma calibração para todo sensor de solo. No M1 ela vem de
// configuração única; por canteiro virá do contexto Cultivo (M3).
type uniformCalibration devicepayload.Calibration

func (u uniformCalibration) Calibration(string, string) (devicepayload.Calibration, bool) {
	return devicepayload.Calibration(u), true
}

// ACL converte lotes brutos usando o registro de tradutores por versão de firmware.
type ACL struct {
	registry *devicepayload.Registry
	cals     devicepayload.CalibrationSource
}

func New(cal devicepayload.Calibration) *ACL {
	return &ACL{registry: devicepayload.NewRegistry(), cals: uniformCalibration(cal)}
}

var units = map[string]string{
	devicepayload.SoilMoistureVWC: "pct",
	devicepayload.AirTempC:        "celsius",
	devicepayload.AirHumidityPct:  "pct",
	devicepayload.IlluminanceLux:  "lux",
	devicepayload.SolarRadiation:  "w_m2",
	devicepayload.WaterLevelOK:    "bool",
}

func (a *ACL) Normalize(b readings.Batch) ([]readings.Reading, error) {
	msg := &brutav1.TelemetriaBruta{
		TenantId: b.Tenant, GardenId: b.Garden, NodeId: b.Node, Sequence: b.Sequence,
		FwVersion: b.FwVersion, ReadAt: timestamppb.New(b.ReadAt), Payload: b.Payload,
	}
	m, err := a.registry.Translate(msg, a.cals)
	if err != nil {
		var unsupported *devicepayload.UnsupportedFirmwareError
		if errors.As(err, &unsupported) {
			return nil, domain.NewInvalid(domain.ReasonFirmware, err)
		}
		return nil, domain.NewInvalid(domain.ReasonSchema, err)
	}

	out := make([]readings.Reading, 0, len(m.Readings))
	for _, r := range m.Readings {
		out = append(out, readings.Reading{
			Tenant: b.Tenant, Garden: b.Garden, Node: b.Node, Sensor: r.SensorID, Kind: r.Quantity,
			ReadAt: b.ReadAt, Sequence: b.Sequence, Value: r.Value, Unit: units[r.Quantity], Quality: quality(r.Quality),
		})
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
