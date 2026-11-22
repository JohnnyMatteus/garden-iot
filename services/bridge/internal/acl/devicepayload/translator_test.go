package devicepayload_test

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/johnnysantos/garden-iot/services/bridge/internal/acl/devicepayload"
)

// Os payloads de entrada v1 são os mesmos que os testes do firmware geram (golden em C), então
// uma mudança de formato no firmware quebra este contrato.
const firmwareGoldenDir = "../../../../../firmware/test/golden/v1"

var cal = devicepayload.Calibration{SoilDry: 3000, SoilSaturated: 1200}

func TestContratoPorVersao(t *testing.T) {
	inputs, err := filepath.Glob("testdata/v1/*.input.json")
	require.NoError(t, err)
	golden, err := filepath.Glob(filepath.Join(firmwareGoldenDir, "*.json"))
	require.NoError(t, err)
	require.NotEmpty(t, golden, "golden do firmware não encontrado")
	inputs = append(inputs, golden...)

	reg := devicepayload.NewRegistry()
	for _, in := range inputs {
		name := strings.TrimSuffix(filepath.Base(in), ".json")
		name = strings.TrimSuffix(name, ".input")
		t.Run(name, func(t *testing.T) {
			raw, err := os.ReadFile(in)
			require.NoError(t, err)
			got, err := reg.Translate(raw, cal)
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
	m := translate(t, `{"fw_version":"1.0.0","seq":1,"ts":1732190000,"soil_adc":2100}`)
	require.NotNil(t, m.SoilMoistureVWC)
	assert.InDelta(t, 50, m.SoilMoistureVWC.Value, 1e-9)
	assert.Equal(t, devicepayload.QualityMeasured, m.SoilMoistureVWC.Quality)
}

func TestUmidadeForaDaFaixaSaturaESinalizaSuspeito(t *testing.T) {
	m := translate(t, `{"fw_version":"1.0.0","seq":1,"ts":1732190000,"soil_adc":4095}`)
	assert.Equal(t, 0.0, m.SoilMoistureVWC.Value)
	assert.Equal(t, devicepayload.QualitySuspect, m.SoilMoistureVWC.Quality)

	m = translate(t, `{"fw_version":"1.0.0","seq":1,"ts":1732190000,"soil_adc":1190}`)
	assert.Equal(t, devicepayload.QualityMeasured, m.SoilMoistureVWC.Quality, "dentro da margem")
	assert.Equal(t, 100.0, m.SoilMoistureVWC.Value)
}

func TestLuzViraRadiacaoEstimada(t *testing.T) {
	m := translate(t, `{"fw_version":"1.0.0","seq":1,"ts":1732190000,"lux":10000}`)
	require.NotNil(t, m.SolarRadiation)
	assert.InDelta(t, 79, m.SolarRadiation.Value, 1e-9)
	assert.Equal(t, devicepayload.QualityEstimated, m.SolarRadiation.Quality)
	assert.Equal(t, devicepayload.QualityMeasured, m.IlluminanceLux.Quality)
}

func TestArForaDaFaixaFisicaEhSuspeito(t *testing.T) {
	m := translate(t, `{"fw_version":"1.0.0","seq":1,"ts":1732190000,"air_temp_c":120,"air_rh_pct":55}`)
	assert.Equal(t, devicepayload.QualitySuspect, m.AirTempC.Quality)
	assert.Equal(t, devicepayload.QualityMeasured, m.AirHumidityPct.Quality)
}

func TestVersaoDeFirmwareDesconhecida(t *testing.T) {
	reg := devicepayload.NewRegistry()
	for _, v := range []string{"2.0.0", "0.9.1", "abc"} {
		_, err := reg.Translate([]byte(`{"fw_version":"`+v+`","seq":1,"ts":1}`), cal)
		var unsupported *devicepayload.UnsupportedFirmwareError
		require.True(t, errors.As(err, &unsupported), v)
		assert.Equal(t, v, unsupported.Version)
		assert.Contains(t, err.Error(), v)
	}
}

func TestFirmwareAntigoContinuaAceito(t *testing.T) {
	m := translate(t, `{"fw_version":"1.7.3","seq":9,"ts":1732190000,"dropped":2}`)
	assert.Equal(t, uint32(9), m.Sequence)
	assert.Equal(t, uint32(2), m.Dropped)
}

func TestPayloadInvalido(t *testing.T) {
	reg := devicepayload.NewRegistry()
	for name, raw := range map[string]string{
		"json quebrado":  `{`,
		"sem fw_version": `{"seq":1,"ts":1}`,
		"sem seq":        `{"fw_version":"1.0.0","ts":1}`,
		"sem timestamp":  `{"fw_version":"1.0.0","seq":1,"ts":0}`,
		"tipo errado":    `{"fw_version":"1.0.0","seq":"a","ts":1}`,
	} {
		t.Run(name, func(t *testing.T) {
			_, err := reg.Translate([]byte(raw), cal)
			require.ErrorIs(t, err, devicepayload.ErrInvalidPayload)
		})
	}
}

func TestCalibracaoInvalida(t *testing.T) {
	reg := devicepayload.NewRegistry()
	_, err := reg.Translate([]byte(`{"fw_version":"1.0.0","seq":1,"ts":1,"soil_adc":2000}`),
		devicepayload.Calibration{SoilDry: 1200, SoilSaturated: 3000})
	require.ErrorIs(t, err, devicepayload.ErrInvalidCalibration)
}

func translate(t *testing.T, raw string) devicepayload.Measurements {
	t.Helper()
	m, err := devicepayload.NewRegistry().Translate([]byte(raw), cal)
	require.NoError(t, err)
	return m
}
