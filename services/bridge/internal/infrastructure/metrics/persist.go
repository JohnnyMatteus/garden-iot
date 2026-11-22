package metrics

import (
	"net/http"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"github.com/JohnnyMatteus/garden-iot/services/bridge/internal/domain"
)

// Persist expõe as métricas do consumidor Kafka → TimescaleDB.
type Persist struct {
	registry *prometheus.Registry
	stored   prometheus.Counter
	rejected *prometheus.CounterVec
}

func NewPersist() *Persist {
	p := &Persist{
		registry: prometheus.NewRegistry(),
		stored:   prometheus.NewCounter(prometheus.CounterOpts{Name: "persist_readings_stored_total", Help: "Leituras enviadas ao TimescaleDB."}),
		rejected: prometheus.NewCounterVec(prometheus.CounterOpts{Name: "persist_batches_rejected_total", Help: "Lotes enviados à DLQ, por motivo."}, []string{"reason"}),
	}
	p.registry.MustRegister(p.stored, p.rejected)
	return p
}

func (p *Persist) Stored(n int)             { p.stored.Add(float64(n)) }
func (p *Persist) Rejected(r domain.Reason) { p.rejected.WithLabelValues(string(r)).Inc() }
func (p *Persist) Handler() http.Handler {
	return promhttp.HandlerFor(p.registry, promhttp.HandlerOpts{})
}
