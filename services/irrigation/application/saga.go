package application

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/JohnnyMatteus/garden-iot/services/irrigation/domain/safety"
)

// SagaConfig traz os tempos da saga. Valores padrão em DefaultSagaConfig.
type SagaConfig struct {
	ConfirmTimeout time.Duration // espera pelo estado reportado após um comando
	SettleTime     time.Duration // vazão estabiliza antes de ser conferida
	PollInterval   time.Duration
	StaleAfter     time.Duration // estado reportado mais velho que isso conta como nó mudo
	CloseRetries   int
	Limits         safety.Limits
}

func DefaultSagaConfig() SagaConfig {
	return SagaConfig{ConfirmTimeout: 15 * time.Second, SettleTime: 5 * time.Second, PollInterval: 5 * time.Second,
		StaleAfter: 30 * time.Second, CloseRetries: 3, Limits: safety.DefaultLimits()}
}

// Request é uma rega já aprovada pelas regras de segurança.
type Request struct {
	Ref         NodeRef
	BedID       string
	Duration    time.Duration // teto de tempo
	TargetL     float64       // volume alvo; 0 desliga o corte por volume
	ExpectedLpm float64
}

type Outcome struct {
	Status  Status
	Liters  float64
	Elapsed time.Duration
	Reason  string
}

// Saga executa: abrir válvula → confirmar vazão → fechar por tempo ou volume → registrar.
// Qualquer falha dispara a compensação (fechar tudo) e um alerta (design doc, seção 7).
type Saga struct {
	Actuator Actuator
	Twin     Twin
	Alerts   AlertSink
	Journal  Journal
	Clock    Clock
	Cfg      SagaConfig
}

var errStep = errors.New("passo da saga falhou")

func fail(format string, a ...any) error {
	return fmt.Errorf("%w: %s", errStep, fmt.Sprintf(format, a...))
}

// Run devolve Outcome com Status compensated (e erro nil) quando a falha foi tratada; erro só
// aparece quando nem o registro ou a compensação puderam ser concluídos.
func (s *Saga) Run(ctx context.Context, req Request) (Outcome, error) {
	started := s.Clock.Now()
	liters, runErr := s.execute(ctx, req, started)
	out := Outcome{Liters: liters, Elapsed: s.Clock.Now().Sub(started)}

	// A compensação e o registro não podem morrer junto com o contexto cancelado (desligamento do serviço).
	bg, cancel := context.WithTimeout(context.WithoutCancel(ctx), 2*s.Cfg.ConfirmTimeout*time.Duration(s.Cfg.CloseRetries+1))
	defer cancel()

	var errs []error
	if runErr != nil {
		out.Status, out.Reason = StatusCompensated, runErr.Error()
		if err := s.compensate(bg, req); err != nil {
			errs = append(errs, err)
		}
		errs = append(errs, s.Alerts.Raise(bg, Alert{
			Tenant: req.Ref.Tenant, Garden: req.Ref.Garden, Bed: req.BedID, Rule: "irrigation_saga",
			Severity: s.severity(errs), Message: out.Reason, At: s.Clock.Now(),
		}))
	} else {
		out.Status = StatusCompleted
	}
	errs = append(errs, s.Journal.Record(bg, Record{Ref: req.Ref, BedID: req.BedID, Status: out.Status,
		Reason: out.Reason, Liters: out.Liters, Started: started, Elapsed: out.Elapsed}))
	return out, errors.Join(errs...)
}

// severity é crítica quando não conseguimos fechar a válvula.
func (s *Saga) severity(compErrs []error) Severity {
	if len(compErrs) > 0 {
		return SeverityCritical
	}
	return SeverityWarning
}

