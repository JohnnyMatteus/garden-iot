// Package readings é o caso de uso da persistência: lote bruto → leituras em grandezas de domínio → banco.
package readings

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/JohnnyMatteus/garden-iot/services/bridge/internal/domain"
)

// Reading é uma linha de telemetry.reading.
type Reading struct {
	Tenant   string
	Garden   string
	Node     string
	Sensor   string
	Kind     string
	ReadAt   time.Time
	Sequence uint64
	Value    float64
	Unit     string
	Quality  string
}

// Batch é um lote bruto vindo do tópico telemetria.bruta.v1.
type Batch struct {
	Tenant, Garden, Node string
	Sequence             uint64
	FwVersion            string
	ReadAt               time.Time
	Payload              []byte
}

// Normalizer aplica a ACL (DevicePayloadTranslator) e a calibração. Erros devem ser *domain.InvalidError.
type Normalizer interface {
	Normalize(b Batch) ([]Reading, error)
}

// Store grava as leituras de forma idempotente (reprocessar não duplica).
type Store interface {
	Save(ctx context.Context, rows []Reading) error
}

// DLQ recebe os lotes que nunca serão convertidos.
type DLQ interface {
	PublishRejected(ctx context.Context, r domain.Rejected) error
}

type Metrics interface {
	Stored(n int)
	Rejected(reason domain.Reason)
}

type Ingestor struct {
	normalizer Normalizer
	store      Store
	dlq        DLQ
	metrics    Metrics
	log        *slog.Logger
}

func NewIngestor(n Normalizer, s Store, d DLQ, m Metrics, log *slog.Logger) *Ingestor {
	return &Ingestor{normalizer: n, store: s, dlq: d, metrics: m, log: log}
}

// Ingest converte e grava um conjunto de lotes numa única escrita. Retornar erro significa que nada
// deve ser confirmado no Kafka; lotes inválidos vão para a DLQ e não impedem os demais.
func (i *Ingestor) Ingest(ctx context.Context, batches []Batch) error {
	var rows []Reading
	for _, b := range batches {
		out, err := i.normalizer.Normalize(b)
		if err != nil {
			if err := i.reject(ctx, b, err); err != nil {
				return err
			}
			continue
		}
		rows = append(rows, out...)
	}
	if len(rows) == 0 {
		return nil
	}
	if err := i.store.Save(ctx, rows); err != nil {
		return fmt.Errorf("gravação: %w", err)
	}
	i.metrics.Stored(len(rows))
	return nil
}

func (i *Ingestor) reject(ctx context.Context, b Batch, cause error) error {
	invalid, ok := domain.AsInvalid(cause)
	if !ok {
		invalid = domain.NewInvalid(domain.ReasonSchema, cause)
	}
	i.metrics.Rejected(invalid.Reason)
	i.log.Warn("lote rejeitado", "no", b.Node, "seq", b.Sequence, "motivo", string(invalid.Reason), "erro", invalid.Err)
	return i.dlq.PublishRejected(ctx, domain.Rejected{
		Topic:  fmt.Sprintf("horta/%s/%s/%s/telemetria", b.Tenant, b.Garden, b.Node),
		Reason: invalid.Reason, Detail: invalid.Err.Error(), Payload: b.Payload, ReceivedAt: time.Now(),
	})
}
