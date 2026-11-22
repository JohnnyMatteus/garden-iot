package agronomy

import "math"

// InterceptionMM é a chuva do dia que fica na folhagem e na superfície e nunca chega ao solo.
const InterceptionMM = 2.0

// EffectiveRain devolve a chuva que chega à zona radicular: o que passa da interceptação.
func EffectiveRain(rainMM float64) float64 { return math.Max(0, rainMM-InterceptionMM) }

// Balance é o balanço hídrico de um canteiro: DepletionMM é o déficit acumulado na zona radicular
// (0 = capacidade de campo, TAW = ponto de murcha).
type Balance struct {
	DepletionMM float64
	TAWMM       float64
	RAWMM       float64
}

// NewBalance começa o canteiro na capacidade de campo.
func NewBalance(c Crop, s Soil) Balance { return Balance{TAWMM: c.TAW(s), RAWMM: c.RAW(s)} }

// Day é o que aconteceu num dia: ET₀, Kc do estágio, chuva medida e rega aplicada (lâmina em mm).
type Day struct {
	ET0MM        float64
	Kc           float64
	RainMM       float64
	IrrigationMM float64
}

// ETc é a necessidade da cultura no dia: Kc × ET₀.
func (d Day) ETc() float64 { return d.Kc * d.ET0MM }

// Apply avança um dia: déficit = anterior + ETc − chuva efetiva − irrigação, limitado a [0, TAW].
// Excesso de água abaixo de zero é percolação profunda e não vira crédito para dias seguintes.
func (b Balance) Apply(d Day) Balance {
	dep := b.DepletionMM + d.ETc() - EffectiveRain(d.RainMM) - d.IrrigationMM
	b.DepletionMM = math.Min(b.TAWMM, math.Max(0, dep))
	return b
}

// NeedsWater diz se o déficit passou da água facilmente disponível.
func (b Balance) NeedsWater() bool { return b.DepletionMM >= b.RAWMM }

// IrrigationDepthMM é a lâmina que devolve o canteiro à capacidade de campo.
func (b Balance) IrrigationDepthMM() float64 { return b.DepletionMM }

// VolumeLiters converte lâmina (mm) em litros para a área (m²): 1 mm sobre 1 m² = 1 L.
func VolumeLiters(depthMM, areaM2 float64) float64 { return depthMM * areaM2 }
