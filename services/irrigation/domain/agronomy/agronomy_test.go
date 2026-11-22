package agronomy_test

import (
	"math"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/JohnnyMatteus/garden-iot/services/irrigation/domain/agronomy"
)

// Exemplo 18 da FAO-56 (Uccle, Bruxelas, 6 de julho): ET₀ publicado = 3,9 mm/dia.
var uccle = agronomy.DailyWeather{
	TminC: 12.3, TmaxC: 21.5, RHminPct: 63, RHmaxPct: 84,
	WindMS: 10 / 3.6, WindHeightM: 10, SolarMJm2Day: 22.07,
	LatitudeDeg: 50.8, ElevationM: 100, DayOfYear: 187,
}

func TestSaturationVaporPressureMatchesFAOTable(t *testing.T) {
	// Anexo 2 da FAO-56: eₛ(20 °C) = 2,338 kPa; eₛ(12,3 °C) = 1,431; eₛ(21,5 °C) = 2,564.
	assert.InDelta(t, 2.338, agronomy.SaturationVaporPressure(20), 0.001)
	assert.InDelta(t, 1.431, agronomy.SaturationVaporPressure(12.3), 0.001)
	assert.InDelta(t, 2.564, agronomy.SaturationVaporPressure(21.5), 0.001)
}

func TestWindAt2m(t *testing.T) {
	// 10 km/h a 10 m → 2,078 m/s a 2 m (exemplo 18); a 2 m não muda.
	assert.InDelta(t, 2.078, agronomy.WindAt2m(10/3.6, 10), 0.001)
	assert.Equal(t, 3.0, agronomy.WindAt2m(3, 2))
	assert.Equal(t, 3.0, agronomy.WindAt2m(3, 0))
}

func TestET0FAOExample18(t *testing.T) {
	et0, err := agronomy.ET0(uccle)
	require.NoError(t, err)
	assert.InDelta(t, 3.9, et0, 0.05, "publicado: 3,9 mm/dia")
	assert.InDelta(t, 3.88, et0, 0.01, "calculado à mão com os mesmos passos (ver roteiro)")
}

func TestET0WindIncreasesDemand(t *testing.T) {
	calm := uccle
	calm.WindMS = 0.5
	windy := uccle
	windy.WindMS = 6
	a, _ := agronomy.ET0(calm)
	b, _ := agronomy.ET0(windy)
	assert.Greater(t, b, a)
}

func TestET0NeverNegative(t *testing.T) {
	cold := agronomy.DailyWeather{TminC: -5, TmaxC: -2, RHminPct: 90, RHmaxPct: 100, SolarMJm2Day: 0,
		LatitudeDeg: 50, DayOfYear: 355}
	et0, err := agronomy.ET0(cold)
	require.NoError(t, err)
	assert.GreaterOrEqual(t, et0, 0.0)
}

func TestET0RejectsInvalid(t *testing.T) {
	bad := uccle
	bad.TmaxC = 5
	_, err := agronomy.ET0(bad)
	assert.ErrorIs(t, err, agronomy.ErrInvalidWeather)
	bad = uccle
	bad.RHminPct = 95
	_, err = agronomy.ET0(bad)
	assert.ErrorIs(t, err, agronomy.ErrInvalidWeather)
}

func TestWattsToMJPerDay(t *testing.T) {
	assert.InDelta(t, 22.0, agronomy.WattsToMJPerDay(254.63), 0.01)
}

func TestKcInterpolatesByStage(t *testing.T) {
	tomato := agronomy.Crops["tomato"]
	cases := []struct {
		stage    agronomy.Stage
		progress float64
		want     float64
	}{
		{agronomy.StageInitial, 0.9, 0.60},
		{agronomy.StageDevelopment, 0, 0.60},
		{agronomy.StageDevelopment, 0.5, 0.875}, // (0,60 + 1,15) / 2
		{agronomy.StageMid, 0, 1.15},
		{agronomy.StageLate, 1, 0.80},
		{agronomy.StageLate, 0.5, 0.975}, // (1,15 + 0,80) / 2
		{agronomy.StageDevelopment, 7, 1.15},
	}
	for _, c := range cases {
		got, err := tomato.Kc(c.stage, c.progress)
		require.NoError(t, err)
		assert.InDelta(t, c.want, got, 1e-9, "%s %.1f", c.stage, c.progress)
	}
	_, err := tomato.Kc("flor", 0)
	assert.Error(t, err)
}

