package kafka

import (
	"context"
	"fmt"

	"github.com/twmb/franz-go/pkg/sr"
	"google.golang.org/protobuf/proto"

	"github.com/JohnnyMatteus/garden-iot/services/bridge/internal/domain"
)

// Mapper converte o lote de domínio na mensagem gerada a partir de contracts/proto.
type Mapper func(domain.Telemetry) proto.Message

// ProtoEncoder registra o schema no Schema Registry e serializa no formato de fio do Confluent.
type ProtoEncoder struct {
	serde  *sr.Serde
	mapper Mapper
}

// NewProtoEncoder registra (ou reaproveita) o schema do subject e devolve o encoder.
// sample é uma instância da mensagem principal do .proto, usada para ligar o tipo ao id do schema.
func NewProtoEncoder(ctx context.Context, reg *sr.Client, subject, schema string, sample proto.Message, mapper Mapper) (*ProtoEncoder, error) {
	ss, err := reg.CreateSchema(ctx, subject, sr.Schema{Schema: schema, Type: sr.TypeProtobuf})
	if err != nil {
		return nil, fmt.Errorf("registro do schema %q: %w", subject, err)
	}
	var serde sr.Serde
	serde.Register(ss.ID, sample,
		sr.EncodeFn(func(v any) ([]byte, error) { return proto.Marshal(v.(proto.Message)) }),
		// a mensagem principal é a primeira do arquivo: índice [0]
		sr.Index(0),
	)
	return &ProtoEncoder{serde: &serde, mapper: mapper}, nil
}

func (e *ProtoEncoder) Encode(t domain.Telemetry) ([]byte, error) {
	return e.serde.Encode(e.mapper(t))
}
