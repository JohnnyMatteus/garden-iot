package normalizer_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/JohnnyMatteus/garden-iot/services/bridge/internal/acl/devicepayload"
	"github.com/JohnnyMatteus/garden-iot/services/bridge/internal/domain"
	"github.com/JohnnyMatteus/garden-iot/services/bridge/internal/infrastructure/normalizer"
	"github.com/JohnnyMatteus/garden-iot/services/bridge/internal/readings"
)

var cal = devicepayload.Calibration{SoilDry: 3000, SoilSaturated: 1200}

func batch(payload string, fw string) readings.Batch {
	return readings.Batch{Tenant: "t1", Garden: "h1", Node: "n1", Sequence: 42, FwVersion: fw, ReadAt: time.Unix(1732190000, 0), Payload: []byte(payload)}
}

func TestNormalizesFirmwareV1(t *testing.T) {
	rows, err := normalizer.New(cal).Normalize(batch(
		`{"fw":"1.0.0","seq":42,"ts":1732190000,"r":[{"id":"soil-1","k":"soil_adc","v":2100},{"id":"air-1","k":"air_temp_c","v":23.5},{"id":"air-1","k":"air_rh_pct","v":61.2},{"id":"light-1","k":"light_lux","v":12000}]}`, "1.0.0"))
	require.NoError(t, err)

	byKind := map[string]readings.Reading{}
	for _, r := range rows {
		byKind[r.Kind] = r
		assert.Equal(t, "t1", r.Tenant)
		assert.EqualValues(t, 42, r.Sequence)
	}
	assert.InDelta(t, 50, byKind["soil_moisture_vwc_pct"].Value, 0.01)
	assert.Equal(t, "estimated", byKind["solar_radiation_w_m2"].Quality)
	assert.Equal(t, "ok", byKind["air_temp_c"].Quality)
	assert.Len(t, rows, 5)
}

func TestUnknownFirmwareIsInvalidWithReason(t *testing.T) {
	_, err := normalizer.New(cal).Normalize(batch(`{}`, "9.0.0"))
	ie, ok := domain.AsInvalid(err)
	require.True(t, ok)
	assert.Equal(t, domain.ReasonFirmware, ie.Reason)
}

func TestBrokenPayloadIsInvalid(t *testing.T) {
	_, err := normalizer.New(cal).Normalize(batch(`{`, "1.0.0"))
	_, ok := domain.AsInvalid(err)
	assert.True(t, ok)
}
