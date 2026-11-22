package domain

import (
	"sort"
	"strings"
	"time"
)

// Nomes das grandezas usados pela API; a fonte é contracts/quantity (módulo à parte, sem dependência aqui).
const (
	KindSoilMoisture = "soil_moisture_vwc_pct"
	KindAirTemp      = "air_temp_c"
)

// MoistureBand é a faixa de umidade volumétrica do solo (%) em que a cultura está bem servida.
type MoistureBand struct{ Min, Max float64 }

// Faixa padrão por cultura, usada até o planejador de irrigação publicar a calibração de cada canteiro.
var cropBands = map[string]MoistureBand{
	"alface": {30, 40},
	"tomate": {25, 35},
}

var defaultBand = MoistureBand{25, 35}

func BandFor(crop string) MoistureBand {
	if b, ok := cropBands[strings.ToLower(strings.TrimSpace(crop))]; ok {
		return b
	}
	return defaultBand
}

type MoistureStatus string

const (
	MoistureUnknown MoistureStatus = "unknown"
	MoistureBelow   MoistureStatus = "below"
	MoistureWithin  MoistureStatus = "within"
	MoistureAbove   MoistureStatus = "above"
)

// Classify compara a leitura com a faixa; sem leitura o estado é desconhecido, nunca "dentro".
func (b MoistureBand) Classify(v *float64) MoistureStatus {
	switch {
	case v == nil:
		return MoistureUnknown
	case *v < b.Min:
		return MoistureBelow
	case *v > b.Max:
		return MoistureAbove
	default:
		return MoistureWithin
	}
}

// Deficit é a distância em pontos percentuais até o topo da faixa (0 quando já está cheio).
func (b MoistureBand) Deficit(v *float64) *float64 {
	if v == nil {
		return nil
	}
	d := b.Max - *v
	if d < 0 {
		d = 0
	}
	return &d
}

// BedOverview junta o canteiro, a última leitura e o estado da válvula para o mapa da horta.
type BedOverview struct {
	Bed            Bed
	Band           MoistureBand
	Status         MoistureStatus
	Moisture       *Reading
	AirTemp        *Reading
	SensorNodeID   string
	ActuatorNodeID string
	Valve          *NodeState
	OpenCommand    *Command
}

// Irrigating: o nó reportou a válvula aberta (o desejado sozinho não conta como rega em curso).
func (o BedOverview) Irrigating() bool {
	return o.Valve != nil && o.Valve.Reported != nil && o.Valve.Reported.Valve == ValveOpen
}

type DecisionKind string

const (
	DecisionIrrigation DecisionKind = "irrigation"
	DecisionAlert      DecisionKind = "alert"
)

// Decision é uma linha do histórico de decisões do canteiro: comandos de rega e alertas das
// regras de segurança, com o motivo em texto.
type Decision struct {
	At              time.Time
	Kind            DecisionKind
	Outcome         string
	Reason          string
	NodeID          string
	DurationSeconds int
}

func DecisionFromCommand(c Command) Decision {
	return Decision{
		At: c.CreatedAt, Kind: DecisionIrrigation, Outcome: string(c.Status), NodeID: c.NodeID,
		DurationSeconds: c.DurationSeconds, Reason: "Rega manual pedida por " + c.RequestedBy,
	}
}

func DecisionFromAlert(a Alert) Decision {
	d := Decision{At: a.RaisedAt, Kind: DecisionAlert, Outcome: string(a.Severity), Reason: a.Message}
	if a.NodeID != nil {
		d.NodeID = *a.NodeID
	}
	return d
}

// SortDecisions ordena do mais recente ao mais antigo e corta em limit.
func SortDecisions(ds []Decision, limit int) []Decision {
	sort.SliceStable(ds, func(i, j int) bool { return ds[i].At.After(ds[j].At) })
	if limit > 0 && len(ds) > limit {
		ds = ds[:limit]
	}
	return ds
}

// AggregatePoint é um balde de tempo de uma grandeza.
type AggregatePoint struct {
	At            time.Time
	Avg, Min, Max float64
	Count         int
}

func BedBand(b Bed) MoistureBand { return BandFor(b.Crop) }
