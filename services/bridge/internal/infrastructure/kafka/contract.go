package kafka

import (
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"

	brutav1 "github.com/JohnnyMatteus/garden-iot/contracts/gen/go/telemetria/bruta/v1"
	"github.com/JohnnyMatteus/garden-iot/services/bridge/internal/domain"
)

// ToBruta mapeia o lote de domínio para a mensagem do contrato telemetria.bruta.v1.
func ToBruta(t domain.Telemetry) proto.Message {
	return &brutav1.TelemetriaBruta{
		TenantId:   t.Tenant,
		GardenId:   t.Horta,
		NodeId:     t.NoID,
		Sequence:   t.Sequence,
		FwVersion:  t.FwVersion,
		ReadAt:     timestamppb.New(t.ReadAt),
		ReceivedAt: timestamppb.New(t.ReceivedAt),
		Payload:    t.Payload,
	}
}

// BrutaSample liga o tipo gerado ao id do schema no encoder.
func BrutaSample() proto.Message { return &brutav1.TelemetriaBruta{} }