func (s *Saga) execute(ctx context.Context, req Request, started time.Time) (float64, error) {
	cfg := s.Cfg
	if req.Duration <= 0 || req.ExpectedLpm <= 0 {
		return 0, fail("duração %s e vazão esperada %.2f L/min devem ser positivas", req.Duration, req.ExpectedLpm)
	}

	// 1. abrir: grava o desejado antes, para que o reportado divergente seja visível mesmo se o comando se perder.
	if err := s.Twin.SetDesired(ctx, req.Ref, ValveOpen); err != nil {
		return 0, fail("gravar estado desejado: %v", err)
	}
	if err := s.Actuator.Open(ctx, req.Ref, req.Duration); err != nil {
		return 0, fail("abrir válvula: %v", err)
	}
	if err := s.awaitValve(ctx, req.Ref, ValveOpen); err != nil {
		return 0, err
	}

	// 2 e 3. confirmar vazão e acompanhar até o tempo ou volume.
	if err := s.Clock.Sleep(ctx, cfg.SettleTime); err != nil {
		return 0, err
	}
	var liters float64
	last := s.Clock.Now()
	for {
		st, ok := s.Twin.Reported(ctx, req.Ref)
		now := s.Clock.Now()
		switch {
		case !ok || now.Sub(st.At) > cfg.StaleAfter:
			return liters, fail("nó sem estado recente durante a rega")
		case st.Valve != ValveOpen:
			return liters, fail("estado desejado aberto, reportado %s", st.Valve)
		case st.FlowLpm == nil:
			return liters, fail("nó não reportou vazão")
		}
		if v := safety.CheckFlow(cfg.Limits, req.ExpectedLpm, *st.FlowLpm); v != nil {
			return liters, fail("%s", v.Detail)
		}
		liters += *st.FlowLpm * now.Sub(last).Minutes()
		last = now

		elapsed := now.Sub(started)
		if (req.TargetL > 0 && liters >= req.TargetL) || elapsed >= req.Duration {
			break
		}
		wait := min(cfg.PollInterval, req.Duration-elapsed)
		if err := s.Clock.Sleep(ctx, wait); err != nil {
			return liters, err
		}
	}

	// 4. fechar e confirmar.
	if err := s.close(ctx, req.Ref); err != nil {
		return liters, err
	}
	return liters, nil
}

// close fecha com tentativas limitadas e só dá certo com o nó reportando "fechada".
func (s *Saga) close(ctx context.Context, ref NodeRef) error {
	if err := s.Twin.SetDesired(ctx, ref, ValveClosed); err != nil {
		return fail("gravar estado desejado: %v", err)
	}
	var last error
	for i := 0; i < max(1, s.Cfg.CloseRetries); i++ {
		if last = s.Actuator.Close(ctx, ref); last == nil {
			if last = s.awaitValve(ctx, ref, ValveClosed); last == nil {
				return nil
			}
		}
	}
	return fail("fechar válvula após %d tentativas: %v", s.Cfg.CloseRetries, last)
}

// compensate é o "fechar tudo": tenta mesmo que a falha original tenha sido do próprio fechamento.
func (s *Saga) compensate(ctx context.Context, req Request) error {
	if err := s.close(ctx, req.Ref); err != nil {
		return fmt.Errorf("compensação: %w", err)
	}
	return nil
}

// awaitValve espera o reportado alcançar o desejado dentro de ConfirmTimeout.
func (s *Saga) awaitValve(ctx context.Context, ref NodeRef, want Valve) error {
	deadline := s.Clock.Now().Add(s.Cfg.ConfirmTimeout)
	step := max(time.Second, s.Cfg.PollInterval/5)
	for {
		if st, ok := s.Twin.Reported(ctx, ref); ok && st.Valve == want && s.Clock.Now().Sub(st.At) <= s.Cfg.StaleAfter {
			return nil
		}
		if !s.Clock.Now().Before(deadline) {
			return fail("válvula não reportou %s em %s", want, s.Cfg.ConfirmTimeout)
		}
		if err := s.Clock.Sleep(ctx, step); err != nil {
			return err
		}
	}
}
