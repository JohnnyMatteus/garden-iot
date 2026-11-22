// Package agronomy implementa o modelo agronômico da FAO-56: ET₀ de Penman-Monteith,
// coeficiente de cultura e balanço hídrico do canteiro (design doc, seção 6).
package agronomy

import (
	"errors"
	"fmt"
	"math"
)

// DailyWeather são as médias e extremos do dia calculados a partir dos sensores do nó.
type DailyWeather struct {
	TminC, TmaxC float64
	RHminPct     float64
	RHmaxPct     float64
	WindMS       float64 // velocidade média medida à altura WindHeightM
	WindHeightM  float64 // altura do anemômetro; 0 assume 2 m
	SolarMJm2Day float64 // radiação solar incidente Rs
	LatitudeDeg  float64
	ElevationM   float64
	DayOfYear    int
}

var ErrInvalidWeather = errors.New("dados meteorológicos inválidos")

func (w DailyWeather) validate() error {
	switch {
	case w.TmaxC < w.TminC:
		return fmt.Errorf("%w: Tmax %.1f < Tmin %.1f", ErrInvalidWeather, w.TmaxC, w.TminC)
	case w.RHminPct < 0 || w.RHmaxPct > 100 || w.RHminPct > w.RHmaxPct:
		return fmt.Errorf("%w: umidade relativa %.0f–%.0f%%", ErrInvalidWeather, w.RHminPct, w.RHmaxPct)
	case w.WindMS < 0 || w.SolarMJm2Day < 0:
		return fmt.Errorf("%w: vento ou radiação negativos", ErrInvalidWeather)
	case w.DayOfYear < 1 || w.DayOfYear > 366:
		return fmt.Errorf("%w: dia do ano %d", ErrInvalidWeather, w.DayOfYear)
	case math.Abs(w.LatitudeDeg) > 66:
		return fmt.Errorf("%w: latitude %.1f fora da faixa suportada", ErrInvalidWeather, w.LatitudeDeg)
	}
	return nil
}

// SaturationVaporPressure é eₛ(T) em kPa (FAO-56, eq. 11).
func SaturationVaporPressure(tC float64) float64 {
	return 0.6108 * math.Exp(17.27*tC/(tC+237.3))
}

// slopeVaporPressure é Δ em kPa/°C (eq. 13).
func slopeVaporPressure(tC float64) float64 {
	return 4098 * SaturationVaporPressure(tC) / math.Pow(tC+237.3, 2)
}

// WindAt2m converte o vento medido a z metros para 2 m (eq. 47).
func WindAt2m(u, heightM float64) float64 {
	if heightM == 0 || heightM == 2 {
		return u
	}
	return u * 4.87 / math.Log(67.8*heightM-5.42)
}

// extraterrestrialRadiation é Ra em MJ/m²/dia (eq. 21).
func extraterrestrialRadiation(latDeg float64, doy int) float64 {
	const gsc = 0.0820
	phi := latDeg * math.Pi / 180
	j := float64(doy)
	dr := 1 + 0.033*math.Cos(2*math.Pi/365*j)
	decl := 0.409 * math.Sin(2*math.Pi/365*j-1.39)
	ws := math.Acos(-math.Tan(phi) * math.Tan(decl))
	return 24 * 60 / math.Pi * gsc * dr *
		(ws*math.Sin(phi)*math.Sin(decl) + math.Cos(phi)*math.Cos(decl)*math.Sin(ws))
}

// ET0 devolve a evapotranspiração de referência diária em mm/dia (FAO-56, eq. 6), com G = 0.
func ET0(w DailyWeather) (float64, error) {
	if err := w.validate(); err != nil {
		return 0, err
	}
	tmean := (w.TmaxC + w.TminC) / 2
	esMean := (SaturationVaporPressure(w.TmaxC) + SaturationVaporPressure(w.TminC)) / 2
	ea := (SaturationVaporPressure(w.TminC)*w.RHmaxPct/100 + SaturationVaporPressure(w.TmaxC)*w.RHminPct/100) / 2

	pressure := 101.3 * math.Pow((293-0.0065*w.ElevationM)/293, 5.26)
	gamma := 0.000665 * pressure
	u2 := WindAt2m(w.WindMS, w.WindHeightM)

	ra := extraterrestrialRadiation(w.LatitudeDeg, w.DayOfYear)
	rso := (0.75 + 2e-5*w.ElevationM) * ra
	rns := (1 - 0.23) * w.SolarMJm2Day
	const sigma = 4.903e-9
	// A razão Rs/Rso é limitada a 1 (céu limpo) como manda a FAO-56.
	ratio := 1.0
	if rso > 0 {
		ratio = math.Min(w.SolarMJm2Day/rso, 1)
	}
	rnl := sigma * (math.Pow(w.TmaxC+273.16, 4) + math.Pow(w.TminC+273.16, 4)) / 2 *
		(0.34 - 0.14*math.Sqrt(ea)) * (1.35*ratio - 0.35)
	rn := rns - rnl

	delta := slopeVaporPressure(tmean)
	num := 0.408*delta*rn + gamma*900/(tmean+273)*u2*(esMean-ea)
	den := delta + gamma*(1+0.34*u2)
	return math.Max(0, num/den), nil
}

// WattsToMJPerDay converte a média diária de radiação em W/m² (grandeza solar_radiation_w_m2)
// para MJ/m²/dia.
func WattsToMJPerDay(meanWm2 float64) float64 { return meanWm2 * 0.0864 }
