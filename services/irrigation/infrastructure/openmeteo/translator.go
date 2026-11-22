// Package openmeteo é a camada anticorrupção (ACL) entre a API da Open-Meteo e o domínio de
// irrigação: nomes, unidades e formato do provedor não passam daqui.
package openmeteo

import (
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/JohnnyMatteus/garden-iot/services/irrigation/domain/weather"
)

var ErrUnexpectedResponse = errors.New("resposta da Open-Meteo fora do contrato esperado")

// response é o recorte da resposta que o tradutor entende. A consulta usa timeformat=unixtime,
// timezone=UTC e wind_speed_unit=ms; o tradutor confere as unidades devolvidas em vez de confiar.
type response struct {
	Units  map[string]string `json:"hourly_units"`
	Hourly struct {
		Time   []int64    `json:"time"`
		Temp   []*float64 `json:"temperature_2m"`
		Precip []*float64 `json:"precipitation"`
		Wind   []*float64 `json:"wind_speed_10m"`
	} `json:"hourly"`
}

// WeatherTranslator converte a resposta bruta em weather.Forecast.
type WeatherTranslator struct{}

// Translate considera as horas em [from, from+horizon). Valores nulos (horas sem dado) são ignorados;
// janela sem nenhuma hora é erro, nunca "sem chuva".
func (WeatherTranslator) Translate(raw []byte, from time.Time, horizon time.Duration) (weather.Forecast, error) {
	var r response
	if err := json.Unmarshal(raw, &r); err != nil {
		return weather.Forecast{}, fmt.Errorf("%w: %v", ErrUnexpectedResponse, err)
	}
	for field, want := range map[string]string{"time": "unixtime", "temperature_2m": "°C", "precipitation": "mm", "wind_speed_10m": "m/s"} {
		if got := r.Units[field]; got != want {
			return weather.Forecast{}, fmt.Errorf("%w: unidade de %s é %q, esperado %q", ErrUnexpectedResponse, field, got, want)
		}
	}
	n := len(r.Hourly.Time)
	if len(r.Hourly.Temp) != n || len(r.Hourly.Precip) != n || len(r.Hourly.Wind) != n {
		return weather.Forecast{}, fmt.Errorf("%w: séries com tamanhos diferentes", ErrUnexpectedResponse)
	}

	f := weather.Forecast{From: from, Horizon: horizon}
	hours, tempSeen := 0, false
	end := from.Add(horizon)
	for i, ts := range r.Hourly.Time {
		t := time.Unix(ts, 0).UTC()
		// A hora cheia que contém "from" já está em curso e conta.
		if t.Before(from.Truncate(time.Hour)) || !t.Before(end) {
			continue
		}
		hours++
		if p := r.Hourly.Precip[i]; p != nil {
			f.RainMM += *p
		}
		if w := r.Hourly.Wind[i]; w != nil && *w > f.MaxWindMS {
			f.MaxWindMS = *w
		}
		if v := r.Hourly.Temp[i]; v != nil && (!tempSeen || *v < f.MinTempC) {
			f.MinTempC, tempSeen = *v, true
		}
	}
	if hours == 0 || !tempSeen {
		return weather.Forecast{}, fmt.Errorf("%w: nenhuma hora válida na janela", ErrUnexpectedResponse)
	}
	return f, nil
}
