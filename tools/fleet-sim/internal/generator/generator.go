// Package generator produz os lotes de telemetria de um nó, de forma determinística por semente.
package generator

import (
	"encoding/json"
	"math"
	"math/rand/v2"
	"time"

	"github.com/JohnnyMatteus/garden-iot/tools/fleet-sim/internal/profile"
)

// FirmwareVersion é a versão cujo payload o simulador reproduz.
const FirmwareVersion = "1.0.0"

// Batch é um lote pronto para publicar.
type Batch struct {
	Seq     uint32
	Payload []byte
}

type payload struct {
	FwVersion    string   `json:"fw_version"`
	Seq          uint32   `json:"seq"`
	Ts           int64    `json:"ts"`
	Dropped      uint32   `json:"dropped"`
	SoilADC      int      `json:"soil_adc"`
	AirTempC     float64  `json:"air_temp_c"`
	AirRHPct     float64  `json:"air_rh_pct"`
	Lux          float64  `json:"lux"`
	WaterLevelOK bool     `json:"water_level_ok"`
	WindMS       *float64 `json:"wind_ms,omitempty"` // fora do firmware v1; consumidores ignoram campos desconhecidos
	RainMM       *float64 `json:"rain_mm,omitempty"`
}

// Node gera os lotes de um nó; cada nó tem o próprio gerador de números aleatórios.
type Node struct {
	p        profile.Profile
	rng      *rand.Rand
	start    time.Time
	interval time.Duration
	rain     float64
}

func NewNode(p profile.Profile, seed uint64, index int, start time.Time, interval time.Duration) *Node {
	return &Node{p: p, rng: rand.New(rand.NewPCG(seed, uint64(index)+1)), start: start, interval: interval}
}

// Batch devolve o lote do tick (0, 1, 2...). Deve ser chamado em ordem crescente de tick.
func (n *Node) Batch(tick int, dropped uint32) Batch {
	at := n.start.Add(time.Duration(tick) * n.interval)
	hours := at.Sub(n.start).Hours()
	day := math.Sin(2 * math.Pi * (float64(at.Hour()) + float64(at.Minute())/60 - 9) / 24) // pico às 15h

	if n.rng.Float64() < n.p.RainMM.Chance {
		n.rain = n.p.RainMM.Amount * (0.5 + n.rng.Float64())
	} else {
		n.rain = 0
	}

	soil := n.p.SoilADC.Base + n.p.SoilADC.DriftPerHour*hours
	if n.p.StuckSoil {
		soil = n.p.SoilADC.Base
	} else {
		soil += n.noise(n.p.SoilADC.Noise) - n.rain*400
	}
	if n.p.SoilADC.Floor > 0 && soil < n.p.SoilADC.Floor {
		soil = n.p.SoilADC.Floor
	}

	temp := n.p.AirTempC.Base + n.p.AirTempC.DailyAmplitude*day + n.noise(n.p.AirTempC.Noise)
	rh := clamp(n.p.AirRHPct.Base+n.p.AirRHPct.DailyAmplitude*day+n.noise(n.p.AirRHPct.Noise)+n.rain*8, 0, 100)
	lux := math.Max(0, n.p.Lux.Peak*math.Max(0, day)+n.noise(n.p.Lux.Noise))
	wind := math.Max(0, n.p.WindMS.Base+n.noise(n.p.WindMS.Noise))
	rain := n.rain

	seq := uint32(tick + 1)
	body, _ := json.Marshal(payload{
		FwVersion: FirmwareVersion, Seq: seq, Ts: at.Unix(), Dropped: dropped,
		SoilADC: int(math.Round(soil)), AirTempC: round2(temp), AirRHPct: round2(rh), Lux: round2(lux),
		WaterLevelOK: true, WindMS: ptr(round2(wind)), RainMM: ptr(round2(rain)),
	})
	return Batch{Seq: seq, Payload: body}
}

func (n *Node) noise(amp float64) float64 { return (n.rng.Float64()*2 - 1) * amp }

func clamp(v, lo, hi float64) float64 { return math.Min(math.Max(v, lo), hi) }
func round2(v float64) float64        { return math.Round(v*100) / 100 }
func ptr(v float64) *float64          { return &v }
