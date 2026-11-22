// Package application orquestra o caso de uso da bridge sobre portas.
package application

import (
	"context"

	"github.com/JohnnyMatteus/garden-iot/services/bridge/internal/domain"
)

// PayloadTranslator valida e traduz o payload do firmware (ACL, ADR 0007).
// Erros de conteúdo devem ser *domain.InvalidError.
type PayloadTranslator interface {
	Translate(topic domain.Topic, payload []byte) (domain.Telemetry, error)
}

// TelemetryPublisher grava no telemetria.bruta.v1 e só retorna após o Kafka confirmar.
type TelemetryPublisher interface {
	Publish(ctx context.Context, t domain.Telemetry) error
}

// DLQPublisher grava mensagens rejeitadas com o motivo.
type DLQPublisher interface {
	PublishRejected(ctx context.Context, r domain.Rejected) error
}

// Deduplicator reserva um id de mensagem; Claim devolve false se já foi visto.
type Deduplicator interface {
	Claim(ctx context.Context, id string) (bool, error)
	Release(ctx context.Context, id string) error
}

// Metrics registra o que a bridge faz com cada mensagem.
type Metrics interface {
	Forwarded(tenant string)
	Duplicate()
	Invalid(reason domain.Reason)
	Failed(stage string)
	Latency(seconds float64)
}
