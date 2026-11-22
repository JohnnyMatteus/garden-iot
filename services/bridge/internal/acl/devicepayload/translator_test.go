package devicepayload_test

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	brutav1 "github.com/JohnnyMatteus/garden-iot/contracts/gen/go/telemetria/bruta/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/JohnnyMatteus/garden-iot/services/bridge/internal/acl/devicepayload"
)

// Os payloads golden do firmware (gerados pelos testes em C) entram aqui como corpo do lote:
// mudar o formato num lado sem o outro quebra este contrato.
const firmwareGoldenDir = "../../../../../firmware/test/golden/v1"

var cals = devicepayload.StaticCalibrations{
	"no-01/soil-1": {SoilDry: 3000, SoilSaturated: 1200},
}

func envelope(fw string, payload string) *brutav1.TelemetriaBruta {
	return &brutav1.TelemetriaBruta{
		TenantId: "t1", GardenId: "h1", NodeId: "no-01", Sequence: 42, FwVersion: fw,
		Payload: []byte(payload),
	}
}

func TestContratoPorVersao(t *testing.T) {
	inputs, err := filepath.Glob("testdata/v1/*.input.json")
	require.NoError(t, err)
	golden, err := filepath.Glob(filepath.Join(firmwareGoldenDir, "*.json"))
	require.NoError(t, err)
	require.NotEmpty(t, golden, "golden do firmware não encontrado")
	inputs = append(inputs, golden...)

	reg := devicepayload.NewRegistry()
	for _, in := range inputs {
		name := strings.TrimSuffix(strings.TrimSuffix(filepath.Base(in), ".json"), ".input")
		t.Run(name, func(t *testing.T) {
			raw, err := os.ReadFile(in)
			require.NoError(t, err)
			msg := &brutav1.TelemetriaBruta{
				TenantId: "t1", GardenId: "h1", NodeId: "no-01", Sequence: 42, FwVersion: "1.0.0",
				ReadAt: timestamppb.New(time.Unix(1732190000, 0)), Payload: raw,
			}
			got, err := reg.Translate(msg, cals)
			require.NoError(t, err)

			want, err := os.ReadFile(filepath.Join("testdata/v1", name+".expected.json"))
			require.NoError(t, err)
			gotJSON, err := json.Marshal(got)
			require.NoError(t, err)
			assert.JSONEq(t, string(want), string(gotJSON))
		})
	}
}

func TestUmidadeDoSoloCalibrada(t *testing.T) {
	m := translate(t, `{"ts":1732190000,"r":[{"id":"soil-1","k":"soil_adc","v":2100}]}`)
	require.Len(t, m.Readings, 1)
	assert.Equal(t, devicepayload.SoilMoistureVWC, m.Readings[0].Quantity)
	assert.InDelta(t, 50, m.Readings[0].Value, 1e-9)
	assert.Equal(t, devicepayload.QualityMeasured, m.Readings[0].Quality)
}

func TestUmidadeForaDaFaixaSaturaESinalizaSuspeito(t *testing.T) {
	m := translate(t, `{"ts":1,"r":[{"id":"soil-1","k":"soil_adc","v":4095}]}`)
	assert.Equal(t, 0.0, m.Readings[0].Value)
	assert.Equal(t, devicepayload.QualitySuspect, m.Readings[0].Quality)

	m = translate(t, `{"ts":1,"r":[{"id":"soil-1","k":"soil_adc","v":1190}]}`)
	assert.Equal(t, devicepayload.QualityMeasured, m.Readings[0].Quality, "dentro da margem")
	assert.Equal(t, 100.0, m.Readings[0].Value)
}

func TestLuzViraRadiacaoEstimada(t *testing.T) {
	m := translate(t, `{"ts":1,"r":[{"id":"light-1","k":"light_lux","v":10000}]}`)
	require.Len(t, m.Readings, 2)
	assert.Equal(t, devicepayload.IlluminanceLux, m.Readings[0].Quantity)
	assert.Equal(t, devicepayload.QualityMeasured, m.Readings[0].Quality)
	assert.Equal(t, devicepayload.SolarRadiation, m.Readings[1].Quantity)
	assert.InDelta(t, 79, m.Readings[1].Value, 1e-9)
	assert.Equal(t, devicepayload.QualityEstimated, m.Readings[1].Quality)
}

