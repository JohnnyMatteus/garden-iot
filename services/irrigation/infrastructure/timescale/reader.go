// Package timescale lê a telemetria normalizada (tabela telemetry.reading) para o planejador.
// Usa um papel somente leitura (telemetry_ops no ambiente local).
package timescale

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/JohnnyMatteus/garden-iot/contracts/quantity"
	"github.com/JohnnyMatteus/garden-iot/services/irrigation/application"
	"github.com/JohnnyMatteus/garden-iot/services/irrigation/domain/agronomy"
	"github.com/JohnnyMatteus/garden-iot/services/irrigation/domain/safety"
)

const (
	// soilFreshness é a idade máxima da leitura de solo para o sensor valer como "saudável".
	soilFreshness = 30 * time.Minute
	// stuckWindow e stuckMinSamples definem "sensor travado": variância zero por horas (design doc, seção 5).
	stuckWindow     = 3 * time.Hour
	stuckMinSamples = 10
	// minDaySamples evita fechar o dia com um nó que ficou quase todo offline.
	minDaySamples = 24
)

type Reader struct{ pool *pgxpool.Pool }

var (
	_ application.SensorSource  = (*Reader)(nil)
	_ application.ClimateSource = (*Reader)(nil)
)

func NewReader(ctx context.Context, dsn string) (*Reader, error) {
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		return nil, err
	}
	return &Reader{pool: pool}, nil
}

func (r *Reader) Close() { r.pool.Close() }

type latest struct {
	value   float64
	readAt  time.Time
	quality string
}

func (r *Reader) latest(ctx context.Context, bed application.Bed, kind string, since time.Time) (*latest, error) {
	var l latest
	err := r.pool.QueryRow(ctx, `SELECT value, read_at, quality FROM telemetry.reading
		WHERE tenant_id=$1 AND garden_id=$2 AND node_id=$3 AND kind=$4 AND read_at > $5
		ORDER BY read_at DESC LIMIT 1`, bed.Ref.Tenant, bed.Ref.Garden, bed.Ref.Node, kind, since).
		Scan(&l.value, &l.readAt, &l.quality)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	return &l, err
}

func (r *Reader) Snapshot(ctx context.Context, bed application.Bed, now time.Time) (application.Sensors, error) {
	var s application.Sensors
	if l, err := r.latest(ctx, bed, quantity.SoilMoisture, now.Add(-soilFreshness)); err != nil {
		return s, err
	} else if l != nil {
		s.SoilMoisturePct = &l.value
		s.SoilHealthy = l.quality == "ok"
		if s.SoilHealthy {
			stuck, err := r.stuck(ctx, bed, now)
			if err != nil {
				return s, err
			}
			s.SoilHealthy = !stuck
		}
	}
	if l, err := r.latest(ctx, bed, quantity.AirTemp, now.Add(-soilFreshness)); err != nil {
		return s, err
	} else if l != nil && l.quality == "ok" {
		s.AirTempC = &l.value
	}
	if l, err := r.latest(ctx, bed, quantity.WaterLevelOK, now.Add(-soilFreshness)); err != nil {
		return s, err
	} else if l != nil {
		s.Reservoir = safety.ReservoirOK
		if l.value < 0.5 {
			s.Reservoir = safety.ReservoirLow
		}
	}
	return s, nil
}

func (r *Reader) stuck(ctx context.Context, bed application.Bed, now time.Time) (bool, error) {
	var n int
	var sd *float64
	err := r.pool.QueryRow(ctx, `SELECT count(*), stddev_pop(value) FROM telemetry.reading
		WHERE tenant_id=$1 AND garden_id=$2 AND node_id=$3 AND kind=$4 AND read_at > $5`,
		bed.Ref.Tenant, bed.Ref.Garden, bed.Ref.Node, quantity.SoilMoisture, now.Add(-stuckWindow)).Scan(&n, &sd)
	return err == nil && n >= stuckMinSamples && sd != nil && *sd == 0, err
}

type stats struct {
	n        int
	min, max float64
	avg      float64
}

func (r *Reader) dayStats(ctx context.Context, bed application.Bed, kind string, from, to time.Time) (stats, error) {
	var s stats
	var mn, mx, av *float64
	err := r.pool.QueryRow(ctx, `SELECT count(*), min(value), max(value), avg(value) FROM telemetry.reading
		WHERE tenant_id=$1 AND garden_id=$2 AND node_id=$3 AND kind=$4 AND quality <> 'suspect'
		AND read_at >= $5 AND read_at < $6`, bed.Ref.Tenant, bed.Ref.Garden, bed.Ref.Node, kind, from, to).
		Scan(&s.n, &mn, &mx, &av)
	if err != nil || s.n == 0 {
		return s, err
	}
	s.min, s.max, s.avg = *mn, *mx, *av
	return s, nil
}

// Day monta o clima do dia UTC "day" (truncado) com temperatura, umidade e radiação do nó.
// Vento fica 0: o chamador aplica o valor padrão (nenhuma grandeza de vento no firmware v1).
func (r *Reader) Day(ctx context.Context, bed application.Bed, day time.Time) (agronomy.DailyWeather, error) {
	from := day.UTC().Truncate(24 * time.Hour)
	to := from.Add(24 * time.Hour)
	temp, err := r.dayStats(ctx, bed, quantity.AirTemp, from, to)
	if err != nil {
		return agronomy.DailyWeather{}, err
	}
	rh, err := r.dayStats(ctx, bed, quantity.AirHumidity, from, to)
	if err != nil {
		return agronomy.DailyWeather{}, err
	}
	sun, err := r.dayStats(ctx, bed, quantity.SolarRadiation, from, to)
	if err != nil {
		return agronomy.DailyWeather{}, err
	}
	for name, s := range map[string]stats{"temperatura": temp, "umidade": rh, "radiação": sun} {
		if s.n < minDaySamples {
			return agronomy.DailyWeather{}, fmt.Errorf("%s: %d leituras em %s, mínimo %d", name, s.n, from.Format("2006-01-02"), minDaySamples)
		}
	}
	return agronomy.DailyWeather{
		TminC: temp.min, TmaxC: temp.max, RHminPct: rh.min, RHmaxPct: rh.max,
		SolarMJm2Day: agronomy.WattsToMJPerDay(sun.avg),
		LatitudeDeg:  bed.LatDeg, ElevationM: bed.ElevationM, DayOfYear: from.YearDay(),
	}, nil
}
