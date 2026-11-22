package domain_test

import (
	"errors"
	"testing"
	"time"

	"github.com/JohnnyMatteus/garden-iot/services/api/domain"
)

func f(v float64) *float64 { return &v }

func TestIrrigationPolicy(t *testing.T) {
	actuator := domain.Node{ID: "n3", Kind: domain.ActuatorNode}
	sensor := domain.Node{ID: "n1", Kind: domain.SensorNode}
	p := domain.DefaultIrrigationPolicy()

	tests := []struct {
		name  string
		role  domain.Role
		node  domain.Node
		dur   time.Duration
		cond  domain.Conditions
		rules []string // vazio = permitido
		err   error
	}{
		{name: "permitido sem dados", role: domain.RoleOperator, node: actuator, dur: time.Minute},
		{name: "permitido nos limites", role: domain.RoleOwner, node: actuator, dur: 30 * time.Minute,
			cond: domain.Conditions{AirTempC: f(2), ReservoirPct: f(10)}},
		{name: "leitor não opera", role: domain.RoleViewer, node: actuator, dur: time.Minute, err: domain.ErrForbidden},
		{name: "nó sensor", role: domain.RoleOwner, node: sensor, dur: time.Minute, rules: []string{domain.RuleNodeKind}},
		{name: "duração acima do limite", role: domain.RoleOwner, node: actuator, dur: 31 * time.Minute, rules: []string{domain.RuleMaxDuration}},
		{name: "duração zero", role: domain.RoleOwner, node: actuator, dur: 0, rules: []string{domain.RuleMaxDuration}},
		{name: "geada", role: domain.RoleOwner, node: actuator, dur: time.Minute,
			cond: domain.Conditions{AirTempC: f(1.9)}, rules: []string{domain.RuleFrost}},
		{name: "vento com aspersão", role: domain.RoleOwner, node: actuator, dur: time.Minute,
			cond: domain.Conditions{WindMS: f(5.1), Irrigation: domain.Sprinkler}, rules: []string{domain.RuleWind}},
		{name: "vento com gotejamento é permitido", role: domain.RoleOwner, node: actuator, dur: time.Minute,
			cond: domain.Conditions{WindMS: f(9), Irrigation: domain.Drip}},
		{name: "reservatório baixo", role: domain.RoleOwner, node: actuator, dur: time.Minute,
			cond: domain.Conditions{ReservoirPct: f(9)}, rules: []string{domain.RuleReservoir}},
		{name: "comando em aberto", role: domain.RoleOwner, node: actuator, dur: time.Minute,
			cond: domain.Conditions{OpenCommand: true}, rules: []string{domain.RuleOpenCommand}},
		{name: "várias violações juntas", role: domain.RoleOwner, node: actuator, dur: time.Hour,
			cond:  domain.Conditions{AirTempC: f(0), ReservoirPct: f(0)},
			rules: []string{domain.RuleMaxDuration, domain.RuleFrost, domain.RuleReservoir}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := p.Evaluate(tc.role, tc.node, tc.dur, tc.cond)
			if tc.err != nil {
				if !errors.Is(err, tc.err) {
					t.Fatalf("erro = %v, esperado %v", err, tc.err)
				}
				return
			}
			var pe *domain.PolicyError
			if len(tc.rules) == 0 {
				if err != nil {
					t.Fatalf("esperava permitido, veio %v", err)
				}
				return
			}
			if !errors.As(err, &pe) {
				t.Fatalf("esperava PolicyError, veio %v", err)
			}
			if len(pe.Violations) != len(tc.rules) {
				t.Fatalf("violações = %v, esperado %v", pe.Violations, tc.rules)
			}
			for i, r := range tc.rules {
				if pe.Violations[i].Rule != r {
					t.Errorf("violação %d = %s, esperado %s", i, pe.Violations[i].Rule, r)
				}
			}
		})
	}
}

func TestNodeStateInSync(t *testing.T) {
	open := &domain.ActuatorState{Valve: domain.ValveOpen}
	closed := &domain.ActuatorState{Valve: domain.ValveClosed}
	cases := []struct {
		name string
		s    domain.NodeState
		want bool
	}{
		{"vazio", domain.NodeState{}, true},
		{"desejado sem reportado", domain.NodeState{Desired: open}, false},
		{"iguais", domain.NodeState{Desired: open, Reported: open}, true},
		{"diferentes", domain.NodeState{Desired: open, Reported: closed}, false},
	}
	for _, c := range cases {
		if got := c.s.InSync(); got != c.want {
			t.Errorf("%s: InSync = %v", c.name, got)
		}
	}
}

func TestMoistureBandClassifyAndDeficit(t *testing.T) {
	b := domain.BandFor("Alface ")
	v := func(x float64) *float64 { return &x }
	cases := []struct {
		in      *float64
		want    domain.MoistureStatus
		deficit float64
	}{{v(20), domain.MoistureBelow, 20}, {v(35), domain.MoistureWithin, 5}, {v(45), domain.MoistureAbove, 0}, {nil, domain.MoistureUnknown, -1}}
	for _, c := range cases {
		if got := b.Classify(c.in); got != c.want {
			t.Errorf("%v: %s, esperado %s", c.in, got, c.want)
		}
		if d := b.Deficit(c.in); (d == nil) != (c.deficit < 0) || (d != nil && *d != c.deficit) {
			t.Errorf("%v: déficit %v", c.in, d)
		}
	}
	if domain.BandFor("jiló") != (domain.MoistureBand{Min: 25, Max: 35}) {
		t.Error("cultura desconhecida usa a faixa padrão")
	}
}