func TestArForaDaFaixaFisicaEhSuspeito(t *testing.T) {
	m := translate(t, `{"ts":1,"r":[{"id":"air-1","k":"air_temp_c","v":120},{"id":"air-1","k":"air_rh_pct","v":55}]}`)
	assert.Equal(t, devicepayload.QualitySuspect, m.Readings[0].Quality)
	assert.Equal(t, devicepayload.QualityMeasured, m.Readings[1].Quality)
}

func TestGrandezaDesconhecidaEIgnorada(t *testing.T) {
	m := translate(t, `{"ts":1,"r":[{"id":"wind-1","k":"wind_ms","v":3}],"extra":true}`)
	assert.Empty(t, m.Readings)
}

func TestVersaoDeFirmwareDesconhecida(t *testing.T) {
	reg := devicepayload.NewRegistry()
	for _, v := range []string{"2.0.0", "0.9.1", "abc"} {
		_, err := reg.Translate(envelope(v, `{"ts":1,"r":[]}`), cals)
		var unsupported *devicepayload.UnsupportedFirmwareError
		require.True(t, errors.As(err, &unsupported), v)
		assert.Equal(t, v, unsupported.Version)
		assert.Contains(t, err.Error(), v)
	}
}

func TestFirmwareAntigoContinuaAceito(t *testing.T) {
	m := translate2(t, "1.7.3", `{"ts":1,"dropped":2,"r":[]}`)
	assert.Equal(t, uint64(42), m.Sequence)
	assert.Equal(t, uint32(2), m.Dropped)
}

func TestPayloadInvalido(t *testing.T) {
	reg := devicepayload.NewRegistry()
	for name, raw := range map[string]string{
		"json quebrado":  `{`,
		"tipo errado":    `{"ts":"a"}`,
		"leitura sem v":  `{"ts":1,"r":[{"id":"air-1","k":"air_temp_c"}]}`,
		"leitura sem id": `{"ts":1,"r":[{"k":"air_temp_c","v":1}]}`,
	} {
		t.Run(name, func(t *testing.T) {
			_, err := reg.Translate(envelope("1.0.0", raw), cals)
			require.ErrorIs(t, err, devicepayload.ErrInvalidPayload)
		})
	}
	_, err := reg.Translate(&brutav1.TelemetriaBruta{FwVersion: "1.0.0", Payload: []byte(`{"ts":0,"r":[]}`)}, cals)
	require.ErrorIs(t, err, devicepayload.ErrInvalidPayload, "sem timestamp")
	_, err = reg.Translate(&brutav1.TelemetriaBruta{Payload: []byte(`{}`)}, cals)
	require.ErrorIs(t, err, devicepayload.ErrInvalidPayload, "sem fw_version")
}

func TestCalibracao(t *testing.T) {
	reg := devicepayload.NewRegistry()
	body := `{"ts":1,"r":[{"id":"soil-1","k":"soil_adc","v":2000}]}`

	_, err := reg.Translate(envelope("1.0.0", body), devicepayload.StaticCalibrations{})
	require.ErrorIs(t, err, devicepayload.ErrCalibrationMissing)

	_, err = reg.Translate(envelope("1.0.0", body), devicepayload.StaticCalibrations{
		"no-01/soil-1": {SoilDry: 1200, SoilSaturated: 3000}})
	require.ErrorIs(t, err, devicepayload.ErrInvalidCalibration)
}

func translate(t *testing.T, payload string) devicepayload.Measurements {
	return translate2(t, "1.0.0", payload)
}

func translate2(t *testing.T, fw, payload string) devicepayload.Measurements {
	t.Helper()
	msg := envelope(fw, payload)
	msg.ReadAt = nil // o ts do corpo vale quando a bridge não preencheu read_at
	m, err := devicepayload.NewRegistry().Translate(msg, cals)
	require.NoError(t, err)
	return m
}
