package application

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/JohnnyMatteus/garden-iot/services/bridge/internal/domain"
)

// Handler processa uma mensagem MQTT. Retornar nil significa "pode confirmar ao broker";
// retornar erro significa "não confirme, tente de novo".
type Handler struct {
	translator PayloadTranslator
	telemetry  TelemetryPublisher
	dlq        DLQPublisher
	dedup      Deduplicator
	metrics    Metrics
	log        *slog.Logger
	now        func() time.Time
}

func NewHandler(t PayloadTranslator, p TelemetryPublisher, d DLQPublisher, dd Deduplicator, m Metrics, log *slog.Logger) *Handler {
	return &Handler{translator: t, telemetry: p, dlq: d, dedup: dd, metrics: m, log: log, now: time.Now}
}

func (h *Handler) Handle(ctx context.Context, topic string, payload []byte) error {
	received := h.now()

	telemetry, err := h.decode(topic, payload, received)
	if err != nil {
		return h.reject(ctx, topic, payload, received, err)
	}

	first, err := h.dedup.Claim(ctx, telemetry.MessageID())
	if err != nil {
		h.metrics.Failed("dedup")
		return fmt.Errorf("deduplicação: %w", err)
	}
	if !first {
		h.metrics.Duplicate()
		h.log.Debug("duplicada descartada", "id", telemetry.MessageID())
		return nil
	}

	if err := h.telemetry.Publish(ctx, telemetry); err != nil {
		// libera a reserva para a reentrega do broker não ser tomada por duplicada
		if relErr := h.dedup.Release(context.WithoutCancel(ctx), telemetry.MessageID()); relErr != nil {
			h.log.Error("falha ao liberar reserva", "id", telemetry.MessageID(), "erro", relErr)
		}
		h.metrics.Failed("kafka")
		return fmt.Errorf("publicação no kafka: %w", err)
	}

	h.metrics.Forwarded(telemetry.Tenant)
	h.metrics.Latency(received.Sub(telemetry.ReadAt).Seconds())
	return nil
}

func (h *Handler) decode(topic string, payload []byte, received time.Time) (domain.Telemetry, error) {
	parsed, err := domain.ParseTopic(topic)
	if err != nil {
		return domain.Telemetry{}, err
	}
	t, err := h.translator.Translate(parsed, payload)
	if err != nil {
		if _, ok := domain.AsInvalid(err); ok {
			return domain.Telemetry{}, err
		}
		return domain.Telemetry{}, domain.NewInvalid(domain.ReasonSchema, err)
	}
	// o tópico é autenticado pela ACL do broker; o payload nunca pode sobrescrever a identidade
	if t.Tenant != parsed.Tenant || t.Horta != parsed.Horta || t.NoID != parsed.NoID {
		return domain.Telemetry{}, domain.NewInvalid(domain.ReasonIdentity,
			fmt.Errorf("payload de %s/%s/%s em tópico de %s/%s/%s", t.Tenant, t.Horta, t.NoID, parsed.Tenant, parsed.Horta, parsed.NoID))
	}
	t.ReceivedAt = received
	return t, nil
}

func (h *Handler) reject(ctx context.Context, topic string, payload []byte, received time.Time, cause error) error {
	invalid, _ := domain.AsInvalid(cause)
	h.metrics.Invalid(invalid.Reason)
	h.log.Warn("mensagem inválida", "topico", topic, "motivo", string(invalid.Reason), "erro", invalid.Err)

	err := h.dlq.PublishRejected(ctx, domain.Rejected{
		Topic: topic, Reason: invalid.Reason, Detail: invalid.Err.Error(), Payload: payload, ReceivedAt: received,
	})
	if err != nil {
		h.metrics.Failed("dlq")
		return fmt.Errorf("publicação na dlq: %w", err)
	}
	return nil
}
