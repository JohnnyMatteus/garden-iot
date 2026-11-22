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
	"github.com/JohnnyMatteus/garden-iot/services/irrigation/domain/safety"
	"github.com/JohnnyMatteus/garden-iot/services/irrigation/domain/weather"
)

type fakeWeather struct {
	f   weather.Forecast
	err error
}

func (w fakeWeather) Forecast(context.Context, float64, float64, time.Time, time.Duration) (weather.Forecast, error) {
	return w.f, w.err
}

func fp(v float64) *float64 { return &v }

var bed = application.Bed{ID: "b1", Ref: ref, Crop: agronomy.Crops["tomato"], Soil: agronomy.LoamSoil,
	Stage: agronomy.StageMid, AreaM2: 2, Method: safety.Drip, FlowLpm: 4, MoistureThresholdPct: 35}

func planner(w application.WeatherProvider, sk *sinks) *application.Planner {
	return &application.Planner{Weather: w, Alerts: sk, Limits: safety.DefaultLimits(), Horizon: 12 * time.Hour,
		MaxDuration: 30 * time.Minute, Log: slog.New(slog.NewTextHandler(io.Discard, nil))}
}

func dry() agronomy.Balance {
	b := agronomy.NewBalance(bed.Crop, bed.Soil)
	b.DepletionMM = 41.4 // ≥ RAW 39,2
	return b
}

var now = time.Date(2026, 11, 22, 6, 0, 0, 0, time.UTC)

func TestModelTriggersWhenSensorUnhealthy(t *testing.T) {
	d, err := planner(fakeWeather{f: weather.Forecast{MinTempC: 15}}, &sinks{}).Decide(context.Background(), bed, dry(),
		application.Sensors{AirTempC: fp(20)}, now)
	require.NoError(t, err)
	assert.Equal(t, application.DecisionIrrigate, d.Kind)
	require.NotNil(t, d.Request)
	// 41,4 mm × 2 m² = 82,8 L; a 4 L/min são 20,7 min.
	assert.InDelta(t, 82.8, d.Request.TargetL, 0.01)
	assert.Equal(t, 20*time.Minute+42*time.Second, d.Request.Duration)
	assert.Contains(t, d.Reasons[0], "modelo")
}

func TestHealthySensorOverridesModel(t *testing.T) {
	d, err := planner(nil, &sinks{}).Decide(context.Background(), bed, dry(),
		application.Sensors{SoilMoisturePct: fp(60), SoilHealthy: true}, now)
	require.NoError(t, err)
	assert.Equal(t, application.DecisionSkip, d.Kind, "modelo pede água, mas o solo medido está úmido")
}

func TestHealthySensorTriggersEvenIfModelIsWet(t *testing.T) {
	wet := agronomy.NewBalance(bed.Crop, bed.Soil)
	wet.DepletionMM = 10
	d, err := planner(nil, &sinks{}).Decide(context.Background(), bed, wet,
		application.Sensors{SoilMoisturePct: fp(30), SoilHealthy: true, AirTempC: fp(20)}, now)
	require.NoError(t, err)
	assert.Equal(t, application.DecisionIrrigate, d.Kind)
	assert.InDelta(t, 20, d.Request.TargetL, 0.01)
}

func TestNoNeedSkips(t *testing.T) {
	d, _ := planner(nil, &sinks{}).Decide(context.Background(), bed, agronomy.NewBalance(bed.Crop, bed.Soil), application.Sensors{}, now)
	assert.Equal(t, application.DecisionSkip, d.Kind)
	assert.Nil(t, d.Request)
}

func TestForecastRainPostpones(t *testing.T) {
	w := fakeWeather{f: weather.Forecast{RainMM: 9, MinTempC: 15, Horizon: 12 * time.Hour}}
	d, err := planner(w, &sinks{}).Decide(context.Background(), bed, dry(), application.Sensors{}, now)
	require.NoError(t, err)
	assert.Equal(t, application.DecisionPostpone, d.Kind)
	assert.Nil(t, d.Request)
}

func TestFrostBlocksAndRaisesAlert(t *testing.T) {
	sk := &sinks{}
	d, err := planner(nil, sk).Decide(context.Background(), bed, dry(), application.Sensors{AirTempC: fp(0.5)}, now)
	require.NoError(t, err)
	assert.Equal(t, application.DecisionBlock, d.Kind)
	require.Len(t, sk.alerts, 1)
	assert.Equal(t, "frost", sk.alerts[0].Rule)
}

func TestWeatherFailureDegradesToLocalMeasurements(t *testing.T) {
	d, err := planner(fakeWeather{err: errors.New("503")}, &sinks{}).Decide(context.Background(), bed, dry(),
		application.Sensors{AirTempC: fp(20)}, now)
	require.NoError(t, err)
	assert.Equal(t, application.DecisionIrrigate, d.Kind)
}

func TestDurationCappedAtMax(t *testing.T) {
	slow := bed
	slow.FlowLpm = 1 // 82,8 L levariam 82,8 min
	d, err := planner(nil, &sinks{}).Decide(context.Background(), slow, dry(), application.Sensors{}, now)
	require.NoError(t, err)
	assert.Equal(t, 30*time.Minute, d.Request.Duration)
	assert.InDelta(t, 30, d.Request.TargetL, 0.01)
}