func TestTAWAndRAW(t *testing.T) {
	// Tomate: Zr 0,70 m, solo franco (0,27 − 0,13): TAW = 1000 × 0,14 × 0,70 = 98 mm; RAW = 0,40 × 98 = 39,2 mm.
	tomato := agronomy.Crops["tomato"]
	assert.InDelta(t, 98, tomato.TAW(agronomy.LoamSoil), 1e-9)
	assert.InDelta(t, 39.2, tomato.RAW(agronomy.LoamSoil), 1e-9)
}

func TestEffectiveRain(t *testing.T) {
	assert.Equal(t, 0.0, agronomy.EffectiveRain(1.5))
	assert.Equal(t, 0.0, agronomy.EffectiveRain(2))
	assert.InDelta(t, 6.0, agronomy.EffectiveRain(8), 1e-9)
}

func TestWaterBalanceWeek(t *testing.T) {
	// Tomate em fase intermediária (Kc 1,15), solo franco: TAW 98, RAW 39,2.
	// Dia 1: ET₀ 4,0 → ETc 4,60 ; déficit 4,60
	// Dia 2: ET₀ 5,0 → ETc 5,75 ; déficit 10,35
	// Dia 3: ET₀ 4,0, chuva 8 mm (efetiva 6) → 10,35 + 4,60 − 6 = 8,95
	// Dia 4: ET₀ 6,0 → ETc 6,90 ; rega de 5 mm → 8,95 + 6,90 − 5 = 10,85
	// Dia 5: chuva 30 mm (efetiva 28) → 10,85 + ETc(4,0)=4,60 − 28 < 0 → 0 (percolação)
	b := agronomy.NewBalance(agronomy.Crops["tomato"], agronomy.LoamSoil)
	days := []struct {
		d    agronomy.Day
		want float64
	}{
		{agronomy.Day{ET0MM: 4, Kc: 1.15}, 4.60},
		{agronomy.Day{ET0MM: 5, Kc: 1.15}, 10.35},
		{agronomy.Day{ET0MM: 4, Kc: 1.15, RainMM: 8}, 8.95},
		{agronomy.Day{ET0MM: 6, Kc: 1.15, IrrigationMM: 5}, 10.85},
		{agronomy.Day{ET0MM: 4, Kc: 1.15, RainMM: 30}, 0},
	}
	for i, c := range days {
		b = b.Apply(c.d)
		assert.InDelta(t, c.want, b.DepletionMM, 1e-9, "dia %d", i+1)
	}
	assert.False(t, b.NeedsWater())
}

func TestWaterBalanceTriggerAndCap(t *testing.T) {
	b := agronomy.NewBalance(agronomy.Crops["tomato"], agronomy.LoamSoil)
	for i := 0; i < 8; i++ { // 8 × 5,175 = 41,4 mm
		b = b.Apply(agronomy.Day{ET0MM: 4.5, Kc: 1.15})
	}
	assert.InDelta(t, 41.4, b.DepletionMM, 1e-9)
	assert.True(t, b.NeedsWater(), "41,4 ≥ RAW 39,2")
	assert.InDelta(t, 41.4, b.IrrigationDepthMM(), 1e-9)

	for i := 0; i < 40; i++ {
		b = b.Apply(agronomy.Day{ET0MM: 8, Kc: 1.15})
	}
	assert.Equal(t, 98.0, b.DepletionMM, "déficit nunca passa de TAW")
}

func TestVolumeLiters(t *testing.T) {
	assert.InDelta(t, 82.8, agronomy.VolumeLiters(41.4, 2), 1e-9)
	assert.False(t, math.IsNaN(agronomy.VolumeLiters(0, 0)))
}
