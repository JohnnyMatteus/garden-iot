// Package application orquestra a rega: planejamento (balanço + regras) e a saga de execução.
// Depende só do domínio e destas portas.
package application

import (
	"context"
	"github.com/JohnnyMatteus/garden-iot/services/irrigation/domain/agronomy"
	"github.com/JohnnyMatteus/garden-iot/services/irrigation/domain/weather"
	"time"
)

// NodeRef identifica o nó atuador pelo tópico MQTT; a ACL do broker garante o isolamento.
type NodeRef struct{ Tenant, Garden, Node string }

type Valve string

const (
	ValveOpen   Valve = "open"
	ValveClosed Valve = "closed"
)

// ActuatorState é o estado reportado pelo nó (tópico .../estado).
type ActuatorState struct {
	Valve   Valve
	FlowLpm *float64
	At      time.Time
}

// Actuator publica comandos para o nó. Nunca decide: o intertravamento final é do firmware (ADR 0004).
type Actuator interface {
	// Open envia open_valve e só retorna depois de cmd/resposta; recusa ou silêncio viram erro.
	Open(ctx context.Context, ref NodeRef, maxDuration time.Duration) error
	// Close envia close_valve e espera a resposta.
	Close(ctx context.Context, ref NodeRef) error
}

// Twin guarda o estado desejado e o reportado do atuador (ADR 0003).
type Twin interface {
	SetDesired(ctx context.Context, ref NodeRef, v Valve) error
	Reported(ctx context.Context, ref NodeRef) (ActuatorState, bool)
}

type Severity string

const (
	SeverityWarning  Severity = "warning"
	SeverityCritical Severity = "critical"
)

type Alert struct {
	Tenant, Garden, Bed string
	Rule                string
	Severity            Severity
	Message             string
	At                  time.Time
}

type AlertSink interface {
	Raise(ctx context.Context, a Alert) error
}

type Status string

const (
	StatusCompleted   Status = "completed"
	StatusCompensated Status = "compensated"
)

// Record é o registro final de uma rega (RegaConcluida ou rega compensada).
type Record struct {
	Ref     NodeRef
	BedID   string
	Status  Status
	Reason  string
	Liters  float64
	Started time.Time
	Elapsed time.Duration
}

type Journal interface {
	Record(ctx context.Context, r Record) error
}

// Clock isola o tempo para a saga ser testável sem dormir de verdade.
type Clock interface {
	Now() time.Time
	Sleep(ctx context.Context, d time.Duration) error
}

// WeatherProvider devolve a previsão resumida para a janela a partir de "from".
type WeatherProvider interface {
	Forecast(ctx context.Context, latDeg, lonDeg float64, from time.Time, horizon time.Duration) (weather.Forecast, error)
}

// SensorSource entrega o retrato dos sensores do canteiro (telemetria normalizada).
type SensorSource interface {
	Snapshot(ctx context.Context, bed Bed, now time.Time) (Sensors, error)
}

// ClimateSource resume o clima de um dia fechado a partir dos sensores do nó.
type ClimateSource interface {
	Day(ctx context.Context, bed Bed, day time.Time) (agronomy.DailyWeather, error)
}
