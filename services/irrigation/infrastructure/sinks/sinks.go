// Package sinks traz os destinos de alertas e do registro de regas. Por ora saem como log
// estruturado (JSON em stdout); a gravação em garden.alert exige um papel de escrita dedicado.
package sinks

import (
	"context"
	"log/slog"

	"github.com/JohnnyMatteus/garden-iot/services/irrigation/application"
)

type Log struct{ L *slog.Logger }

func (s Log) Raise(_ context.Context, a application.Alert) error {
	s.L.Warn("alerta de irrigação", "tenant", a.Tenant, "garden", a.Garden, "bed", a.Bed,
		"rule", a.Rule, "severity", a.Severity, "message", a.Message, "at", a.At)
	return nil
}

func (s Log) Record(_ context.Context, r application.Record) error {
	s.L.Info("rega registrada", "tenant", r.Ref.Tenant, "garden", r.Ref.Garden, "node", r.Ref.Node, "bed", r.BedID,
		"status", r.Status, "reason", r.Reason, "liters", r.Liters, "started", r.Started, "elapsed", r.Elapsed)
	return nil
}
