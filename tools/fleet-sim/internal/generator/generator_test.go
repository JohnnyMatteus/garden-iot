package generator_test

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/JohnnyMatteus/garden-iot/tools/fleet-sim/internal/generator"
	"github.com/JohnnyMatteus/garden-iot/tools/fleet-sim/internal/profile"
)

var start = time.Date(2026, 11, 21, 6, 0, 0, 0, time.UTC)

func profiles(t *testing.T) map[string]profile.Profile {
	m, err := profile.Load()
	require.NoError(t, err)
	return m
}

func series(t *testing.T, name string, seed uint64, n int) [][]byte {
	node := generator.NewNode(profiles(t)[name], seed, 0, start, time.Minute)
	var out [][]byte
	for i := 0; i < n; i++ {
		out = append(out, node.Batch(i, 0).Payload)
	}
	return out
}

func decode(t *testing.T, raw []byte) map[string]any {
	var m map[string]any
	require.NoError(t, json.Unmarshal(raw, &m))
	return m
}

func TestSameSeedSameReadings(t *testing.T) {
	assert.Equal(t, series(t, "sunny", 42, 30), series(t, "sunny", 42, 30))
	assert.NotEqual(t, series(t, "sunny", 42, 30), series(t, "sunny", 43, 30))
}

func TestSequenceGrowsAndFollowsFirmwareFormat(t *testing.T) {
	for i, raw := range series(t, "sunny", 1, 5) {
		m := decode(t, raw)
		assert.EqualValues(t, i+1, m["seq"])
		assert.Equal(t, "1.0.0", m["fw_version"])
		assert.Contains(t, m, "soil_adc")
		assert.Contains(t, m, "ts")
	}
}

func TestStuckSensorKeepsSoilConstant(t *testing.T) {
	seen := map[float64]bool{}
	temps := map[float64]bool{}
	for _, raw := range series(t, "stuck-sensor", 7, 20) {
		m := decode(t, raw)
		seen[m["soil_adc"].(float64)] = true
		temps[m["air_temp_c"].(float64)] = true
	}
	assert.Len(t, seen, 1)
	assert.Greater(t, len(temps), 1)
}

func TestRainRaisesHumidityAndLowersSoilReading(t *testing.T) {
	avg := func(name, key string) float64 {
		sum := 0.0
		for _, raw := range series(t, name, 3, 60) {
			sum += decode(t, raw)[key].(float64)
		}
		return sum / 60
	}
	assert.Greater(t, avg("rain", "air_rh_pct"), avg("sunny", "air_rh_pct"))
	assert.Less(t, avg("rain", "soil_adc"), avg("sunny", "soil_adc"))
	assert.Greater(t, avg("strong-wind", "wind_ms"), avg("sunny", "wind_ms"))
}
