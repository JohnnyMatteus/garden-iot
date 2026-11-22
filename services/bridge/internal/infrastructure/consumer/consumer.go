// Package consumer lê telemetria.bruta.v1 em lotes e entrega ao caso de uso, confirmando o offset
// só depois da gravação (at-least-once; a idempotência fica na chave natural da tabela).
package consumer

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/twmb/franz-go/pkg/kgo"
	"github.com/twmb/franz-go/pkg/sr"
	"google.golang.org/protobuf/proto"

	brutav1 "github.com/JohnnyMatteus/garden-iot/contracts/gen/go/telemetria/bruta/v1"
	"github.com/JohnnyMatteus/garden-iot/services/bridge/internal/domain"
	"github.com/JohnnyMatteus/garden-iot/services/bridge/internal/readings"
)

type Ingester interface {
	Ingest(ctx context.Context, batches []readings.Batch) error
}

type DLQ interface {
	PublishRejected(ctx context.Context, r domain.Rejected) error
}

type Consumer struct {
	client    *kgo.Client
	ingester  Ingester
	dlq       DLQ
	batchSize int
	log       *slog.Logger
}

// NewClient cria o cliente do grupo, com commit manual.
func NewClient(brokers []string, group, topic string) (*kgo.Client, error) {
	return kgo.NewClient(
		kgo.SeedBrokers(brokers...), kgo.ConsumerGroup(group), kgo.ConsumeTopics(topic),
		kgo.DisableAutoCommit(), kgo.ConsumeResetOffset(kgo.NewOffset().AtStart()),
	)
}

func New(client *kgo.Client, ing Ingester, dlq DLQ, batchSize int, log *slog.Logger) *Consumer {
	return &Consumer{client: client, ingester: ing, dlq: dlq, batchSize: batchSize, log: log}
}

// Run bloqueia até o contexto ser cancelado. Falha de gravação não confirma o offset e repete o lote.
func (c *Consumer) Run(ctx context.Context) error {
	for {
		fetches := c.client.PollRecords(ctx, c.batchSize)
		if ctx.Err() != nil {
			return nil
		}
		if errs := fetches.Errors(); len(errs) > 0 {
			c.log.Error("erro de fetch", "erros", fmt.Sprint(errs))
			sleep(ctx, time.Second)
			continue
		}
		records := fetches.Records()
		if len(records) == 0 {
			continue
		}
		if err := c.processWithRetry(ctx, records); err != nil {
			return nil // contexto cancelado durante o retry: nada confirmado
		}
		if err := c.client.CommitRecords(ctx, records...); err != nil && !errors.Is(err, context.Canceled) {
			c.log.Error("falha no commit", "erro", err)
		}
	}
}

// processWithRetry repete o mesmo conjunto até gravar; sem confirmação, o Kafka reentrega após reinício.
func (c *Consumer) processWithRetry(ctx context.Context, records []*kgo.Record) error {
	for {
		err := c.process(ctx, records)
		if err == nil {
			return nil
		}
		if ctx.Err() != nil {
			return ctx.Err()
		}
		c.log.Error("falha ao processar lote, nova tentativa", "erro", err)
		sleep(ctx, time.Second)
	}
}

func (c *Consumer) process(ctx context.Context, records []*kgo.Record) error {
	batches := make([]readings.Batch, 0, len(records))
	for _, rec := range records {
		b, err := Decode(rec.Value)
		if err != nil {
			if err := c.dlq.PublishRejected(ctx, domain.Rejected{
				Topic: rec.Topic, Reason: domain.ReasonDecode, Detail: err.Error(), Payload: rec.Value, ReceivedAt: time.Now(),
			}); err != nil {
				return err
			}
			continue
		}
		batches = append(batches, b)
	}
	return c.ingester.Ingest(ctx, batches)
}

// Decode remove o cabeçalho do Schema Registry (magic byte, id, índice) e lê a mensagem Protobuf.
func Decode(value []byte) (readings.Batch, error) {
	var h sr.ConfluentHeader
	_, rest, err := h.DecodeID(value)
	if err != nil {
		return readings.Batch{}, fmt.Errorf("cabeçalho do registry: %w", err)
	}
	_, rest, err = h.DecodeIndex(rest, 8)
	if err != nil {
		return readings.Batch{}, fmt.Errorf("índice da mensagem: %w", err)
	}
	var m brutav1.TelemetriaBruta
	if err := proto.Unmarshal(rest, &m); err != nil {
		return readings.Batch{}, fmt.Errorf("protobuf: %w", err)
	}
	return readings.Batch{
		Tenant: m.TenantId, Garden: m.GardenId, Node: m.NodeId, Sequence: m.Sequence,
		FwVersion: m.FwVersion, ReadAt: m.ReadAt.AsTime(), Payload: m.Payload,
	}, nil
}

func sleep(ctx context.Context, d time.Duration) {
	select {
	case <-ctx.Done():
	case <-time.After(d):
	}
}
