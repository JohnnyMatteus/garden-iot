// Package safety reúne as regras de segurança e intertravamento da irrigação (design doc, seção 6.4).
// É determinístico e sem E/S: toda rega, automática, manual ou proposta pela IA, passa por aqui (ADR 0006).
package safety

import (
	"fmt"
	"math"

	"github.com/JohnnyMatteus/garden-iot/services/irrigation/domain/agronomy"
	"github.com/JohnnyMatteus/garden-iot/services/irrigation/domain/weather"
)

type Method string

const (
	Drip      Method = "drip"
	Sprinkler Method = "sprinkler"
)

type Rule string

const (
	RuleRain      Rule = "rain_forecast"
	RuleWind      Rule = "wind"
	RuleFrost     Rule = "frost"
	RuleReservoir Rule = "reservoir"
	RuleFlow      Rule = "flow_mismatch"
	RuleMinDepth  Rule = "min_depth"
)

type Action string

const (
	Allow    Action = "allow"
	Postpone Action = "postpone"
	Block    Action = "block"
)

// Limits são os limiares das regras; Default segue o design doc.
type Limits struct {
	WindMaxMS      float64 // acima disso, aspersão é adiada
	FrostC         float64 // abaixo disso, não rega
	RainPostponeMM float64 // chuva prevista que dispensa a rega
	MinDepthMM     float64 // lâmina menor que isso não vale abrir a válvula
	FlowTolerance  float64 // fração de divergência da vazão esperada
}

func DefaultLimits() Limits {
	return Limits{WindMaxMS: 5, FrostC: 2, RainPostponeMM: 5, MinDepthMM: 2, FlowTolerance: 0.30}
}

type Reservoir string

const (
	ReservoirUnknown Reservoir = ""
	ReservoirOK      Reservoir = "ok"
	ReservoirLow     Reservoir = "low"
)

// Context é o que se sabe no instante da decisão. Forecast nil significa previsão indisponível:
// a rega segue só com as medições locais (o ganho da previsão é opcional, a segurança não).
type Context struct {
	Method    Method
	AirTempC  *float64
	WindMS    *float64
	Reservoir Reservoir
	Forecast  *weather.Forecast
}

// Violation é uma regra acionada. NeedsAlert indica que o dono deve ser avisado.
type Violation struct {
	Rule       Rule
	Action     Action
	Detail     string
	NeedsAlert bool
}

// Verdict é o parecer: Action é a mais restritiva entre as violações; DepthMM já vem reduzido
// pela chuva prevista quando a rega ainda vale a pena.
type Verdict struct {
	Action     Action
	DepthMM    float64
	Violations []Violation
}

var severity = map[Action]int{Allow: 0, Postpone: 1, Block: 2}

// Evaluate aplica as regras à lâmina que o balanço hídrico pediu.
func Evaluate(l Limits, c Context, depthMM float64) Verdict {
	v := Verdict{Action: Allow, DepthMM: depthMM}
	add := func(x Violation) {
		v.Violations = append(v.Violations, x)
		if severity[x.Action] > severity[v.Action] {
			v.Action = x.Action
		}
	}

	if c.Reservoir == ReservoirLow {
		add(Violation{RuleReservoir, Block, "reservatório abaixo do mínimo: bomba bloqueada", true})
	}

	if t, src := lowestTemp(c); t != nil && *t < l.FrostC {
		add(Violation{RuleFrost, Block, fmt.Sprintf("%.1f °C (%s) abaixo de %.1f °C: risco de geada", *t, src, l.FrostC), true})
	}

	if c.Method == Sprinkler {
		if w := windFor(c); w != nil && *w > l.WindMaxMS {
			add(Violation{RuleWind, Postpone, fmt.Sprintf("vento %.1f m/s acima de %.1f m/s com aspersão", *w, l.WindMaxMS), false})
		}
	}

	if f := c.Forecast; f != nil && f.RainMM > 0 {
		switch {
		case f.RainMM >= l.RainPostponeMM:
			add(Violation{RuleRain, Postpone, fmt.Sprintf("previsão de %.1f mm de chuva em %s", f.RainMM, f.Horizon), false})
		default:
			v.DepthMM = math.Max(0, depthMM-agronomy.EffectiveRain(f.RainMM))
		}
	}

	if v.Action == Allow && v.DepthMM < l.MinDepthMM {
		add(Violation{RuleMinDepth, Postpone, fmt.Sprintf("lâmina de %.1f mm abaixo do mínimo de %.1f mm após a previsão de chuva", v.DepthMM, l.MinDepthMM), false})
	}
	return v
}

func lowestTemp(c Context) (*float64, string) {
	t, src := c.AirTempC, "medida"
	if f := c.Forecast; f != nil && (t == nil || f.MinTempC < *t) {
		m := f.MinTempC
		t, src = &m, "prevista"
	}
	return t, src
}

// windFor prefere a medição local; a previsão só entra se o nó não tem anemômetro.
func windFor(c Context) *float64 {
	if c.WindMS != nil {
		return c.WindMS
	}
	if c.Forecast != nil {
		return &c.Forecast.MaxWindMS
	}
	return nil
}

// CheckFlow compara a vazão medida com a esperada durante a rega. Vazão acima sugere vazamento;
// abaixo, entupimento. Em ambos os casos a saga fecha a válvula e alerta.
func CheckFlow(l Limits, expectedLpm, measuredLpm float64) *Violation {
	if expectedLpm <= 0 {
		return &Violation{RuleFlow, Block, "vazão esperada inválida", true}
	}
	dev := (measuredLpm - expectedLpm) / expectedLpm
	if math.Abs(dev) <= l.FlowTolerance+1e-9 { // folga contra erro de ponto flutuante no limite
		return nil
	}
	cause := "possível vazamento"
	if dev < 0 {
		cause = "possível entupimento"
	}
	return &Violation{RuleFlow, Block,
		fmt.Sprintf("vazão %.2f L/min diverge %.0f%% da esperada %.2f L/min: %s", measuredLpm, dev*100, expectedLpm, cause), true}
}
