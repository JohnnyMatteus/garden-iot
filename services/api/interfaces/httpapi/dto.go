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
