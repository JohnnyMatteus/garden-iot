package openmeteo_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/JohnnyMatteus/garden-iot/services/irrigation/infrastructure/openmeteo"
)

// Respostas gravadas no formato real da Open-Meteo (timeformat=unixtime, UTC, m/s); 22/11/2026 06:00 UTC.
var from = time.Unix(1763791200, 0).UTC()

func load(t *testing.T, name string) []byte {
	t.Helper()
	b, err := os.ReadFile("testdata/" + name + ".json")
	require.NoError(t, err)
	return b
}

func TestTranslateRain(t *testing.T) {
	f, err := openmeteo.WeatherTranslator{}.Translate(load(t, "rain"), from, 12*time.Hour)
	require.NoError(t, err)
	assert.InDelta(t, 5.2, f.RainMM, 1e-9, "0,4 + 1,2 + 2,5 + 1,1")
	assert.Equal(t, 12*time.Hour, f.Horizon)
	assert.InDelta(t, 2.6, f.MaxWindMS, 1e-9, "1,5 + 0,1 × 11 na última hora da janela")
}

func TestTranslateWindowLimitsHours(t *testing.T) {
	// Só 3 h: a chuva das horas 3 a 6 fica de fora (horas 0, 1 e 2).
	f, err := openmeteo.WeatherTranslator{}.Translate(load(t, "rain"), from, 3*time.Hour)
	require.NoError(t, err)
	assert.Equal(t, 0.0, f.RainMM)
}

func TestTranslateClear(t *testing.T) {
	f, err := openmeteo.WeatherTranslator{}.Translate(load(t, "clear"), from, 24*time.Hour)
	require.NoError(t, err)
	assert.Equal(t, 0.0, f.RainMM)
	assert.Less(t, f.MinTempC, 10.0)
}

func TestTranslateColdWindy(t *testing.T) {
	f, err := openmeteo.WeatherTranslator{}.Translate(load(t, "cold_windy"), from, 12*time.Hour)
	require.NoError(t, err)
	assert.Less(t, f.MinTempC, 2.0)
	assert.Equal(t, 7.5, f.MaxWindMS)
}

func TestTranslateRejectsWrongUnits(t *testing.T) {
	_, err := openmeteo.WeatherTranslator{}.Translate(load(t, "kmh"), from, 12*time.Hour)
	assert.ErrorIs(t, err, openmeteo.ErrUnexpectedResponse)
	assert.ErrorContains(t, err, "km/h")
}

func TestTranslateRejectsGarbageAndEmptyWindow(t *testing.T) {
	_, err := openmeteo.WeatherTranslator{}.Translate([]byte(`{"error":true}`), from, time.Hour)
	assert.ErrorIs(t, err, openmeteo.ErrUnexpectedResponse)
	_, err = openmeteo.WeatherTranslator{}.Translate(load(t, "clear"), from.Add(10*24*time.Hour), 6*time.Hour)
	assert.ErrorIs(t, err, openmeteo.ErrUnexpectedResponse, "sem dados na janela nunca vira 'sem chuva'")
}

func TestClientUsesRecordedResponse(t *testing.T) {
	var query string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		query = r.URL.RawQuery
		_, _ = w.Write(load(t, "rain"))
	}))
	defer srv.Close()
	c := openmeteo.NewClient()
	c.BaseURL = srv.URL
	f, err := c.Forecast(context.Background(), -23.55, -46.63, from, 12*time.Hour)
	require.NoError(t, err)
	assert.InDelta(t, 5.2, f.RainMM, 1e-9)
	assert.Contains(t, query, "wind_speed_unit=ms")
	assert.Contains(t, query, "timeformat=unixtime")
}

func TestClientFailsOnServerError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(503) }))
	defer srv.Close()
	c := openmeteo.NewClient()
	c.BaseURL = srv.URL
	_, err := c.Forecast(context.Background(), 0, 0, from, time.Hour)
	assert.Error(t, err)
}
