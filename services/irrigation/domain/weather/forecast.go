// Package weather é o modelo de previsão do tempo no vocabulário do domínio de irrigação.
// Nenhum campo do provedor externo aparece aqui (ACL em infrastructure/openmeteo).
package weather

import "time"

// Forecast é o resumo da previsão para a janela [From, From+Horizon) em unidades SI.
type Forecast struct {
	From      time.Time
	Horizon   time.Duration
	RainMM    float64 // chuva acumulada na janela
	MinTempC  float64
	MaxWindMS float64
}
