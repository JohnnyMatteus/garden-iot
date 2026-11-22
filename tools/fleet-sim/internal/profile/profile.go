// Package profile define os perfis de comportamento dos nós simulados.
package profile

import (
	"embed"
	"fmt"
	"io/fs"
	"sort"

	"gopkg.in/yaml.v3"
)

//go:embed profiles/*.yaml
var builtin embed.FS

// Curve descreve uma grandeza: valor base, variação diária, deriva por hora e ruído.
type Curve struct {
	Base           float64 `yaml:"base"`
	DailyAmplitude float64 `yaml:"daily_amplitude"`
	DriftPerHour   float64 `yaml:"drift_per_hour"`
	Noise          float64 `yaml:"noise"`
	Floor          float64 `yaml:"floor"`
}

type Light struct {
	Peak  float64 `yaml:"peak"`
	Noise float64 `yaml:"noise"`
}

type Rain struct {
	Chance float64 `yaml:"chance"`
	Amount float64 `yaml:"amount"`
}

// Drop faz o nó cair sem aviso depois de AfterTicks e voltar após DownTicks.
type Drop struct {
	AfterTicks int `yaml:"after_ticks"`
	DownTicks  int `yaml:"down_ticks"`
}

type Profile struct {
	Name        string `yaml:"name"`
	Description string `yaml:"description"`
	SoilADC     Curve  `yaml:"soil_adc"`
	StuckSoil   bool   `yaml:"stuck_soil"`
	AirTempC    Curve  `yaml:"air_temp_c"`
	AirRHPct    Curve  `yaml:"air_rh_pct"`
	Lux         Light  `yaml:"lux"`
	WindMS      Curve  `yaml:"wind_ms"`
	RainMM      Rain   `yaml:"rain_mm"`
	Drop        *Drop  `yaml:"drop"`
}

// Load devolve os perfis embutidos por nome.
func Load() (map[string]Profile, error) {
	files, err := fs.Glob(builtin, "profiles/*.yaml")
	if err != nil {
		return nil, err
	}
	out := map[string]Profile{}
	for _, f := range files {
		raw, err := builtin.ReadFile(f)
		if err != nil {
			return nil, err
		}
		var p Profile
		if err := yaml.Unmarshal(raw, &p); err != nil {
			return nil, fmt.Errorf("%s: %w", f, err)
		}
		out[p.Name] = p
	}
	return out, nil
}

// Names lista os perfis disponíveis em ordem estável.
func Names(m map[string]Profile) []string {
	names := make([]string, 0, len(m))
	for n := range m {
		names = append(names, n)
	}
	sort.Strings(names)
	return names
}
