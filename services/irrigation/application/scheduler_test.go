package application_test

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/JohnnyMatteus/garden-iot/services/irrigation/application"
	"github.com/JohnnyMatteus/garden-iot/services/irrigation/domain/agronomy"
)

type fakeSensors struct{ s application.Sensors }

func (f fakeSensors) Snapshot(context.Context, application.Bed, time.Time) (application.Sensors, error) {
	return f.s, nil
}

type fakeClimate struct {
	w   agronomy.DailyWeather
	err error
}

func (f fakeClimate) Day(context.Context, application.Bed, time.Time) (agronomy.DailyWeather, error) {
	return f.w, f.err
}

func scheduler(t *testing.T, climate application.ClimateSource, sensors application.Sensors) (*application.Scheduler, *fakeNode, *sinks) {
	t.Helper()
	saga, node, sk := setup()
	saga.Cfg.ConfirmTimeout = 15 * time.Second
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	return &application.Scheduler{
		Planner: planner(nil, sk), Saga: saga, Sensors: fakeSensors{sensors}, Climate: climate,
		Beds: []application.Bed{bed}, Clock: saga.Clock, Log: log, DefaultWindMS: 2,
	}, node, sk
}

func TestCloseDayAccumulatesDeficitFromMeasuredClimate(t *testing.T) {
	// Exemplo 18 da FAO-56 (ET₀ 3,88) com Kc intermediário do tomate (1,15): ETc = 4,46 mm.
	w := agronomy.DailyWeather{TminC: 12.3, TmaxC: 21.5, RHminPct: 63, RHmaxPct: 84, WindMS: 10 / 3.6, WindHeightM: 10,
		SolarMJm2Day: 22.07, LatitudeDeg: 50.8, ElevationM: 100, DayOfYear: 187}
	s, _, _ := scheduler(t, fakeClimate{w: w}, application.Sensors{})
	s.CloseDay(context.Background(), now)
	bal, ok := s.Balance("b1")
	require.True(t, ok)
	assert.InDelta(t, 3.88*1.15, bal.DepletionMM, 0.02)
}

func TestCloseDayWithoutClimateKeepsBalance(t *testing.T) {
	s, _, _ := scheduler(t, fakeClimate{err: errors.New("sem leituras")}, application.Sensors{})
	s.CloseDay(context.Background(), now)
	bal, _ := s.Balance("b1")
	assert.Equal(t, 0.0, bal.DepletionMM)
}

func TestTickIrrigatesWhenSensorSaysDry(t *testing.T) {
	s, node, sk := scheduler(t, fakeClimate{}, application.Sensors{SoilMoisturePct: fp(20), SoilHealthy: true, AirTempC: fp(22)})
	// Balanço começa em capacidade de campo (déficit 0): lâmina 0 não vale a rega (mínimo 2 mm).
	s.Tick(context.Background())
	assert.Empty(t, node.calls, "sensor seco mas modelo sem déficit: lâmina abaixo do mínimo adia")
	assert.Empty(t, sk.records)
}

func TestTickRunsSagaAndCreditsIrrigation(t *testing.T) {
	w := agronomy.DailyWeather{TminC: 20, TmaxC: 34, RHminPct: 20, RHmaxPct: 50, WindMS: 3, WindHeightM: 2,
		SolarMJm2Day: 26, LatitudeDeg: -23.5, ElevationM: 760, DayOfYear: 20}
	s, node, sk := scheduler(t, fakeClimate{w: w}, application.Sensors{SoilMoisturePct: fp(20), SoilHealthy: true, AirTempC: fp(22)})
	for i := 0; i < 9; i++ { // dias quentes empurram o déficit além do mínimo da lâmina
		s.CloseDay(context.Background(), now)
	}
	before, _ := s.Balance("b1")
	require.Greater(t, before.DepletionMM, 2.0)

	node.flow = fp(4)
	s.Tick(context.Background())
	require.Len(t, sk.records, 1)
	assert.Equal(t, application.StatusCompleted, sk.records[0].Status)
	assert.Equal(t, []string{"open", "close"}, node.calls)

	// O fechamento do dia desconta o que foi aplicado.
	s.CloseDay(context.Background(), now)
	after, _ := s.Balance("b1")
	assert.Less(t, after.DepletionMM, before.DepletionMM)
}
