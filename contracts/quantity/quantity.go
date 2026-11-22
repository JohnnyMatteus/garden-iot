// Package quantity é a fonte única dos nomes das grandezas normalizadas (coluna `kind` da
// telemetria). Persistência, API, painéis e o serviço de irrigação só usam estes nomes.
package quantity

const (
	SoilMoisture   = "soil_moisture_vwc_pct"
	AirTemp        = "air_temp_c"
	AirHumidity    = "air_humidity_pct"
	Illuminance    = "illuminance_lux"
	SolarRadiation = "solar_radiation_w_m2"
	WaterLevelOK   = "water_level_ok"
)

// All lista todas as grandezas; o teste de contrato cobra que README, painéis e exemplos a respeitem.
var All = []string{SoilMoisture, AirTemp, AirHumidity, Illuminance, SolarRadiation, WaterLevelOK}
