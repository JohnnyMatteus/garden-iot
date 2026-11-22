package safety_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/JohnnyMatteus/garden-iot/services/irrigation/domain/safety"
	"github.com/JohnnyMatteus/garden-iot/services/irrigation/domain/weather"
)

func f(v float64) *float64 { return &v }

var lim = safety.DefaultLimits()

func rules(v safety.Verdict) []safety.Rule {
	var out []safety.Rule
	for _, x := range v.Violations {
		out = append(out, x.Rule)
	}
	return out
}

func TestAllowsCalmWarmDay(t *testing.T) {
	v := safety.Evaluate(lim, safety.Context{Method: safety.Drip, AirTempC: f(22), WindMS: f(1), Reservoir: safety.ReservoirOK}, 10)
	assert.Equal(t, safety.Allow, v.Action)
	assert.Equal(t, 10.0, v.DepthMM)
	assert.Empty(t, v.Violations)
}

func TestPostponesOnForecastRain(t *testing.T) {
	fc := &weather.Forecast{RainMM: 6, Horizon: 12 * time.Hour, MinTempC: 15}
	v := safety.Evaluate(lim, safety.Context{Method: safety.Drip, Forecast: fc}, 10)
	assert.Equal(t, safety.Postpone, v.Action)
	assert.Equal(t, []safety.Rule{safety.RuleRain}, rules(v))
}

func TestReducesDepthOnLightForecastRain(t *testing.T) {
	// 4 mm previstos → 2 mm efetivos (descontada a interceptação) → lâmina 10 − 2 = 8.
	fc := &weather.Forecast{RainMM: 4, MinTempC: 15}
	v := safety.Evaluate(lim, safety.Context{Method: safety.Drip, Forecast: fc}, 10)
	assert.Equal(t, safety.Allow, v.Action)
	assert.InDelta(t, 8.0, v.DepthMM, 1e-9)
}

func TestPostponesWhenReducedDepthTooSmall(t *testing.T) {
	fc := &weather.Forecast{RainMM: 4, MinTempC: 15}
	v := safety.Evaluate(lim, safety.Context{Method: safety.Drip, Forecast: fc}, 3.5) // 3,5 − 2 = 1,5 < 2
	assert.Equal(t, safety.Postpone, v.Action)
	assert.Equal(t, []safety.Rule{safety.RuleMinDepth}, rules(v))
}

func TestStrongWindOnlyMattersForSprinkler(t *testing.T) {
	sprinkler := safety.Evaluate(lim, safety.Context{Method: safety.Sprinkler, WindMS: f(5.1)}, 10)
	assert.Equal(t, safety.Postpone, sprinkler.Action)
	assert.Equal(t, []safety.Rule{safety.RuleWind}, rules(sprinkler))
	assert.False(t, sprinkler.Violations[0].NeedsAlert)

	drip := safety.Evaluate(lim, safety.Context{Method: safety.Drip, WindMS: f(9)}, 10)
	assert.Equal(t, safety.Allow, drip.Action)

	edge := safety.Evaluate(lim, safety.Context{Method: safety.Sprinkler, WindMS: f(5)}, 10)
	assert.Equal(t, safety.Allow, edge.Action, "exatamente no limite ainda rega")
}

func TestWindFallsBackToForecastWhenNodeHasNoAnemometer(t *testing.T) {
	fc := &weather.Forecast{MaxWindMS: 8, MinTempC: 15}
	v := safety.Evaluate(lim, safety.Context{Method: safety.Sprinkler, Forecast: fc}, 10)
	assert.Equal(t, safety.Postpone, v.Action)
}

func TestFrostBlocksAndAlerts(t *testing.T) {
	v := safety.Evaluate(lim, safety.Context{Method: safety.Drip, AirTempC: f(1.5)}, 10)
	assert.Equal(t, safety.Block, v.Action)
	require.Len(t, v.Violations, 1)
	assert.Equal(t, safety.RuleFrost, v.Violations[0].Rule)
	assert.True(t, v.Violations[0].NeedsAlert)

	edge := safety.Evaluate(lim, safety.Context{Method: safety.Drip, AirTempC: f(2)}, 10)
	assert.Equal(t, safety.Allow, edge.Action, "2 °C não é geada (regra é < 2)")
}

func TestForecastFrostBlocksEvenIfNowIsWarm(t *testing.T) {
	fc := &weather.Forecast{MinTempC: 0.5}
	v := safety.Evaluate(lim, safety.Context{Method: safety.Drip, AirTempC: f(8), Forecast: fc}, 10)
	assert.Equal(t, safety.Block, v.Action)
}

func TestLowReservoirBlocksAndAlerts(t *testing.T) {
	v := safety.Evaluate(lim, safety.Context{Method: safety.Drip, Reservoir: safety.ReservoirLow}, 10)
	assert.Equal(t, safety.Block, v.Action)
	assert.True(t, v.Violations[0].NeedsAlert)
}

func TestMostRestrictiveActionWins(t *testing.T) {
	v := safety.Evaluate(lim, safety.Context{Method: safety.Sprinkler, WindMS: f(8), AirTempC: f(0), Reservoir: safety.ReservoirLow}, 10)
	assert.Equal(t, safety.Block, v.Action)
	assert.ElementsMatch(t, []safety.Rule{safety.RuleReservoir, safety.RuleFrost, safety.RuleWind}, rules(v))
}

func TestCheckFlowTolerance(t *testing.T) {
	cases := []struct {
		name      string
		measured  float64
		violation bool
	}{
		{"exata", 2.0, false},
		{"limite superior", 2.6, false},
		{"limite inferior", 1.4, false},
		{"vazamento", 2.7, true},
		{"entupimento", 1.3, true},
		{"sem vazão", 0, true},
	}
	for _, c := range cases {
		got := safety.CheckFlow(lim, 2.0, c.measured)
		assert.Equal(t, c.violation, got != nil, c.name)
		if got != nil {
			assert.Equal(t, safety.Block, got.Action)
			assert.True(t, got.NeedsAlert)
		}
	}
	assert.Contains(t, safety.CheckFlow(lim, 2, 3).Detail, "vazamento")
	assert.Contains(t, safety.CheckFlow(lim, 2, 1).Detail, "entupimento")
	assert.NotNil(t, safety.CheckFlow(lim, 0, 1), "vazão esperada inválida falha fechada")
}
