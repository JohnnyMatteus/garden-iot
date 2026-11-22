package application

import (
	"context"
	"fmt"
	"log/slog"
	"math"
	"time"

	"github.com/JohnnyMatteus/garden-iot/services/irrigation/domain/agronomy"
	"github.com/JohnnyMatteus/garden-iot/services/irrigation/domain/safety"
)

// Bed é um canteiro irrigável: cultura, solo, geometria e o nó atuador que o serve.
type Bed struct {
	ID             string
	Ref            NodeRef
	Crop           agronomy.Crop
	Soil           agronomy.Soil
	Stage          agronomy.Stage
	StageProgress  float64
	AreaM2         float64
	Method         safety.Method
	FlowLpm        float64 // vazão esperada com a válvula aberta
	LatDeg, LonDeg float64
	ElevationM     float64
	// MoistureThresholdPct é o limite calibrado do sensor: abaixo dele, rega.
	MoistureThresholdPct float64
}

// Sensors é o retrato dos sensores no instante da decisão. SoilHealthy é falso quando a leitura
// está ausente, velha ou anômala (travada, fora de faixa); aí o modelo assume (ADR 0005).
type Sensors struct {
	SoilMoisturePct *float64
	SoilHealthy     bool
	AirTempC        *float64
	WindMS          *float64
	Reservoir       safety.Reservoir
}

type DecisionKind string

const (
	DecisionIrrigate DecisionKind = "irrigate"
	DecisionSkip     DecisionKind = "skip"
	DecisionPostpone DecisionKind = "postpone"
	DecisionBlock    DecisionKind = "block"
)

// Decision é explicável: Reasons diz por que, Verdict traz as regras acionadas.
type Decision struct {
	Kind    DecisionKind
	Reasons []string
	Request *Request
	Verdict safety.Verdict
}

// Planner decide se um canteiro deve ser irrigado agora.
type Planner struct {
	Weather     WeatherProvider
	Alerts      AlertSink
	Limits      safety.Limits
	Horizon     time.Duration
	MaxDuration time.Duration // teto de tempo por rega, alinhado ao do firmware
	Log         *slog.Logger
}

// Decide combina balanço hídrico, sensor, previsão e regras de segurança. A previsão é opcional:
// se o provedor falhar, segue com as medições locais.
func (p *Planner) Decide(ctx context.Context, bed Bed, bal agronomy.Balance, s Sensors, now time.Time) (Decision, error) {
	d := Decision{}
	need, why := triggered(bed, bal, s)
	if !need {
		d.Kind, d.Reasons = DecisionSkip, []string{why}
		return d, nil
	}

	sctx := safety.Context{Method: bed.Method, AirTempC: s.AirTempC, WindMS: s.WindMS, Reservoir: s.Reservoir}
	if p.Weather != nil {
		if f, err := p.Weather.Forecast(ctx, bed.LatDeg, bed.LonDeg, now, p.Horizon); err != nil {
			p.Log.Warn("previsão indisponível, seguindo só com medições locais", "bed", bed.ID, "err", err)
		} else {
			sctx.Forecast = &f
		}
	}

	d.Verdict = safety.Evaluate(p.Limits, sctx, bal.IrrigationDepthMM())
	d.Reasons = []string{why}
	for _, v := range d.Verdict.Violations {
		d.Reasons = append(d.Reasons, string(v.Rule)+": "+v.Detail)
		if v.NeedsAlert && p.Alerts != nil {
			if err := p.Alerts.Raise(ctx, Alert{Tenant: bed.Ref.Tenant, Garden: bed.Ref.Garden, Bed: bed.ID,
				Rule: string(v.Rule), Severity: SeverityWarning, Message: v.Detail, At: now}); err != nil {
				return d, fmt.Errorf("alerta: %w", err)
			}
		}
	}
	switch d.Verdict.Action {
	case safety.Block:
		d.Kind = DecisionBlock
		return d, nil
	case safety.Postpone:
		d.Kind = DecisionPostpone
		return d, nil
	}

	liters := agronomy.VolumeLiters(d.Verdict.DepthMM, bed.AreaM2)
	dur := time.Duration(liters / bed.FlowLpm * float64(time.Minute))
	if p.MaxDuration > 0 && dur > p.MaxDuration {
		dur = p.MaxDuration
		liters = bed.FlowLpm * dur.Minutes()
	}
	d.Kind = DecisionIrrigate
	d.Request = &Request{Ref: bed.Ref, BedID: bed.ID, Duration: dur.Round(time.Second), TargetL: math.Round(liters*100) / 100, ExpectedLpm: bed.FlowLpm}
	return d, nil
}

// triggered aplica a fusão da seção 6.2: sensor saudável tem prioridade; senão o balanço manda.
func triggered(bed Bed, bal agronomy.Balance, s Sensors) (bool, string) {
	if s.SoilHealthy && s.SoilMoisturePct != nil {
		if *s.SoilMoisturePct <= bed.MoistureThresholdPct {
			return true, fmt.Sprintf("sensor: umidade %.0f%% ≤ limite %.0f%%", *s.SoilMoisturePct, bed.MoistureThresholdPct)
		}
		return false, fmt.Sprintf("sensor: umidade %.0f%% acima do limite %.0f%% (tem prioridade sobre o modelo)", *s.SoilMoisturePct, bed.MoistureThresholdPct)
	}
	if bal.NeedsWater() {
		return true, fmt.Sprintf("modelo: déficit %.1f mm ≥ RAW %.1f mm (sensor indisponível ou anômalo)", bal.DepletionMM, bal.RAWMM)
	}
	return false, fmt.Sprintf("modelo: déficit %.1f mm abaixo do RAW %.1f mm", bal.DepletionMM, bal.RAWMM)
}
