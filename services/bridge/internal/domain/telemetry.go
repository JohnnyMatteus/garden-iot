// Package domain contém os tipos da bridge sem dependência de infraestrutura.
package domain

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"
)

// Topic identifica o nó a partir do tópico MQTT horta/{tenant}/{horta}/{no}/telemetria.
type Topic struct {
	Tenant string
	Horta  string
	NoID   string
}

// ParseTopic extrai tenant, horta e nó; qualquer outro formato é inválido.
func ParseTopic(topic string) (Topic, error) {
	parts := strings.Split(topic, "/")
	if len(parts) != 5 || parts[0] != "horta" || parts[4] != "telemetria" {
		return Topic{}, NewInvalid(ReasonTopic, fmt.Errorf("tópico inesperado %q", topic))
	}
	for _, p := range parts[1:4] {
		if p == "" || strings.ContainsAny(p, "+#") {
			return Topic{}, NewInvalid(ReasonTopic, fmt.Errorf("tópico inesperado %q", topic))
		}
	}
	return Topic{Tenant: parts[1], Horta: parts[2], NoID: parts[3]}, nil
}

// Telemetry é o lote já validado, pronto para o tópico telemetria.bruta.v1.
type Telemetry struct {
	Tenant     string
	Horta      string
	NoID       string
	Sequence   uint64
	FwVersion  string
	ReadAt     time.Time
	ReceivedAt time.Time
	Payload    []byte
}

// MessageID é a chave de idempotência (no_id, sequencia).
func (t Telemetry) MessageID() string {
	return t.Tenant + "/" + t.NoID + "/" + strconv.FormatUint(t.Sequence, 10)
}

// Reason classifica por que uma mensagem foi rejeitada; vira label de métrica e campo da DLQ.
type Reason string

const (
	ReasonTopic    Reason = "topico_invalido"
	ReasonDecode   Reason = "payload_invalido"
	ReasonSchema   Reason = "schema_invalido"
	ReasonFirmware Reason = "firmware_desconhecido"
	ReasonIdentity Reason = "identidade_divergente"
)

// InvalidError marca uma mensagem que nunca será aceita, por mais que se tente de novo.
type InvalidError struct {
	Reason Reason
	Err    error
}

func (e *InvalidError) Error() string {
	return fmt.Sprintf("mensagem inválida (%s): %v", e.Reason, e.Err)
}
func (e *InvalidError) Unwrap() error { return e.Err }

func NewInvalid(reason Reason, err error) *InvalidError {
	return &InvalidError{Reason: reason, Err: err}
}

// AsInvalid devolve o erro de validação, se houver.
func AsInvalid(err error) (*InvalidError, bool) {
	var ie *InvalidError
	return ie, errors.As(err, &ie)
}

// Rejected é o registro enviado à DLQ.
type Rejected struct {
	Topic      string
	Reason     Reason
	Detail     string
	Payload    []byte
	ReceivedAt time.Time
}
