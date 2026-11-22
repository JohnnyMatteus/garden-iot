// Package kafka publica a telemetria (Protobuf + Schema Registry) e a DLQ.
package kafka

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/twmb/franz-go/pkg/kgo"

	"github.com/JohnnyMatteus/garden-iot/services/bridge/internal/domain"
)

// Encoder serializa o lote no formato do Schema Registry (magic byte + id + Protobuf).
type Encoder interface {
	Encode(t domain.Telemetry) ([]byte, error)
}

// Producer entrega as mensagens ao Kafka de forma síncrona e idempotente.
type Producer struct {
	client   *kgo.Client
	encoder  Encoder
	topic    string
	dlqTopic string
}

// NewClient cria o cliente franz-go; produtor idempotente com acks=all é o padrão da biblioteca.
func NewClient(brokers []string, opts ...kgo.Opt) (*kgo.Client, error) {
	base := []kgo.Opt{kgo.SeedBrokers(brokers...), kgo.RecordPartitioner(kgo.StickyKeyPartitioner(nil))}
	return kgo.NewClient(append(base, opts...)...)
}

func NewProducer(client *kgo.Client, enc Encoder, topic, dlqTopic string) *Producer {
	return &Producer{client: client, encoder: enc, topic: topic, dlqTopic: dlqTopic}
}

// Publish usa o no_id como chave para manter a ordem por nó na partição.
func (p *Producer) Publish(ctx context.Context, t domain.Telemetry) error {
	value, err := p.encoder.Encode(t)
	if err != nil {
		return fmt.Errorf("codificação: %w", err)
	}
	rec := &kgo.Record{
		Topic: p.topic, Key: []byte(t.NoID), Value: value,
		Headers: []kgo.RecordHeader{
			{Key: "tenant", Value: []byte(t.Tenant)},
			{Key: "sequencia", Value: []byte(strconv.FormatUint(t.Sequence, 10))},
		},
	}
	return p.client.ProduceSync(ctx, rec).FirstErr()
}

type dlqValue struct {
	Topic      string `json:"topico_mqtt"`
	Reason     string `json:"motivo"`
	Detail     string `json:"detalhe"`
	ReceivedAt string `json:"recebido_em"`
	Payload    []byte `json:"payload"`
}

// PublishRejected grava a mensagem original com o motivo, sem schema (JSON) porque o conteúdo é inválido por definição.
func (p *Producer) PublishRejected(ctx context.Context, r domain.Rejected) error {
	value, err := json.Marshal(dlqValue{
		Topic: r.Topic, Reason: string(r.Reason), Detail: r.Detail,
		ReceivedAt: r.ReceivedAt.UTC().Format("2006-01-02T15:04:05.000Z"), Payload: r.Payload,
	})
	if err != nil {
		return err
	}
	rec := &kgo.Record{
		Topic: p.dlqTopic, Value: value,
		Headers: []kgo.RecordHeader{{Key: "motivo", Value: []byte(r.Reason)}},
	}
	return p.client.ProduceSync(ctx, rec).FirstErr()
}
