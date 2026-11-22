package domain

import (
	"fmt"
	"strings"
	"time"
)

// Regras de segurança aplicáveis a um pedido de rega manual (design doc, seção 6.4).
const (
	RuleRole        = "role"
	RuleNodeKind    = "node_kind"
	RuleMaxDuration = "max_duration"
	RuleFrost       = "frost"
	RuleWind        = "wind"
	RuleReservoir   = "reservoir"
	RuleOpenCommand = "open_command"
)

type Violation struct {
	Rule   string
	Reason string
}

// PolicyError agrupa todas as regras violadas, para a resposta explicar tudo de uma vez.
type PolicyError struct{ Violations []Violation }

func (e *PolicyError) Error() string {
	parts := make([]string, len(e.Violations))
	for i, v := range e.Violations {
		parts[i] = v.Rule + ": " + v.Reason
	}
	return "rega bloqueada pela política de segurança: " + strings.Join(parts, "; ")
}

// Conditions é o contexto do momento. Ponteiro nulo significa "sem dado recente": a regra
// não bloqueia (o intertravamento final é do firmware, ADR 0004).
type Conditions struct {
	AirTempC     *float64
	WindMS       *float64
	ReservoirPct *float64
	Irrigation   IrrigationType
	OpenCommand  bool
}

type IrrigationPolicy struct {
	MaxDuration     time.Duration
	MinAirTempC     float64
	MaxWindMS       float64
	MinReservoirPct float64
}

func DefaultIrrigationPolicy() IrrigationPolicy {
	return IrrigationPolicy{MaxDuration: 30 * time.Minute, MinAirTempC: 2, MaxWindMS: 5, MinReservoirPct: 10}
}

// Evaluate devolve nil quando a rega pode ser enviada ao nó.
func (p IrrigationPolicy) Evaluate(role Role, node Node, duration time.Duration, c Conditions) error {
	var v []Violation
	add := func(rule, format string, a ...any) { v = append(v, Violation{rule, fmt.Sprintf(format, a...)}) }

	if !role.CanOperate() {
		return ErrForbidden
	}
	if node.Kind != ActuatorNode {
		add(RuleNodeKind, "o nó %s não é atuador", node.ID)
	}
	if duration <= 0 || duration > p.MaxDuration {
		add(RuleMaxDuration, "duração deve estar entre 1 s e %s", p.MaxDuration)
	}
	if c.OpenCommand {
		add(RuleOpenCommand, "já existe comando em aberto para o nó")
	}
	if c.AirTempC != nil && *c.AirTempC < p.MinAirTempC {
		add(RuleFrost, "temperatura %.1f °C abaixo de %.1f °C", *c.AirTempC, p.MinAirTempC)
	}
	if c.Irrigation == Sprinkler && c.WindMS != nil && *c.WindMS > p.MaxWindMS {
		add(RuleWind, "vento %.1f m/s acima de %.1f m/s com aspersão", *c.WindMS, p.MaxWindMS)
	}
	if c.ReservoirPct != nil && *c.ReservoirPct < p.MinReservoirPct {
		add(RuleReservoir, "reservatório em %.0f%%, mínimo %.0f%%", *c.ReservoirPct, p.MinReservoirPct)
	}
	if len(v) > 0 {
		return &PolicyError{Violations: v}
	}
	return nil
}
