// Package domain contém o modelo e as regras de segurança da API, sem dependência de infraestrutura.
package domain

import (
	"errors"
	"regexp"
	"time"
)

var (
	ErrNotFound     = errors.New("não encontrado")
	ErrConflict     = errors.New("já existe")
	ErrForbidden    = errors.New("papel sem permissão")
	ErrInvalidInput = errors.New("entrada inválida")
)

var identifierRE = regexp.MustCompile(`^[a-z0-9-]{1,32}$`)

// ValidIdentifier diz se o texto serve como id de tenant, horta, canteiro ou nó.
func ValidIdentifier(s string) bool { return identifierRE.MatchString(s) }

type Role string

const (
	RoleOwner    Role = "owner"
	RoleOperator Role = "operator"
	RoleViewer   Role = "viewer"
)

func (r Role) Valid() bool { return r == RoleOwner || r == RoleOperator || r == RoleViewer }

// CanOperate: pode pedir rega e reconhecer alertas.
func (r Role) CanOperate() bool { return r == RoleOwner || r == RoleOperator }

// Principal é a identidade autenticada; o tenant sempre vem do token.
type Principal struct {
	Subject  string
	TenantID string
	Role     Role
}

type Garden struct {
	ID        string
	Name      string
	CreatedAt time.Time
}

type IrrigationType string

const (
	Drip      IrrigationType = "drip"
	Sprinkler IrrigationType = "sprinkler"
)

type Bed struct {
	ID         string
	GardenID   string
	Name       string
	Crop       string
	Irrigation IrrigationType
}

type NodeKind string

const (
	SensorNode   NodeKind = "sensor"
	ActuatorNode NodeKind = "actuator"
)

type Node struct {
	ID         string
	GardenID   string
	Kind       NodeKind
	BedID      *string
	FwVersion  string
	LastSeenAt *time.Time
}

type Reading struct {
	NodeID   string
	SensorID string
	Kind     string
	ReadAt   time.Time
	Value    float64
	Unit     string
	Quality  string
}

type Valve string

const (
	ValveOpen   Valve = "open"
	ValveClosed Valve = "closed"
)

type ActuatorState struct {
	Valve   Valve
	FlowLpm *float64
	At      time.Time
}

// NodeState é o gêmeo digital: o que a nuvem quer (desejado) e o que o nó informou (reportado).
type NodeState struct {
	NodeID   string
	Desired  *ActuatorState
	Reported *ActuatorState
}

// InSync é verdadeiro quando o nó confirmou a posição desejada da válvula.
func (s NodeState) InSync() bool {
	if s.Desired == nil || s.Reported == nil {
		return s.Desired == nil && s.Reported == nil
	}
	return s.Desired.Valve == s.Reported.Valve
}

type Severity string

type Alert struct {
	ID             string
	GardenID       string
	NodeID         *string
	Severity       Severity
	Kind           string
	Message        string
	RaisedAt       time.Time
	AcknowledgedAt *time.Time
}

type CommandStatus string

const (
	CommandPending   CommandStatus = "pending"
	CommandSent      CommandStatus = "sent"
	CommandConfirmed CommandStatus = "confirmed"
	CommandRejected  CommandStatus = "rejected"
	CommandExpired   CommandStatus = "expired"
	CommandFailed    CommandStatus = "failed"
)

const ActionOpenValve = "open_valve"

type Command struct {
	ID              string
	GardenID        string
	NodeID          string
	Action          string
	DurationSeconds int
	Status          CommandStatus
	RequestedBy     string
	CreatedAt       time.Time
	ExpiresAt       time.Time
}
