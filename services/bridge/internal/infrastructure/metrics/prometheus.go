// Package metrics expõe as métricas da bridge no formato Prometheus.
package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"net/http"

	"github.com/JohnnyMatteus/garden-iot/services/bridge/internal/domain"
)

type Prometheus struct {
	registry  *prometheus.Registry
	forwarded *prometheus.CounterVec
	duplicate prometheus.Counter
	invalid   *prometheus.CounterVec
	failed    *prometheus.CounterVec
	latency   prometheus.Histogram
}

func NewPrometheus() *Prometheus {
	p := &Prometheus{
		registry: prometheus.NewRegistry(),
		forwarded: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "bridge_messages_forwarded_total", Help: "Lotes publicados em telemetria.bruta.v1."}, []string{"tenant"}),
		duplicate: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "bridge_messages_duplicate_total", Help: "Lotes descartados por já terem sido vistos."}),
		invalid: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "bridge_messages_invalid_total", Help: "Mensagens rejeitadas e enviadas à DLQ, por motivo."}, []string{"reason"}),
		failed: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "bridge_processing_failures_total", Help: "Falhas que impedem o ack MQTT, por etapa."}, []string{"stage"}),
		latency: prometheus.NewHistogram(prometheus.HistogramOpts{
			Name: "bridge_ingest_delay_seconds", Help: "Atraso entre a leitura no nó e a chegada à bridge.",
			Buckets: []float64{0.1, 0.5, 1, 2, 5, 10, 30, 60, 300, 3600}}),
	}
	p.registry.MustRegister(p.forwarded, p.duplicate, p.invalid, p.failed, p.latency)
	return p
}

func (p *Prometheus) Forwarded(tenant string) { p.forwarded.WithLabelValues(tenant).Inc() }
func (p *Prometheus) Duplicate()              { p.duplicate.Inc() }
func (p *Prometheus) Invalid(r domain.Reason) { p.invalid.WithLabelValues(string(r)).Inc() }
func (p *Prometheus) Failed(stage string)     { p.failed.WithLabelValues(stage).Inc() }
func (p *Prometheus) Latency(seconds float64) { p.latency.Observe(seconds) }

// Registry permite a outros adaptadores registrarem métricas próprias.
func (p *Prometheus) Registry() *prometheus.Registry { return p.registry }

func (p *Prometheus) Handler() http.Handler {
	return promhttp.HandlerFor(p.registry, promhttp.HandlerOpts{})
}
