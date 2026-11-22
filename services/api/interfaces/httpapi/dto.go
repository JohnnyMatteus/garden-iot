package httpapi

import (
	"time"

	"github.com/JohnnyMatteus/garden-iot/services/api/domain"
)

type gardenDTO struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	CreatedAt time.Time `json:"createdAt"`
}

type bedDTO struct {
	ID         string `json:"id"`
	GardenID   string `json:"gardenId,omitempty"`
	Name       string `json:"name"`
	Crop       string `json:"crop,omitempty"`
	Irrigation string `json:"irrigation"`
}

type nodeDTO struct {
	ID         string     `json:"id"`
	GardenID   string     `json:"gardenId,omitempty"`
	Kind       string     `json:"kind"`
	BedID      *string    `json:"bedId,omitempty"`
	FwVersion  string     `json:"fwVersion,omitempty"`
	LastSeenAt *time.Time `json:"lastSeenAt,omitempty"`
}

type readingDTO struct {
	NodeID   string    `json:"nodeId"`
	SensorID string    `json:"sensorId"`
	Kind     string    `json:"kind"`
	ReadAt   time.Time `json:"readAt"`
	Value    float64   `json:"value"`
	Unit     string    `json:"unit"`
	Quality  string    `json:"quality"`
}

type actuatorStateDTO struct {
	Valve   string    `json:"valve"`
	FlowLpm *float64  `json:"flowLpm,omitempty"`
	At      time.Time `json:"at"`
}

type nodeStateDTO struct {
	NodeID   string            `json:"nodeId"`
	Desired  *actuatorStateDTO `json:"desired,omitempty"`
	Reported *actuatorStateDTO `json:"reported,omitempty"`
	InSync   bool              `json:"inSync"`
}

type alertDTO struct {
	ID             string     `json:"id"`
	NodeID         *string    `json:"nodeId,omitempty"`
	Severity       string     `json:"severity"`
	Kind           string     `json:"kind"`
	Message        string     `json:"message"`
	RaisedAt       time.Time  `json:"raisedAt"`
	AcknowledgedAt *time.Time `json:"acknowledgedAt,omitempty"`
}

type commandDTO struct {
	ID              string    `json:"id"`
	NodeID          string    `json:"nodeId"`
	Action          string    `json:"action"`
	DurationSeconds int       `json:"durationSeconds"`
	Status          string    `json:"status"`
	RequestedBy     string    `json:"requestedBy"`
	CreatedAt       time.Time `json:"createdAt"`
	ExpiresAt       time.Time `json:"expiresAt"`
}

func toGarden(g domain.Garden) gardenDTO { return gardenDTO{g.ID, g.Name, g.CreatedAt} }
func toBed(b domain.Bed) bedDTO {
	return bedDTO{b.ID, b.GardenID, b.Name, b.Crop, string(b.Irrigation)}
}
func toNode(n domain.Node) nodeDTO {
	return nodeDTO{n.ID, n.GardenID, string(n.Kind), n.BedID, n.FwVersion, n.LastSeenAt}
}
func toReading(r domain.Reading) readingDTO {
	return readingDTO{r.NodeID, r.SensorID, r.Kind, r.ReadAt, r.Value, r.Unit, r.Quality}
}
func toActuator(s *domain.ActuatorState) *actuatorStateDTO {
	if s == nil {
		return nil
	}
	return &actuatorStateDTO{string(s.Valve), s.FlowLpm, s.At}
}
func toState(s domain.NodeState) nodeStateDTO {
	return nodeStateDTO{s.NodeID, toActuator(s.Desired), toActuator(s.Reported), s.InSync()}
}
func toAlert(a domain.Alert) alertDTO {
	return alertDTO{a.ID, a.NodeID, string(a.Severity), a.Kind, a.Message, a.RaisedAt, a.AcknowledgedAt}
}
func toCommand(c domain.Command) commandDTO {
	return commandDTO{c.ID, c.NodeID, c.Action, c.DurationSeconds, string(c.Status), c.RequestedBy, c.CreatedAt, c.ExpiresAt}
}

func mapList[T, U any](in []T, f func(T) U) []U {
	out := make([]U, 0, len(in))
	for _, v := range in {
		out = append(out, f(v))
	}
	return out
}

type valveDTO struct {
	Desired  *actuatorStateDTO `json:"desired,omitempty"`
	Reported *actuatorStateDTO `json:"reported,omitempty"`
	InSync   bool              `json:"inSync"`
}

type bedOverviewDTO struct {
	bedDTO
	IdealMinPct    float64     `json:"idealMinPct"`
	IdealMaxPct    float64     `json:"idealMaxPct"`
	MoistureStatus string      `json:"moistureStatus"`
	MoisturePct    *float64    `json:"moisturePct,omitempty"`
	DeficitPct     *float64    `json:"deficitPct,omitempty"`
	AirTempC       *float64    `json:"airTempC,omitempty"`
	ReadAt         *time.Time  `json:"readAt,omitempty"`
	SensorNodeID   string      `json:"sensorNodeId,omitempty"`
	ActuatorNodeID string      `json:"actuatorNodeId,omitempty"`
	Valve          *valveDTO   `json:"valve,omitempty"`
	Irrigating     bool        `json:"irrigating"`
	OpenCommand    *commandDTO `json:"openCommand,omitempty"`
}

type overviewDTO struct {
	GardenID    string           `json:"gardenId"`
	GeneratedAt time.Time        `json:"generatedAt"`
	OpenAlerts  int              `json:"openAlerts"`
	Beds        []bedOverviewDTO `json:"beds"`
}

type decisionDTO struct {
	At              time.Time `json:"at"`
	Kind            string    `json:"kind"`
	Outcome         string    `json:"outcome"`
	Reason          string    `json:"reason"`
	NodeID          string    `json:"nodeId,omitempty"`
	DurationSeconds int       `json:"durationSeconds,omitempty"`
}

type aggregatePointDTO struct {
	At    time.Time `json:"at"`
	Avg   float64   `json:"avg"`
	Min   float64   `json:"min"`
	Max   float64   `json:"max"`
	Count int       `json:"count"`
}

func toBedOverview(o domain.BedOverview) bedOverviewDTO {
	d := bedOverviewDTO{
		bedDTO: toBed(o.Bed), IdealMinPct: o.Band.Min, IdealMaxPct: o.Band.Max, MoistureStatus: string(o.Status),
		SensorNodeID: o.SensorNodeID, ActuatorNodeID: o.ActuatorNodeID, Irrigating: o.Irrigating(),
	}
	if o.Moisture != nil {
		d.MoisturePct, d.ReadAt = &o.Moisture.Value, &o.Moisture.ReadAt
		d.DeficitPct = o.Band.Deficit(&o.Moisture.Value)
	}
	if o.AirTemp != nil {
		d.AirTempC = &o.AirTemp.Value
	}
	if o.Valve != nil {
		d.Valve = &valveDTO{toActuator(o.Valve.Desired), toActuator(o.Valve.Reported), o.Valve.InSync()}
	}
	if o.OpenCommand != nil {
		c := toCommand(*o.OpenCommand)
		d.OpenCommand = &c
	}
	return d
}

func toDecision(d domain.Decision) decisionDTO {
	return decisionDTO{d.At, string(d.Kind), d.Outcome, d.Reason, d.NodeID, d.DurationSeconds}
}

func toAggregatePoint(p domain.AggregatePoint) aggregatePointDTO {
	return aggregatePointDTO{p.At, p.Avg, p.Min, p.Max, p.Count}
}
