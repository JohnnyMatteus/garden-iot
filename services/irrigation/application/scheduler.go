package application

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/JohnnyMatteus/garden-iot/services/irrigation/domain/agronomy"
)

// Scheduler mantém o balanço hídrico por canteiro e dispara a saga quando o planejador manda.
type Scheduler struct {
	Planner *Planner
	Saga    *Saga
	Sensors SensorSource
	Climate ClimateSource
	Beds    []Bed
	Clock   Clock
	Log     *slog.Logger
	// DefaultWindMS é usado quando o nó não mede vento (firmware v1): subestima ET₀ em dia ventoso.
	DefaultWindMS float64

	mu    sync.Mutex
	state map[string]*bedState
}

type bedState struct {
	balance     agronomy.Balance
	irrigatedMM float64 // lâmina aplicada no dia corrente, descontada no fechamento do dia
}

func (s *Scheduler) bedState(b Bed) *bedState {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.state == nil {
		s.state = map[string]*bedState{}
	}
	st, ok := s.state[b.ID]
	if !ok {
		st = &bedState{balance: agronomy.NewBalance(b.Crop, b.Soil)}
		s.state[b.ID] = st
	}
	return st
}

// Balance devolve o balanço atual do canteiro, para diagnóstico.
func (s *Scheduler) Balance(bedID string) (agronomy.Balance, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	st, ok := s.state[bedID]
	if !ok {
		return agronomy.Balance{}, false
	}
	return st.balance, true
}

// CloseDay fecha o dia: ET₀ do clima medido, ETc pelo Kc do estágio e déficit do canteiro.
// Dia sem dados suficientes mantém o déficit (e registra), em vez de inventar clima.
func (s *Scheduler) CloseDay(ctx context.Context, day time.Time) {
	for _, bed := range s.Beds {
		st := s.bedState(bed)
		w, err := s.Climate.Day(ctx, bed, day)
		if err != nil {
			s.Log.Warn("dia sem clima suficiente, balanço mantido", "bed", bed.ID, "err", err)
			continue
		}
		if w.WindMS == 0 {
			w.WindMS, w.WindHeightM = s.DefaultWindMS, 2
		}
		et0, err := agronomy.ET0(w)
		if err != nil {
			s.Log.Warn("ET₀ não calculada", "bed", bed.ID, "err", err)
			continue
		}
		kc, err := bed.Crop.Kc(bed.Stage, bed.StageProgress)
		if err != nil {
			s.Log.Warn("Kc inválido", "bed", bed.ID, "err", err)
			continue
		}
		s.mu.Lock()
		st.balance = st.balance.Apply(agronomy.Day{ET0MM: et0, Kc: kc, IrrigationMM: st.irrigatedMM})
		st.irrigatedMM = 0
		dep := st.balance.DepletionMM
		s.mu.Unlock()
		s.Log.Info("dia fechado", "bed", bed.ID, "et0_mm", et0, "kc", kc, "deficit_mm", dep)
	}
}

// Tick avalia todos os canteiros e executa as regas aprovadas, uma de cada vez por canteiro.
func (s *Scheduler) Tick(ctx context.Context) {
	for _, bed := range s.Beds {
		if err := s.tickBed(ctx, bed); err != nil {
			s.Log.Error("falha ao processar canteiro", "bed", bed.ID, "err", err)
		}
	}
}

func (s *Scheduler) tickBed(ctx context.Context, bed Bed) error {
	now := s.Clock.Now()
	sensors, err := s.Sensors.Snapshot(ctx, bed, now)
	if err != nil {
		return fmt.Errorf("sensores: %w", err)
	}
	st := s.bedState(bed)
	s.mu.Lock()
	bal := st.balance
	s.mu.Unlock()

	d, err := s.Planner.Decide(ctx, bed, bal, sensors, now)
	if err != nil {
		return err
	}
	s.Log.Info("decisão de rega", "bed", bed.ID, "decision", d.Kind, "reasons", d.Reasons)
	if d.Kind != DecisionIrrigate {
		return nil
	}
	out, err := s.Saga.Run(ctx, *d.Request)
	s.mu.Lock()
	st.irrigatedMM += out.Liters / bed.AreaM2
	s.mu.Unlock()
	s.Log.Info("rega encerrada", "bed", bed.ID, "status", out.Status, "liters", out.Liters, "reason", out.Reason)
	return err
}

// Run executa Tick a cada interval e CloseDay na virada do dia (UTC) até o contexto terminar.
func (s *Scheduler) Run(ctx context.Context, interval time.Duration) {
	day := s.Clock.Now().UTC().Truncate(24 * time.Hour)
	for {
		s.Tick(ctx)
		if err := s.Clock.Sleep(ctx, interval); err != nil {
			return
		}
		if today := s.Clock.Now().UTC().Truncate(24 * time.Hour); today.After(day) {
			s.CloseDay(ctx, day)
			day = today
		}
	}
}
