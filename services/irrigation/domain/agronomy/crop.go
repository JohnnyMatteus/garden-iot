package agronomy

import (
	"fmt"
	"math"
)

// Stage é o estágio fenológico usado para escolher o Kc.
type Stage string

const (
	StageInitial     Stage = "initial"
	StageDevelopment Stage = "development"
	StageMid         Stage = "mid"
	StageLate        Stage = "late"
)

// Crop reúne os parâmetros da FAO-56 (tabelas 12 e 22) por cultura. Valores são editáveis por canteiro.
type Crop struct {
	Name       string
	KcIni      float64
	KcMid      float64
	KcEnd      float64
	DepletionP float64 // fração p de água facilmente disponível
	RootDepthM float64
}

// Crops é o catálogo padrão.
var Crops = map[string]Crop{
	"tomato":  {Name: "tomato", KcIni: 0.60, KcMid: 1.15, KcEnd: 0.80, DepletionP: 0.40, RootDepthM: 0.70},
	"lettuce": {Name: "lettuce", KcIni: 0.70, KcMid: 1.00, KcEnd: 0.95, DepletionP: 0.30, RootDepthM: 0.30},
	"carrot":  {Name: "carrot", KcIni: 0.70, KcMid: 1.05, KcEnd: 0.95, DepletionP: 0.35, RootDepthM: 0.50},
	"pepper":  {Name: "pepper", KcIni: 0.60, KcMid: 1.05, KcEnd: 0.90, DepletionP: 0.30, RootDepthM: 0.50},
}

// Kc devolve o coeficiente de cultura. progress (0..1) é o avanço dentro dos estágios de transição:
// o desenvolvimento interpola de KcIni a KcMid e a fase final, de KcMid a KcEnd.
func (c Crop) Kc(stage Stage, progress float64) (float64, error) {
	p := math.Min(1, math.Max(0, progress))
	switch stage {
	case StageInitial:
		return c.KcIni, nil
	case StageDevelopment:
		return c.KcIni + (c.KcMid-c.KcIni)*p, nil
	case StageMid:
		return c.KcMid, nil
	case StageLate:
		return c.KcMid + (c.KcEnd-c.KcMid)*p, nil
	}
	return 0, fmt.Errorf("estágio desconhecido %q", stage)
}

// Soil traz as umidades volumétricas (m³/m³) na capacidade de campo e no ponto de murcha.
type Soil struct {
	FieldCapacity float64
	WiltingPoint  float64
}

// LoamSoil é o padrão quando o canteiro não informa o solo (FAO-56, tabela 19, franco).
var LoamSoil = Soil{FieldCapacity: 0.27, WiltingPoint: 0.13}

// TAW é a água total disponível na zona radicular em mm (eq. 82).
func (c Crop) TAW(s Soil) float64 { return 1000 * (s.FieldCapacity - s.WiltingPoint) * c.RootDepthM }

// RAW é a água facilmente disponível em mm (eq. 83).
func (c Crop) RAW(s Soil) float64 { return c.DepletionP * c.TAW(s) }
